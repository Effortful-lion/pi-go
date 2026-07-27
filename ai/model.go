// Package ai 提供多提供商 LLM 统一接口。
// 本文件定义 ModelInfo 和 Model 接口。
package ai

import "context"

// ModelInfo 模型元信息。
type ModelInfo struct {
	ID            string // 模型唯一标识，如 "gpt-4o", "claude-sonnet-4-5"
	Name          string // 展示名称
	ProviderID    string // 所属提供商 ID
	ContextWindow int    // 上下文窗口大小（tokens），0 表示未知
	MaxTokens     int    // 最大输出 tokens，0 表示未知
}

// Model 模型实例，绑定到一个具体的 Provider，提供 Chat 能力。
type Model interface {
	// Info 返回模型元信息。
	Info() ModelInfo

	// Chat 使用该模型进行对话，返回流式事件通道。
	Chat(ctx context.Context, context Context) Stream
}
