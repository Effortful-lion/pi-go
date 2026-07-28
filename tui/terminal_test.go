package tui

import (
	"strings"
	"testing"
)

func TestStyleText_Plain(t *testing.T) {
	result := StyleText("hello", Style{})
	if result != "hello" {
		t.Errorf("expected plain text, got %q", result)
	}
}

func TestStyleText_Bold(t *testing.T) {
	result := StyleText("hello", Style{Bold: true})
	if !strings.HasPrefix(result, "\033[1m") {
		t.Errorf("expected bold prefix \\033[1m, got %q", result)
	}
	if !strings.HasSuffix(result, "\033[0m") {
		t.Errorf("expected reset suffix \\033[0m, got %q", result)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("expected to contain 'hello', got %q", result)
	}
}

func TestStyleText_Red(t *testing.T) {
	result := StyleText("error", Style{Fg: ColorRed})
	if !strings.HasPrefix(result, "\033[31m") {
		t.Errorf("expected red prefix \\033[31m, got %q", result)
	}
	if !strings.HasSuffix(result, "\033[0m") {
		t.Errorf("expected reset suffix, got %q", result)
	}
}

func TestStyleText_BoldRed(t *testing.T) {
	result := StyleText("alert", Style{Fg: ColorRed, Bold: true})
	if !strings.HasPrefix(result, "\033[1;31m") {
		t.Errorf("expected bold+red prefix \\033[1;31m, got %q", result)
	}
}

func TestStyleText_AllStyles(t *testing.T) {
	result := StyleText("full", Style{Fg: ColorGreen, Bold: true, Dim: true, Italic: true, Underline: true})
	if !strings.HasPrefix(result, "\033[1;2;3;4;32m") {
		t.Errorf("expected all styles prefix \\033[1;2;3;4;32m, got %q", result)
	}
}

func TestStyleText_DefaultColor(t *testing.T) {
	// ColorDefault (39) should not produce color code
	result := StyleText("default", Style{Fg: ColorDefault})
	if result != "default" {
		t.Errorf("expected plain text for ColorDefault, got %q", result)
	}
}

func TestStyleText_ZeroColor(t *testing.T) {
	// 0 color should not produce color code
	result := StyleText("zero", Style{Fg: 0})
	if result != "zero" {
		t.Errorf("expected plain text for zero color, got %q", result)
	}
}

func TestUtf8ByteLen(t *testing.T) {
	tests := []struct {
		name string
		b    byte
		want int
	}{
		{"ASCII", 'A', 1},
		{"2-byte lead (C3)", 0xC3, 2},
		{"3-byte lead (E4, 中)", 0xE4, 3},
		{"3-byte lead (E5, 好)", 0xE5, 3},
		{"4-byte lead (F0)", 0xF0, 4},
		{"continuation (B8)", 0xB8, 0},
		{"invalid (FE)", 0xFE, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utf8ByteLen(tt.b); got != tt.want {
				t.Errorf("utf8ByteLen(0x%X) = %d, want %d", tt.b, got, tt.want)
			}
		})
	}
}

func TestConvenienceFunctions(t *testing.T) {
	const testText = "text"
	tests := []struct {
		name string
		fn   func(string) string
		want string
	}{
		{"Bold", Bold, "\033[1mtext\033[0m"},
		{"Dim", Dim, "\033[2mtext\033[0m"},
		{"Red", Red, "\033[31mtext\033[0m"},
		{"Green", Green, "\033[32mtext\033[0m"},
		{"Blue", Blue, "\033[34mtext\033[0m"},
		{"Cyan", Cyan, "\033[36mtext\033[0m"},
		{"Gray", Gray, "\033[90mtext\033[0m"},
		{"Yellow", Yellow, "\033[33mtext\033[0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(testText)
			if result != tt.want {
				t.Errorf("%s(%q) = %q, want %q", tt.name, testText, result, tt.want)
			}
		})
	}
}
