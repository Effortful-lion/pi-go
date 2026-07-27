// Command pi is the Pi Agent CLI — an interactive AI coding assistant.
//
// Usage:
//
//	pi -api-key="sk-..."
//	pi -model="gpt-4o-mini" -api-key="sk-..."
//
// Environment variables:
//
//	OPENAI_API_KEY    API key (used when -api-key is not set)
//	PI_PROVIDER       LLM provider (default: openai)
//	PI_MODEL          Model ID (default: gpt-4o)
//	PI_BASE_URL       Custom API base URL
//	PI_SYSTEM_PROMPT  System prompt
//	PI_MAX_STEPS      Max tool-calling steps (default: 10)
//	PI_TEMPERATURE    Sampling temperature (default: 0.7)
//	PI_MAX_TOKENS     Max output tokens (default: 0 = no limit)
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/Effortful-lion/pi-go/agent"
	"github.com/Effortful-lion/pi-go/ai/providers/openai"
	"github.com/Effortful-lion/pi-go/tui"
)

const version = "0.1.0"

const defaultSystemPrompt = `You are Pi Agent, an AI coding assistant.
You help with writing code, debugging, answering technical questions, and more.
Be concise, helpful, and use tools when appropriate.`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "pi: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 命令行参数
	var (
		provider     string
		model        string
		apiKey       string
		baseURL      string
		systemPrompt string
		maxSteps     int
		temperature  float64
		maxTokens    int
		showVersion  bool
	)

	flag.StringVar(&provider, "provider", "openai", "LLM provider")
	flag.StringVar(&model, "model", "gpt-4o", "Model ID")
	flag.StringVar(&apiKey, "api-key", "", "API key")
	flag.StringVar(&baseURL, "base-url", "", "Custom API base URL")
	flag.StringVar(&systemPrompt, "system-prompt", "", "System prompt")
	flag.IntVar(&maxSteps, "max-steps", 10, "Max tool-calling steps")
	flag.Float64Var(&temperature, "temperature", 0.7, "Sampling temperature")
	flag.IntVar(&maxTokens, "max-tokens", 0, "Max output tokens (0 = no limit)")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.Parse()

	if showVersion {
		fmt.Println("pi version", version)
		return nil
	}

	// 环境变量回退：仅当命令行参数未显式设置时使用
	resolveFromEnv(flag.CommandLine, map[string]string{
		"api-key":       "OPENAI_API_KEY",
		"provider":      "PI_PROVIDER",
		"model":         "PI_MODEL",
		"base-url":      "PI_BASE_URL",
		"system-prompt": "PI_SYSTEM_PROMPT",
		"max-steps":     "PI_MAX_STEPS",
		"temperature":   "PI_TEMPERATURE",
		"max-tokens":    "PI_MAX_TOKENS",
	})

	if apiKey == "" {
		return fmt.Errorf("API key is required: set -api-key flag or OPENAI_API_KEY environment variable")
	}

	// 系统提示词默认值
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}

	// 创建 Provider
	prov := openai.New(openai.Config{
		APIKey:  apiKey,
		BaseURL: baseURL,
	})

	// 创建 Agent
	ag := agent.New(agent.Config{
		Provider:     prov,
		ModelID:      model,
		SystemPrompt: systemPrompt,
		MaxSteps:     maxSteps,
		Temperature:  temperature,
		MaxTokens:    maxTokens,
	})

	// 启动交互式对话
	ui := tui.NewChatUI(ag)
	return ui.Run(context.Background())
}

// resolveFromEnv 将未显式设置的 flag 从环境变量回退。
// 通过 flag.Visit 判断哪些 flag 被显式设置过。
func resolveFromEnv(fs *flag.FlagSet, mapping map[string]string) {
	visited := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})

	for name, envKey := range mapping {
		if visited[name] {
			continue
		}
		if val, ok := os.LookupEnv(envKey); ok && val != "" {
			switch name {
			case "max-steps":
				if n, err := strconv.Atoi(val); err == nil {
					fs.Set(name, val)
					_ = n
				}
			case "temperature":
				if _, err := strconv.ParseFloat(val, 64); err == nil {
					fs.Set(name, val)
				}
			case "max-tokens":
				if _, err := strconv.Atoi(val); err == nil {
					fs.Set(name, val)
				}
			default:
				fs.Set(name, val)
			}
		}
	}
}
