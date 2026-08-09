// Package anthropic 实现 Anthropic Claude Messages API 的 Provider。
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Effortful-lion/pi-go/ai"
	lg "github.com/Effortful-lion/unibase/logx"
)

const (
	defaultBaseURL   = "https://api.anthropic.com/v1"
	anthropicVersion = "2023-06-01"
)

// Config Anthropic Provider 配置。
type Config struct {
	APIKey  string // Anthropic API Key
	BaseURL string // API 地址，默认 https://api.anthropic.com/v1
}

var knownModels = []ai.ModelInfo{
	{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", ProviderID: "anthropic", ContextWindow: 200000, MaxTokens: 8192},
	{ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", ProviderID: "anthropic", ContextWindow: 200000, MaxTokens: 8192},
}

var logger = lg.Module("[anthropic]")

type provider struct {
	cfg    Config
	models []ai.ModelInfo
}

type model struct {
	p    *provider
	info ai.ModelInfo
}

// New 创建 Anthropic Provider。
func New(cfg Config) ai.Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	return &provider{
		cfg:    cfg,
		models: knownModels,
	}
}

func (p *provider) ID() string             { return "anthropic" }
func (p *provider) Name() string           { return "Anthropic" }
func (p *provider) Models() []ai.ModelInfo { return p.models }
func (p *provider) Model(modelID string) (ai.Model, error) {
	for _, mi := range p.models {
		if mi.ID == modelID {
			return &model{p: p, info: mi}, nil
		}
	}
	return nil, fmt.Errorf("model %q not found in provider %s", modelID, p.ID())
}

func (p *provider) Chat(ctx context.Context, modelID string, context ai.Context) ai.Stream {
	m, err := p.Model(modelID)
	if err != nil {
		ch := make(chan ai.Event, 1)
		ch <- ai.Event{Type: ai.EventError, Err: err}
		close(ch)
		return ch
	}
	return m.Chat(ctx, context)
}

func (m *model) Info() ai.ModelInfo { return m.info }

func (m *model) Chat(ctx context.Context, context ai.Context) ai.Stream {
	stream := make(chan ai.Event)
	go func() {
		defer close(stream)
		m.chat(ctx, context, stream)
	}()
	return stream
}

func (m *model) chat(ctx context.Context, ctx2 ai.Context, out chan<- ai.Event) {
	reqBody := m.buildRequest(ctx2)
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("marshal request: %w", err)}
		return
	}

	url := m.p.cfg.BaseURL + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("create request: %w", err)}
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("x-api-key", m.p.cfg.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	logger.Info("anthropic: 发起 Chat 请求", lg.Fields{"model": m.info.ID, "url": url})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("http request: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("chat failed: status=%d body=%s", resp.StatusCode, string(b))}
		return
	}

	out <- ai.Event{Type: ai.EventStart}
	m.parseSSE(resp.Body, out)
}

// --- Anthropic API 请求类型 ---

// chatMessage Anthropic 消息格式。
type chatMessage struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

// contentBlock Anthropic 内容块（text/tool_use/tool_result）。
type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
}

// toolDef Anthropic 工具定义。
type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// chatRequest Anthropic Messages API 请求体。
type chatRequest struct {
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens"`
	Stream      bool          `json:"stream"`
	System      string        `json:"system,omitempty"`
	Messages    []chatMessage `json:"messages"`
	Tools       []toolDef     `json:"tools,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

// --- Anthropic SSE 响应类型 ---

// sseContentBlockStart content_block_start 事件。
type sseContentBlockStart struct {
	Index        int `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
}

// sseContentBlockDelta content_block_delta 事件。
type sseContentBlockDelta struct {
	Index int `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
}

// sseContentBlockStop content_block_stop 事件。
type sseContentBlockStop struct {
	Index int `json:"index"`
}

// sseMessageDelta message_delta 事件。
type sseMessageDelta struct {
	Delta struct {
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// buildRequest 构造 Anthropic Messages API 请求体。
// 关键差异：system prompt 通过顶层 system 字段传递，不作为 message。
func (m *model) buildRequest(ctx ai.Context) chatRequest {
	maxTokens := 4096
	if ctx.MaxTokens > 0 {
		maxTokens = ctx.MaxTokens
	}

	req := chatRequest{
		Model:     m.info.ID,
		MaxTokens: maxTokens,
		Stream:    true,
	}

	// system prompt 作为顶层字段
	collectSystem := func() string {
		var sb strings.Builder
		if ctx.SystemPrompt != "" {
			sb.WriteString(ctx.SystemPrompt)
		}
		for _, msg := range ctx.Messages {
			if msg.Role == ai.RoleSystem {
				if sb.Len() > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString(msg.Content)
			}
		}
		return sb.String()
	}
	if sys := collectSystem(); sys != "" {
		req.System = sys
	}

	// messages: Anthropic 只支持 user/assistant 角色
	msgs := make([]chatMessage, 0)
	for _, msg := range ctx.Messages {
		switch msg.Role {
		case ai.RoleSystem:
			continue // 已被 system 字段处理
		case ai.RoleUser:
			msgs = append(msgs, chatMessage{
				Role: "user",
				Content: []contentBlock{
					{Type: "text", Text: msg.Content},
				},
			})
		case ai.RoleAssistant:
			blocks := make([]contentBlock, 0)
			if msg.Content != "" {
				blocks = append(blocks, contentBlock{Type: "text", Text: msg.Content})
			}
			for _, b := range msg.Blocks {
				if b.Type == ai.BlockToolCall && b.ToolCall != nil {
					blocks = append(blocks, contentBlock{
						Type:  "tool_use",
						ID:    b.ToolCall.ID,
						Name:  b.ToolCall.Name,
						Input: json.RawMessage(b.ToolCall.Arguments),
					})
				}
			}
			msgs = append(msgs, chatMessage{Role: "assistant", Content: blocks})
		case ai.RoleTool:
			msgs = append(msgs, chatMessage{
				Role: "user",
				Content: []contentBlock{
					{
						Type:      "tool_result",
						ToolUseID: msg.ToolCallID,
						Text:      msg.Content,
					},
				},
			})
		}
	}
	req.Messages = msgs

	// tools 格式
	if len(ctx.Tools) > 0 {
		tools := make([]toolDef, len(ctx.Tools))
		for i, t := range ctx.Tools {
			tools[i] = toolDef{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.Parameters,
			}
		}
		req.Tools = tools
	}

	if ctx.Temperature > 0 {
		req.Temperature = ctx.Temperature
	}

	return req
}

// parseSSE 解析 Anthropic SSE 流。
// 事件类型: message_start, content_block_start, content_block_delta, content_block_stop, message_delta, message_stop
func (m *model) parseSSE(r io.Reader, out chan<- ai.Event) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		textSeq     int    = -1 // 当前文本块序号
		toolIdx     int         // 当前工具调用序号
		textStarted bool        // 是否有过文本开始事件
		evtType     string      // 当前事件类型
	)

	type toolState struct {
		id   string
		name string
		args strings.Builder
	}
	tools := make(map[int]*toolState)

	for scanner.Scan() {
		line := scanner.Text()

		// Anthropic SSE 格式: "event: <type>" 后跟 "data: <json>"
		data := ""
		if v, ok := strings.CutPrefix(line, "event: "); ok {
			evtType = strings.TrimSpace(v)
			continue
		}
		if v, ok := strings.CutPrefix(line, "data: "); ok {
			data = strings.TrimSpace(v)
		}
		if data == "" {
			continue
		}

		switch evtType {
		case "message_start":
			// 提取 usage（在 message.usage 中）
		case "content_block_start":
			var cb sseContentBlockStart
			if err := json.Unmarshal([]byte(data), &cb); err != nil {
				continue
			}
			switch cb.ContentBlock.Type {
			case "text":
				textSeq++
				textStarted = true
				out <- ai.Event{Type: ai.EventTextStart, Index: textSeq}
			case "tool_use":
				toolIdx = cb.Index
				tools[toolIdx] = &toolState{id: cb.ContentBlock.ID, name: cb.ContentBlock.Name}
				out <- ai.Event{
					Type:  ai.EventToolCallStart,
					Index: toolIdx,
					TC:    &ai.ToolCall{ID: cb.ContentBlock.ID, Name: cb.ContentBlock.Name},
				}
			}
		case "content_block_delta":
			var cb sseContentBlockDelta
			if err := json.Unmarshal([]byte(data), &cb); err != nil {
				continue
			}
			switch cb.Delta.Type {
			case "text_delta":
				out <- ai.Event{Type: ai.EventTextDelta, Index: textSeq, Text: cb.Delta.Text}
			case "input_json_delta":
				if ts, ok := tools[cb.Index]; ok {
					ts.args.WriteString(cb.Delta.PartialJSON)
					out <- ai.Event{
						Type:  ai.EventToolCallDelta,
						Index: cb.Index,
						TC:    &ai.ToolCall{ID: ts.id, Name: ts.name, Arguments: ts.args.String()},
					}
				}
			}
		case "content_block_stop":
			var cb sseContentBlockStop
			if err := json.Unmarshal([]byte(data), &cb); err != nil {
				continue
			}
			if ts, ok := tools[cb.Index]; ok {
				out <- ai.Event{
					Type:  ai.EventToolCallEnd,
					Index: cb.Index,
					TC:    &ai.ToolCall{ID: ts.id, Name: ts.name, Arguments: ts.args.String()},
				}
				delete(tools, cb.Index)
			}
			// 如果是 tool_use block 结束且没有文本，文本块无需关闭
			if cb.Index == textSeq && textStarted {
				out <- ai.Event{Type: ai.EventTextEnd, Index: textSeq}
				textStarted = false
			}
		case "message_delta":
			var md sseMessageDelta
			if err := json.Unmarshal([]byte(data), &md); err != nil {
				continue
			}
			out <- ai.Event{
				Type:  ai.EventDone,
				Usage: &ai.Usage{CompletionTokens: md.Usage.OutputTokens},
			}
		case "message_stop":
			// 最终结束标志
		case "ping":
			// 心跳，忽略
		}
	}
	if scanner.Err() != nil {
		logger.Error("I/O 错误或缓冲区溢出", lg.Fields{"err": scanner.Err()})
		out <- ai.Event{Type: ai.EventError, Err: scanner.Err()}
	}
}
