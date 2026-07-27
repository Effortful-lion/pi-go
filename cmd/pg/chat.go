package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Effortful-lion/pi-go/agent"
	"github.com/Effortful-lion/pi-go/ai"
	"github.com/Effortful-lion/pi-go/ai/providers/anthropic"
	"github.com/Effortful-lion/pi-go/ai/providers/google"
	"github.com/Effortful-lion/pi-go/ai/providers/openai"
	"github.com/Effortful-lion/pi-go/tui"
)

// runChat 启动交互式 Chat 模式。
func runChat(cfg *PiConfig, cliFlags *ChatFlags) error {
	// 参数解析优先级：命令行 > 环境变量 > 配置文件 > 默认值
	apiKey := resolveString(cfg, "api-key", cliFlags.APIKey, "OPENAI_API_KEY")
	provider := resolveString(cfg, "provider", cliFlags.Provider, "PI_PROVIDER")
	model := resolveString(cfg, "model", cliFlags.Model, "PI_MODEL")
	baseURL := resolveString(cfg, "base-url", cliFlags.BaseURL, "PI_BASE_URL")
	systemPrompt := resolveString(cfg, "system-prompt", cliFlags.SystemPrompt, "PI_SYSTEM_PROMPT")
	maxSteps := resolveInt(cfg, "max-steps", cliFlags.MaxSteps, "PI_MAX_STEPS")
	temperature := resolveFloat(cfg, "temperature", cliFlags.Temperature, "PI_TEMPERATURE")
	maxTokens := resolveInt(cfg, "max-tokens", cliFlags.MaxTokens, "PI_MAX_TOKENS")

	// 默认值
	if provider == "" {
		provider = "openai"
	}
	if model == "" {
		model = "gpt-4o"
	}
	if apiKey == "" {
		return fmt.Errorf("API key is required: set -api-key flag, OPENAI_API_KEY env var, or configure it in ~/.pg-go/config.yaml")
	}
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}
	if maxSteps <= 0 {
		maxSteps = 10
	}

	// 创建 Provider（按 provider 名称选择）
	prov := newProvider(provider, apiKey, baseURL)

	// 创建 Agent
	ag := agent.New(agent.Config{
		Provider:     prov,
		ModelID:      model,
		SystemPrompt: systemPrompt,
		MaxSteps:     maxSteps,
		Temperature:  temperature,
		MaxTokens:    maxTokens,
	})

	// 开始对话
	ui := tui.NewChatUI(ag)
	if cliFlags.Session != "" {
		fmt.Fprintf(os.Stdout, tui.Dim("[session: %s]\n"), cliFlags.Session)
	}
	return ui.Run(context.Background())
}

// newProvider 根据提供商名称创建对应的 Provider。
func newProvider(name, apiKey, baseURL string) ai.Provider {
	switch name {
	case "openai":
		return openai.New(openai.Config{APIKey: apiKey, BaseURL: baseURL})
	case "anthropic":
		return anthropic.New(anthropic.Config{APIKey: apiKey, BaseURL: baseURL})
	case "google", "gemini":
		return google.New(google.Config{APIKey: apiKey, BaseURL: baseURL})
	default:
		fmt.Fprintf(os.Stderr, "warning: unknown provider %q, falling back to openai\n", name)
		return openai.New(openai.Config{APIKey: apiKey, BaseURL: baseURL})
	}
}
