# TUI 层设计文档

## 概述

TUI 层（`tui/`）提供交互式对话所需的终端能力，是 pi-agent CLI 产品的直接 UI 依赖。

核心目标：**用最小代价构建一个"能用、好看"的终端对话界面**，让用户像使用普通 REPL 一样与 AI Agent 交互。

## 架构分层

```
┌──────────────────────────────────────────┐
│  cmd/pi 产品入口                         │
│  创建 ChatUI → 调用 Run()               │
├──────────────────────────────────────────┤
│  tui 层（本文档）                        │
│  ├── terminal.go   终端底层（ANSI/raw）  │
│  └── chat.go       ChatUI 交互对话       │
├──────────────────────────────────────────┤
│  agent 层                                │
│  Agent.Run() 驱动 LLM ↔ 工具循环         │
├──────────────────────────────────────────┤
│  ai 层                                   │
│  Provider → Model → Stream               │
├──────────────────────────────────────────┤
│  lg 层                                   │
│  日志记录                                │
└──────────────────────────────────────────┘
```

## 与 TypeScript 版的关键差异

| 方面 | TypeScript pi-tui | Go 版 |
|------|-------------------|-------|
| 渲染模式 | 差异渲染 + 虚拟 DOM | 行式追加（scroll up） |
| 组件系统 | 组件树 + 协调算法 | 无组件系统，直接操作终端 |
| 状态管理 | 全局事件总线 | 无全局状态，ChatUI 自管理 |
| 依赖 | 底层终端库 | 纯 Go 标准库 + syscall |

**放弃组件系统的理由**：
- pi-agent 只需要一个对话界面，不是通用 TUI 框架
- 差异渲染/组件系统对对话场景过度设计
- Go 标准库的 ANSI 控制能力足够应对需求

## 模块设计

### 1. 终端底层 (`terminal.go`)

提供 ANSI escape code 的 Go 封装，函数式、无状态：

#### 1.1 原始模式（Raw Mode）

```
进入原始模式 → 终端逐字符输入，不用等 Enter
退出原始模式 → 恢复到 cooked mode
```

```go
// EnterRawMode 进入原始终端模式，返回恢复函数。
// 用法：
//   restore, err := tui.EnterRawMode()
//   defer restore()
func EnterRawMode() (restore func(), err error)
```

实现：通过 `syscall` 操作 `termios`（`ECHO`、`ICANON` 等 flag），保存原始状态用于恢复。

#### 1.2 光标控制

```go
// 移动光标
func CursorUp(n int)    // ANSI: \033[{n}A
func CursorDown(n int)  // ANSI: \033[{n}B
func CursorForward(n int)
func CursorBack(n int)

// 其他
func CursorShow()       // ANSI: \033[?25h
func CursorHide()       // ANSI: \033[?25l
func ClearLine()        // ANSI: \033[2K
func ClearScreen()      // ANSI: \033[2J
```

#### 1.3 颜色/样式

```go
type Color int

const (
    ColorDefault Color = 39
    ColorBlack         = 30
    ColorRed           = 31
    ColorGreen         = 32
    ColorYellow        = 33
    ColorBlue          = 34
    ColorMagenta       = 35
    ColorCyan          = 36
    ColorWhite         = 37
    ColorGray          = 90
)

type Style struct {
    Fg Color
    Bg Color
    Bold, Dim, Italic, Underline bool
}

// StyleText 用指定样式包裹文本。
func StyleText(text string, s Style) string

// 常用快捷函数
func Bold(text string) string
func Dim(text string) string
func Red(text string) string
func Green(text string) string
func Blue(text string) string
func Cyan(text string) string
func Gray(text string) string
```

#### 1.4 行编辑

```go
// EditLine 单行文本编辑器。
// 支持: ← → Home End Backspace Delete Enter Ctrl+C Ctrl+D
// 返回: 编辑完成的文本, 是否取消
func EditLine(prompt string) (text string, cancelled bool)
```

键盘事件映射：
```
Enter       → 确认输入，返回当前文本
Ctrl+C      → 取消（返回 ""，cancelled=true）
Ctrl+D      → 在空行时取消，否则删除光标后字符
← →         → 光标左右移动
Home (^A)   → 光标移到行首
End (^E)    → 光标移到行尾
Backspace   → 删除光标前字符
Delete      → 删除光标后字符
```

实现细节：
- 通过 `os.Stdin.Read()` 读取原始字节序列
- ANSI escape 序列解析（如 `\033[C` = 右箭头）
- 内联重绘（不换行，重写同一行）

### 2. ChatUI (`chat.go`)

#### 2.1 结构体

```go
type ChatUI struct {
    agent  *agent.Agent    // Agent 实例（生命周期由外部管理）
    prompt string          // 输入提示符，默认 "> "
}

// NewChatUI 创建对话 UI。
func NewChatUI(ag *agent.Agent) *ChatUI
```

#### 2.2 Run 方法

```go
// Run 启动交互式对话循环。
// 阻塞直到用户退出（Ctrl+C / Ctrl+D 在空行）。
func (ui *ChatUI) Run(ctx context.Context) error
```

**交互循环**：

```
1. 进入原始模式
2. 打印欢迎信息
3. for {
    显示 prompt → 读取用户输入（EditLine）
    if 用户取消 → break
    if 空输入 → continue

    打印用户输入回显（💬 You: xxx）

    调用 agent.Run(ctx, input)
    消费 agent.Event 流：
      EventStepStart → 无 UI 动作（仅日志）
      EventTextDelta → 实时写入终端（无缓冲）
      EventToolCall → 打印 "[调用工具: {...}]"（灰色）
      EventToolResult → 打印 "[工具结果: ...]"（灰色）
      EventStepEnd → 打印 Usage 统计（灰色）
      EventDone → 换行
      EventError → 打印错误信息（红色）

    换行
  }
4. 恢复终端模式
5. 打印告别信息
```

**流式渲染的关键**：
- Agent 返回 `agent.Stream` 通道
- ChatUI 在 goroutine 中消费通道
- 在等待事件的同时，主 goroutine 阻塞住，不读取新的用户输入
- 这意味着对话是"一问一答"的串行模式，不是多轮并发

#### 2.3 外观设计

```
┌─ Pi Agent ──────────────────────────────────┐
│                                              │
│  Hello! I'm Pi Agent, an AI coding assistant.│
│  Type your question below. Ctrl+C to exit.   │
│                                              │
│  💬 What is Go?                              │  ← 青色
│                                              │
│  🤖 Go (or Golang) is a statically typed,     │  ← 流式输出
│  compiled programming language designed at    │
│  Google...                                    │
│                              [2.3k tokens]   │  ← 灰色，Usage 统计
│                                              │
│  > █                                         │  ← 输入提示符
└──────────────────────────────────────────────┘
```

角色标记：
- `💬` You（青色/蓝色）：用户输入回显
- `🤖` Assistant（绿色/默认）：LLM 回复
- `🔧` 工具调用（灰色）：`[调用工具 get_weather]`
- `[2.3k tokens]`（灰色）：Usage 统计

## 测试策略

### terminal_test.go
- ANSI 输出格式验证（StyleText 生成正确的 escape 序列）
- 光标控制序列验证
- 不需要测试 raw mode（依赖 syscall，极难 mock）

### chat_test.go
- Mock Agent（返回预设的 agent.Event 序列）
- 验证 ChatUI 的渲染输出（重定向 stdout 捕获）
- 关键路径测试：
  1. 正常对话（文本响应）
  2. 工具调用流程
  3. 错误处理
  4. 空输入/Ctrl+C 退出

## 依赖关系

```
tui → agent（Agent 实例）
tui → lg（日志）
tui 不直接依赖 ai 层（通过 agent 间接使用）
```

## 文件结构

```
tui/
├── terminal.go        # ANSI 控制 + raw mode + 行编辑
├── terminal_test.go   # ANSI 输出验证
├── chat.go            # ChatUI 交互对话
└── chat_test.go       # ChatUI 流程测试
```

## 后续扩展方向（本次不实现）

- **多行输入**：当前仅单行，后续支持 Alt+Enter 换行
- **历史记录**：↑ ↓ 浏览历史输入
- **Ctrl+L 清屏**
- **语法高亮**：代码块内着色
- **文件拖放**：粘贴文件路径
- **Markdown 渲染**：终端中渲染表格、列表等
