# 学习总结：Agent 运行时

## 开发时序：第 5 步

### 终于到"大脑"了

前面 4 步构建了基础设施：日志（输出）→ AI 接口（与 LLM 对话）→ Tool 接口（工具能力）。现在，需要把这些拼起来，构建一个 **能自主思考、能调用工具、能多轮对话** 的 Agent 运行时。

这是整个项目的 **核心** —— 前面的所有模块都是为了服务它。

---

## 概念解释

### Agent 运行时做什么？

简单说，就是实现一个 **无限循环**：

```
while 还没完成:
    1. 把对话历史 + 用户最新消息 发给 LLM
    2. LLM 回复了内容 + 工具调用指令吗？
       ├── 只有文本 → 完成！输出文本
       └── 有工具调用 → 执行工具 → 把结果追加到历史 → 回到第一步
```

这就是经典的 **ReAct（Reasoning + Acting）循环**。

### 一个完整的 Agent 对话示例

```
User: 北京和上海哪个更热？

Step 1: Agent 调用 LLM
  → LLM: "我需要查两个城市的天气" + ToolCall[get_weather("北京"), get_weather("上海")]

Step 2: Agent 执行工具
  → get_weather("北京") → "晴，35°C"
  → get_weather("上海") → "阴，32°C"
  → 注入结果到对话历史

Step 3: Agent 再次调用 LLM
  → LLM: "根据查询结果，北京 35°C 比上海 32°C 更热……"
  → 没有工具调用了，循环结束！

Agent 输出: "北京 35°C 比上海 32°C 更热……"
```

---

## 核心实现

### Config：配置驱动

```go
type Config struct {
    Provider     ai.Provider   // LLM 提供商（如 OpenAI）
    ModelID      string        // 模型 ID（如 "gpt-4o"）
    SystemPrompt string        // 系统提示词
    Tools        []tool.Tool   // 可用工具
    MaxSteps     int           // 最大工具调用轮数
    Temperature  float64       // 随机性
    MaxTokens    int           // 最大输出 token
}
```

**关键决策**：为什么用 Config struct 而不是构造函数参数？
→ 超过 3 个参数时，struct 比逐个传参更清晰。还能给字段加注释，设置默认值。

### Agent 结构体

```go
type Agent struct {
    config    Config
    messages  []ai.Message   // 对话历史（Agent 内部管理）
    toolMap   map[string]tool.Tool // 工具名 → 工具实例
}
```

**关键决策**：为什么对话历史在 Agent 内部而不是外部？
→ 多轮对话需要记忆。Agent 管理自己的历史，调用方不需要关心"这一轮发了哪些消息、上一轮 LLM 回复了什么"。

### Run 方法：核心循环

```go
func (a *Agent) Run(ctx context.Context, userMessage string) Stream {
    ch := make(chan Event, 10)

    go func() {
        defer close(ch)

        // 1. 如果没有历史，初始化系统提示词
        if len(a.messages) == 0 {
            a.messages = append(a.messages, ai.Message{
                Role:    ai.RoleSystem,
                Content: []ai.ContentBlock{{Type: ai.ContentText, Text: a.config.SystemPrompt}},
            })
        }

        // 2. 追加用户消息
        a.messages = append(a.messages, ai.Message{
            Role:    ai.RoleUser,
            Content: []ai.ContentBlock{{Type: ai.ContentText, Text: userMessage}},
        })

        // 3. 工具调用循环
        for step := 1; step <= a.config.MaxSteps; step++ {
            ch <- Event{Type: EventStepStart, Step: step}

            // 构建请求上下文
            context := ai.Context{
                Messages:    a.messages,
                Tools:       a.toolDefinitions(),
                Temperature: a.config.Temperature,
                MaxTokens:   a.config.MaxTokens,
            }

            // 调用 LLM
            stream := a.config.Provider.Chat(ctx, context)

            var fullText string
            var toolCalls []ai.ToolCall

            // 消费 LLM 流式输出
            for event := range stream {
                switch event.Type {
                case ai.EventTextDelta:
                    fullText += event.Delta
                    ch <- Event{Type: EventTextDelta, Delta: event.Delta}
                case ai.EventToolCallStart:
                    toolCalls = append(toolCalls, *event.ToolCall)
                    ch <- Event{Type: EventToolCall, ToolCall: event.ToolCall}
                case ai.EventToolCallEnd:
                    // 工具调用定义完成
                case ai.EventDone:
                    // 附带 Usage 统计
                    ch <- Event{Type: EventStepEnd, Step: step, Usage: event.Usage}
                case ai.EventError:
                    ch <- Event{Type: EventError, Error: event.Error}
                    return
                }
            }

            // 如果没有工具调用 → 完成
            if len(toolCalls) == 0 {
                a.messages = append(a.messages, ai.Message{
                    Role:    ai.RoleAssistant,
                    Content: []ai.ContentBlock{{Type: ai.ContentText, Text: fullText}},
                })
                ch <- Event{Type: EventDone}
                return
            }

            // 有工具调用 → 构建 assistant 消息（包含 tool_call）
            assistantContent := []ai.ContentBlock{}
            if fullText != "" {
                assistantContent = append(assistantContent,
                    ai.ContentBlock{Type: ai.ContentText, Text: fullText})
            }
            for _, tc := range toolCalls {
                tc := tc
                assistantContent = append(assistantContent,
                    ai.ContentBlock{Type: ai.ContentToolCall, ToolCall: &tc})
            }
            a.messages = append(a.messages, ai.Message{
                Role:    ai.RoleAssistant,
                Content: assistantContent,
            })

            // 执行每个工具
            for _, tc := range toolCalls {
                tool, ok := a.toolMap[tc.Name]
                if !ok {
                    // 未知工具 → 注入错误信息
                    a.messages = append(a.messages, ai.Message{
                        Role:    ai.RoleTool,
                        Content: []ai.ContentBlock{{Type: ai.ContentText, Text: "error: unknown tool " + tc.Name}},
                    })
                    continue
                }

                ch <- Event{Type: EventToolCall, ToolCall: &tc}

                result, err := tool.Execute(ctx, tc.Arguments)
                if err != nil {
                    result = "error: " + err.Error()
                }

                ch <- Event{Type: EventToolResult, ToolName: tc.Name, Result: result}

                // 工具结果注入对话历史
                a.messages = append(a.messages, ai.Message{
                    Role:    ai.RoleTool,
                    Content: []ai.ContentBlock{{Type: ai.ContentText, Text: result}},
                })
            }
        }

        // 超过 maxSteps → 强制结束
        ch <- Event{
            Type:  EventDone,
            Error: fmt.Errorf("max steps (%d) reached", a.config.MaxSteps),
        }
    }()

    return ch
}
```

### 循环逻辑的几种结果

| 情况 | 结果 |
|------|------|
| LLM 直接返回文本，无工具调用 | 循环立即结束，输出文本 |
| LLM 调用工具 → 注入结果 → LLM 再次回复 | 可能多轮 loop |
| 工具执行出错 | 错误信息作为 ToolResult 注入，LLM 会尝试修正或报错 |
| 循环次数 > MaxSteps | 强制结束，返回 error |

---

## Agent 事件 vs AI 事件

Agent 有自己的一套事件系统，比 AI 层的事件更高层：

| AI 事件 | Agent 事件 | 区别 |
|---------|-----------|------|
| `EventTextDelta` | `EventTextDelta` | Agent 透传 AI 层的文本增量 |
| `EventToolCallStart/Delta/End` | `EventToolCall` | Agent 合并为一个事件，加上 Step 编号 |
| `EventDone` | `EventDone` | Agent 的 Done 表示整个 Run 结束 |
| 无 | `EventStepStart/StepEnd` | Agent 新加的，标记每个工具调用轮次 |
| 无 | `EventToolResult` | Agent 新加的，携带工具执行结果 |

**设计思想**：AI 层事件描述"LLM 在做什么"，Agent 事件描述"Agent 在做什么"。两层各自独立，减少耦合。

---

## 测试策略：Mock 一切

Agent 的测试不依赖真实 API，而是创建 mock Provider + mock Tool：

```go
// mockProvider 返回预设的回复
type mockProvider struct {
    responses []ai.Event // 预设的事件序列
}

// mockTool 返回预设的结果
type mockTool struct {
    name   string
    result string
}

// 测试：纯文本对话
func TestAgent_TextOnly(t *testing.T) {
    mock := &mockProvider{
        responses: []ai.Event{
            {Type: ai.EventTextDelta, Delta: "你好"},
            {Type: ai.EventTextDelta, Delta: "！"},
            {Type: ai.EventDone},
        },
    }
    ag := agent.New(agent.Config{Provider: mock, MaxSteps: 10})
    stream := ag.Run(context.Background(), "hello")
    // 验证输出...
}
```

5 个测试场景全部通过：
- 纯文本对话
- 单次工具调用
- 多轮工具调用（循环验证）
- maxSteps 超限
- 工具执行失败恢复

---

## 思想总结

| 思想 | 体现 |
|------|------|
| **ReAct 循环** | LLM ↔ 工具交替，直到任务完成 |
| **对话历史管理** | Agent 内部维护 messages，支持多轮记忆 |
| **两层事件** | AI 层事件（LLM 视角）vs Agent 事件（运行时视角） |
| **优雅的错误处理** | 工具失败不中断循环，将错误作为 ToolResult 注入 |
| **防御性设计** | MaxSteps 防止无限循环；未知工具优雅降级 |
| **Mock 测试** | mock Provider + mock Tool，不依赖外部服务 |
