# Emoji 结构化输出实现计划

## 当前状态

- **分支**: `feature/emoji`
- **已阅读设计文档**: `docs/emoji设计.md`, `docs/emoji实现计划.md`
- **已了解代码结构**: `ai/types.go`, `agent/event.go`, `tui/chat.go`

## 目标

实现可插拔的 emoji 主题系统，让 Agent 回复更结构化，支持主题切换和降级。

## 实施范围

### 阶段 1: 创建 `emoji` 基础包

新增 `emoji/` 目录，包含以下文件：

#### 1.1 `emoji/theme.go`
- 定义 `Slot` 类型（语义槽位）
- 定义 8 个核心槽位常量：`SlotAssistant`, `SlotUser`, `SlotToolCall`, `SlotToolResult`, `SlotStep`, `SlotSuccess`, `SlotWarning`, `SlotError`
- 定义 `Theme` 结构体：`Name` + `Slots` map

#### 1.2 `emoji/registry.go`
- 定义 `Registry` 结构体：管理主题注册
- 方法：`Register(name string, theme Theme)`, `Get(name string) (Theme, bool)`
- 内置注册：default, minimal, monochrome

#### 1.3 `emoji/resolver.go`
- 定义 `Resolver` 结构体：包含 registry + current theme name
- 方法：`Resolve(slot Slot) string`
- 实现回退链：当前主题 → registry → 内置默认 → 纯文本回退

#### 1.4 `emoji/builtin/default.go`
- 提供完整的 emoji 主题（使用实际 emoji 字符）

#### 1.5 `emoji/builtin/minimal.go`
- 提供极简主题（仅用 ASCII 符号）

#### 1.6 `emoji/builtin/monochrome.go`
- 提供单色主题（使用简单几何符号）

#### 1.7 `emoji/emoji_test.go`
- 测试主题注册和回退
- 测试默认主题和降级主题

### 阶段 2: 修改 `ai/types.go`

在 `Context` 结构体中新增可选字段：
```go
type Context struct {
    Messages     []Message
    Tools        []ToolDefinition
    SystemPrompt string
    MaxTokens    int
    Temperature  float64
    EmojiTheme   string  // 新增：emoji 主题名称
}
```

在 `Context.BuildSystemPrompt()` 或相关方法中：
- 如果 `EmojiTheme` 非空，注入结构化表达约束到 system prompt
- 不要求模型输出固定字符，只要求语义化表达

### 阶段 3: 修改 `tui/chat.go`

将硬编码的 emoji/前缀替换为主题解析：

#### 3.1 修改 `ChatUI` 结构体
- 新增 `emojiResolver *emoji.Resolver` 字段

#### 3.2 修改 `NewChatUI`
- 接收可选参数 `WithEmojiResolver(resolver *emoji.Resolver)`

#### 3.3 修改渲染方法
- `printUserInput`: `💬` → `resolver.Resolve(emoji.SlotUser) + " "`
- `renderAgentStream`:
  - 前缀 `🤖` → `resolver.Resolve(emoji.SlotAssistant) + " "`
  - `[调用工具 %s]` → `resolver.Resolve(emoji.SlotToolCall) + " " + toolName`
  - `[工具结果]` → `resolver.Resolve(emoji.SlotToolResult) + " "`
  - `✖` (error) → `resolver.Resolve(emoji.SlotError) + " "`
- `ExportConversation`:
  - `## 👤 用户` → 使用 resolver
  - `## 🤖 助手` → 使用 resolver

### 阶段 4: 修改 `cmd/pg/main.go` 或 `cmd/pg/chat.go`

在 CLI 入口支持 `--emoji-theme` 参数或配置项：
- 读取配置优先级：命令行 > 配置文件 > 环境变量 > 默认
- 创建 resolver 并传递给 ChatUI

### 阶段 5: 补测试

- `emoji/emoji_test.go`: 主题解析和回退测试
- `tui/chat_test.go`: 更新现有测试，验证前缀渲染

## 改动文件清单

### 新增文件
1. `emoji/theme.go`
2. `emoji/registry.go`
3. `emoji/resolver.go`
4. `emoji/builtin/default.go`
5. `emoji/builtin/minimal.go`
6. `emoji/builtin/monochrome.go`
7. `emoji/emoji_test.go`

### 修改文件
1. `ai/types.go` - 在 `Context` 中新增 `EmojiTheme` 字段
2. `tui/chat.go` - 移除硬编码 emoji，使用 resolver
3. `cmd/pg/chat.go` 或 `cmd/pg/main.go` - 支持主题配置
4. `cmd/pg/config.go` - 新增 `EmojiTheme` 配置项

### 可能涉及的已有文件
- `agent/agent.go` - 在构建 Context 时透传主题信息
- `cmd/pg/config.go` - 新增配置项

## API 设计

### emoji.Theme
```go
type Slot string // 语义槽位

const (
    SlotAssistant   Slot = "assistant"
    SlotUser        Slot = "user"
    SlotToolCall    Slot = "tool_call"
    SlotToolResult  Slot = "tool_result"
    SlotStep        Slot = "step"
    SlotSuccess     Slot = "success"
    SlotWarning     Slot = "warning"
    SlotError       Slot = "error"
)

type Theme struct {
    Name  string
    Slots map[Slot]string
}
```

### emoji.Registry
```go
type Registry struct {
    themes map[string]Theme
}

func NewRegistry() *Registry
func (r *Registry) Register(theme Theme)
func (r *Registry) Get(name string) (Theme, bool)
```

### emoji.Resolver
```go
type Resolver struct {
    registry *Registry
    themeName string
    fallback  Theme // 纯文本回退主题
}

func NewResolver(registry *Registry, themeName string) *Resolver
func (res *Resolver) Resolve(slot Slot) string
```

### ai.Context 新增字段
```go
type Context struct {
    // ... 现有字段 ...
    EmojiTheme string // 可选：emoji 主题名称
}
```

### tui.ChatUI 新增配置
```go
type ChatUIOption func(*ChatUI)
func WithEmojiResolver(resolver *emoji.Resolver) ChatUIOption
```

## 配置方案

在 `cmd/pg/config.go` 中新增：
```go
type Config struct {
    // ... 现有字段 ...
    EmojiTheme string `json:"emoji_theme" yaml:"emoji_theme"`
}
```

支持环境变量 `PIGO_EMOJI_THEME`。

## 验收标准

1. ✅ 能按名称获取主题
2. ✅ 主题缺字段时能自动回退到纯文本
3. ✅ tui/chat.go 不再硬编码 emoji
4. ✅ 支持命令行参数/环境变量配置主题
5. ✅ 默认主题效果不退化
6. ✅ 核心路径测试通过

## 注意事项

1. **不改 agent 事件协议**：agent/event.go 完全不动
2. **纯文本回退**：必须保证无 emoji 时可正常使用
3. **回退链**：用户主题 → 项目默认 → 内置默认 → 纯文本
4. **Go 风格**：用 interface/struct/error 惯用模式，不照搬 TS

## 预期效果

### 默认主题
```
💬 用户输入
🤖 助手回复
[调用工具 read_file]  # 工具调用
[工具结果] 文件内容    # 工具结果
```

### Minimal 主题
```
> 用户输入
> 助手回复
[tool: read_file]
[result] 文件内容
```

### Monochrome 主题
```
[U] 用户输入
[A] 助手回复
[T] read_file
[R] 文件内容
```
