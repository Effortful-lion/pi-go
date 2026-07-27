# 扩展计划：TUI 层

> 对应设计文档：`docs/tui-layer-design.md`
> 源文件"后续扩展方向（本次不实现）"章节

---

## 已实现

| 模块 | 文件 | 说明 |
|------|------|------|
| 终端底层 | `tui/terminal.go` | Raw Mode、光标控制、ANSI 颜色、EditLine 行编辑 |
| ChatUI | `tui/chat.go` | 交互式对话循环、流式渲染、角色标记 |

---

## 扩展项

### P0 — 历史记录（↑↓ 浏览）

**描述**：按 ↑ ↓ 键浏览历史输入，与 Bash/readline 行为一致。

**为什么需要**：
- 这是终端交互的基本体验。没有历史记录，用户每次都需要重新输入，体验很差
- 在当前单行编辑中嵌入历史浏览是最自然的方案

**实现**：
```go
// terminal.go 中 EditLine 增加历史记录支持
type LineEditor struct {
    history    []string
    historyIdx int  // -1 表示不在浏览历史中
    // ...
}

func (e *LineEditor) HandleKey(key int) {
    switch key {
    case UpKey:
        if e.historyIdx < len(e.history)-1 {
            e.historyIdx++
            e.SetLine(e.history[len(e.history)-1-e.historyIdx])
        }
    case DownKey:
        if e.historyIdx > 0 {
            e.historyIdx--
            e.SetLine(e.history[len(e.history)-1-e.historyIdx])
        } else {
            e.historyIdx = -1
            e.SetLine("")
        }
    // ...
    }
}

func (e *LineEditor) AddHistory(line string) {
    // 去重：连续相同的不重复添加
    if len(e.history) > 0 && e.history[len(e.history)-1] == line {
        return
    }
    e.history = append(e.history, line)
    // 限制历史记录数量
    if len(e.history) > 1000 {
        e.history = e.history[len(e.history)-1000:]
    }
}
```

**影响范围**：
- `tui/terminal.go`：EditLine 增加历史记录能力
- `tui/chat.go`：每次用户输入后调用 `AddHistory`

---

### P0 — 多行输入

**描述**：支持 Alt+Enter（或 Shift+Enter）换行输入多行内容。

**为什么需要**：
- 当前只能输入单行，用户无法粘贴代码片段或多行问题
- 代码类对话特别需要多行输入能力

**实现要点**：
- 方案一（推荐）：Alt+Enter 换行 → 插入换行符到输入行数组 → 重新渲染（类似 QQ/微信的 Shift+Enter 发多行消息）
- 方案二：进入独立的多行编辑模式（`Ctrl+X Ctrl+E` 打开 `$EDITOR`，类似 `git commit`）
- 渲染需处理光标在多行间移动

```go
case AltEnterKey:
    line = append(line[:cursor], append([]rune{'\n'}, line[cursor:]...)...)
    cursor++
```

**影响范围**：
- `tui/terminal.go`：EditLine 支持多行文本数组而非单行
- `tui/chat.go`：无变更，EditLine 返回值自然包含换行

---

### P1 — 语法高亮

**描述**：对代码块进行语法着色。

**为什么需要**：
- AI 编程助手的输出大量包含代码，原生黑白色阅读体验差
- 高亮后代码可读性和美观度大幅提升

**实现思路**：
- 方案一（轻量）：使用 [chroma](https://github.com/alecthomas/chroma) 库做词法分析→ANSI 色码
- 方案二（手工）：自己实现常见语言（Go/Python/JS/Shell）的极简高亮
- 实时性要求：流式场景下，每收到一个 TextDelta 需要重新高亮当前代码块（性能敏感）

**API 设计**：
```go
// tui/highlight.go
type Highlighter interface {
    Highlight(code string, language string) string  // 返回 ANSI 着色后的文本
}

// 注册语言
func RegisterHighlighter(lang string, h Highlighter)
```

**挑战**：
- 流式注入时，中间状态的代码片段是不完整的（如 `func main(`），高亮可能出错
- 需要判断当前是否在代码块内（``` 标记检测）

---

### P1 — 终端 Markdown 渲染

**描述**：将 LLM 输出的 Markdown 格式渲染为终端样式。

**当前状态**：纯文本输出，不处理 Markdown 格式标记。

**需要支持的元素**：
- 标题：`# Heading` → 加粗 + 下划线
- 粗体/斜体：`**bold**` / `*italic*` → ANSI 样式
- 代码（行内）：`` `code` `` → 灰色背景
- 列表：`- item` / `1. item` → 缩进前缀
- 引用：`> quote` → 竖线 + 灰色
- 表格：`| a | b |` → 对齐渲染

**实现思路**：
- 使用 [glamour](https://github.com/charmbracelet/glamour) 库（Charm 出品，专为终端 Markdown 设计）
- 或者自己实现流式友好的简化版（不用等待完整的 Markdown 文档）

---

### P1 — Ctrl+L 清屏

**描述**：按 `Ctrl+L` 清空终端屏幕，类似 Bash 行为。

**实现**：
```go
case CtrlLKey:
    fmt.Print(ClearScreen())
    // 重绘当前输入状态
    renderInput()
```

**影响范围**：`tui/terminal.go`，极小变更

---

### P2 — 文件路径粘贴

**描述**：支持拖放文件/粘贴文件路径到终端，自动识别并确认。

**为什么需要**：
- "帮我修改这个文件" → 用户拖入文件路径 → Agent 读取文件 → 修改
- 减少手动输入路径的错误和麻烦

**实现**：
- 检测输入中的文件路径（以 `/`、`./`、`~/` 开头且文件存在）
- 自动展开并高亮显示

---

### P2 — 输入补全

**描述**：Tab 键触发文件名/路径自动补全。

**为什么需要**：
- 和 Shell 的补全体验一致
- 编程场景下经常需要输入文件路径

---

### P2 — 对话导出

**描述**：支持将当前对话导出为 Markdown 文件。

**实现**：
```go
func (ui *ChatUI) ExportConversation(path string) error
```

保存为 `2024-01-15-pi-agent-conversation.md` 格式。

---

## 优先级总结

| 优先级 | 项目 | 工作量 | 依赖 |
|--------|------|--------|------|
| P0 | 历史记录 ↑↓ | 中 | 无 |
| P0 | 多行输入 Alt+Enter | 中 | 无 |
| P1 | 语法高亮 | 大 | 可能需要 chroma 库 |
| P1 | Markdown 渲染 | 大 | 可能需要 glamour 库 |
| P1 | Ctrl+L 清屏 | 极小 | 无 |
| P2 | 文件路径粘贴 | 小 | 无 |
| P2 | Tab 补全 | 中 | 无 |
| P2 | 对话导出 | 小 | 无 |
