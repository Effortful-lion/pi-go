// Package ai 提供多提供商 LLM 统一接口。
// 本文件定义 Provider 接口——LLM 提供商的统一抽象。
package ai

import "context"

// Provider LLM 提供商。
//
// 每个 Provider 管理一组模型，负责认证和流式对话。
// 典型用法：
//
//	p := openai.New(openai.Config{APIKey: "sk-..."})
//	stream := p.Chat(ctx, "gpt-4o", context)
//	for event := range stream { ... }
type Provider interface {
	// ID 提供商唯一标识，如 "openai", "anthropic"。
	ID() string

	// Name 展示名称。
	Name() string

	// Models 返回该提供商所有可用模型的元信息列表。
	Models() []ModelInfo

	// Model 获取具体的模型实例。
	Model(modelID string) (Model, error)

	// Chat 使用指定模型进行对话，返回事件流。
	// 等价于 Model(modelID).Chat(ctx, context)，是快捷方法。
	Chat(ctx context.Context, modelID string, context Context) Stream
}
