# Emoji 主题系统设计

## 目标

为 `pi-go` 提供一套可插拔、可替换的 emoji 主题系统，用于两件事：

1. 让 Agent 回复更结构化
2. 让终端展示更生动，但不把 emoji 写死在业务代码里

## 设计原则

- 不改 `agent` 的核心事件协议
- `ai` 负责“怎么引导模型表达”
- `tui` 负责“怎么渲染给用户看”
- emoji 必须可替换、可降级、可测试

## 总体架构

新增独立 `emoji/` 包，作为语义映射层。

```text
ai.Context -> system prompt 里的 emoji/style 约束
agent.Event -> 保持不变
emoji.Theme -> 语义到符号的映射
tui -> 读取主题并渲染
```

## 核心组件

### `emoji.Theme`

定义一组语义槽位，而不是单个字符。

建议槽位：

- `assistant`
- `user`
- `toolCall`
- `toolResult`
- `step`
- `success`
- `warning`
- `error`

### `emoji.Registry`

管理主题注册、默认主题、用户自定义主题和回退链。

### `emoji.Resolver`

根据当前场景解析最终显示值：

1. 用户指定主题
2. 项目默认主题
3. 内置默认主题
4. 纯文本回退

## 数据流

1. `agent` 产出标准事件
2. `tui` 读取事件
3. `tui` 通过 `emoji.Resolver` 获取符号
4. `tui` 统一渲染前缀、状态、结果
5. `ai.Context.SystemPrompt` 注入输出约束，提示模型使用结构化表达

## 配置方式

优先级建议：

1. 命令行参数
2. 配置文件
3. 环境变量
4. 默认主题

建议支持：

- `emoji_theme: default`
- `emoji_theme: minimal`
- `emoji_theme: monochrome`

## 输出约束

system prompt 中补充一段轻量规范：

- 回复优先使用短段落和明确小标题
- 工具调用前后使用一致的语义标记
- 不强制模型生成 emoji 字符本身，只要求语义化表达

这样可以避免模型过度依赖具体字符，也方便主题替换。

## 兼容策略

- 终端不支持彩色或 emoji 时，自动降级为 ASCII 前缀
- 主题缺失槽位时，使用默认值补齐
- 主题解析失败时，直接回退到内置主题

## 建议目录

```text
emoji/
  theme.go
  registry.go
  resolver.go
  builtin/
    default.go
    minimal.go
    monochrome.go
```

## 预期改动点

- `ai/types.go`：为上下文增加可选输出风格入口
- `agent/`：保持事件不变，只在构建上下文时透传风格信息
- `tui/chat.go`：移除硬编码 emoji 前缀，改为主题解析
- `tui/markdown.go`：不直接依赖 emoji，保持纯渲染职责

## 测试策略

- 主题解析测试
- 回退链测试
- `tui` 渲染前缀测试
- system prompt 片段测试

## 范围边界

本设计只解决“结构化表达的 emoji 主题化”。

不做：

- 富表情包系统
- 复杂动画
- 网络化主题下载
- 运行时在线切换整套 UI 风格

# Emoji 主题系统实现计划

## 目标

实现一套可插拔、可替换的 emoji 主题系统，满足以下目标：

1. 让 Agent 的回复更结构化
2. 让终端展示更生动
3. 允许不同主题在不改业务代码的情况下切换

## 实施范围

本次实现只覆盖当前 `pi-go` 的本地主题系统，不做远程主题分发，不做复杂动画，不改 `agent` 核心事件协议。

## 总体原则

- `agent` 事件保持稳定
- `ai` 只负责提示词层面的语义约束
- `tui` 只负责展示层渲染
- emoji 的具体字符必须集中管理

## 分阶段计划

### 阶段 1：新增 `emoji` 基础包

实现可复用的主题抽象和解析逻辑。

预计文件：

- `emoji/theme.go`
- `emoji/registry.go`
- `emoji/resolver.go`
- `emoji/builtin/default.go`
- `emoji/builtin/minimal.go`
- `emoji/builtin/monochrome.go`

任务：

- 定义主题槽位
- 定义主题注册接口
- 定义主题解析和回退链
- 提供内置主题

验收：

- 能按名称拿到主题
- 主题缺字段时能自动回退
- 纯文本模式可用

### 阶段 2：接入 `ai` 层提示词

把 emoji 语义约束注入系统提示词，但不要求模型输出固定字符。

预计文件：

- `ai/types.go`
- `agent/agent.go`
- `docs/ai-layer-design.md`（如需补充说明）

任务：

- 为 `ai.Context` 增加可选输出风格入口
- 在构建 system prompt 时透传主题约束
- 保持现有消息结构不变

验收：

- 未配置时行为不变
- 配置主题后 prompt 中能出现结构化表达约束

### 阶段 3：接入 `tui` 渲染

用主题系统替换当前硬编码 emoji 前缀。

预计文件：

- `tui/chat.go`

任务：

- 将 `💬`、`🤖`、工具提示等前缀改为主题解析结果
- 支持不同主题下的统一展示
- 保持现有 Markdown 渲染逻辑不变

验收：

- 默认主题下显示效果不退化
- 切换主题后前缀会变化
- 主题缺失时能正确降级

### 阶段 4：补测试

为主题解析和展示链路补核心测试。

预计文件：

- `emoji/*_test.go`
- `tui/chat_test.go`

任务：

- 测试主题注册和回退
- 测试默认主题和降级主题
- 测试 TUI 前缀渲染

验收：

- 核心路径测试通过
- 主题变更不会破坏现有对话输出

## 配置设计

建议支持以下配置项：

- `emoji_theme`

优先级：

1. 命令行参数
2. 配置文件
3. 环境变量
4. 默认主题

## 主题语义槽位

建议最少支持以下槽位：

- `assistant`
- `user`
- `toolCall`
- `toolResult`
- `step`
- `success`
- `warning`
- `error`

## 回退规则

1. 先找用户指定主题
2. 再找项目默认主题
3. 再找内置默认主题
4. 最后退化为 ASCII 前缀

## 交付标准

完成后应满足：

- 不改 `agent` 事件协议
- `emoji` 能独立注册和解析
- `tui` 不再硬编码展示 emoji
- 主题可替换且有降级路径

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
# Emoji 主题系统使用指南

## 概述

`pi-go` 提供了一套可插拔的 emoji 主题系统，用于结构化 Agent 回复和终端展示。

## 内置主题

系统内置 3 个主题：

### 1. default（默认）
使用完整 emoji 字符，提供丰富的视觉反馈：
```
💬 用户输入
🤖 助手回复
🔧 read_file   # 工具调用
📋 文件内容    # 工具结果
```

### 2. minimal
仅用 ASCII 符号，适合不支持 emoji 的终端：
```
> 用户输入
> 助手回复
[tool] read_file
[result] 文件内容
```

### 3. monochrome
使用简单几何符号，保持单色风格：
```
[U] 用户输入
[A] 助手回复
[T] read_file
[R] 文件内容
```

## 配置方式

### 1. 命令行参数（最高优先级）

```bash
pg -emoji-theme=minimal
pg -emoji-theme=monochrome
```

### 2. 环境变量

```bash
export PIGO_EMOJI_THEME=minimal
pg
```

### 3. 配置文件

编辑 `~/.pi-go/config.yaml`：

```yaml
provider: openai
model: gpt-4o
api_key: sk-...
emoji_theme: minimal
```

或使用命令设置：

```bash
pg config set emoji_theme minimal
```

### 4. 默认值

未配置时自动使用 `default` 主题。

## 主题槽位

每个主题定义以下 8 个语义槽位：

| 槽位 | 说明 | default | minimal | monochrome |
|------|------|---------|---------|------------|
| `assistant` | 助手标识 | 🤖 | > | [A] |
| `user` | 用户标识 | 💬 | > | [U] |
| `tool_call` | 工具调用 | 🔧 | [tool] | [T] |
| `tool_result` | 工具结果 | 📋 | [result] | [R] |
| `step` | 步骤 | ▶ | -> | [>] |
| `success` | 成功 | ✅ | [ok] | [+] |
| `warning` | 警告 | ⚠️ | [!] | [!] |
| `error` | 错误 | ✖ | [x] | [-] |

## 自定义主题

可以通过代码注册自定义主题：

```go
import "github.com/Effortful-lion/pi-go/emoji"

// 创建自定义主题
customTheme := emoji.NewTheme("my-theme", map[emoji.Slot]string{
    emoji.SlotAssistant:  "🤔",
    emoji.SlotUser:       "👤",
    emoji.SlotToolCall:   "⚙️",
    emoji.SlotToolResult: "📄",
    // ... 其他槽位
})

// 注册到全局注册表
emoji.DefaultRegistry.Register(customTheme)

// 使用自定义主题
resolver := emoji.NewResolver(emoji.DefaultRegistry, "my-theme")
```

## 技术细节

### 回退链

1. 用户指定的主题名称
2. Registry 中的同名主题
3. 内置 `default` 主题
4. 纯文本回退（`[A]`, `[U]` 等）

### 包依赖

- `emoji` 包独立，不依赖 `tui` 或 `agent`
- `tui` 通过 `emoji.Resolver` 获取符号
- `agent` 事件协议完全不受影响

### API 参考

```go
// 核心类型
type Slot string
type Theme struct{ Name string; Slots map[Slot]string }
type Registry struct{ /* ... */ }
type Resolver struct{ /* ... */ }

// 主要方法
registry.Register(theme)
registry.Get(name) (Theme, bool)
resolver.Resolve(slot) string
resolver.SetTheme(name)

// 全局实例
emoji.DefaultRegistry  // 内置所有主题
emoji.DefaultResolver  // 使用 default 主题
```

## 兼容性

- 终端不支持 emoji 时，使用 `minimal` 或 `monochrome` 主题
- 主题缺失槽位时，自动回退到纯文本
- 主题解析失败时，直接使用内置回退链
