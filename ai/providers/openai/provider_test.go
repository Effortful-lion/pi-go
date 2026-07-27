package openai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Effortful-lion/pi-go/ai"
)

func TestProvider_Chat_TextStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"id\":\"c1\",\"choices\":[{\"delta\":{}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var events []ai.Event
	for ev := range p.Chat(ctx, "gpt-4o-mini", ai.Context{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
	}) {
		events = append(events, ev)
	}

	types := eventTypes(events)
	expected := []ai.EventType{
		ai.EventStart,
		ai.EventTextStart,
		ai.EventTextDelta,
		ai.EventTextDelta,
		ai.EventTextEnd,
		ai.EventDone,
	}
	if len(types) != len(expected) {
		t.Fatalf("expected %d events, got %d: %v", len(expected), len(types), types)
	}
	for i, et := range expected {
		if types[i] != et {
			t.Errorf("event[%d]: want %v, got %v", i, et, types[i])
		}
	}

	// 验证文本内容拼接
	var text string
	for _, ev := range events {
		if ev.Type == ai.EventTextDelta {
			text += ev.Text
		}
	}
	if text != "Hello world" {
		t.Errorf("text = %q, want %q", text, "Hello world")
	}
}

func TestProvider_Chat_ToolCallStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			`{"id":"c2","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather"}}]}}]}`,
			`{"id":"c2","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]}}]}`,
			`{"id":"c2","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"Beijing\""}}]}}]}`,
			`{"id":"c2","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"}"}}]}}]}`,
			`{"id":"c2","choices":[{"delta":{}}]}`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", c)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var events []ai.Event
	for ev := range p.Chat(ctx, "gpt-4o-mini", ai.Context{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "weather in Beijing?"}},
		Tools:    []ai.ToolDefinition{{Name: "get_weather"}},
	}) {
		events = append(events, ev)
	}

	// 检查事件序列
	var hasToolStart, hasToolDelta, hasToolEnd bool
	var lastTC *ai.ToolCall
	for _, ev := range events {
		switch ev.Type {
		case ai.EventToolCallStart:
			hasToolStart = true
			lastTC = ev.TC
		case ai.EventToolCallDelta:
			hasToolDelta = true
		case ai.EventToolCallEnd:
			hasToolEnd = true
		}
	}
	if !hasToolStart || !hasToolDelta || !hasToolEnd {
		t.Errorf("missing tool call lifecycle: start=%v delta=%v end=%v", hasToolStart, hasToolDelta, hasToolEnd)
	}
	if lastTC == nil || lastTC.Name != "get_weather" {
		t.Errorf("tool call name = %q, want %q", lastTC.Name, "get_weather")
	}
	if lastTC == nil || lastTC.Arguments != `{"city":"Beijing"}` {
		t.Errorf("tool call args = %q, want %q", lastTC.Arguments, `{"city":"Beijing"}`)
	}
}

func TestProvider_Chat_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer srv.Close()

	p := New(Config{APIKey: "bad-key", BaseURL: srv.URL})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var events []ai.Event
	for ev := range p.Chat(ctx, "gpt-4o-mini", ai.Context{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
	}) {
		events = append(events, ev)
	}

	if len(events) == 0 || events[len(events)-1].Type != ai.EventError {
		t.Error("expected error event on non-2xx status")
	}
}

func TestProvider_Chat_ModelNotFound(t *testing.T) {
	p := New(Config{APIKey: "key"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var events []ai.Event
	for ev := range p.Chat(ctx, "nonexistent", ai.Context{}) {
		events = append(events, ev)
	}
	if len(events) == 0 || events[0].Type != ai.EventError {
		t.Error("expected error event for unknown model")
	}
}

func eventTypes(events []ai.Event) []ai.EventType {
	types := make([]ai.EventType, len(events))
	for i, e := range events {
		types[i] = e.Type
	}
	return types
}
