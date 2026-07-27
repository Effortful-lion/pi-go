// Package agent 提供 Agent 运行时——LLM 与工具调用的自动循环。
// 本文件定义 Agent 级别的事件系统（比 ai.Event 更高层的抽象）。
package agent

import (
	"github.com/Effortful-lion/pi-go/ai"
)

// EventType Agent 事件类型。
type EventType int

const (
	EventStepStart  EventType = iota // 一轮 LLM 调用开始
	EventStepEnd                      // 一轮 LLM 调用结束（含 Usage）
	EventToolCall                     // 工具调用
	EventToolResult                   // 工具返回结果
	EventTextDelta                    // LLM 文本增量（透传给上层）
	EventDone                         // Agent 运行结束
	EventError                        // Agent 运行出错
)

// Event Agent 运行时产生的事件。
type Event struct {
	Type       EventType
	Text       string     // TextDelta: 增量文本
	ToolCall   *ai.ToolCall // ToolCall: 工具调用信息
	ToolResult string     // ToolResult: 工具执行结果
	Usage      *ai.Usage  // StepEnd: token 用量统计
	Step       int        // 当前步骤编号（从 1 开始）
	Err        error      // Error: 错误详情
}

// Stream Agent 事件通道。
// 用法: for event := range stream { ... }
type Stream <-chan Event
