package builtin

import (
	"github.com/Effortful-lion/pi-go/emoji"
)

// MonochromeTheme 单色主题：使用简单几何符号
var MonochromeTheme = emoji.NewTheme("monochrome", map[emoji.Slot]string{
	emoji.SlotAssistant:  "[A]",
	emoji.SlotUser:       "[U]",
	emoji.SlotToolCall:   "[T]",
	emoji.SlotToolResult: "[R]",
	emoji.SlotStep:       "[>]",
	emoji.SlotSuccess:    "[+]",
	emoji.SlotWarning:    "[!]",
	emoji.SlotError:      "[-]",
})
