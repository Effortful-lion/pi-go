package builtin

import (
	"github.com/Effortful-lion/pi-go/emoji"
)

// MinimalTheme 极简主题：仅用 ASCII 符号
var MinimalTheme = emoji.NewTheme("minimal", map[emoji.Slot]string{
	emoji.SlotAssistant:  ">",
	emoji.SlotUser:       ">",
	emoji.SlotToolCall:   "[tool]",
	emoji.SlotToolResult: "[result]",
	emoji.SlotStep:       "->",
	emoji.SlotSuccess:    "[ok]",
	emoji.SlotWarning:    "[!]",
	emoji.SlotError:      "[x]",
})
