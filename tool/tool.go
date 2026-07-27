// Package tool 定义 Agent 工具的接口。
//
// 工具是 Agent 能力边界的关键：LLM 通过 ToolDefinition 了解可用工具，
// Agent 运行时通过 Tool 接口执行工具并获取结果。
package tool

import (
	"context"

	"github.com/Effortful-lion/pi-go/ai"
)

// Tool Agent 可调用的工具。
//
// 每个 Tool 实现提供：
//   - Name：唯一标识，LLM 在 tool_call 中使用此名称
//   - Definition：传递给 LLM 的元信息（名称、描述、参数 JSON Schema）
//   - Execute：运行时执行，参数为 JSON 字符串，返回结果文本
//
// 典型实现是在 Name 和 Definition 返回固定值，在 Execute 中执行实际逻辑。
type Tool interface {
	Name() string
	Definition() ai.ToolDefinition
	Execute(ctx context.Context, args string) (string, error)
}
