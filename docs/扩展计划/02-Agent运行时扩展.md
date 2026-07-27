# 扩展计划：Agent 运行时

> 对应设计文档：`docs/agent-layer-design.md`
> 源文件"后续扩展方向（本次不实现）"章节

---

## 已实现

| 模块 | 文件 | 说明 |
|------|------|------|
| Agent 事件 | `agent/event.go` | 6 种 AgentEventType + Stream |
| Agent 核心 | `agent/agent.go` | Run 方法：LLM ↔ 工具自动循环 |
| 测试 | `agent/agent_test.go` | 5 个场景 mock 测试 |

---

## 扩展项

### P0 — Reset()：重置对话

**描述**：提供 `Agent.Reset()` 方法，清空对话历史，开始全新对话。

**为什么需要**：
- ChatUI 中用户可能需要"开始新对话"
- 当前 Agent 创建后对话历史一直累积，无法清理

**实现**：
```go
func (a *Agent) Reset() {
    a.messages = nil  // 清空历史，下次 Run 会重新设置 system prompt
}
```

**影响范围**：极小，`agent/agent.go` 新增一个公开方法

---

### P0 — [待考虑]Subscribe()：多事件监听器

**描述**：从单消费者 channel 改为支持多个订阅者（观察者模式）。

**为什么需要**：
- 当前 Stream 是单消费者（`<-chan Event`），谁调用 Run 谁消费
- 实际场景需要多个消费者同时监听：TUI 消费文本进行渲染、Logger 消费事件进行记录、外部 Hook 消费做审计
- Go channel 天然不支持多播，需要自己实现

**实现思路**：
```go
// agent/subscriber.go

type Subscriber struct {
    ID string
    Ch chan Event
}

func (a *Agent) Subscribe(id string) *Subscriber {
    sub := &Subscriber{ID: id, Ch: make(chan Event, 100)}
    a.mu.Lock()
    a.subs = append(a.subs, sub)
    a.mu.Unlock()
    return sub
}

func (a *Agent) Unsubscribe(id string) {
    a.mu.Lock()
    defer a.mu.Unlock()
    for i, sub := range a.subs {
        if sub.ID == id {
            close(sub.Ch)
            a.subs = append(a.subs[:i], a.subs[i+1:]...)
            return
        }
    }
}

// Run 中发送事件时改为广播
func (a *Agent) broadcast(event Event) {
    a.mu.RLock()
    defer a.mu.RUnlock()
    for _, sub := range a.subs {
        select {
        case sub.Ch <- event:
        default:  // 消费者慢，跳过（不阻塞）
        }
    }
}
```

**影响范围**：
- `agent/agent.go`：Run 内部的 `ch <- event` 改为 `a.broadcast(event)`
- 新增 `agent/subscriber.go`

---

### P1 — Continue()：上下文注入续写

**描述**：允许在 Agent 对话中注入额外上下文，再继续对话。

**为什么需要**：
- 用户中断 Agent 后想补充信息继续
- 外部系统需要"塞"一段上下文给 Agent（如"数据库查询结果如下..."）

**实现**：
```go
func (a *Agent) Continue(ctx context.Context, extraContext string) Stream {
    if extraContext != "" {
        a.messages = append(a.messages, ai.Message{
            Role:    ai.RoleUser,
            Content: []ai.ContentBlock{{Type: ai.ContentText, Text: extraContext}},
        })
    }
    return a.Run(ctx, "")  // 不追加新用户消息，直接让 LLM 基于历史回复
}
```

---

### P1 — [待考虑]ToolChoice：工具调用策略

**描述**：配置 LLM 的工具调用行为：强制调用 / 禁用调用 / 自动选择。

**为什么需要**：
- `forced`：某些场景必须用工具（如"必须查数据库"）
- `none`：纯文本对话，不允许工具调用（节省成本）
- `auto`：当前默认行为，LLM 自行判断

**实现**：
- 扩展 `agent.Config` 增加 `ToolChoice ToolChoiceMode` 字段
- 映射到各 Provider 的对应 API 参数（OpenAI 的 `tool_choice`、Anthropic 的 `tool_choice`）
- 需要在 `ai.Context` 也增加对应字段

---

### P1 — [待考虑]Parallel Tool Calls：并行工具执行

**描述**：当 LLM 一次返回多个独立的工具调用时，并行执行它们。

**为什么需要**：
- 减少工具调用轮数（如同时查北京和上海的天气）
- 当前实现：串行逐个执行，耗时累加
- 并行执行：所有工具同时启动，等待最慢的一个

**实现**：
```go
func (a *Agent) executeParallel(ctx context.Context, toolCalls []ai.ToolCall) []ToolResult {
    var wg sync.WaitGroup
    results := make([]ToolResult, len(toolCalls))

    for i, tc := range toolCalls {
        wg.Add(1)
        go func(idx int, call ai.ToolCall) {
            defer wg.Done()
            tool := a.toolMap[call.Name]
            result, err := tool.Execute(ctx, call.Arguments)
            results[idx] = ToolResult{Name: call.Name, Result: result, Error: err}
        }(i, tc)
    }

    wg.Wait()
    return results
}
```

**注意**：需要考虑工具间是否有依赖关系。简单方案：LLM 返回的同一组 tool_calls 是可并行的，不同组的顺序执行。

---

## 优先级总结

| 优先级 | 项目 | 工作量 |
|--------|------|--------|
| P0 | Reset() | 极小 |
| P0 | Subscribe() 多事件监听 | 中 |
| P1 | Continue() 上下文注入 | 小 |
| P1 | ToolChoice 策略 | 中 |
| P1 | Parallel Tool Calls | 中 |
