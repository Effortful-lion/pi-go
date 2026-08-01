//go:build !windows

package main

import (
	"flag"
	"os"
)

// run ChatFlags 定义见 main.go

func run() error {
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
		printVersion()
		return nil
	case "help", "-help", "--help":
		printHelp()
		return nil
	default:
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
	fs.StringVar(&flags.EmojiTheme, "emoji-theme", "", "Emoji theme (default/minimal/monochrome)")
	fs.IntVar(&flags.MaxSteps, "max-steps", 0, "Max tool-calling steps")
	fs.Float64Var(&flags.Temperature, "temperature", 0, "Sampling temperature")
	fs.IntVar(&flags.MaxTokens, "max-tokens", 0, "Max output tokens")
	fs.StringVar(&flags.Session, "session", "", "Session name for resume")
	fs.Parse(args)

	cfg := loadConfig()
	return runChat(cfg, &flags)
}
