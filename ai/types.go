// Package ai 提供多提供商 LLM 统一接口。
// 本文件定义消息、内容块、工具、上下文等基础类型。
package ai

// Role 消息角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 一条对话消息。
//
// UserMessage/SystemMessage 使用 Content 字段携带文本内容，
// AssistantMessage 使用 Blocks 字段携带富内容（文本、思考、工具调用等），
// ToolResultMessage 使用 Content 携带工具执行结果，ToolCallID 回溯到对应 ToolCall。
type Message struct {
	Role       Role           `json:"role,omitempty"`
	Content    string         `json:"content,omitempty"`      // 纯文本内容（User/System/Tool 消息使用）
	Blocks     []ContentBlock `json:"blocks,omitempty"`       // 富内容块（Assistant 消息使用）
	ToolCallID string         `json:"tool_call_id,omitempty"` // 工具调用回溯 ID（Tool 消息使用）
	ToolName   string         `json:"tool_name,omitempty"`    // 工具名称（Tool 消息使用）
}

// ContentBlockType 内容块类型。
type ContentBlockType string

const (
	BlockText     ContentBlockType = "text"
	BlockThinking ContentBlockType = "thinking"
	BlockToolCall ContentBlockType = "tool_call"
	BlockImage    ContentBlockType = "image"
)

// ContentBlock 富文本内容块，仅在 Assistant 消息中出现。
// 单字段有效
type ContentBlock struct {
	Type     ContentBlockType
	Text     string    // BlockText 时使用
	Thinking string    // BlockThinking 时使用
	ToolCall *ToolCall // BlockToolCall 时使用
	ImageURL string    // BlockImage 时使用
}

// ToolDefinition Agent 可调用的工具定义。
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema 格式的参数定义
}

// ToolCall 助手发起的工具调用请求。
type ToolCall struct {
	ID        string // 本次调用的唯一 ID，用于 ToolResult 回溯
	Name      string // 工具名
	Arguments string // JSON 格式的调用参数
}

// Context 一次 Chat 请求的完整上下文。
type Context struct {
	Messages     []Message
	Tools        []ToolDefinition
	SystemPrompt string  // 快捷设置 SystemMessage（自动排在 Messages 最前）
	MaxTokens    int     // 最大输出 tokens，0 表示使用模型默认值
	Temperature  float64 // 采样温度，0 表示使用模型默认值
}
