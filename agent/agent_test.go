package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/Effortful-lion/pi-go/ai"
	"github.com/Effortful-lion/pi-go/tool"
)

// ============================================================================
// Mock Provider
// ============================================================================

// mockProvider 按调用顺序返回预设的 ai.Event 序列。
// 每次 Chat 调用消耗 calls 中的一个序列（FIFO）。
type mockProvider struct {
	calls    [][]ai.Event // 第 i 次 Chat 调用返回的事件序列
	callNum  int
}

func (m *mockProvider) ID() string                { return "mock" }
func (m *mockProvider) Name() string              { return "Mock Provider" }
func (m *mockProvider) Models() []ai.ModelInfo    { return []ai.ModelInfo{{ID: "mock-model", Name: "Mock Model", ProviderID: "mock"}} }
func (m *mockProvider) Model(modelID string) (ai.Model, error) {
	if modelID == "mock-model" {
		return &mockModel{p: m, modelID: modelID}, nil
	}
	return nil, errors.New("unknown model")
}

func (m *mockProvider) Chat(ctx context.Context, modelID string, context ai.Context) ai.Stream {
	ch := make(chan ai.Event, 16)
	go func() {
		defer close(ch)
		if m.callNum >= len(m.calls) {
			ch <- ai.Event{Type: ai.EventError, Err: errors.New("no more mock calls")}
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

// mockModel 简单的 mock model，绑定到 mockProvider。
type mockModel struct {
	p       *mockProvider
	modelID string
}

func (m *mockModel) Info() ai.ModelInfo {
	return ai.ModelInfo{ID: m.modelID, Name: "Mock Model", ProviderID: "mock"}
}

func (m *mockModel) Chat(ctx context.Context, context ai.Context) ai.Stream {
	return m.p.Chat(ctx, m.modelID, context)
}

// ============================================================================
// Mock Tool
// ============================================================================

type mockTool struct {
	name       string
	definition ai.ToolDefinition
	result     string
	execErr    error
	executed   bool
	lastArgs   string
}

func (t *mockTool) Name() string                  { return t.name }
func (t *mockTool) Definition() ai.ToolDefinition { return t.definition }

func (t *mockTool) Execute(ctx context.Context, args string) (string, error) {
	t.executed = true
	t.lastArgs = args
	if t.execErr != nil {
		return "", t.execErr
	}
	return t.result, nil
}

// ============================================================================
// 辅助函数
// ============================================================================

// makeTextEvents 创建纯文本 AI 事件序列。
func makeTextEvents(text string) []ai.Event {
	return []ai.Event{
		{Type: ai.EventTextStart, Index: 0},
		{Type: ai.EventTextDelta, Text: text, Index: 0},
		{Type: ai.EventTextEnd, Index: 0},
		{Type: ai.EventDone},
	}
}

// makeToolCallEvents 创建单次工具调用 AI 事件序列。
func makeToolCallEvents(id, name, args string, index int) []ai.Event {
	return []ai.Event{
		{Type: ai.EventToolCallStart, TC: &ai.ToolCall{ID: id, Name: name}, Index: index},
		{Type: ai.EventToolCallDelta, TC: &ai.ToolCall{Arguments: args}, Index: index},
		{Type: ai.EventToolCallEnd, Index: index},
		{Type: ai.EventDone},
	}
}

// collectEvents 从 Stream 收集所有事件，返回列表。
func collectEvents(stream Stream) []Event {
	var events []Event
	for e := range stream {
		events = append(events, e)
	}
	return events
}

// lastEvent 返回事件流中最后一个事件。
func lastEvent(events []Event) Event {
	if len(events) == 0 {
		return Event{}
	}
	return events[len(events)-1]
}

// findEventByType 从事件列表中查找指定类型的第一个事件。
func findEventByType(events []Event, typ EventType) (Event, bool) {
	for _, e := range events {
		if e.Type == typ {
			return e, true
		}
	}
	return Event{}, false
}

// ============================================================================
// 测试
// ============================================================================

func TestAgent_PlainText(t *testing.T) {
	prov := &mockProvider{
		calls: [][]ai.Event{
			makeTextEvents("Hello, world!"),
		},
	}

	ag := New(Config{
		Provider: prov,
		ModelID:  "mock-model",
	})

	events := collectEvents(ag.Run(context.Background(), "Hi"))

	// 最终事件应为 AgentDone
	last := lastEvent(events)
	if last.Type != EventDone {
		t.Fatalf("expected EventDone, got %v", last.Type)
	}

	// 应该有文本增量
	_, found := findEventByType(events, EventTextDelta)
	if !found {
		t.Error("expected at least one EventTextDelta")
	}
}

func TestAgent_SingleToolCall(t *testing.T) {
	mt := &mockTool{
		name:       "get_weather",
		definition: ai.ToolDefinition{Name: "get_weather", Description: "Get weather"},
		result:     `{"city":"Beijing","temp":25}`,
	}

	prov := &mockProvider{
		calls: [][]ai.Event{
			// 第一次调用：LLM 返回工具调用
			makeToolCallEvents("call_1", "get_weather", `{"city":"Beijing"}`, 0),
			// 第二次调用：LLM 在工具结果基础上返回最终文本
			makeTextEvents("Beijing is 25°C today."),
		},
	}

	ag := New(Config{
		Provider: prov,
		ModelID:  "mock-model",
		Tools:    []tool.Tool{mt},
	})

	events := collectEvents(ag.Run(context.Background(), "What's the weather?"))

	// 应该有工具调用事件
	tcEvt, found := findEventByType(events, EventToolCall)
	if !found {
		t.Fatal("expected EventToolCall")
	}
	if tcEvt.ToolCall.Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %q", tcEvt.ToolCall.Name)
	}

	// 应该有工具结果事件
	_, found = findEventByType(events, EventToolResult)
	if !found {
		t.Error("expected EventToolResult")
	}

	// 工具应该被执行了
	if !mt.executed {
		t.Error("mock tool should be executed")
	}

	// 最终事件应为 AgentDone
	last := lastEvent(events)
	if last.Type != EventDone {
		t.Fatalf("expected EventDone, got %v", last.Type)
	}

	// 应该有两次 StepStart/StepEnd（两轮 LLM 调用）
	stepStarts := 0
	for _, e := range events {
		if e.Type == EventStepStart {
			stepStarts++
		}
	}
	if stepStarts != 2 {
		t.Errorf("expected 2 step starts, got %d", stepStarts)
	}
}

func TestAgent_MultiRoundToolCalls(t *testing.T) {
	mt1 := &mockTool{
		name:       "get_city",
		definition: ai.ToolDefinition{Name: "get_city", Description: "Get city"},
		result:     "Beijing",
	}
	mt2 := &mockTool{
		name:       "get_weather",
		definition: ai.ToolDefinition{Name: "get_weather", Description: "Get weather"},
		result:     "Sunny, 25°C",
	}

	prov := &mockProvider{
		calls: [][]ai.Event{
			// 第一轮：调 get_city
			makeToolCallEvents("call_1", "get_city", `{}`, 0),
			// 第二轮：调 get_weather
			makeToolCallEvents("call_2", "get_weather", `{"city":"Beijing"}`, 0),
			// 第三轮：最终回答
			makeTextEvents("Beijing: Sunny, 25°C."),
		},
	}

	ag := New(Config{
		Provider: prov,
		ModelID:  "mock-model",
		Tools:    []tool.Tool{mt1, mt2},
	})

	events := collectEvents(ag.Run(context.Background(), "What's the weather in my city?"))

	last := lastEvent(events)
	if last.Type != EventDone {
		t.Fatalf("expected EventDone, got %v", last.Type)
	}

	// 应该有三轮调用
	stepStarts := 0
	for _, e := range events {
		if e.Type == EventStepStart {
			stepStarts++
		}
	}
	if stepStarts != 3 {
		t.Errorf("expected 3 step starts, got %d", stepStarts)
	}

	// 两个工具都应该被执行
	if !mt1.executed {
		t.Error("get_city should be executed")
	}
	if !mt2.executed {
		t.Error("get_weather should be executed")
	}
}

func TestAgent_MaxStepsExceeded(t *testing.T) {
	mt := &mockTool{
		name:       "loop_tool",
		definition: ai.ToolDefinition{Name: "loop_tool", Description: "A tool that loops"},
		result:     "looping",
	}

	prov := &mockProvider{
		calls: [][]ai.Event{
			makeToolCallEvents("call_1", "loop_tool", `{}`, 0),
			makeToolCallEvents("call_2", "loop_tool", `{}`, 0),
			makeToolCallEvents("call_3", "loop_tool", `{}`, 0),
		},
	}

	ag := New(Config{
		Provider: prov,
		ModelID:  "mock-model",
		Tools:    []tool.Tool{mt},
		MaxSteps: 2,
	})

	events := collectEvents(ag.Run(context.Background(), "Start"))

	last := lastEvent(events)
	if last.Type != EventError {
		t.Fatalf("expected EventError for max steps exceeded, got %v", last.Type)
	}
	if last.Err == nil {
		t.Error("expected non-nil error")
	}
}

func TestAgent_ToolExecutionError(t *testing.T) {
	mt := &mockTool{
		name:       "bad_tool",
		definition: ai.ToolDefinition{Name: "bad_tool", Description: "Always fails"},
		execErr:    errors.New("execution failed"),
	}

	prov := &mockProvider{
		calls: [][]ai.Event{
			makeToolCallEvents("call_1", "bad_tool", `{}`, 0),
			makeTextEvents("I'm done despite the error."),
		},
	}

	ag := New(Config{
		Provider: prov,
		ModelID:  "mock-model",
		Tools:    []tool.Tool{mt},
	})

	events := collectEvents(ag.Run(context.Background(), "Do something"))

	// 工具执行失败时，结果会包含错误信息，但 Agent 应该继续运行
	trEvt, found := findEventByType(events, EventToolResult)
	if !found {
		t.Fatal("expected EventToolResult even on tool error")
	}
	if trEvt.ToolResult == "" {
		t.Error("tool result should contain error info")
	}

	last := lastEvent(events)
	if last.Type != EventDone {
		t.Fatalf("expected EventDone after tool error, got %v", last.Type)
	}
}
