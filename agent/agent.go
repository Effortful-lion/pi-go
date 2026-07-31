// Package agent 提供 Agent 运行时——LLM 与工具调用的自动循环。
//
// Agent 核心职责：
//   - 接收用户输入，管理对话历史
//   - 调用 LLM（通过 ai.Provider），收集文本响应和工具调用
//   - 自动执行工具调用并注入结果，往复直到 LLM 给出最终回答
//
// 使用示例：
//
//	prov := openai.New(openai.Config{APIKey: "sk-..."})
//	ag := agent.New(agent.Config{
//	    Provider: prov,
//	    ModelID:  "gpt-4o",
//	    Tools:    []tool.Tool{myTool},
//	})
//	for evt := range ag.Run(ctx, "请帮我查询天气") {
//	    switch evt.Type {
//	    case agent.EventTextDelta:
//	        fmt.Print(evt.Text)
//	    case agent.EventToolCall:
//	        fmt.Println("[调用工具]", evt.ToolCall.Name)
//	    case agent.EventDone:
//	        fmt.Println("\n完成")
//	    }
//	}
package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/Effortful-lion/pi-go/ai"
	"github.com/Effortful-lion/pi-go/lg"
	"github.com/Effortful-lion/pi-go/tool"
)

var logger = lg.Module("[agent]")

// 默认最大工具调用轮数。
const defaultMaxSteps = 10

// emojiThemeHint 是当配置了 emoji 主题时注入 system prompt 的结构化表达约束。
// 遵循 docs/emoji设计.md 第 83-88 行的设计规范。
const emojiThemeHint = `


[输出风格要求]
- 回复优先使用短段落和明确的小标题来组织内容
- 工具调用前后使用一致的语义标记（如标题、分隔线等）
- 不需要生成具体的 emoji 字符，只需保持清晰的语义结构`

// Config Agent 配置。
type Config struct {
	Provider     ai.Provider
	ModelID      string
	SystemPrompt string
	EmojiTheme   string   // 可选：emoji 主题名称，非空时在 system prompt 中注入结构化表达约束
	Tools        []tool.Tool
	MaxSteps     int     // 最大工具调用循环轮数，≤0 时使用默认值 10
	Temperature  float64 // LLM 采样温度，0 表示使用模型默认值
	MaxTokens    int     // LLM 最大输出 tokens，0 表示不限制
}

// Agent 对话型 Agent，管理 LLM ↔ 工具的自动循环。
type Agent struct {
	cfg      Config
	toolMap  map[string]tool.Tool
	messages []ai.Message
}

// New 创建 Agent。
func New(cfg Config) *Agent {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = defaultMaxSteps
	}

	tm := make(map[string]tool.Tool, len(cfg.Tools))
	for _, t := range cfg.Tools {
		tm[t.Name()] = t
	}

	a := &Agent{
		cfg:     cfg,
		toolMap: tm,
	}

	// 构建 system prompt，如果配置了 EmojiTheme 则追加结构化表达约束
	sysPrompt := cfg.SystemPrompt
	if cfg.EmojiTheme != "" {
		sysPrompt += emojiThemeHint
	}

	if sysPrompt != "" {
		a.messages = append(a.messages, ai.Message{
			Role:    ai.RoleSystem,
			Content: sysPrompt,
		})
	}

	return a
}

// Run 启动一轮 Agent 对话循环。
//
// 返回 Stream 事件通道。goroutine 中执行 LLM 调用和工具循环，
// 完成后自动关闭通道。userInput 为空时不追加任何消息（用于 Continue 场景）。
func (a *Agent) Run(ctx context.Context, userInput string) Stream {
	out := make(chan Event, 8)

	// 将用户输入入队（空输入跳过，用于 Continue 场景）
	if userInput != "" {
		a.messages = append(a.messages, ai.Message{
			Role:    ai.RoleUser,
			Content: userInput,
		})
	}

	go func() {
		defer close(out)
		a.runLoop(ctx, out)
	}()

	return out
}

// Continue 在现有对话基础上继续，可选注入额外上下文。
//
// extraContext 非空时会被作为一条 user 消息追加到历史中。
// 不追加新的空 user 消息，直接让 LLM 基于当前历史继续生成。
func (a *Agent) Continue(ctx context.Context, extraContext string) Stream {
	if extraContext != "" {
		a.messages = append(a.messages, ai.Message{
			Role:    ai.RoleUser,
			Content: extraContext,
		})
	}
	return a.Run(ctx, "")
}

// Reset 清空对话历史，保留 system prompt，开始新对话。
func (a *Agent) Reset() {
	a.messages = nil
	if a.cfg.SystemPrompt != "" {
		a.messages = append(a.messages, ai.Message{
			Role:    ai.RoleSystem,
			Content: a.cfg.SystemPrompt,
		})
	}
}

// Messages 返回当前对话历史快照（只读）。
func (a *Agent) Messages() []ai.Message {
	cp := make([]ai.Message, len(a.messages))
	copy(cp, a.messages)
	return cp
}

// runLoop Agent 对话循环核心，从 Run 和 Continue 中抽取。
func (a *Agent) runLoop(ctx context.Context, out chan<- Event) {
	for step := 1; step <= a.cfg.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			out <- Event{Type: EventError, Step: step, Err: err}
			return
		}

		out <- Event{Type: EventStepStart, Step: step}

		// 构造 ai.Context
		actx := a.buildContext()

		// 调用 LLM
		stream := a.cfg.Provider.Chat(ctx, a.cfg.ModelID, actx)

		// 消费 ai.Event 流
		textParts := make([]string, 0, 4)          // 累积文本片段
		toolCallAcc := make(map[int]*ai.ToolCall)  // index → 累积中的工具调用
		toolCallsOrdered := make([]*ai.ToolCall, 0) // 按 index 排序的最终工具调用列表
		var lastUsage *ai.Usage

		for evt := range stream {
			switch evt.Type {
			case ai.EventTextStart:
				// 文本块开始，当前不做额外处理
			case ai.EventTextDelta:
				textParts = append(textParts, evt.Text)
				out <- Event{Type: EventTextDelta, Text: evt.Text, Step: step}
			case ai.EventTextEnd:
				// 文本块结束
			case ai.EventThinkingStart, ai.EventThinkingDelta, ai.EventThinkingEnd:
				// 思考内容暂不暴露给上层，仅做日志记录
			case ai.EventToolCallStart:
				tc := &ai.ToolCall{
					ID:   evt.TC.ID,
					Name: evt.TC.Name,
				}
				toolCallAcc[evt.Index] = tc
			case ai.EventToolCallDelta:
				if tc, ok := toolCallAcc[evt.Index]; ok {
					tc.Arguments += evt.TC.Arguments
				}
			case ai.EventToolCallEnd:
				if tc, ok := toolCallAcc[evt.Index]; ok {
					toolCallsOrdered = append(toolCallsOrdered, tc)
					delete(toolCallAcc, evt.Index)
				}
			case ai.EventDone:
				lastUsage = evt.Usage
			case ai.EventError:
				out <- Event{Type: EventError, Step: step, Err: evt.Err}
				return
			}
		}

		// 流结束，发送 StepEnd
		out <- Event{Type: EventStepEnd, Step: step, Usage: lastUsage}

		// 构建 AssistantMessage
		assistantMsg := ai.Message{
			Role: ai.RoleAssistant,
		}
		// 拼接文本
		for _, p := range textParts {
			assistantMsg.Content += p
		}
		// 添加工具调用 blocks
		for _, tc := range toolCallsOrdered {
			assistantMsg.Blocks = append(assistantMsg.Blocks, ai.ContentBlock{
				Type:     ai.BlockToolCall,
				ToolCall: tc,
			})
		}

		// 没有工具调用 → 对话结束
		if len(toolCallsOrdered) == 0 {
			a.messages = append(a.messages, assistantMsg)
			out <- Event{Type: EventDone, Step: step}
			return
		}

		// 有工具调用 → 执行并注入结果
		a.messages = append(a.messages, assistantMsg)

		for _, tc := range toolCallsOrdered {
			out <- Event{Type: EventToolCall, Step: step, ToolCall: tc}

			t, ok := a.toolMap[tc.Name]
			if !ok {
				err := fmt.Errorf("unknown tool: %s", tc.Name)
				logger.Warn(err.Error())
				out <- Event{Type: EventError, Step: step, Err: err}
				return
			}

			result, execErr := t.Execute(ctx, tc.Arguments)
			if execErr != nil {
				result = fmt.Sprintf("tool execution error: %v", execErr)
				logger.Warn("tool execution failed", lg.Fields{"tool": tc.Name, "error": execErr.Error()})
			}

			out <- Event{Type: EventToolResult, Step: step, ToolResult: result}

			// 注入工具结果到历史
			a.messages = append(a.messages, ai.Message{
				Role:       ai.RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
			})
		}
	}

	// 超过 MaxSteps
	out <- Event{
		Type: EventError,
		Step: a.cfg.MaxSteps,
		Err:  errors.New("max steps exceeded: agent could not complete within the step limit"),
	}
}

// buildContext 从 Agent 当前状态构建 ai.Context。
func (a *Agent) buildContext() ai.Context {
	// 收集所有工具的 Definition
	toolDefs := make([]ai.ToolDefinition, 0, len(a.cfg.Tools))
	for _, t := range a.cfg.Tools {
		toolDefs = append(toolDefs, t.Definition())
	}

	ctx := ai.Context{
		Messages:    a.messages,
		Tools:       toolDefs,
		MaxTokens:   a.cfg.MaxTokens,
		Temperature: a.cfg.Temperature,
		EmojiTheme:  a.cfg.EmojiTheme,
	}

	return ctx
}
