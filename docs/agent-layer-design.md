# Agent 运行时层设计文档

## 概述

Agent 运行时层（`agent/`）在 AI 层之上构建，提供工具调用循环、状态管理和事件驱动的 Agent 运行时。它是 pi-go 标准库的第二个核心模块。

Agent 的核心职责是：**管理 LLM ↔ 工具调用的自动循环，直到 LLM 给出最终回答**。

## 架构分层

```
┌──────────────────────────────────────────┐
│  cmd/pi 产品层                           │
│  使用 Agent.Run() 驱动交互式对话         │
├──────────────────────────────────────────┤
│  agent 层（本文档）                      │
│  Agent 运行时 + 工具调度 + 事件流        │
│  ├── agent.go      Agent 核心循环        │
│  ├── event.go      Agent 级事件类型      │
│  └── agent_test.go 循环测试              │
├──────────────────────────────────────────┤
│  tool 层                                 │
│  工具接口定义（被 agent 层调用）          │
│  └── tool.go       Tool 接口            │
├──────────────────────────────────────────┤
│  ai 层                                   │
│  Provider → Model → Stream               │
├──────────────────────────────────────────┤
│  lg 层                                   │
│  日志记录                                │
└──────────────────────────────────────────┘
```

## 模块设计

### 1. Tool 接口 (`tool/tool.go`)

Tool 接口放在独立的 `tool/` 包中，定义工具的最小契约。Agent 运行时通过此接口调用工具。

```go
// Tool Agent 可调用的工具
type Tool interface {
    // Name 工具名称（唯一标识，LLM 通过此名称调用）
    Name() string

    // Definition 返回传递给 LLM 的工具元信息（name, description, parameters JSON Schema）
    Definition() ai.ToolDefinition

    // Execute 执行工具，接收 JSON 格式的参数字符串，返回结果文本
    Execute(ctx context.Context, args string) (string, error)
}
```

**设计决策**：
- 接口极小化：只有 3 个方法，最小契约原则
- `Execute` 的参数和返回值都是 `string`，与 LLM 的工具调用协议对齐（JSON arguments → JSON/text result）
- 放在独立 `tool/` 包而非 `agent/tool.go`，因为 `tool/` 是独立的知识域

### 2. Agent 事件系统 (`agent/event.go`)

Agent 层有自己的事件类型，比 AI 层的 `Event` 更高层：

```go
type AgentEventType int

const (
    AgentStepStart    AgentEventType = iota  // 开始一轮 LLM 调用
    AgentStepEnd                             // 一轮 LLM 调用结束（含 Usage）
    AgentToolCall                            // 发起工具调用
    AgentToolResult                          // 工具返回结果
    AgentTextDelta                           // LLM 文本增量（透传）
    AgentDone                                // Agent 运行结束
    AgentError                               // Agent 运行出错
)

type AgentEvent struct {
    Type     AgentEventType
    Text     string      // AgentTextDelta: 增量文本
    ToolCall *ToolCall   // AgentToolCall: 工具调用
    ToolResult string    // AgentToolResult: 工具返回
    Usage    *ai.Usage   // AgentStepEnd: token 用量
    Step     int         // 当前步骤编号（从 1 开始）
    Err      error       // AgentError: 错误
}
```

**AgentEvent vs ai.Event**：
| 层级 | Event | 职责 |
|------|-------|------|
| ai.Event | 底层事件（TextDelta, ToolCallDelta, Done, Error） | 直接映射 LLM 响应 |
| agent.AgentEvent | 高层事件（Step, ToolCall, ToolResult, Done） | 带有 Agent 运行时语义 |

Agent 运行时会消费 `ai.Event` 流，翻译为 `agent.AgentEvent` 流输出。

### 3. Agent 核心 (`agent/agent.go`)

#### 3.1 结构体

```go
// Config Agent 配置
type Config struct {
    Provider    ai.Provider
    ModelID     string
    SystemPrompt string
    Tools       []tool.Tool
    MaxSteps    int           // 最大工具调用循环次数，默认 10
    Temperature float64       // LLM 温度
    MaxTokens   int           // LLM 最大输出 tokens
}

// Agent 对话型 Agent
type Agent struct {
    cfg         Config
    toolMap     map[string]tool.Tool  // name → Tool
    messages    []ai.Message          // 对话历史
    logger      *lg.Module
}
```

#### 3.2 Run 方法

```go
// Run 启动 Agent 对话循环
func (a *Agent) Run(ctx context.Context, userInput string) Stream
```

**Run 循环逻辑**：

```
输入：userInput（用户消息文本）

1. 用户消息入队
   messages = append(messages, UserMessage(userInput))

2. 工具调用循环（for step := 1; step <= maxSteps; step++）：
   a. 发送 AgentStepStart 事件
   b. 构造 ai.Context{ Messages: messages, Tools: toolDefs, ... }
   c. 调用 provider.Chat(ctx, modelID, context)
   d. 消费 ai.Event 流：
      - EventTextDelta → 透传为 AgentTextDelta
      - EventToolCallStart/Delta/End → 累积工具调用列表
      - EventDone → 发送 AgentStepEnd（携带 Usage）
      - EventError → 发送 AgentError，结束循环
   e. 如果 LLM 发起了工具调用：
      - 输出 AgentToolCall 事件（每个工具调用）
      - 执行 Tool.Execute()，输出 AgentToolResult 事件
      - 将 AssistantMessage(toolCalls) + ToolResultMessages 追加到 messages
      - 继续循环到 step+1
   f. 如果 LLM 没有工具调用：
      - 保存 AssistantMessage 到 messages
      - 发送 AgentDone，结束循环

3. 如果超过 maxSteps 仍未结束：
   - 发送 AgentError(err: "max steps exceeded")
```

#### 3.3 Stream 类型

与 AI 层保持一致，使用 Go channel：

```go
type Stream <-chan AgentEvent
```

#### 3.4 关键设计细节

**对话历史管理**：
- Agent 持有 `messages []ai.Message`，完整保存对话历史
- 每次 `Run()` 不重置历史，支持多轮对话
- 工具调用结果注入到历史中，LLM 可以"看到"之前的工具调用结果

**工具调用协议**：
- LLM 输出的 ToolCall → Agent 执行 → 结果作为 ToolResultMessage 注入历史
- ToolResultMessage 格式：
  ```
  Role: tool
  Content: tool.Execute() 的返回值
  ToolCallID: 对应 ToolCall.ID
  ToolName: 对应 ToolCall.Name
  ```

**错误处理**：
- LLM 调用失败 → AgentError，流终止
- 工具执行 panic → recover，AgentError
- 超时 → context 传播取消
- maxSteps 超限 → AgentError

## 测试策略

使用 mock Provider 和 mock Tool 进行单元测试：

```go
// mockProvider 实现 ai.Provider 接口，返回预设的 ai.Event 序列
type mockProvider struct {
    events []ai.Event  // 预设的事件序列
}

// mockTool 实现 tool.Tool 接口
type mockTool struct {
    name       string
    definition ai.ToolDefinition
    result     string  // 固定返回值
}
```

测试场景：
1. **纯文本对话**：LLM 直接返回文本，无工具调用
2. **单次工具调用**：LLM 调 1 次工具 → 结果注入 → LLM 给出最终回答
3. **多轮工具调用**：LLM 连续调 2-3 次工具 → 每轮都要正确注入结果
4. **maxSteps 超限**：LLM 持续调用工具超过 maxSteps → 报错
5. **工具执行失败**：Tool.Execute() 返回 error → AgentError

## 与 TypeScript 版的关键差异

| 方面 | TypeScript 版 | Go 版 |
|------|---------------|-------|
| 工具调用 | 通过 event callback 驱动 | Tool 接口 + Execute，agent 主动调用 |
| 状态管理 | AgentState 独立对象 | Agent.messages 内置 |
| 事件系统 | EventEmitter subscribe 模式 | Go channel 流式消费 |
| 循环控制 | AgentLoop 独立组件 | Run() 方法内 for 循环 |
| 异步 | Promise/async-await | goroutine + channel |

## 文件结构

```
agent/
├── agent.go           # Agent 结构体 + Run 循环
├── event.go           # AgentEventType, AgentEvent, Stream
└── agent_test.go      # mock provider/mock tool 测试

tool/
└── tool.go            # Tool 接口定义
```

## 依赖关系

```
agent → tool（Tool 接口）
agent → ai（Provider, Model, Context, Event, Stream, Message, ToolDefinition, ToolCall）
agent → lg（日志）
```

## 后续扩展方向（本次不实现）

- **Reset()**：重置对话历史（开始新对话）
- **Subscribe()**：支持多个事件监听器（观察者模式，当前是单消费者）
- **Continue()**：允许外部注入额外上下文（如用户中断后继续）
- **ToolChoice**：指定强制/禁止/自动工具调用
- **Parallel Tool Calls**：并行执行多个独立工具调用
