// Package ai 提供多提供商 LLM 统一接口。
// 本文件定义流式事件系统：事件类型、Event 结构体、Stream 通道类型。
package ai

// EventType 流式事件类型。
type EventType int

const (
	EventStart         EventType = iota // 整个流开始
	EventTextStart                      // 文本内容块开始
	EventTextDelta                      // 文本增量
	EventTextEnd                        // 文本内容块结束
	EventThinkingStart                  // 思考内容块开始（Claude 等模型支持）
	EventThinkingDelta                  // 思考增量
	EventThinkingEnd                    // 思考内容块结束
	EventToolCallStart                  // 工具调用开始
	EventToolCallDelta                  // 工具调用参数增量
	EventToolCallEnd                    // 工具调用结束
	EventDone                           // 整个响应结束
	EventError                          // 错误（流中断）
)

// Event 流式响应中的单个事件。
type Event struct {
	Type  EventType
	Text  string   // TextDelta/ThinkingDelta: 增量文本
	Index int      // 内容块序号，区分多个同类型块混排
	TC    *ToolCall // ToolCallStart/End/Delta: 工具调用信息
	Usage *Usage   // Done: token 用量
	Err   error    // Error: 错误详情
}

// Usage token 用量统计。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Stream 事件通道，Provider.Chat() 的返回值。
// 用法: for event := range stream { ... }
// 关闭 channel 意味着流已结束（正常或异常）。
type Stream <-chan Event
