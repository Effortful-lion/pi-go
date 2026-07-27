# AI 层设计文档

## 概述

AI 层（`ai/`）是整个 pi-go Agent 系统的底层基础，提供多提供商 LLM 统一接口。它将 OpenAI、Anthropic、Google 等不同 LLM 提供商的差异抽象为统一接口，让上层 Agent 运行时无需关心底层提供商细节。

## 架构分层

```
┌──────────────────────────────────────────┐
│  agent 层                                │
│  使用 Provider.Model → Context → Stream  │
├──────────────────────────────────────────┤
│  ai 层（本文档）                          │
│  Provider 接口 + Model 定义 + 事件系统    │
│  ├── types.go      基础类型              │
│  ├── provider.go   Provider 接口         │
│  ├── model.go      ModelInfo/Model       │
│  ├── event.go      事件类型 + Stream     │
│  └── providers/    具体提供商实现         │
│      └── openai/   OpenAI Provider      │
├──────────────────────────────────────────┤
│  lg 层                                   │
│  日志记录                                │
└──────────────────────────────────────────┘
```

## 模块设计

### 1. 基础类型 (`types.go`)

#### 1.1 Message 消息体系

```go
// Role 消息角色
type Role string

const (
    RoleSystem    Role = "system"
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
)

// Message 一条对话消息
type Message struct {
    Role    Role
    Content string            // UserMessage/SystemMessage 的文本内容
    Blocks  []ContentBlock    // AssistantMessage 的内容块（富文本）
    ToolCallID string         // ToolResultMessage 的回溯ID
    ToolName   string         // ToolResultMessage 的工具名
}
```

**设计决策**：Go 语言不使用 TypeScript 的 tagged union 模式。改用单一 `Message` struct + Role 字段区分消息类型。AssistantMessage 使用 `Blocks` 字段携带富内容，UserMessage/SystemMessage 使用 `Content` 字段携带纯文本。

#### 1.2 ContentBlock 内容块

```go
// ContentBlockType 内容块类型
type ContentBlockType string

const (
    BlockText     ContentBlockType = "text"
    BlockThinking ContentBlockType = "thinking"
    BlockToolCall ContentBlockType = "tool_call"
    BlockImage    ContentBlockType = "image"
)

// ContentBlock 富文本内容块（仅 AssistantMessage 使用）
type ContentBlock struct {
    Type      ContentBlockType
    Text      string       // BlockText: 文本内容
    Thinking  string       // BlockThinking: 思考内容（如 Claude thinking）
    ToolCall  *ToolCall    // BlockToolCall: 工具调用
    ImageURL  string       // BlockImage: 图片URL
}
```

**设计决策**：Go 不适合用 interface 实现 sealed union。使用 flat struct + Type 字段更好，避免类型断言泛滥。上层通过 Type 做 switch 判断。

#### 1.3 工具定义与调用

```go
// ToolDefinition Agent 可使用的工具定义
type ToolDefinition struct {
    Name        string
    Description string
    Parameters  map[string]any  // JSON Schema 参数定义
}

// ToolCall 助手发起的工具调用
type ToolCall struct {
    ID         string
    Name       string
    Arguments  string  // JSON 格式的参数
}
```

#### 1.4 Context 对话上下文

```go
// Context 一次 Chat 请求的完整上下文
type Context struct {
    Messages     []Message
    Tools        []ToolDefinition
    SystemPrompt string  // 快捷设置 SystemMessage（放在 Messages 最前面）
    MaxTokens    int     // 0 表示不限制
    Temperature  float64 // 0 表示使用提供商默认值
}
```

### 2. 事件系统 (`event.go`)

#### 2.1 事件类型

模仿 SSE 流式响应的生命周期，定义一组连续的事件类型：

```go
type EventType int

const (
    EventStart          EventType = iota  // 流开始
    EventTextStart                       // 文本块开始
    EventTextDelta                       // 文本增量
    EventTextEnd                         // 文本块结束
    EventThinkingStart                   // 思考开始（Claude等支持）
    EventThinkingDelta                   // 思考增量
    EventThinkingEnd                     // 思考结束
    EventToolCallStart                   // 工具调用开始
    EventToolCallDelta                   // 工具调用参数增量
    EventToolCallEnd                     // 工具调用结束
    EventDone                            // 整个响应结束（携带 Usage）
    EventError                           // 错误（流中断）
)
```

#### 2.2 Event 结构体

```go
// Event 流式响应中的单个事件
type Event struct {
    Type     EventType
    Text     string   // TextDelta/ThinkingDelta: 增量文本
    Index    int      // 内容块索引（多个 text/thinking/toolcall 混排时区分）
    ToolCall *ToolCall // ToolCallStart/End/Delta: 工具调用
    Usage    *Usage   // Done: token 用量统计
    Err      error    // Error: 错误信息
}

// Usage token 用量
type Usage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

#### 2.3 Stream 类型

```go
// Stream 事件通道，Provider.Chat() 的返回值
// 用法：for event := range stream { ... }
type Stream <-chan Event
```

**设计决策**：使用 Go channel 作为流式传输机制。channel 天然支持 for-range 消费，不需要实现复杂的 Iterator 模式。关闭 channel 表示流结束。

### 3. Model 定义 (`model.go`)

```go
// ModelInfo 模型元信息
type ModelInfo struct {
    ID            string  // "gpt-4o", "claude-sonnet-4-5"
    Name          string  // 展示名称
    ProviderID    string  // 所属 provider
    ContextWindow int     // 上下文窗口大小（tokens）
    MaxTokens     int     // 最大输出 tokens
}

// Model 模型实例接口（后续 agent 层使用）
type Model interface {
    Info() ModelInfo
    Chat(ctx context.Context, context Context) Stream
}
```

**设计决策**：`ModelInfo` 是纯数据（值类型），`Model` 是活的对象（绑定 Provider 的 Chat 能力）。上层 agent 通过 Provider 获取 Model。

### 4. Provider 接口 (`provider.go`)

```go
// Provider LLM 提供商
type Provider interface {
    // ID 提供商唯一标识，如 "openai", "anthropic"
    ID() string

    // Name 展示名称
    Name() string

    // Models 返回该提供商所有可用模型的元信息列表
    Models() []ModelInfo

    // Model 获取具体的模型实例
    Model(modelID string) (Model, error)

    // Chat 使用指定模型进行对话，返回事件流
    // 等价于 Model(modelID).Chat(ctx, context)，快捷方法
    Chat(ctx context.Context, modelID string, context Context) Stream
}
```

**设计决策**：`Provider` 既是模型目录（列出模型），也是对话入口（Chat）。这比 TypeScript 版更简洁：不需要单独的 Models 聚合器，Provider 本身就能列出模型和发起对话。

### 5. OpenAI 实现 (`providers/openai/provider.go`)

#### 5.1 核心职责

- 向 OpenAI Chat Completions Streaming API (`POST /v1/chat/completions`) 发起请求
- 解析 SSE 流式响应，转换为标准 `Event` 流
- 将 OpenAI 的响应格式映射到 ai 层统一类型

#### 5.2 配置

```go
// Config OpenAI Provider 配置
type Config struct {
    APIKey  string
    BaseURL string  // 默认 https://api.openai.com/v1
}

// New 创建 OpenAI Provider
func New(cfg Config) *Provider
```

#### 5.3 HTTP 请求流程

1. 构造 OpenAI Chat Completions 请求体（JSON）
2. 设置 HTTP headers（Authorization, Content-Type, Accept: text/event-stream）
3. 发送 POST 请求，`stream: true`
4. 逐行读取 SSE 响应，解析 `data:` 行
5. 将 OpenAI chunk 映射为 `Event`，写入 channel
6. 读取到 `[DONE]` 后发送 `EventDone`，关闭 channel
7. 任何错误发送 `EventError`，关闭 channel

#### 5.4 SSE 解析

```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"}}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":" world"}}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":""},"finish_reason":"stop"}],"usage":{...}}

data: [DONE]
```

主要 delta 类型：
- `delta.content` → `EventTextDelta`
- `delta.tool_calls[].function.name` → `EventToolCallStart`
- `delta.tool_calls[].function.arguments` → `EventToolCallDelta`
- `finish_reason: "stop"` → 最后一个 delta 后发送 `EventDone`

#### 5.5 消息映射

| OpenAI 格式 | ai 层类型 |
|-------------|-----------|
| `role: "user"` | `RoleUser` |
| `role: "assistant"` | `RoleAssistant` |
| `role: "system"` | `RoleSystem` |
| `role: "tool"` | `RoleTool` |
| `tool_calls` in assistant | `ContentBlock{Type: BlockToolCall}` |

#### 5.6 依赖

- 仅使用标准库：`net/http`, `encoding/json`, `bufio`, `context`
- 不引入第三方 HTTP 客户端或 OpenAI SDK
- 使用 `lg` 包记录关键日志（请求 URL、状态码、错误）

## 关键设计决策

### 为什么用 channel 而不是回调

Go 的 channel 是天然的流式数据管道，配合 `for-range` 消费非常简洁。TypeScript 版使用 callback/EventEmitter 是因为 JS 没有类似的原生原语。

### 为什么 Message 是 flat struct 而不是 interface union

Go 的 interface + type switch 是可行的，但会导致大量运行时类型断言。flat struct + Role 字段更符合 Go 风格，编码体验更好。

### 为什么不引入第三方 SDK

保持零外部依赖是 Go 标准库哲学。OpenAI 的 API 足够简单，用 `net/http` 直接调用即可。如果后续某提供商协议很复杂，可以单独引入。

### 为什么不先做 CredentialStore

认证持久化应该在 Agent 层处理，而不是 AI 层。AI 层只负责"给定 API Key 如何调用"，不负责 Key 从哪里来。

## 文件结构

```
ai/
├── types.go                  # Message, ContentBlock, ToolDefinition, Context
├── event.go                  # EventType, Event, Stream, Usage
├── model.go                  # ModelInfo, Model 接口
├── provider.go               # Provider 接口
└── providers/
    └── openai/
        ├── provider.go       # OpenAI Provider 实现
        └── provider_test.go  # 测试
```
