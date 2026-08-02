//go:build !windows

package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Effortful-lion/pi-go/agent"
	"github.com/Effortful-lion/pi-go/ai"
	"github.com/Effortful-lion/pi-go/emoji"
	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// Mock Agent（用于测试）
// ============================================================================

// mockAgent 按调用顺序返回预设的 agent.Event 序列。
type mockAgent struct {
	calls   [][]agent.Event
	callNum int
}

func (m *mockAgent) Run(ctx context.Context, userInput string) agent.Stream {
	ch := make(chan agent.Event, 16)
	go func() {
		defer close(ch)
		if m.callNum >= len(m.calls) {
			ch <- agent.Event{Type: agent.EventError, Err: errors.New("no more mock calls")}
			return
		}
		events := m.calls[m.callNum]
		m.callNum++
		for _, e := range events {
			select {
			case <-ctx.Done():
				return
			case ch <- e:
			}
		}
	}()
	return ch
}

// ============================================================================
// 辅助函数
// ============================================================================

// makeTextAgentEvents 创建纯文本的 Agent 事件序列。
func makeTextAgentEvents(text string) []agent.Event {
	return []agent.Event{
		{Type: agent.EventStepStart, Step: 1},
		{Type: agent.EventTextDelta, Text: text, Step: 1},
		{Type: agent.EventStepEnd, Step: 1, Usage: &ai.Usage{TotalTokens: 100}},
		{Type: agent.EventDone, Step: 1},
	}
}

// makeToolCallAgentEvents 创建包含工具调用的 Agent 事件序列（两轮）。
func makeToolCallAgentEvents(toolName, toolResult, finalText string) []agent.Event {
	return []agent.Event{
		// 第一轮：工具调用
		{Type: agent.EventStepStart, Step: 1},
		{Type: agent.EventToolCall, Step: 1, ToolCall: &ai.ToolCall{Name: toolName}},
		{Type: agent.EventToolResult, Step: 1, ToolResult: toolResult},
		{Type: agent.EventStepEnd, Step: 1, Usage: &ai.Usage{TotalTokens: 500}},
		// 第二轮：最终回答
		{Type: agent.EventStepStart, Step: 2},
		{Type: agent.EventTextDelta, Text: finalText, Step: 2},
		{Type: agent.EventStepEnd, Step: 2, Usage: &ai.Usage{TotalTokens: 300}},
		{Type: agent.EventDone, Step: 2},
	}
}

// ============================================================================
// 测试
// ============================================================================

func TestChatModel_HandleStream_TextResponse(t *testing.T) {
	ma := &mockAgent{
		calls: [][]agent.Event{
			makeTextAgentEvents("Hello, this is a test response."),
		},
	}

	resolver := emoji.DefaultResolver
	m := NewChatModel(&agent.Agent{}, resolver, nil)
	stream := ma.Run(context.Background(), "Hello")

	// 模拟流式消费（通过 waitForStream Cmd 逐条处理）
	evt, ok := <-stream
	if !ok {
		t.Fatal("expected event, got closed channel")
	}
	msg := streamMsg(evt)

	// 第一个事件应该是 StepStart，进入流模式
	m.running = true
	m.firstLine = true
	m.stream = stream

	// 处理事件直到 Done
	for {
		nextModel, cmd := m.handleStream(msg, stream)
		m = nextModel.(*ChatModel)
		if cmd == nil {
			break
		}
		// 执行 cmd 获取下一条消息
		nextMsg := cmd()
		if _, isDone := nextMsg.(streamDoneMsg); isDone {
			break
		}
		if sm, ok := nextMsg.(streamMsg); ok {
			msg = sm
		} else {
			break
		}
	}

	output := m.output.String()
	if !strings.Contains(output, "Hello") {
		t.Error("output should contain the response text")
	}
}

func TestChatModel_HandleStream_ToolCallResponse(t *testing.T) {
	ma := &mockAgent{
		calls: [][]agent.Event{
			makeToolCallAgentEvents("get_weather", `{"city":"Beijing","temp":25}`, "Beijing is 25°C"),
		},
	}

	resolver := emoji.DefaultResolver
	m := NewChatModel(&agent.Agent{}, resolver, nil)
	stream := ma.Run(context.Background(), "What's the weather?")

	evt, ok := <-stream
	if !ok {
		t.Fatal("expected event, got closed channel")
	}
	msg := streamMsg(evt)

	m.running = true
	m.firstLine = true
	m.stream = stream

	for {
		nextModel, cmd := m.handleStream(msg, stream)
		m = nextModel.(*ChatModel)
		if cmd == nil {
			break
		}
		nextMsg := cmd()
		if _, isDone := nextMsg.(streamDoneMsg); isDone {
			break
		}
		if sm, ok := nextMsg.(streamMsg); ok {
			msg = sm
		} else {
			break
		}
	}

	output := m.output.String()
	if !strings.Contains(output, "get_weather") {
		t.Error("output should contain tool call name")
	}
	if !strings.Contains(output, "Beijing is 25°C") {
		t.Error("output should contain final response")
	}
}

func TestChatModel_HandleStream_ErrorResponse(t *testing.T) {
	ma := &mockAgent{
		calls: [][]agent.Event{
			{
				{Type: agent.EventStepStart, Step: 1},
				{Type: agent.EventError, Step: 1, Err: errors.New("connection failed")},
			},
		},
	}

	resolver := emoji.DefaultResolver
	m := NewChatModel(&agent.Agent{}, resolver, nil)
	stream := ma.Run(context.Background(), "error")

	evt, ok := <-stream
	if !ok {
		t.Fatal("expected event, got closed channel")
	}
	msg := streamMsg(evt)

	m.running = true
	m.firstLine = true
	m.stream = stream

	for {
		nextModel, cmd := m.handleStream(msg, stream)
		m = nextModel.(*ChatModel)
		if cmd == nil {
			break
		}
		nextMsg := cmd()
		if _, isDone := nextMsg.(streamDoneMsg); isDone {
			break
		}
		if sm, ok := nextMsg.(streamMsg); ok {
			msg = sm
		} else {
			break
		}
	}

	output := m.output.String()
	if !strings.Contains(output, "connection failed") {
		t.Errorf("output should contain error message, got: %q", output)
	}
}

func TestChatModel_HandleStream_UsageDisplay(t *testing.T) {
	ma := &mockAgent{
		calls: [][]agent.Event{
			makeTextAgentEvents("Short reply."),
		},
	}

	resolver := emoji.DefaultResolver
	m := NewChatModel(&agent.Agent{}, resolver, nil)
	stream := ma.Run(context.Background(), "hi")

	evt, ok := <-stream
	if !ok {
		t.Fatal("expected event, got closed channel")
	}
	msg := streamMsg(evt)

	m.running = true
	m.firstLine = true
	m.stream = stream

	for {
		nextModel, cmd := m.handleStream(msg, stream)
		m = nextModel.(*ChatModel)
		if cmd == nil {
			break
		}
		nextMsg := cmd()
		if _, isDone := nextMsg.(streamDoneMsg); isDone {
			break
		}
		if sm, ok := nextMsg.(streamMsg); ok {
			msg = sm
		} else {
			break
		}
	}

	output := m.output.String()
	// 100 tokens < 1000，应该显示为 "100 tokens"
	if !strings.Contains(output, "100") {
		t.Errorf("output should contain token count, got: %q", output)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 100, "short"},
		{"hello world", 5, "hello…"},
		{"abc", 3, "abc"},
		{"abcd", 4, "abcd"},
		{"abcde", 4, "abcd…"},
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.want)
		}
	}
}

func TestEscSubmitStrategy_ShouldInsertNewline(t *testing.T) {
	s := EscSubmitStrategy{}

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}

	// Enter 应该插入换行
	if !s.ShouldInsertNewline(enterMsg, false) {
		t.Error("Enter should insert newline")
	}
	// Esc 不应该插入换行
	if s.ShouldInsertNewline(escMsg, false) {
		t.Error("Esc should not insert newline")
	}
	// running 时不插入换行
	if s.ShouldInsertNewline(enterMsg, true) {
		t.Error("Enter should not insert newline when running")
	}
}

func TestEscSubmitStrategy_ShouldSubmit(t *testing.T) {
	s := EscSubmitStrategy{}

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}

	// Esc 应该提交
	if !s.ShouldSubmit(escMsg, "hello", false) {
		t.Error("Esc should submit")
	}
	// Enter 不应该提交
	if s.ShouldSubmit(enterMsg, "hello", false) {
		t.Error("Enter should not submit")
	}
	// running 时不提交
	if s.ShouldSubmit(escMsg, "hello", true) {
		t.Error("Esc should not submit when running")
	}
}

func TestEscSubmitStrategy_HelpText(t *testing.T) {
	s := EscSubmitStrategy{}
	text := s.HelpText()
	if text == "" {
		t.Error("HelpText should not be empty")
	}
}

func TestChatModel_SubmitViaStrategy(t *testing.T) {
	resolver := emoji.DefaultResolver
	m := NewChatModel(&agent.Agent{}, resolver, nil)

	m.textarea.SetValue("hello")

	// 模拟 Esc 按键（通过 handleKeyMsg）
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := m.handleKeyMsg(escMsg)

	// 提交后应该有 cmd（waitForStream）
	if cmd == nil {
		t.Error("submit should return a cmd")
	}

	// 提交后 textarea 应该被清空
	if m.textarea.Value() != "" {
		t.Error("textarea should be reset after submit")
	}
}
