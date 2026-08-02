// Package openai 实现 OpenAI 兼容协议的 Provider。
package openai

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
	"github.com/Effortful-lion/pi-go/lg"
)

const defaultBaseURL = "https://api.openai.com/v1"

// Config OpenAI Provider 配置。
type Config struct {
	APIKey  string // OpenAI API Key
	BaseURL string // API 地址，默认 https://api.openai.com/v1
}

var knownModels = []ai.ModelInfo{
	{ID: "gpt-4o", Name: "GPT-4o", ProviderID: "openai", ContextWindow: 128000, MaxTokens: 16384},
	{ID: "gpt-4o-mini", Name: "GPT-4o Mini", ProviderID: "openai", ContextWindow: 128000, MaxTokens: 16384},
}

var logger = lg.Module("[openai]")

// provider 实现 ai.Provider 接口。
type provider struct {
	cfg    Config
	models []ai.ModelInfo
}

// model 实现 ai.Model 接口，绑定到一个 provider 和模型 ID。
type model struct {
	p    *provider
	info ai.ModelInfo
}

// New 创建 OpenAI Provider。
func New(cfg Config) ai.Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	return &provider{
		cfg:    cfg,
		models: knownModels,
	}
}

// --- ai.Provider 实现 ---

func (p *provider) ID() string             { return "openai" }
func (p *provider) Name() string           { return "OpenAI" }
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

// --- ai.Model 实现 ---

func (m *model) Info() ai.ModelInfo { return m.info }

func (m *model) Chat(ctx context.Context, context ai.Context) ai.Stream {
	stream := make(chan ai.Event)
	go func() {
		defer close(stream)
		m.chat(ctx, context, stream)
	}()
	return stream
}

func (m *model) chat(ctx context.Context, context ai.Context, out chan<- ai.Event) {
	reqBody := m.buildRequest(context)
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		logger.Error("openai: 序列化请求失败", lg.Fields{"error": err})
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("marshal request: %w", err)}
		return
	}

	url := m.p.cfg.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		logger.Error("openai: 创建 HTTP 请求失败", lg.Fields{"error": err, "url": url})
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("create request: %w", err)}
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+m.p.cfg.APIKey)

	logger.Info("openai: 发起 Chat 请求", lg.Fields{"model": m.info.ID, "url": url})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("openai: HTTP 请求失败", lg.Fields{"error": err})
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("http request: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		logger.Error("openai: 非 2xx 响应", lg.Fields{"status": resp.StatusCode, "body": string(b)})
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("chat failed: status=%d body=%s", resp.StatusCode, string(b))}
		return
	}

	out <- ai.Event{Type: ai.EventStart}
	m.parseSSE(resp.Body, out)
}

// buildRequest 构造 OpenAI Chat Completions 请求体。
func (m *model) buildRequest(ctx ai.Context) map[string]any {
	messages := make([]map[string]any, 0)

	if ctx.SystemPrompt != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": ctx.SystemPrompt,
		})
	}

	for _, msg := range ctx.Messages {
		switch msg.Role {
		case ai.RoleUser, ai.RoleSystem:
			messages = append(messages, map[string]any{
				"role":    string(msg.Role),
				"content": msg.Content,
			})
		case ai.RoleAssistant:
			am := map[string]any{"role": "assistant"}
			if msg.Content != "" {
				am["content"] = msg.Content
			}
			if len(msg.Blocks) > 0 {
				toolCalls := buildToolCalls(msg.Blocks)
				if len(toolCalls) > 0 {
					am["tool_calls"] = toolCalls
				}
			}
			messages = append(messages, am)
		case ai.RoleTool:
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": msg.ToolCallID,
				"content":      msg.Content,
			})
		}
	}

	req := map[string]any{
		"model":    m.info.ID,
		"messages": messages,
		"stream":   true,
	}

	if len(ctx.Tools) > 0 {
		tools := make([]map[string]any, len(ctx.Tools))
		for i, t := range ctx.Tools {
			tools[i] = map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Parameters,
				},
			}
		}
		req["tools"] = tools
	}
	if ctx.MaxTokens > 0 {
		req["max_tokens"] = ctx.MaxTokens
	}
	if ctx.Temperature > 0 {
		req["temperature"] = ctx.Temperature
	}

	return req
}

// buildToolCalls 将 ai.ContentBlock 转换为 OpenAI tool_calls JSON 格式。
func buildToolCalls(blocks []ai.ContentBlock) []map[string]any {
	var res []map[string]any
	for _, b := range blocks {
		if b.Type == ai.BlockToolCall && b.ToolCall != nil {
			res = append(res, map[string]any{
				"id":   b.ToolCall.ID,
				"type": "function",
				"function": map[string]any{
					"name":      b.ToolCall.Name,
					"arguments": b.ToolCall.Arguments,
				},
			})
		}
	}
	return res
}

// parseSSE 解析 SSE 流，将每个事件转换为 ai.Event 发送。
func (m *model) parseSSE(r io.Reader, out chan<- ai.Event) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// 流状态追踪
	var textSeq int                         // 文本块序号计数器
	var curTextIdx = -1                     // 当前正在进行中的文本块序号，-1 表示无
	toolCalls := make(map[int]*toolCallAcc) // key: index，跨 chunk 累积工具调用

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var chunk oaiChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta

		// 处理文本 delta
		if delta.Content != "" {
			if curTextIdx < 0 {
				curTextIdx = textSeq
				out <- ai.Event{Type: ai.EventTextStart, Index: curTextIdx}
				textSeq++
			}
			out <- ai.Event{Type: ai.EventTextDelta, Index: curTextIdx, Text: delta.Content}
		} else if curTextIdx >= 0 {
			// 文本块结束：当前 delta 不再有 content
			out <- ai.Event{Type: ai.EventTextEnd, Index: curTextIdx}
			curTextIdx = -1
		}

		// 处理工具调用
		for _, tc := range delta.ToolCalls {
			acc, ok := toolCalls[tc.Index]
			if !ok {
				acc = &toolCallAcc{tc: &ai.ToolCall{ID: tc.ID, Name: tc.Function.Name}}
				toolCalls[tc.Index] = acc
				out <- ai.Event{Type: ai.EventToolCallStart, TC: acc.tc, Index: tc.Index}
			}
			if tc.ID != "" {
				acc.tc.ID = tc.ID
			}
			if tc.Function.Name != "" {
				acc.tc.Name = tc.Function.Name
			}
			if len(tc.Function.Arguments) > 0 {
				acc.tc.Arguments += decodeArg(tc.Function.Arguments)
				out <- ai.Event{Type: ai.EventToolCallDelta, TC: acc.tc, Index: tc.Index}
			}
		}
	}
	if scanner.Err() != nil {
		logger.Error("I/O 错误或缓冲区溢出", lg.Fields{"err": scanner.Err()})
		out <- ai.Event{Type: ai.EventError, Err: scanner.Err()}
	}

	// 关闭未完成的文本块
	if curTextIdx >= 0 {
		out <- ai.Event{Type: ai.EventTextEnd, Index: curTextIdx}
	}

	// 关闭所有工具调用块
	for idx, acc := range toolCalls {
		out <- ai.Event{Type: ai.EventToolCallEnd, TC: acc.tc, Index: idx}
	}

	out <- ai.Event{Type: ai.EventDone}
}

// toolCallAcc 跨 chunk 累积工具调用的中间状态。
type toolCallAcc struct {
	tc *ai.ToolCall
}

// oaiChunk OpenAI SSE 单个 chunk 的 JSON 结构。
type oaiChunk struct {
	Choices []struct {
		Delta struct {
			Content   string             `json:"content"`
			ToolCalls []oaiToolCallChunk `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

// decodeArg 将 OpenAI 二次编码的 arguments 字段解码为纯文本字符串。
// OpenAI SSE 中 tool_calls[].function.arguments 是 JSON 字符串值（自带外层引号），
// 需要先解包为 Go string，才能直接追加到累积缓冲区。
func decodeArg(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return string(raw) // fallback: 直接使用原始字节
	}
	return s
}

// oaiToolCallChunk 工具调用 delta chunk。
type oaiToolCallChunk struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}
