package qwen

import (
	"context"

	"github.com/Effortful-lion/pi-go/ai"
	"github.com/Effortful-lion/pi-go/lg"
)

// 这是 openai风格 的本地提供商，兼容 openAI 的接口

const defaultBaseURL = "http://localhost:1234/v1"

// provider
type Config struct {
	APIKey  string // 密钥
	BaseURL string // 访问base地址
}

// 记录各个服务商 provider 旗下支持哪些模型
var knowModels = []ai.ModelInfo{
	{ID: "qwen3.5-4b-mlx", Name: "QWEN-3.5", ContextWindow: 15694, ProviderID: "lmstudio", MaxTokens: 16384},
}

// logger
var logger = lg.Module("[lm-studio]")

// type Provider interface {
// 	// ID 提供商唯一标识，如 "openai", "anthropic"。
// 	ID() string

// 	// Name 展示名称。
// 	Name() string

// 	// Models 返回该提供商所有可用模型的元信息列表。
// 	Models() []ModelInfo

// 	// Model 获取具体的模型实例。
// 	Model(modelID string) (Model, error)

//		// Chat 使用指定模型进行对话，返回事件流。
//		// 等价于 Model(modelID).Chat(ctx, context)，是快捷方法。
//		Chat(ctx context.Context, modelID string, context Context) Stream
//	}
//
// provider 实现 ai.Provider 接口。
type provider struct {
	cfg    Config
	models []ai.ModelInfo
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

func (p *provider) Chat(ctx context.Context, modelID string, context ai.Context) ai.Stream {

}
