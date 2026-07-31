package myopenai

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

// 这是 openai风格 的本地提供商，兼容 openAI 的接口

// logger
var logger = lg.Module("my-openai")

const defaultBaseURL = "http://localhost:1234/v1"

// 记录各个服务商 provider 旗下支持哪些模型
var knowModels = []ai.ModelInfo{
	{ID: "qwen3.5-4b-mlx", Name: "qwen3.5-4b-mlx", ContextWindow: 15694, ProviderID: "my-openai", MaxTokens: 16384},
}

// provider 实现 ai.Provider 接口。
type Config struct {
	APIKey  string // 密钥
	BaseURL string // 访问base地址
}

// ========================= Provider ========================
type provider struct {
	cfg    Config
	models []ai.ModelInfo
}

// New 创建 LM Studio Provider
func New(cfg Config) ai.Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	return &provider{
		cfg:    cfg,
		models: knowModels,
	}
}

func (p *provider) ID() string {
	return "my-openai"
}

func (p *provider) Name() string {
	return "MyOpenAI"
}

func (p *provider) Models() []ai.ModelInfo {
	return p.models
}

func (p *provider) Model(modelID string) (ai.Model, error) {
	for _, modelInfo := range p.models {
		if modelInfo.ID == modelID {
			return &model{p: p, info: modelInfo}, nil
		}
	}
	return nil, fmt.Errorf("model %q not found in provider %s", modelID, p.ID())
}

// 具体的某个 model 进行chat
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

// ============================== Model ===========================
type model struct {
	p    *provider
	info ai.ModelInfo
}

// Info 返回模型元信息。
func (m *model) Info() ai.ModelInfo {
	return ai.ModelInfo{
		ID:            m.info.ID,
		Name:          m.info.Name,
		ProviderID:    m.info.ProviderID,
		ContextWindow: m.info.ContextWindow,
		MaxTokens:     m.info.MaxTokens,
	}
}

// Chat 使用该模型进行对话，返回流式事件通道。
func (m *model) Chat(ctx context.Context, context ai.Context) ai.Stream {
	stream := make(chan ai.Event)
	go func() {
		defer close(stream)
		m.chat(ctx, context, stream)
	}()
	return stream
}

// chat 基于模型进行对话，解析事件流
func (m *model) chat(ctx context.Context, context ai.Context, out chan<- ai.Event) {
	// TODO 实现
	// buildOpenAIRuquestBody
	reqBody := m.buildOpenAIRuquestBody(context)
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		logger.Error("my-openai: 序列化请求失败", lg.Fields{"error": err})
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("marshal request: %w", err)}
		return
	}
	// header
	url := m.p.cfg.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		logger.Error("openai: 创建 HTTP 请求失败", lg.Fields{"error": err, "url": url})
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("create request: %w", err)}
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+m.p.cfg.APIKey) // openAI 风格
	// doreq
	logger.Info("my-openai: 发起 Chat 请求", lg.Fields{"model": m.info.ID, "url": url})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("my-openai: HTTP 请求失败", lg.Fields{"error": err})
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("http request: %w", err)}
		return
	}
	// resp.Body 通常来自 TCP 连接, 关闭就是释放 TCP 连接
	defer resp.Body.Close()
	// parseresponse
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		logger.Error("my-openai: 非 2xx 响应", lg.Fields{"status": resp.StatusCode, "body": string(b)})
		out <- ai.Event{Type: ai.EventError, Err: fmt.Errorf("chat failed: status=%d body=%s", resp.StatusCode, string(b))}
		return
	}
	// 标志整个流开始
	out <- ai.Event{Type: ai.EventStart}
	// 接收流 & 解析流响应 & 将响应解析为事件发送
	m.parseSSE(resp.Body, out)
}

// accumulator = 累积器
// toolCallAccumulator 用于跨 chunk 把碎片累积起来，等流结束可组成完整的工具调用
type toolCallAccumulator struct {
	tc *ai.ToolCall
}

// message chunk | openAI 单个chunk的消息 json 结构
// choices 是一次请求返回的候选（允许多个choice，但是一般只有一个）
// Delta 表示本次chunk的内容（增量的，后续被聚合的目标）
// content & toolcalls 互斥，不会同时有内容
type messageChunk struct {
	Choices []struct {
		Delta struct {
			Content   string          `json:"content"`
			ToolCalls []toolCallChunk `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

// toolcall chunk 工具调用请求chunk
// Index 工具序号。标记工具（同一个工具，每一个chunk有同一个toolcall）
// ID 工具调用的唯一 ID 和函数名。标记工具调用开始：它们往往只出现在该工具的第一个 chunk 里（后续即使是同一个工具，也不出现 ID）
// Name 工具名
// Arguments 工具参数（json.RawMessage）
type toolCallChunk struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

// 目的：为了得到不带引号的字符串值
func decodeArg(raw json.RawMessage) string {
	var s string
	// json.Unmarshal 会尝试解析这些字节，但如果不是有效的 JSON，它会返回错误
	// json.Unmarshal 作用：判断是否为有效的json
	if err := json.Unmarshal(raw, &s); err != nil {
		return string(raw) // fallback: 直接使用原始字节
	}
	return s
}

// parseSSE 解析 SSE 流，将每个事件转换为 ai.Event 发送。
func (m *model) parseSSE(r io.Reader, out chan<- ai.Event) {
	sc := bufio.NewScanner(r)
	// 预分配容量 64*1024 + 最大缓冲容量 1024*1024
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// 流状态追踪
	var textSeq int                                 // 文本块序号计数器
	var curTextIdx = -1                             // 当前正在进行中的文本块序号，-1 表示无
	toolCalls := make(map[int]*toolCallAccumulator) // key: index，跨 chunk 累积工具调用
	// 循环读取流，直到遇到 data == "[DONE]" （"data: xxxxx"  =>  data == xxxxx）
	for sc.Scan() {
		line := sc.Text()
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
		// 每个 data 都是一个 delta chunk
		var chunk messageChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		// 如果响应候选为空
		if len(chunk.Choices) == 0 {
			continue
		}
		// 一般选择第一个候选作为增量回答
		delta := chunk.Choices[0].Delta

		// 处理文本 delta
		/*
					OpenAI 的流式响应（SSE）中，文本块（Content Block）的返回顺序不一定是 0, 1, 2...。
			 		- 情况 A（正常）：模型先生成索引 0 的块，再生成索引 1 的块。
			 		- 情况 B（乱序）：模型先生成索引 2 的块，再生成索引 0 的块（这种情况虽然少见，但在某些流式场景或特定模型下可能发生）。
			 		这段代码的作用就是：不管接收顺序如何，始终按 0, 1, 2... 的顺序输出。
		*/
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

		// TODO 处理工具调用 这块有点...
		for _, tc := range delta.ToolCalls {
			acc, ok := toolCalls[tc.Index]
			if !ok {
				acc = &toolCallAccumulator{tc: &ai.ToolCall{ID: tc.ID, Name: tc.Function.Name}}
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

	if err := sc.Err(); err != nil {
		logger.Errorf("parseSSE|Scanner|error:%v", err.Error())
	}

	// 关闭未完成的文本块
	if curTextIdx >= 0 {
		out <- ai.Event{Type: ai.EventTextEnd, Index: curTextIdx}
	}

	// 关闭所有工具调用块
	for idx, acc := range toolCalls {
		out <- ai.Event{Type: ai.EventToolCallEnd, TC: acc.tc, Index: idx}
	}
	// 标志流结束
	out <- ai.Event{Type: ai.EventDone}
}

type OpenAIRuquest struct {
	Model       string              `json:"model"`
	Messages    []ai.Message        `json:"messages"`
	Stream      bool                `json:"stream"`
	Tools       []ai.ToolDefinition `json:"tools"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
}

func (m *model) buildOpenAIRuquestBody(c ai.Context) OpenAIRuquest {
	// request 结构: 遵循什么？就是我们说的：
	// TODO 比如message 历史、系统提示词、ai反馈词、用户提示词等需要组合顺序？这里可能得查一下了（最后验证我们的写法）

	// 系统提示词
	message := make([]ai.Message, 0)
	if c.SystemPrompt != "" {
		message = append(message, ai.Message{
			Role:    ai.RoleSystem,
			Content: c.SystemPrompt,
		})
	}

	// 之前的历史记录
	for _, msg := range c.Messages {
		switch msg.Role {
		case ai.RoleSystem:
			if len(message) > 0 {
				// 有了系统提示词，不用了
				continue
			}
			message = append(message, ai.Message{
				Role:    ai.RoleSystem,
				Content: msg.Content,
			})
		case ai.RoleTool:
			if len(msg.ToolCallID) > 0 {
				message = append(message, ai.Message{
					Role:       ai.RoleTool,
					ToolCallID: msg.ToolCallID,
					Content:    msg.Content,
				})
			}
		case ai.RoleAssistant:
			message = append(message, ai.Message{
				Role:    ai.RoleAssistant,
				Content: msg.Content,
				Blocks:  msg.Blocks,
			})
		case ai.RoleUser:
			message = append(message, ai.Message{
				Role:    ai.RoleUser,
				Content: msg.Content,
			})
		}
	}

	// build - req
	req := OpenAIRuquest{
		Model:    m.info.ID,
		Messages: message,
		Stream:   true,
		Tools:    make([]ai.ToolDefinition, 0),
	}

	// 如果有工具，说明需要工具调用
	for _, tool := range c.Tools {
		req.Tools = append(req.Tools, tool)
	}

	if c.MaxTokens > 0 {
		req.MaxTokens = c.MaxTokens
	}
	if c.Temperature > 0 {
		req.Temperature = c.Temperature
	}
	return req
}
