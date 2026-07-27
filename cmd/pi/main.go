// Command pi is the Pi Agent CLI — an interactive AI coding assistant.
//
// Usage:
//
//	pi                                    # 启动对话（默认命令）
//	pi chat                               # 同上
//	pi config init                        # 创建默认配置
//	pi config show                        # 显示当前配置
//	pi config set <key> <value>           # 修改配置
//	pi version                            # 显示版本
//	pi help                               # 显示帮助
//
// Flags (仅 chat 命令):
//
//	-model="gpt-4o"        模型 ID
//	-api-key="sk-..."      API Key
//	-provider="openai"     LLM 提供商 (openai/anthropic/google)
//	-base-url=""           自定义 API 地址
//	-system-prompt=""      系统提示词
//	-max-steps=10          工具调用最大轮数
//	-temperature=0.7       采样温度
//	-max-tokens=0          最大输出 token 数
//	-session=""            Session 名称（恢复对话）
//
// 参数优先级: 命令行 > 环境变量 > 配置文件 > 默认值
package main

import (
	"flag"
	"fmt"
	"os"
)

const version = "1.0.0"

const defaultSystemPrompt = `You are Pi Agent, an AI coding assistant.
You help with writing code, debugging, answering technical questions, and more.
Be concise, helpful, and use tools when appropriate.`

// ChatFlags chat 命令的命令行参数。
type ChatFlags struct {
	Provider     string
	Model        string
	APIKey       string
	BaseURL      string
	SystemPrompt string
	MaxSteps     int
	Temperature  float64
	MaxTokens    int
	Session      string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "pi: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 子命令路由
	args := os.Args[1:]
	if len(args) == 0 {
		return runChatCmd(args)
	}

	switch args[0] {
	case "chat":
		return runChatCmd(args[1:])
	case "config":
		runConfig(args[1:])
		return nil
	case "version", "-version", "--version":
		fmt.Println("pi version", version)
		return nil
	case "help", "-help", "--help":
		printHelp()
		return nil
	default:
		// 非子命令 → 可能是 -flag 直传形式
		if len(args) > 0 && args[0][0] == '-' {
			return runChatCmd(args)
		}
		printHelp()
		return nil
	}
}

func runChatCmd(args []string) error {
	var flags ChatFlags

	fs := flag.NewFlagSet("chat", flag.ExitOnError)
	fs.StringVar(&flags.Provider, "provider", "", "LLM provider (openai/anthropic/google)")
	fs.StringVar(&flags.Model, "model", "", "Model ID")
	fs.StringVar(&flags.APIKey, "api-key", "", "API key")
	fs.StringVar(&flags.BaseURL, "base-url", "", "Custom API base URL")
	fs.StringVar(&flags.SystemPrompt, "system-prompt", "", "System prompt")
	fs.IntVar(&flags.MaxSteps, "max-steps", 0, "Max tool-calling steps")
	fs.Float64Var(&flags.Temperature, "temperature", 0, "Sampling temperature")
	fs.IntVar(&flags.MaxTokens, "max-tokens", 0, "Max output tokens")
	fs.StringVar(&flags.Session, "session", "", "Session name for resume")
	fs.Parse(args)

	cfg := loadConfig()
	return runChat(cfg, &flags)
}

func printHelp() {
	fmt.Print(`Pi Agent — AI Coding Assistant

用法:
  pi                           启动交互式对话（默认）
  pi chat [flags]              同上
  pi config init               创建默认配置文件 (~/.pi-go/config.yaml)
  pi config show               显示当前配置
  pi config set <key> <value>  修改配置项
  pi version                   显示版本信息
  pi help                      显示此帮助

Flags:
  -provider      LLM 提供商 (openai/anthropic/google)
  -model         Model ID
  -api-key       API Key
  -base-url      自定义 API 地址
  -system-prompt 系统提示词
  -max-steps     工具调用最大轮数 (默认 10)
  -temperature   采样温度 (默认 0.7)
  -max-tokens    最大输出 token 数
  -session       Session 名称（恢复对话）

配置文件: ~/.pi-go/config.yaml 或 ./.pi-go.yaml
环境变量: OPENAI_API_KEY, PI_PROVIDER, PI_MODEL, PI_BASE_URL,
          PI_SYSTEM_PROMPT, PI_MAX_STEPS, PI_TEMPERATURE, PI_MAX_TOKENS
`)
}
