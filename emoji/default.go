package emoji

// initDefaultRegistry 创建并初始化默认主题注册表
// 包含：default、minimal、monochrome 三个内置主题
func initDefaultRegistry() *Registry {
	registry := NewRegistry()

	// 注册默认主题
	registry.Register(NewTheme("default", map[Slot]string{
		SlotAssistant:  "🤖",
		SlotUser:       "💬",
		SlotToolCall:   "🔧",
		SlotToolResult: "📋",
		SlotStep:       "▶",
		SlotSuccess:    "✅",
		SlotWarning:    "⚠️",
		SlotError:      "✖",
	}))

	// 注册极简主题
	registry.Register(NewTheme("minimal", map[Slot]string{
		SlotAssistant:  ">",
		SlotUser:       ">",
		SlotToolCall:   "[tool]",
		SlotToolResult: "[result]",
		SlotStep:       "->",
		SlotSuccess:    "[ok]",
		SlotWarning:    "[!]",
		SlotError:      "[x]",
	}))

	// 注册单色主题
	registry.Register(NewTheme("monochrome", map[Slot]string{
		SlotAssistant:  "[A]",
		SlotUser:       "[U]",
		SlotToolCall:   "[T]",
		SlotToolResult: "[R]",
		SlotStep:       "[>]",
		SlotSuccess:    "[+]",
		SlotWarning:    "[!]",
		SlotError:      "[-]",
	}))

	return registry
}

// DefaultRegistry 全局默认主题注册表（只读访问）
// 包含所有内置主题
var DefaultRegistry = initDefaultRegistry()

// DefaultResolver 全局默认解析器
// 使用 "default" 主题
var DefaultResolver = NewResolver(DefaultRegistry, "default")
