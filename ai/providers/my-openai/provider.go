package qwen

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Effortful-lion/pi-go/ai"
	"github.com/Effortful-lion/pi-go/lg"
)

// 这是 openai风格 的本地提供商，兼容 openAI 的接口

const defaultBaseURL = "http://localhost:1234/v1"

// 记录各个服务商 provider 旗下支持哪些模型
var knowModels = []ai.ModelInfo{
	{ID: "qwen3.5-4b-mlx", Name: "QWEN-3.5", ContextWindow: 15694, ProviderID: "lmstudio", MaxTokens: 16384},
}

// logger
var logger = lg.Module("[lm-studio]")

// provider 实现 ai.Provider 接口。
type Config struct {
	APIKey  string // 密钥
	BaseURL string // 访问base地址
}

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
}

type provider struct {
	cfg    Config
	models []ai.ModelInfo
}

// New 创建 LM Studio Provider
func New(cfg Config) ai.Provider {
	return &provider{
		cfg:    cfg,
		models: knowModels,
	}
}

func (p *provider) ID() string {
	return "lm-studio"
}

func (p *provider) Name() string {
	return "LM Studio"
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

// buildRequest 构造 OpenAI Chat Completions 请求体。
func (m *model) buildRequest(ctx ai.Context) map[string]any {
	panic("not implemented")
}

// buildToolCalls 将 ai.ContentBlock 转换为 OpenAI tool_calls JSON 格式。
func buildToolCalls(blocks []ai.ContentBlock) []map[string]any {
	panic("not implementd")
}

// parseSSE 解析 SSE 流，将每个事件转换为 ai.Event 发送。
func (m *model) parseSSE(r io.Reader, out chan<- ai.Event) {
	panic("not implementd")
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
// Index 工具序号（同一个工具，每一个chunk有同一个toolcall）
// ID 工具调用的唯一 ID 和函数名。它们往往只出现在该工具的第一个 chunk 里
// Name 工具名
// Arguments 工具参数（json.RawMessage）
// TODO 搞清楚 json.RawMessage
type toolCallChunk struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}
