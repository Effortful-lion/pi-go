# 终端 UI 微调计划

## 概述

根据 AI 回答后的体验反馈，对终端 UI 进行 5 个方面的微调，提升感官和可读性。

## 需求清单

| # | 需求 | 当前状态 | 目标状态 |
|---|------|---------|---------|
| 1 | 去掉重复显示用户输入 | `printUserInput` 在 Agent 输出前又回显了一次用户输入 | 用户输入只显示一次（输入时的回显），不在回答前再次显示 |
| 2 | 输出去掉 Markdown 语法 | AI 返回的内容包含 `**加粗**`、`### 标题`、`` `代码` `` 等裸 Markdown 标记 | 通过 ANSI 样式、emoji、空行/换行、结构化缩进等替代 Markdown，保证结构化可读性 |
| 3 | 去掉 `[INFO]` 日志输出 | provider 中 `logger.Info("发起 Chat 请求")` 写入日志文件，但日志文件仍包含 INFO 级别日志；`initLogging` 已重定向到文件但 **用户截图显示终端出现了 `[INFO]` 输出** | 按日志级别分文件存储：INFO 级别日志写入独立文件，ERROR/WARN 单独存放；确保 INFO 日志不出现在终端 |
| 4 | 输入输出框化 | 输入输出散落无边框 | 输入输出在一个视觉框（box）中，类似 Claude Code / Codex 风格 |
| 5 | 欢迎语修正 | `Pi Agent — AI Coding Assistant` | `Pi Go — AI Coding Assistant` |

---

## 详细方案

### 1. 去掉重复显示用户输入

**问题分析**：`tui/chat.go` 的 `Run()` 方法中：
- 用户输入时，`LineEditor.ReadLine()` 在 raw mode 下读入，终端不回显
- 输入完成后，`printUserInput(input)` 手动回显一次：显示 `💬 用户输入内容`
- 然后 Agent 开始回答

实际上，raw mode 下的 LineEditor 本身应该已经显示了用户输入（实时按键回显）。`printUserInput` 是多余的回显。

**修改文件**：`tui/chat.go`

**修改内容**：删除 `printUserInput(input)` 调用，或者将其改为只显示一个换行分隔符。

```go
// 修改前
le.AddHistory(input)
ui.printUserInput(input)    // ← 删除此行
stream := ui.agent.Run(ctx, input)

// 修改后
le.AddHistory(input)
fmt.Fprintln(ui.out)         // 只换行分隔
stream := ui.agent.Run(ctx, input)
```

---

### 2. 输出去掉 Markdown 语法

**问题分析**：当前 `renderAgentStream` 直接输出 `evt.Text`（流式文本），不经过 `RenderMarkdown()` 处理。AI 返回的原始文本包含 `**bold**`、`### heading`、`` `code` `` 等 Markdown 标记。

**方案**：修改 `renderAgentStream`，对流式文本进行实时 Markdown 转换：
- `**text**` → `Bold(text)` (ANSI 加粗)
- `*text*` → `Italic(text)` (ANSI 斜体)
- `### heading` → `Bold(heading)` + 换行
- `` `code` `` → `Cyan(code)` (ANSI 青色)
- `- list item` → `Green("  •") + " " + item`

但由于是流式文本（分 chunk 到达），无法做完整 Markdown 解析。更好的方案是：**在 Agent 层累积完整文本后再渲染**，或**对流式文本做简单的行内标记替换**。

**实际方案**：使用已有的 `tui/markdown.go` 中的 `RenderMarkdown()` 函数，但需要改造 Agent 流，累积完整文本后一次性渲染。

**修改文件**：
- `agent/agent.go`：Agent 输出改为累积完整文本后发送 `EventTextComplete`
- `tui/chat.go`：`renderAgentStream` 收到完整文本后调用 `RenderMarkdown()`

或者更简单的方案：**在 `renderAgentStream` 中对每个 `EventTextDelta` 做简单的实时转换**（行内标记如 `**bold**`、`` `code` ``），列表/标题等块级标记由于依赖换行，可以通过累积行来处理。

**选定方案（更简单可行）**：

由于 `RenderMarkdown()` 已经支持各种 Markdown 语法转 ANSI，我们采用**逐行缓冲**方案：
- 在 `renderAgentStream` 中维护一个行缓冲区
- 文本 delta 累积到缓冲区
- 遇到 `\n` 时，将该行通过 `RenderMarkdown()` 渲染后输出
- 流结束时，将剩余缓冲也渲染输出

```go
// renderAgentStream 中新增行缓冲
var lineBuf strings.Builder
for evt := range stream {
    case agent.EventTextDelta:
        lineBuf.WriteString(evt.Text)
        // 按行处理
        for {
            s := lineBuf.String()
            idx := strings.IndexByte(s, '\n')
            if idx < 0 {
                break
            }
            line := s[:idx+1]
            lineBuf.Reset()
            lineBuf.WriteString(s[idx+1:])
            fmt.Fprint(ui.out, RenderMarkdown(line))
        }
}
// 流结束后，输出剩余缓冲
if lineBuf.Len() > 0 {
    fmt.Fprint(ui.out, RenderMarkdown(lineBuf.String()))
}
```

**修改文件**：`tui/chat.go`

---

### 3. 去掉 `[INFO]` 日志输出 —— 按级别分文件存储

**问题分析**：用户的截图显示终端出现了：
```
[INFO] [my-openai] my-openai: 发起 Chat 请求 provider.go:135 model=qwen3.5-4b-mlx url=http://localhost:1234/v1/chat/completions
```

这说明日志没有完全被重定向到文件。检查 `initLogging()`：

```go
func initLogging() {
    lg.SetPath("logs", lg.LevelInfo,
        lg.NewLogNamePattern().Module().Char("_").Date("2006-01-02"),
    )
}
```

`lg.SetPath` 已经将 `defaultLogger` 替换为 FileWriter。但问题是：**各 provider 使用 `lg.Module("my-openai")` 获取的是包级别的 `defaultLogger.Module("my-openai")`**，而 `lg.SetPath` 替换了 `defaultLogger`，所以理论上应该已经重定向了。

**真正原因**：检查 `lg.SetPath` 的实现（`logger.go:267-322`），它调用了 `SetDefault(New(fw))`，将 defaultLogger 替换为 New(fw)。但 `Module()` 方法返回的子 Logger 使用的是 `l.writer`（第49行），而不是 defaultLogger。子 Logger 的 writer 是在 `Module()` 调用时从父 Logger 继承的。

**问题根源**：provider 在 `init()` 或包级别初始化时通过 `var logger = lg.Module("my-openai")` 获取了 Logger，此时 defaultLogger 是 ConsoleWriter（输出到 stdout）。当 `initLogging()` 调用 `SetPath()` 替换 defaultLogger 时，**已经创建的 module Logger 不会自动更新 writer**。

**方案**：

修改 `lg.SetPath` 或提供机制让已存在的 module Logger 更新 writer。

最简单方案：**给 `SetPath` 增加能力，遍历更新所有已注册的 module Logger**。但 Go 没有这样的注册机制。

**更简单方案**：在 `cmd/pg/main.go` 的 `initLogging()` 中，先设置日志，然后 provider 的 logger 延迟初始化（不在包级别用 `var` 初始化）。

但这需要改动 provider 代码。

**最佳方案**：改造 `lg` 包，使 `Module()` 返回的 Logger 的 writer 是**间接引用**（通过指针），这样当 `SetPath` 更新 defaultLogger 的 writer 时，子 Logger 自动跟随。

实际上更简单：**让 `Module()` 不固定 writer，而是每次 log 时动态获取**。但这样改动较大。

**最小改动方案**：`lg.SetPath` 除了替换 `defaultLogger`，同时更新所有 provider 包中的 Logger。但需要 provider 暴露 SetLogger。

**最终选定方案**：在 `lg` 包中增加一个 `RefreshModuleWriters` 机制，或者在 `SetPath` 后让 `cmd/pg` 手动重新设置各 provider 的 logger。更优雅的方案是**修改 `Logger` 结构，使 writer 为指针引用**。

**确定方案**：改造 `lg.Logger`，使 `writer` 通过 `*Writer`（指针的指针）引用，这样 defaultLogger 的 writer 被替换时，所有 module Logger 自动跟随。

```go
type Logger struct {
    module     string
    writer     *Writer    // 改为指针，跟随父 Logger 的 writer 变化
    fields     Fields
    callerSkip int
}
```

但这也有问题，Go 的 interface 本身就是引用类型。

**最终最终方案（最简）**：在 `lg.SetPath` 中，替换 defaultLogger 后，将新的 FileWriter 同时暴露为一个全局变量，让各 provider 可以通过 `lg.GetDefaultWriter()` 重新设置。

实际上，让我们重新审视问题。当前 `lg.SetPath` 的实现：

```go
func SetPath(dir string, level Level, pattern *LogNamePattern, opts ...LogOption) error {
    // ...
    fw, err := NewFileWriterWithLogName(dir, level, logName)
    // ...
    SetDefault(New(fw))    // 替换 defaultLogger
    SetFrameWriter(fw)
    // ...
}
```

问题：`lg.Module("my-openai")` 调用时（在包初始化阶段），defaultLogger 还是 ConsoleWriter。之后 `initLogging()` 替换了 defaultLogger，但 `my-openai` 的 logger 已经创建，它的 writer 指向旧的 ConsoleWriter。

**最简修复**：provider 包不在包级别创建 logger，而是在首次使用时懒初始化，或者提供 `SetLogger` 方法。

但考虑到改动最小化：

**最终方案**：在 `lg` 包中增加一个简单的机制——让 `Module()` 创建的 Logger 在每次 `log()` 时从 defaultLogger 动态获取 writer。

```go
// Module 返回的子 Logger 不固定 writer，写日志时动态获取
func (l *Logger) Module(module string) *Logger {
    return &Logger{
        module:     module,
        writer:     nil,  // nil 表示从 defaultLogger 继承
        fields:     l.fields,
        callerSkip: l.callerSkip,
    }
}
```

但这样改动较大。

**真正最简方案**：在 `cmd/pg/main.go` 的 `initLogging()` 最后，让各 provider 的包级别 logger 重新绑定。

provider 暴露 `SetLogger`：

```go
// ai/providers/my-openai/provider.go
func SetLogger(l *lg.Logger) {
    logger = l
}
```

然后在 `initLogging()` 中调用。但 `initLogging` 在 `cmd/pg` 中，不应该知道 provider 细节。

**终极最简方案**：**不修改 provider，而是修改 `lg.SetPath`，让它在替换 defaultLogger 后，用一个全局变量保存新的 defaultLogger，然后 Module() 在 log 时始终使用最新的 defaultLogger 的 writer。**

具体实现：在 `lg/logger.go` 中：

```go
// Module 创建的子 Logger 的 writer 指向 defaultLogger 的 writer 地址（指针）
func Module(module string) *Logger {
    return &Logger{
        module:     module,
        writerRef:  &defaultLogger.writer,  // 保存 writer 的指针
        fields:     defaultLogger.fields,
        callerSkip: 3, // Module → Debug/Info... → caller
    }
}
```

这样当 defaultLogger.writer 被 `SetPath` 替换时，子 Logger 自动跟随。

**选定方案**：改造 `lg.Logger`，增加 `writerRef` 字段用于 module Logger 的延迟绑定。

```go
type Logger struct {
    module     string
    writer     Writer    // 直接绑定的 writer（用于 New() 创建的 Logger）
    writerRef  *Writer   // 间接引用（用于 Module() 创建的子 Logger，跟随父 Logger 变化）
    fields     Fields
    callerSkip int
}

func (l *Logger) getWriter() Writer {
    if l.writerRef != nil {
        return *l.writerRef
    }
    return l.writer
}
```

**修改文件**：
- `lg/logger.go`：增加 `writerRef` 字段和 `getWriter()` 方法，修改 `Module()` 和 `log()`
- `ai/providers/my-openai/provider.go`：将 `logger.Info` 改为 `logger.Debug`（或保持 Info 但确保写入文件）

同时，`initLogging()` 改为按级别分目录：

```go
func initLogging() {
    lg.SetPath("logs", lg.LevelInfo,
        lg.NewLogNamePattern().Module().Char("_").Date("2006-01-02"),
        lg.WithLevelDir(lg.LevelInfo, "info"),
        lg.WithLevelDir(lg.LevelError, "error"),
    )
}
```

这样 INFO 日志写入 `logs/info/pg_YYYY-MM-DD.log`，ERROR 写入 `logs/error/pg_YYYY-MM-DD.log`，同时也会写入 `logs/pg_YYYY-MM-DD.log`（汇总）。

---

### 4. 输入输出框化

**方案**：类似 Claude Code / Codex 风格，用 Unicode 框线字符（`┌─┐│└┘`）绘制输入输出框。

```
┌──────────────────────────────────────────────┐
│ 💬 你好，你来给我分析一下如何学习ai大模型？   │
└──────────────────────────────────────────────┘
┌──────────────────────────────────────────────┐
│ 🤖 你好！作为 Pi Go，我很乐意为你规划...     │
│                                              │
│ 学习 AI 大模型是一个从理解概念到掌握工程...   │
│                                              │
│ ▶ 第一阶段：夯实基础                          │
│   • 编程语言：Python，掌握 NumPy...           │
│   • 线性代数与微积分                          │
│   • 概率统计                                 │
│ ...                                          │
└──────────────────────────────────────────────┘
```

**实现**：
- 添加 `tui/box.go`：实现 `Box` 组件，用 Unicode 框线绘制
- 修改 `tui/chat.go`：`printUserInput` 和 `renderAgentStream` 使用框线包裹

**修改文件**：
- 新建 `tui/box.go`：框线绘制工具
- 修改 `tui/chat.go`：集成框线

**框线宽度**：根据终端宽度自适应，默认 80 列（如果终端更窄则取终端宽度 - 4）。

---

### 5. 欢迎语修正

**修改文件**：
- `tui/chat.go` 第201行：`"Pi Agent"` → `"Pi Go"`
- `cmd/pg/main.go` 第124行：`"Pi Agent"` → `"Pi Go"`（help 输出）
- `cmd/pg/main.go` 第39行：system prompt 中的 `"Pi Agent"` → `"Pi Go"`

---

## 修改文件清单

| 文件 | 修改内容 |
|------|---------|
| `lg/logger.go` | 增加 `writerRef` 字段，`Module()` 使用延迟绑定，`log()` 使用 `getWriter()` |
| `cmd/pg/main.go` | `initLogging()` 按级别分目录；`"Pi Agent"` → `"Pi Go"` |
| `tui/chat.go` | 去掉重复回显；Markdown 渲染集成；框线集成；`"Pi Agent"` → `"Pi Go"` |
| `tui/box.go`（新建） | Unicode 框线绘制工具 |
| `change-log/2026-8-1.md` | 记录变更 |

---

## 开发顺序

1. `lg/logger.go` — 修复 module Logger 的 writer 绑定问题
2. `cmd/pg/main.go` — 日志分级 + 文案修正
3. `tui/box.go` — 框线工具
4. `tui/chat.go` — 去掉回显 + Markdown 渲染 + 框线集成 + 文案修正
5. `change-log/2026-8-1.md` — 变更日志
# TUI 中文输入问题归档

## 问题现象

1. TUI 压根不能输入汉字。
2. 可以输入单个汉字，但一次输入多个汉字时，只处理首个字符，后续字符被截断或丢失。

## 处理方法

1. 不把输入按“单字节按键”理解，改为先按 UTF-8 完整解码。
2. 对中文这类多字节输入，按 rune 逐个插入编辑缓冲区，而不是只消费首字节。
3. 对连续输入保持同一轮处理链路，避免把一串字符拆成只剩第一个字符的片段。
4. 用测试覆盖单字汉字、连续汉字、中文和 ASCII 混输三类场景。

## 结论

这类问题的核心不是“中文字符本身”，而是终端输入在字节、rune、按键事件之间的边界处理错误。修复原则是：输入先完整解码，再逐字符落到编辑缓冲区。
