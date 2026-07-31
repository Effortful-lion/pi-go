package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PiConfig 存储 Pi Agent 所有配置项。
type PiConfig struct {
	Provider     string  `yaml:"provider"`
	Model        string  `yaml:"model"`
	APIKey       string  `yaml:"api_key"`
	BaseURL      string  `yaml:"base_url,omitempty"`
	SystemPrompt string  `yaml:"system_prompt,omitempty"`
	MaxSteps     int     `yaml:"max_steps"`
	Temperature  float64 `yaml:"temperature"`
	MaxTokens    int     `yaml:"max_tokens,omitempty"`
}

// configFileExists 检查是否存在任何配置文件。
func configFileExists() bool {
	paths := []string{
		".pi-go.yaml",
	}
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".pi-go", "config.yaml"))
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// loadConfig 从以下路径按优先级搜索配置文件：
//  1. ./.pi-go.yaml (当前目录)
//  2. ~/.pi-go/config.yaml (用户目录)
//
// 无配置文件时返回零值配置。
func loadConfig() *PiConfig {
	paths := []string{
		".pi-go.yaml",
	}
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".pi-go", "config.yaml"))
	}

	var cfg PiConfig
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			continue
		}
		return &cfg
	}
	return &cfg
}

// saveConfig 将配置保存到 ~/.pi-go/config.yaml。
func saveConfig(cfg *PiConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("无法获取用户目录: %w", err)
	}
	cfgDir := filepath.Join(home, ".pi-go")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置: %w", err)
	}
	header := "# Pi Agent Configuration\n# See: pg config init --help\n\n"
	return os.WriteFile(
		filepath.Join(cfgDir, "config.yaml"),
		append([]byte(header), data...),
		0644,
	)
}

// resolveString 解析字符串参数：命令行 > 环境变量 > 配置文件 > 默认值。
func resolveString(cfg *PiConfig, name, cliVal, envKey string) string {
	// 1. 命令行（已在 flag 解析时设置，通过 cliVal 传入）
	if cliVal != "" {
		return cliVal
	}
	// 2. 环境变量
	if envVal := os.Getenv(envKey); envVal != "" {
		return envVal
	}
	// 3. 配置文件
	if cfg == nil {
		return ""
	}
	// 反射获取配置字段值
	return getConfigField(cfg, name)
}

// getConfigField 获取 PiConfig 字段值（字符串类型）。
func getConfigField(cfg *PiConfig, name string) string {
	switch name {
	case "provider":
		return cfg.Provider
	case "model":
		return cfg.Model
	case "api-key":
		return cfg.APIKey
	case "base-url":
		return cfg.BaseURL
	case "system-prompt":
		return cfg.SystemPrompt
	}
	return ""
}

// resolveInt 解析整数参数。
func resolveInt(cfg *PiConfig, name string, cliVal int, envKey string) int {
	if cliVal != 0 {
		return cliVal
	}
	if envVal := os.Getenv(envKey); envVal != "" {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(envVal), "%d", &n); err == nil {
			return n
		}
	}
	switch name {
	case "max-steps":
		if cfg.MaxSteps != 0 {
			return cfg.MaxSteps
		}
	case "max-tokens":
		if cfg.MaxTokens != 0 {
			return cfg.MaxTokens
		}
	}
	return cliVal
}

// resolveFloat 解析浮点数参数。
func resolveFloat(cfg *PiConfig, name string, cliVal float64, envKey string) float64 {
	if cliVal != 0 {
		return cliVal
	}
	if envVal := os.Getenv(envKey); envVal != "" {
		var f float64
		if _, err := fmt.Sscanf(strings.TrimSpace(envVal), "%f", &f); err == nil {
			return f
		}
	}
	if name == "temperature" && cfg.Temperature != 0 {
		return cfg.Temperature
	}
	return cliVal
}
