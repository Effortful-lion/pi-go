package emoji

// Resolver emoji 解析器：根据当前主题和回退链解析最终显示值
type Resolver struct {
	registry  *Registry
	themeName string
	fallback  Theme // 纯文本回退主题
}

// NewResolver 创建解析器
// themeName 为空时自动使用 "default" 主题（如果存在），否则使用纯文本回退
func NewResolver(registry *Registry, themeName string) *Resolver {
	// 如果未指定主题，尝试使用内置 default 主题
	if themeName == "" {
		if _, ok := registry.Get("default"); ok {
			themeName = "default"
		}
	}

	// 内置纯文本回退主题
	fallback := NewTheme("fallback", map[Slot]string{
		SlotAssistant:  "[A]",
		SlotUser:       "[U]",
		SlotToolCall:   "[T]",
		SlotToolResult: "[R]",
		SlotStep:       "[>]",
		SlotSuccess:    "[OK]",
		SlotWarning:    "[!]",
		SlotError:      "[X]",
	})

	return &Resolver{
		registry:  registry,
		themeName: themeName,
		fallback:  fallback,
	}
}

// Resolve 根据槽位解析最终显示字符串
// 回退链：当前主题 → registry → 内置默认 → fallback
func (res *Resolver) Resolve(slot Slot) string {
	// 1. 尝试获取当前主题
	if res.themeName != "" {
		if theme, ok := res.registry.Get(res.themeName); ok {
			if val, ok := theme.Slots[slot]; ok && val != "" {
				return val
			}
		}
	}

	// 2. 尝试 fallback 主题
	if val, ok := res.fallback.Slots[slot]; ok && val != "" {
		return val
	}

	// 3. 最后兜底：返回槽位名称的大写
	return "[" + string(slot) + "]"
}

// ThemeName 返回当前使用的主题名称
func (res *Resolver) ThemeName() string {
	return res.themeName
}

// SetTheme 动态切换主题
func (res *Resolver) SetTheme(name string) {
	res.themeName = name
}
