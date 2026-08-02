//go:build !windows

package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Effortful-lion/pi-go/agent"
	"github.com/Effortful-lion/pi-go/emoji"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ChatModel Bubbletea Model，管理终端 UI 状态。
type ChatModel struct {
	agent         *agent.Agent
	emojiResolver *emoji.Resolver
	textarea      textarea.Model
	inputStrategy InputStrategy // 输入交互策略（可插拔）
	ctx           context.Context

	// 对话输出（累积所有渲染后的 ANSI 字符串）
	output strings.Builder

	// 流式渲染状态
	lineBuf   strings.Builder       // 行缓冲
	firstLine bool                  // 是否在等待首行（用于加 bot 前缀）
	stream    agent.Stream          // 当前正在消费的 Agent 流（nil 表示未在运行）
	mdLine    *MarkdownLineRenderer // 有状态逐行 Markdown 渲染器

	// 历史记录
	history    []string // 提交过的消息
	histIdx    int      // 历史浏览位置（-1 表示新输入）
	savedInput string   // 浏览历史前保存的当前输入

	// 状态
	running  bool // Agent 是否正在运行
	width    int  // 终端宽度
	height   int  // 终端高度
	quitting bool
}

// NewChatModel 创建 ChatModel。
// strategy 为输入交互策略，传 nil 使用默认的 EscSubmitStrategy。
func NewChatModel(ag *agent.Agent, resolver *emoji.Resolver, strategy InputStrategy) *ChatModel {
	if strategy == nil {
		strategy = EscSubmitStrategy{}
	}

	ta := textarea.New()
	ta.Prompt = "> "
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // 不限制
	ta.SetHeight(10) // 多行时阴影能覆盖足够行数

	// 自定义样式：光标行有背景色阴影
	focusStyle, blurStyle := textarea.DefaultStyles()
	cursorLineStyle := lipgloss.NewStyle().Background(lipgloss.Color("236"))
	focusStyle.CursorLine = cursorLineStyle
	blurStyle.CursorLine = cursorLineStyle
	ta.FocusedStyle = focusStyle
	ta.BlurredStyle = blurStyle

	ta.Focus()

	return &ChatModel{
		agent:         ag,
		emojiResolver: resolver,
		textarea:      ta,
		inputStrategy: strategy,
		ctx:           context.Background(),
		histIdx:       -1,
		firstLine:     true,
		mdLine:        NewMarkdownLineRenderer(),
	}
}

// Init 初始化命令。
func (m *ChatModel) Init() tea.Cmd {
	return tea.SetWindowTitle("Pi-Go Agent")
}

// Update 处理消息。
func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		if msg.Paste {
			return m.handlePaste(msg)
		}
		return m.handleKeyMsg(msg)

	case streamMsg:
		return m.handleStream(msg, m.stream)

	case streamDoneMsg:
		m.running = false
		m.stream = nil
		m.textarea.Focus()
		m.textarea.Reset()
		return m, nil
	}

	// 默认：透传给 textarea
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// View 渲染界面。
func (m *ChatModel) View() string {
	var b strings.Builder

	// 对话历史输出
	if m.output.Len() > 0 {
		b.WriteString(m.output.String())
	}

	// 输入区域（running 时隐藏输入框，避免干扰流式输出）
	if !m.running && !m.quitting {
		if m.output.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(m.textarea.View())
		b.WriteString("\n")
		b.WriteString(m.inputStrategy.HelpText())
	}

	return b.String()
}

// handleKeyMsg 处理按键。
func (m *ChatModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 通过 InputStrategy 判断是否提交
	if m.inputStrategy.ShouldSubmit(msg, m.textarea.Value(), m.running) {
		return m.handleSubmit(m.textarea.Value())
	}

	// 通过 InputStrategy 判断是否插入换行
	if m.inputStrategy.ShouldInsertNewline(msg, m.running) {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(tea.KeyMsg{Type: tea.KeyEnter})
		return m, cmd
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyCtrlD:
		if m.textarea.Value() == "" {
			m.quitting = true
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd

	case tea.KeyCtrlL:
		m.output.Reset()
		return m, nil

	case tea.KeyUp:
		return m.handleHistoryUp()

	case tea.KeyDown:
		return m.handleHistoryDown()

	default:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}
}

// handlePaste 处理 Bracketed Paste：直接写入 textarea，不触发 Enter 逻辑。
func (m *ChatModel) handlePaste(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// handleSubmit 提交消息给 Agent。
func (m *ChatModel) handleSubmit(text string) (tea.Model, tea.Cmd) {
	text = strings.TrimSpace(text)
	if text == "" {
		return m, nil
	}

	// 斜杠命令
	if strings.HasPrefix(text, "/") {
		return m.handleCommand(text)
	}

	// 添加到历史
	m.addHistory(text)

	// 在输出中显示用户输入
	userPrefix := m.emojiResolver.Resolve(emojiSlotUser)
	fmt.Fprintf(&m.output, "\n%s %s\n", Green(userPrefix), text)

	// 清空 textarea 并启动 Agent
	m.textarea.Reset()
	m.running = true
	m.firstLine = true
	m.lineBuf.Reset()
	m.mdLine.Reset()

	m.stream = m.agent.Run(m.ctx, text)
	return m, waitForStream(m.stream)
}

// handleCommand 处理斜杠命令。
func (m *ChatModel) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}

	switch parts[0] {
	case "/clear":
		m.output.Reset()
		m.textarea.Reset()
	case "/reset":
		m.agent.Reset()
		m.output.Reset()
		m.textarea.Reset()
		fmt.Fprintln(&m.output, Dim("对话已重置"))
	case "/help":
		fmt.Fprintln(&m.output)
		fmt.Fprintln(&m.output, Bold("可用命令:"))
		fmt.Fprintln(&m.output, Dim("  /clear      清屏"))
		fmt.Fprintln(&m.output, Dim("  /reset      重置对话"))
		fmt.Fprintln(&m.output, Dim("  /export     导出对话到文件"))
		fmt.Fprintln(&m.output, Dim("  /help       显示帮助"))
		fmt.Fprintln(&m.output)
		fmt.Fprintln(&m.output, Bold("快捷键:"))
		fmt.Fprintln(&m.output, Dim("  Enter       换行"))
		fmt.Fprintln(&m.output, Dim("  Esc         发送消息"))
		fmt.Fprintln(&m.output, Dim("  ↑↓          浏览历史记录"))
		fmt.Fprintln(&m.output, Dim("  Ctrl+L      清屏"))
		fmt.Fprintln(&m.output, Dim("  Ctrl+C/Ctrl+D 退出"))
	default:
		fmt.Fprintln(&m.output, Dim(fmt.Sprintf("未知命令: %s (使用 /help 查看命令列表)", parts[0])))
	}

	m.addHistory(input)
	m.textarea.Reset()
	return m, nil
}

// handleHistoryUp 浏览上一条历史。
func (m *ChatModel) handleHistoryUp() (tea.Model, tea.Cmd) {
	if len(m.history) == 0 {
		return m, nil
	}

	// 首次进入历史模式时保存当前输入
	if m.histIdx == -1 {
		m.savedInput = m.textarea.Value()
		m.histIdx = len(m.history) - 1
	} else if m.histIdx > 0 {
		m.histIdx--
	}

	m.textarea.Reset()
	m.textarea.SetValue(m.history[m.histIdx])
	// 光标移到末尾
	m.textarea.CursorEnd()
	return m, nil
}

// handleHistoryDown 浏览下一条历史。
func (m *ChatModel) handleHistoryDown() (tea.Model, tea.Cmd) {
	if m.histIdx == -1 {
		return m, nil
	}

	m.histIdx++
	if m.histIdx >= len(m.history) {
		// 回到新输入
		m.histIdx = -1
		m.textarea.Reset()
		m.textarea.SetValue(m.savedInput)
		m.textarea.CursorEnd()
		m.savedInput = ""
	} else {
		m.textarea.Reset()
		m.textarea.SetValue(m.history[m.histIdx])
		m.textarea.CursorEnd()
	}
	return m, nil
}

// addHistory 添加历史记录（自动去重、裁剪）。
func (m *ChatModel) addHistory(entry string) {
	if entry == "" {
		return
	}
	// 去重
	for i, h := range m.history {
		if h == entry {
			m.history = append(m.history[:i], m.history[i+1:]...)
			break
		}
	}
	m.history = append(m.history, entry)
	if len(m.history) > maxHistory {
		m.history = m.history[len(m.history)-maxHistory:]
	}
	m.histIdx = -1
	m.savedInput = ""
}
