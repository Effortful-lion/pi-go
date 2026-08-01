// Command pg is the Pi Agent CLI — an interactive AI coding assistant.
//
// Usage:
//
//	pg                                    # 启动对话（默认命令）
//	pg chat                               # 同上
//	pg config init                        # 创建默认配置
//	pg config show                        # 显示当前配置
//	pg config set <key> <value>           # 修改配置
//	pg version                            # 显示版本
//	pg help                               # 显示帮助
//
// Flags (仅 chat 命令):
//
//	-model="gpt-4o"        模型 ID
//	-api-key="sk-..."      API Key
//	-provider="openai"     LLM 提供商 (openai/anthropic/google)
//	-base-url=""           自定义 API 地址
//	-system-prompt=""      系统提示词
//	-emoji-theme=""        emoji 主题 (default/minimal/monochrome)
//	-max-steps=10          工具调用最大轮数
//	-temperature=0.7       采样温度
//	-max-tokens=0          最大输出 token 数
//	-session=""            Session 名称（恢复对话）
//
// 参数优先级: 命令行 > 环境变量 > 配置文件 > 默认值
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/Effortful-lion/pi-go/lg"
)

// ldflags 注入变量（构建时设置）。
// 版本号唯一来源是 git tag，不在此硬编码。
// goreleaser 发布时自动注入：-X main.version={{.Version}}
var (
	version   = "dev"   // 发布时自动注入 git tag
	commit    = "none"  // git rev-parse --short HEAD
	buildDate = "none"  // 构建时间 UTC
)

const defaultSystemPrompt = `You are Pi-Go Agent, an AI coding assistant.
You help with writing code, debugging, answering technical questions, and more.
Be concise, helpful, and use tools when appropriate.`

// ChatFlags chat 命令的命令行参数。
type ChatFlags struct {
	Provider     string
	Model        string
	APIKey       string
	BaseURL      string
	SystemPrompt string
	EmojiTheme   string // emoji 主题名称
	MaxSteps     int
	Temperature  float64
	MaxTokens    int
	Session      string
}

func main() {
	initLogging()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "pg: %v\n", err)
		os.Exit(1)
	}
}

// initLogging 将日志输出重定向到 logs/ 目录，按级别分文件存储。
//
// 目录结构:
//
//	logs/info/pg_2006-01-02.log     # INFO 级别
//	logs/error/pg_2006-01-02.log    # ERROR 及以上级别
//
// 不创建根目录汇总文件，避免 INFO 日志同时出现在根目录和子目录。
// INFO 日志不会出现在终端，只有 WARN/ERROR 及以上才会。
func initLogging() {
	_ = lg.SetPath("logs", lg.LevelInfo,
		lg.NewLogNamePattern().Module().Char("_").Date("2006-01-02"),
		lg.WithLevelDir(lg.LevelInfo, "info"),
		lg.WithLevelDir(lg.LevelError, "error"),
	)
}

func printVersion() {
	fmt.Printf("pg version %s\n", version)
	fmt.Printf("  commit:    %s\n", commit)
	fmt.Printf("  built:     %s\n", buildDate)
	fmt.Printf("  go:        %s\n", runtime.Version())
}

func printHelp() {
	fmt.Print(`Pi-Go Agent — AI Coding Assistant

用法:
  pg                           启动交互式对话（默认）
  pg chat [flags]              同上
  pg config init               创建默认配置文件 (~/.pi-go/config.yaml)
  pg config show               显示当前配置
  pg config set <key> <value>  修改配置项
  pg version                   显示版本信息
  pg help                      显示此帮助

Flags:
  -provider      LLM 提供商 (openai/anthropic/google)
  -model         Model ID
  -api-key       API Key
  -base-url      自定义 API 地址
  -system-prompt 系统提示词
  -emoji-theme   emoji 主题 (default/minimal/monochrome)
  -max-steps     工具调用最大轮数 (默认 10)
  -temperature   采样温度 (默认 0.7)
  -max-tokens    最大输出 token 数
  -session       Session 名称（恢复对话）

配置文件: ~/.pi-go/config.yaml 或 ./.pi-go.yaml
环境变量: OPENAI_API_KEY, PI_PROVIDER, PI_MODEL, PI_BASE_URL,
          PI_SYSTEM_PROMPT, PIGO_EMOJI_THEME, PI_MAX_STEPS, PI_TEMPERATURE, PI_MAX_TOKENS
`)
}
