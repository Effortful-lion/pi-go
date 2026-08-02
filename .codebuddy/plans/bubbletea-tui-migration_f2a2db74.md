---
name: bubbletea-tui-migration
overview: 将 TUI 层从自研 raw mode + LineEditor 迁移到 Bubbletea 框架，用 Bubbles/Textarea 实现多行输入，Double Enter 提交 + Bracketed Paste 防粘贴误触发。保留现有的 MarkdownLine、颜色样式等渲染工具函数。
todos:
  - id: add-deps
    content: 引入 Bubbletea/Bubbles/Lipgloss 依赖，更新 go.mod
    status: completed
  - id: create-model
    content: 新建 tui/model.go：定义 ChatModel 结构体，实现 Init/Update/View 方法，处理 tea.WindowSizeMsg 和 tea.KeyMsg 路由
    status: completed
    dependencies:
      - add-deps
  - id: create-input
    content: 新建 tui/input.go：实现 handleEnter（Double Enter 逻辑）、handlePaste（Bracketed Paste 防误触）、handleSubmit（斜杠命令分发 + Agent 调用）和历史浏览
    status: completed
    dependencies:
      - create-model
  - id: create-stream
    content: 新建 tui/stream.go：将 agent.Stream 适配为 Bubbletea Cmd，实现流式事件消费和输出累积
    status: completed
    dependencies:
      - create-model
  - id: refactor-chat
    content: 重构 tui/chat.go：ChatUI.Run() 改为创建 tea.Program 并启动，保留 ExportConversation/printHelp/printWelcome/printGoodbye/truncate
    status: completed
    dependencies:
      - create-input
      - create-stream
  - id: clean-terminal
    content: 清理 tui/terminal.go：删除 raw mode/LineEditor/光标控制/按键读取代码（约700行），仅保留颜色样式函数 + ClearScreen + esc 常量
    status: completed
    dependencies:
      - refactor-chat
  - id: update-tests
    content: 更新 tui/terminal_test.go（删除 LineEditor 相关测试，保留样式测试）和 tui/chat_test.go（适配新 ChatUI API）
    status: completed
    dependencies:
      - clean-terminal
      - refactor-chat
  - id: update-changelog
    content: 更新 change-log/2026-8-01.md 记录 Bubbletea 迁移变更
    status: completed
    dependencies:
      - update-tests
---

## 产品概述

将 pi-go 的终端 UI 层从自研 raw mode + LineEditor 迁移到 Go 生态最成熟的 TUI 框架 Bubbletea + Bubbles/Textarea，实现 100% 跨平台的多行文本输入体验。

## 核心功能

- **Double Enter 提交**：Enter 默认插入换行，光标在空行时再次按 Enter 触发消息发送
- **Bracketed Paste 防误触**：粘贴大段文本时不会因文本中的连续空行导致误提交
- **多行文本编辑**：通过 Bubbles/Textarea 组件支持自由换行、光标移动、文本选择
- **Agent 流式集成**：Agent 事件流通过 Bubbletea Cmd 异步投递，保持流式渲染
- **斜杠命令**：/clear、/reset、/export、/help 命令保留
- **历史记录**：提交的消息保留历史，支持上下键浏览
- **现有渲染层复用**：MarkdownLine、RenderMarkdown、Highlight、ANSI 颜色样式、Box 框线组件全部保留
- **Emoji 主题系统**：保留 emoji 解析器集成

## 技术栈

| 组件 | 包 | 用途 |
| --- | --- | --- |
| TUI 框架 | `github.com/charmbracelet/bubbletea` | Model/Update/View 消息循环，自动管理 raw mode、光标、渲染 |
| 多行输入 | `github.com/charmbracelet/bubbles/textarea` | 多行文本缓冲区、光标移动、文本选区 |
| 样式工具 | `github.com/charmbracelet/lipgloss` | Bubbletea 生态的标准样式库（可选，渐进引入） |
| 现有渲染 | `tui/markdown.go` + `tui/highlight.go` | 保留所有 Markdown 渲染、语法高亮函数 |
| 现有样式 | `tui/terminal.go` (颜色部分) | 保留 StyleText、Bold、Dim、Red、Green 等所有 ANSI 颜色函数 |


## 实现方案

### 整体策略

将 `tui/chat.go` 中的 `ChatUI` 从直接操作 stdin/stdout 的阻塞循环改造为 Bubbletea Model。`ChatUI.Run()` 不再手动进入 raw mode + 循环 ReadLine，而是创建 `tea.Program` 并启动 Bubbletea 消息循环。

**关键决策**：

1. ChatUI 保留为对外 API（`NewChatUI` + `Run`），内部实现完全替换为 Bubbletea
2. Agent Stream 通过 `tea.Cmd` 异步投递，避免阻塞 Update 循环
3. 现有渲染函数（MarkdownLine 等）生成的 ANSI 字符串直接作为 Bubbletea View 的输出
4. 颜色样式函数完全保留，在 View 中拼装输出字符串

### Double Enter 实现

在 `Update` 方法中拦截 `tea.KeyEnter`：

- 检查 textarea 当前值的末尾：如果最后两个字符都是 `\n`（即末尾有两个空行），且文本非空 → 触发提交
- 否则 → 将 Enter 透传给 textarea 插入换行

```
Enter 按下 → 检查 textarea.Value()
  ├── 末尾为 "\n\n" 且 TrimSpace 非空 → 提取文本、清空 textarea、触发 submit
  └── 其他情况 → textarea.Update(tea.KeyMsg{Type: tea.KeyEnter})
```

### Bracketed Paste 防误触

在 `Update` 中拦截 `tea.PasteMsg`：

- 直接将粘贴内容写入 textarea（调用 `textarea.Update(msg)`）
- 跳过 `handleEnter` 逻辑，粘贴内容中的空行不会触发提交判断

### Agent Stream 适配

Agent 的 `Run()` 返回 `agent.Stream`（channel），需要转换为 Bubbletea Cmd：

```
type streamMsg agent.Event

func waitForStream(stream agent.Stream) tea.Cmd {
    return func() tea.Msg {
        evt, ok := <-stream
        if !ok {
            return streamDoneMsg{}
        }
        return streamMsg(evt)
    }
}
```

在 Update 中收到 `streamMsg` 时：

- 累积文本到行缓冲
- 遇到 `\n` 时追加到输出缓冲区
- `EventToolCall`/`EventToolResult`/`EventStepEnd`/`EventError` 直接追加渲染后的文本
- `EventDone` 或 channel 关闭时回到输入状态

### 目录结构

```
tui/
├── model.go          # [NEW] Bubbletea Model 定义 + Init/Update/View
├── input.go          # [NEW] handleEnter + handlePaste + handleSubmit + 命令处理
├── chat.go           # [MODIFY] ChatUI 简化为 Program 封装，保留 ExportConversation/printHelp
├── terminal.go       # [MODIFY] 删除 raw mode/LineEditor/光标控制，仅保留颜色样式 + ClearScreen + esc 常量
├── terminal_test.go  # [MODIFY] 删除 LineEditor 相关测试，保留样式测试
├── chat_test.go      # [MODIFY] 适配新的 ChatUI API
├── markdown.go       # [KEEP] 完全保留
├── highlight.go      # [KEEP] 完全保留
├── box.go            # [KEEP] 完全保留
└── stream.go         # [NEW] Agent Stream → Bubbletea Cmd 适配层
```

## 实现细节

### Model 状态设计

```
type ChatModel struct {
    agent         *agent.Agent
    emojiResolver *emoji.Resolver
    textarea      textarea.Model
    output        strings.Builder  // 对话历史 + AI 回复（渲染后的 ANSI 字符串）
    lineBuf       strings.Builder  // 流式渲染行缓冲
    firstLine     bool             // 流式渲染首行标记
    history       []string         // 输入历史
    histIdx       int              // 历史浏览位置（-1 表示新输入）
    savedInput    string           // 浏览历史前保存的当前输入
    running       bool             // Agent 是否正在运行
    width         int              // 终端宽度
    height        int              // 终端高度
}
```

### 流式渲染输出累积

由于 Bubbletea 的 View 每次返回完整画面，而 Agent 是流式响应的，需要：

1. `output` (strings.Builder) 累积所有渲染后的输出
2. 每次 `streamMsg` 到来时追加到 `output` 并返回新的 View
3. View 中输出 = 欢迎信息 + output.String() + textarea.View()

### 终端尺寸处理

在 `Init` 中通过 `tea.WindowSizeMsg` 获取初始尺寸，并在 `Update` 中响应 `tea.WindowSizeMsg` 更新 `width`/`height`。

### 命令处理

在 `handleSubmit` 中检查提交文本：

- `/clear` → 清空 output builder
- `/reset` → 调用 agent.Reset()，清空 output
- `/export` → 调用 ExportConversation，追加提示文本到 output
- `/help` → 追加帮助文本到 output
- 普通文本 → 调用 agent.Run()，切换到 running 状态

### 需要删除的代码（terminal.go）

以下代码将被完全删除（约700行）：

- `EnterRawMode()` 函数体（约45行）
- `ioctlReadTermios`/`ioctlWriteTermios` 常量
- `CursorUp/Down/Back/Forward/MoveTo/Hide/Show` 系列函数（约55行）
- `ClearLine`/`ClearLineFromCursor`/`LineStart` 函数
- `keyEnter/keyCtrlC/keyCtrlD/keyCtrlL/keyBackspace/keyDelete/keyTab/maxHistory` 常量
- `LineEditor` 结构体及其所有方法（ReadLine、AddHistory、historyPrev/Next、handleTab、renderAll、fullRender、diffRender、diffRange、saveState、renderLine、pathPrefixAt、listCompletions、splitDirPrefix、commonPrefix 等，约400行）
- `joinLines`/`splitLines`/`copyLines`/`cloneRunes`/`runesEqual` 辅助函数
- `isPrintable`/`utf8ByteLen` 函数
- `stdinFD` 常量、`keySeq` 类型
- `readKey`/`readEscapeSeq`/`trimCSI`/`parseKittyKey` 函数（约120行）
- `EditLine` 兼容函数

### 需要保留的代码（terminal.go）

- `esc` 常量
- `StyleText()` 函数 + `Style` 结构体 + `Color` 类型 + `Color*` 常量
- `Bold/Dim/Italic/Underline/Red/Green/Blue/Cyan/Gray/Yellow/Magenta` 便捷函数
- `StripANSI()` 函数
- `ClearScreen()` 函数（保留给命令处理使用）

### 性能注意事项

- `output` 使用 `strings.Builder` 高效追加，避免字符串拼接
- 流式渲染仅在收到 `streamMsg` 时更新 View，不额外触发渲染
- Bubbletea 自带 60fps 限制和差异渲染，无需手动实现

### 兼容性

- 对外 API 不变：`NewChatUI(agent, opts...)` + `ui.Run(ctx)` 签名保持一致
- `cmd/pg/chat.go` 无需修改
- 现有测试中样式相关测试保留，LineEditor 相关测试删除