package emoji

// Slot 语义槽位类型
type Slot string

const (
	// SlotAssistant 助手标识
	SlotAssistant Slot = "assistant"
	// SlotUser 用户标识
	SlotUser Slot = "user"
	// SlotToolCall 工具调用标识
	SlotToolCall Slot = "tool_call"
	// SlotToolResult 工具结果标识
	SlotToolResult Slot = "tool_result"
	// SlotStep 步骤标识
	SlotStep Slot = "step"
	// SlotSuccess 成功标识
	SlotSuccess Slot = "success"
	// SlotWarning 警告标识
	SlotWarning Slot = "warning"
	// SlotError 错误标识
	SlotError Slot = "error"
)

// Theme 主题定义：名称 + 槽位映射
type Theme struct {
	Name  string
	Slots map[Slot]string
}

// NewTheme 创建主题（槽位为 nil 时自动初始化为空 map）
func NewTheme(name string, slots map[Slot]string) Theme {
	if slots == nil {
		slots = make(map[Slot]string)
	}
	return Theme{
		Name:  name,
		Slots: slots,
	}
}
