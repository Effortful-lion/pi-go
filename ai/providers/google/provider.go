// Package google 实现 Google Gemini API 的 Provider。
package google

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

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// Config Google Gemini Provider 配置。
type Config struct {
	APIKey  string // Google AI API Key
	BaseURL string // API 地址，默认 https://generativelanguage.googleapis.com/v1beta
}

var knownModels = []ai.ModelInfo{
	{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", ProviderID: "google", ContextWindow: 1048576, MaxTokens: 8192},
	{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", ProviderID: "google", ContextWindow: 1048576, MaxTokens: 8192},
}

var logger = lg.Module("[google]")

type provider struct {
	cfg    Config
	models []ai.ModelInfo
}

type model struct {
	p    *provider
	info ai.ModelInfo
}

// New 创建 Gemini Provider。
func New(cfg Config) ai.Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	return &provider{
		cfg:    cfg,
		models: knownModels,
	}
}

func (p *provider) ID() string             { return "google" }
func (p *provider) Name() string           { return "Google Gemini" }
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

	url := m.p.cfg.BaseURL + "/models/" + m.info.ID + ":streamGenerateContent?alt=sse&key=" + m.p.cfg.APIKey
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("create request: %w", err)}
		return
	}
	req.Header.Set("Content-Type", "application/json")

	logger.Info("google: 发起 Chat 请求", lg.Fields{"model": m.info.ID, "url": url})

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

// --- Gemini API 请求类型 ---

// chatPart Gemini 消息中的 part（文本/函数调用/函数响应）。
type chatPart struct {
	Text             string              `json:"text,omitempty"`
	FunctionCall     *funcCall           `json:"functionCall,omitempty"`
	FunctionResponse *funcResponse       `json:"functionResponse,omitempty"`
}

// funcCall 函数调用。
type funcCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// funcResponse 函数调用响应（tool result）。
type funcResponse struct {
	Name     string `json:"name"`
	Response struct {
		Content string `json:"content"`
	} `json:"response"`
}

// chatContent Gemini 消息内容（role + parts）。
type chatContent struct {
	Role  string     `json:"role"`
	Parts []chatPart `json:"parts"`
}

// systemInstruction Gemini 系统指令。
type systemInstruction struct {
	Parts []chatPart `json:"parts"`
}

// toolDef Gemini 工具定义。
type toolDef struct {
	FunctionDeclarations []funcDecl `json:"functionDeclarations"`
}

// funcDecl Gemini 函数声明（工具定义中的单个函数）。
type funcDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// generationConfig Gemini 生成配置。
type generationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

// chatRequest Gemini API 请求体。
type chatRequest struct {
	SystemInstruction *systemInstruction `json:"systemInstruction,omitempty"`
	Contents          []chatContent      `json:"contents"`
	Tools             []toolDef          `json:"tools,omitempty"`
	GenerationConfig  *generationConfig  `json:"generationConfig,omitempty"`
}

// buildRequest 构造 Gemini API 请求体。
// Gemini 的 system prompt 通过 systemInstruction 顶层字段传递。
func (m *model) buildRequest(ctx ai.Context) chatRequest {
	req := chatRequest{}

	// 收集 system prompt
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
		req.SystemInstruction = &systemInstruction{
			Parts: []chatPart{{Text: sys}},
		}
	}

	// contents: Gemini 的 messages
	contents := make([]chatContent, 0)
	for _, msg := range ctx.Messages {
		switch msg.Role {
		case ai.RoleSystem:
			continue
		case ai.RoleUser:
			contents = append(contents, chatContent{
				Role:  "user",
				Parts: []chatPart{{Text: msg.Content}},
			})
		case ai.RoleAssistant:
			parts := make([]chatPart, 0)
			if msg.Content != "" {
				parts = append(parts, chatPart{Text: msg.Content})
			}
			for _, b := range msg.Blocks {
				if b.Type == ai.BlockToolCall && b.ToolCall != nil {
					var args map[string]any
					_ = json.Unmarshal([]byte(b.ToolCall.Arguments), &args)
					parts = append(parts, chatPart{
						FunctionCall: &funcCall{
							Name: b.ToolCall.Name,
							Args: args,
						},
					})
				}
			}
			contents = append(contents, chatContent{
				Role:  "model", // Gemini 叫 model 而不是 assistant
				Parts: parts,
			})
		case ai.RoleTool:
			contents = append(contents, chatContent{
				Role: "user",
				Parts: []chatPart{
					{
						FunctionResponse: &funcResponse{
							Name: msg.ToolName,
							Response: struct {
								Content string `json:"content"`
							}{Content: msg.Content},
						},
					},
				},
			})
		}
	}
	req.Contents = contents

	// tools → functionDeclarations
	if len(ctx.Tools) > 0 {
		funcDecls := make([]funcDecl, len(ctx.Tools))
		for i, t := range ctx.Tools {
			funcDecls[i] = funcDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			}
		}
		req.Tools = []toolDef{{FunctionDeclarations: funcDecls}}
	}

	if ctx.MaxTokens > 0 || ctx.Temperature > 0 {
		cfg := &generationConfig{}
		if ctx.MaxTokens > 0 {
			cfg.MaxOutputTokens = ctx.MaxTokens
		}
		if ctx.Temperature > 0 {
			cfg.Temperature = ctx.Temperature
		}
		req.GenerationConfig = cfg
	}

	return req
}

// parseSSE 解析 Gemini SSE 流。
// Gemini SSE 格式：每行 "data: {...}"，内容是单个 JSON 对象（不是数组），
// 或者使用 alt=sse 时的标准 SSE 格式。
func (m *model) parseSSE(r io.Reader, out chan<- ai.Event) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)

	var (
		textSeq   int = -1
		funcIdx   int = -1
		textOpen  bool
		funcOpen  bool
		curFuncID string
		curFuncNm string
		funcArgs  strings.Builder
	)

	for scanner.Scan() {
		line := scanner.Text()
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)

		var chunk messageChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Candidates) == 0 {
			continue
		}

		candidate := chunk.Candidates[0]
		if candidate.Content == nil {
			continue
		}
		content := candidate.Content

		// 检查 finishReason: STOP = 正常结束
		if candidate.FinishReason != "" && candidate.FinishReason != "STOP" {
			continue
		}

		// 处理 parts
		for _, part := range content.Parts {
			if part.Text != "" {
				if !textOpen {
					// 如果之前有 tool_use 在打开中，先关闭
					if funcOpen {
						out <- ai.Event{
							Type:  ai.EventToolCallEnd,
							Index: funcIdx,
							TC:    &ai.ToolCall{ID: curFuncID, Name: curFuncNm, Arguments: funcArgs.String()},
						}
						funcOpen = false
						funcArgs.Reset()
					}

					textSeq++
					textOpen = true
					out <- ai.Event{Type: ai.EventTextStart, Index: textSeq}
				}
				out <- ai.Event{Type: ai.EventTextDelta, Index: textSeq, Text: part.Text}
			}

			if part.FunctionCall != nil {
				if textOpen {
					out <- ai.Event{Type: ai.EventTextEnd, Index: textSeq}
					textOpen = false
				}

				fc := part.FunctionCall
				if !funcOpen || fc.Name != curFuncNm {
					if funcOpen {
						out <- ai.Event{
							Type:  ai.EventToolCallEnd,
							Index: funcIdx,
							TC:    &ai.ToolCall{ID: curFuncID, Name: curFuncNm, Arguments: funcArgs.String()},
						}
						funcArgs.Reset()
					}
					funcIdx++
					funcOpen = true
					curFuncNm = fc.Name
					curFuncID = fc.Name + "_" + fmt.Sprint(funcIdx)
					out <- ai.Event{
						Type:  ai.EventToolCallStart,
						Index: funcIdx,
						TC:    &ai.ToolCall{ID: curFuncID, Name: curFuncNm},
					}
				}
				if fc.Args != nil {
					argsJSON, _ := json.Marshal(fc.Args)
					funcArgs = strings.Builder{}
					funcArgs.Write(argsJSON)
					out <- ai.Event{
						Type:  ai.EventToolCallDelta,
						Index: funcIdx,
						TC:    &ai.ToolCall{ID: curFuncID, Name: curFuncNm, Arguments: funcArgs.String()},
					}
				}
			}
		}
	}

	if scanner.Err() != nil {
		logger.Error("I/O 错误或缓冲区溢出", lg.Fields{"err": scanner.Err()})
		out <- ai.Event{Type: ai.EventError, Err: scanner.Err()}
	}

	// 关闭任何未关闭的块
	if textOpen {
		out <- ai.Event{Type: ai.EventTextEnd, Index: textSeq}
	}
	if funcOpen {
		out <- ai.Event{
			Type:  ai.EventToolCallEnd,
			Index: funcIdx,
			TC:    &ai.ToolCall{ID: curFuncID, Name: curFuncNm, Arguments: funcArgs.String()},
		}
	}
	out <- ai.Event{Type: ai.EventDone}
}

// --- Gemini SSE 响应类型 ---

// messageChunk Gemini SSE 单个块。
type messageChunk struct {
	Candidates []struct {
		Content      *chatContent `json:"content"`
		FinishReason string       `json:"finishReason"`
		Index        int          `json:"index"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}
