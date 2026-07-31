package builtin

import (
	"github.com/Effortful-lion/pi-go/emoji"
)

// DefaultTheme 默认主题：完整 emoji 风格
var DefaultTheme = emoji.NewTheme("default", map[emoji.Slot]string{
	emoji.SlotAssistant:  "🤖",
	emoji.SlotUser:       "💬",
	emoji.SlotToolCall:   "🔧",
	emoji.SlotToolResult: "📋",
	emoji.SlotStep:       "▶",
	emoji.SlotSuccess:    "✅",
	emoji.SlotWarning:    "⚠️",
	emoji.SlotError:      "✖",
})

// Register 注册默认主题到注册表
func Register(registry *emoji.Registry) {
	registry.Register(DefaultTheme)
}
