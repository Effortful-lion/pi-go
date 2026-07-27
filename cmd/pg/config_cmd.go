package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// runConfig 处理 config 子命令。
func runConfig(args []string) {
	if len(args) == 0 {
		args = []string{"show"}
	}
	switch args[0] {
	case "init":
		configInit()
	case "show":
		configShow()
	case "set":
		if len(args) < 3 {
			fmt.Println("用法: pg config set <key> <value>")
			fmt.Println("可设置的 key: provider, model, api_key, base_url, max_steps, temperature, max_tokens")
			return
		}
		configSet(args[1], args[2])
	default:
		fmt.Printf("未知 config 子命令: %s\n", args[0])
		fmt.Println("用法: pg config {init|show|set}")
	}
}

func configInit() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法获取用户目录: %v\n", err)
		return
	}
	cfg := &PiConfig{
		Provider:    "openai",
		Model:       "gpt-4o",
		MaxSteps:    10,
		Temperature: 0.7,
	}
	if err := saveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "创建配置文件失败: %v\n", err)
		return
	}
	fmt.Printf("配置文件已创建: %s\n", home+"/.pi-go/config.yaml")
	fmt.Println("请编辑该文件设置 api_key 和其他选项。")
}

func configShow() {
	configPaths := []string{
		".pi-go.yaml",
	}
	if home, err := os.UserHomeDir(); err == nil {
		configPaths = append(configPaths, home+"/.pi-go/config.yaml")
	}

	found := false
	for _, p := range configPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		found = true
		fmt.Printf("[%s]\n", p)
		fmt.Println(string(data))
	}

	if !found {
		fmt.Println("未找到配置文件。运行 pg config init 创建默认配置。")
		return
	}
}

func configSet(key, value string) {
	cfg := loadConfig()
	// 确保有基础配置
	if cfg == nil {
		cfg = &PiConfig{MaxSteps: 10, Temperature: 0.7}
	}

	switch key {
	case "provider":
		cfg.Provider = value
	case "model":
		cfg.Model = value
	case "api_key":
		cfg.APIKey = value
	case "base_url":
		cfg.BaseURL = value
	case "system_prompt":
		cfg.SystemPrompt = value
	case "max_steps":
		fmt.Sscanf(value, "%d", &cfg.MaxSteps)
	case "temperature":
		fmt.Sscanf(value, "%f", &cfg.Temperature)
	case "max_tokens":
		fmt.Sscanf(value, "%d", &cfg.MaxTokens)
	default:
		fmt.Printf("未知配置项: %s\n", key)
		fmt.Println("可设置的 key: provider, model, api_key, base_url, max_steps, temperature, max_tokens")
		return
	}

	if err := saveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
		return
	}

	data, _ := yaml.Marshal(cfg)
	fmt.Printf("配置已更新:\n%s\n", string(data))
}
