package emoji

import (
	"testing"
)

func TestTheme(t *testing.T) {
	slots := map[Slot]string{
		SlotAssistant: "🤖",
		SlotUser:      "💬",
	}
	theme := NewTheme("test", slots)

	if theme.Name != "test" {
		t.Errorf("Theme.Name = %q, want %q", theme.Name, "test")
	}
	if theme.Slots[SlotAssistant] != "🤖" {
		t.Errorf("Theme.Slots[SlotAssistant] = %q, want %q", theme.Slots[SlotAssistant], "🤖")
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()
	theme := NewTheme("test", map[Slot]string{SlotAssistant: "🤖"})

	registry.Register(theme)

	got, ok := registry.Get("test")
	if !ok {
		t.Fatal("registry.Get(\"test\") = false, want true")
	}
	if got.Name != "test" {
		t.Errorf("theme.Name = %q, want %q", got.Name, "test")
	}

	// 测试不存在的主题
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("registry.Get(\"nonexistent\") = true, want false")
	}
}

func TestRegistryOverwrite(t *testing.T) {
	registry := NewRegistry()
	theme1 := NewTheme("test", map[Slot]string{SlotAssistant: "🤖"})
	theme2 := NewTheme("test", map[Slot]string{SlotAssistant: "👻"})

	registry.Register(theme1)
	registry.Register(theme2)

	got, _ := registry.Get("test")
	if got.Slots[SlotAssistant] != "👻" {
		t.Errorf("after overwrite, SlotAssistant = %q, want %q", got.Slots[SlotAssistant], "👻")
	}
}

func TestResolver(t *testing.T) {
	registry := NewRegistry()
	theme := NewTheme("test", map[Slot]string{
		SlotAssistant:  "🤖",
		SlotUser:       "💬",
		SlotToolCall:   "🔧",
		SlotToolResult: "📋",
		SlotStep:       "▶",
		SlotSuccess:    "✅",
		SlotWarning:    "⚠️",
		SlotError:      "✖",
	})
	registry.Register(theme)

	resolver := NewResolver(registry, "test")

	tests := []struct {
		slot     Slot
		expected string
	}{
		{SlotAssistant, "🤖"},
		{SlotUser, "💬"},
		{SlotToolCall, "🔧"},
		{SlotToolResult, "📋"},
		{SlotStep, "▶"},
		{SlotSuccess, "✅"},
		{SlotWarning, "⚠️"},
		{SlotError, "✖"},
	}

	for _, tt := range tests {
		t.Run(string(tt.slot), func(t *testing.T) {
			got := resolver.Resolve(tt.slot)
			if got != tt.expected {
				t.Errorf("Resolve(%q) = %q, want %q", tt.slot, got, tt.expected)
			}
		})
	}
}

func TestResolverFallback(t *testing.T) {
	registry := NewRegistry()
	// 注册一个只有部分槽位的主题
	registry.Register(NewTheme("partial", map[Slot]string{
		SlotAssistant: "🤖",
		// SlotUser 缺失
	}))

	resolver := NewResolver(registry, "partial")

	// SlotAssistant 应该从主题获取
	if got := resolver.Resolve(SlotAssistant); got != "🤖" {
		t.Errorf("Resolve(SlotAssistant) = %q, want %q", got, "🤖")
	}

	// SlotUser 缺失，应该回退到 fallback
	if got := resolver.Resolve(SlotUser); got != "[U]" {
		t.Errorf("Resolve(SlotUser) = %q, want %q (fallback)", got, "[U]")
	}
}

func TestResolverEmptyTheme(t *testing.T) {
	registry := NewRegistry()
	// 注册一个 default 主题
	registry.Register(NewTheme("default", map[Slot]string{
		SlotAssistant: "🤖",
		SlotUser:      "💬",
	}))

	resolver := NewResolver(registry, "")

	// 空主题应该自动使用 default
	if got := resolver.Resolve(SlotAssistant); got != "🤖" {
		t.Errorf("Resolve(SlotAssistant) = %q, want %q", got, "🤖")
	}
	if got := resolver.Resolve(SlotUser); got != "💬" {
		t.Errorf("Resolve(SlotUser) = %q, want %q", got, "💬")
	}
}

func TestResolverNonexistentTheme(t *testing.T) {
	registry := NewRegistry()
	resolver := NewResolver(registry, "nonexistent")

	// 不存在的主题应该使用 fallback
	if got := resolver.Resolve(SlotError); got != "[X]" {
		t.Errorf("Resolve(SlotError) = %q, want %q", got, "[X]")
	}
}

func TestDefaultRegistry(t *testing.T) {
	// 测试内置主题是否存在
	if !DefaultRegistry.Has("default") {
		t.Error("DefaultRegistry missing \"default\" theme")
	}
	if !DefaultRegistry.Has("minimal") {
		t.Error("DefaultRegistry missing \"minimal\" theme")
	}
	if !DefaultRegistry.Has("monochrome") {
		t.Error("DefaultRegistry missing \"monochrome\" theme")
	}
}

func TestDefaultResolver(t *testing.T) {
	// 测试默认解析器使用 default 主题
	if DefaultResolver.ThemeName() != "default" {
		t.Errorf("DefaultResolver.ThemeName() = %q, want %q", DefaultResolver.ThemeName(), "default")
	}

	got := DefaultResolver.Resolve(SlotAssistant)
	if got != "🤖" {
		t.Errorf("DefaultResolver.Resolve(SlotAssistant) = %q, want %q", got, "🤖")
	}
}

func TestResolverSetTheme(t *testing.T) {
	registry := NewRegistry()
	registry.Register(NewTheme("test1", map[Slot]string{SlotAssistant: "🤖"}))
	registry.Register(NewTheme("test2", map[Slot]string{SlotAssistant: "👻"}))

	resolver := NewResolver(registry, "test1")
	if got := resolver.Resolve(SlotAssistant); got != "🤖" {
		t.Errorf("initial theme, Resolve(SlotAssistant) = %q, want %q", got, "🤖")
	}

	resolver.SetTheme("test2")
	if got := resolver.Resolve(SlotAssistant); got != "👻" {
		t.Errorf("after SetTheme, Resolve(SlotAssistant) = %q, want %q", got, "👻")
	}
}
