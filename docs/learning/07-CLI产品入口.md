# 学习总结：CLI 产品入口

## 开发时序：第 7 步（最后一步）

### 把零件组装成产品

前面 6 步构建了 5 个独立的库模块：

```
ai/     → LLM 交互
agent/  → Agent 运行时
tool/   → 工具接口
tui/    → 终端 UI
lg/     → 日志
```

但它们各是各的——用户不能 import 一堆包然后手动组装。需要一个 `main.go` 把它们拼成一个 **可执行文件**。

---

## 开发思路：胶水代码

`cmd/pi/main.go` 的职责非常单一：**解析参数，创建对象，启动运行**。

它不是"新模块"，而是"胶水代码"——把已有的模块按正确的依赖关系串联起来。

---

## 核心实现

### 1. 命令行参数

```go
const version = "0.1.0"

func main() {
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
    flag.IntVar(&maxTokens, "max-tokens", 0, "Max output tokens")
    flag.BoolVar(&showVersion, "version", false, "Show version")
    flag.Parse()
}
```

**关键决策**：为什么用 `flag` 而不是 `cobra`？
→ 只有 9 个参数、0 个子命令，`flag` 标准库完全够用。引入 `cobra` 会增加数千行依赖，收益为负。

### 2. 参数优先级：命令行 > 环境变量 > 默认值

```go
func resolveFromEnv(fs *flag.FlagSet, mapping map[string]string) {
    // 第一步：记录哪些 flag 被显式设置了
    visited := make(map[string]bool)
    fs.Visit(func(f *flag.Flag) {
        visited[f.Name] = true
    })

    // 第二步：未显式设置的，从环境变量回退
    for name, envKey := range mapping {
        if visited[name] {
            continue // 用户在命令行传了，跳过
        }
        if val, ok := os.LookupEnv(envKey); ok && val != "" {
            fs.Set(name, val)
        }
    }
}

// 使用
resolveFromEnv(flag.CommandLine, map[string]string{
    "api-key":   "OPENAI_API_KEY",
    "model":     "PI_MODEL",
    "max-steps": "PI_MAX_STEPS",
    // ...
})
```

**关键设计**：`flag.Visit` 会遍历所有 **显式设置过** 的 flag。利用这个特性区分"用户设置的"和"默认值"。

```
场景 1: pi -api-key="sk-xxx"
  → visited["api-key"] = true，跳过环境变量

场景 2: export OPENAI_API_KEY="sk-xxx" && pi
  → visited["api-key"] = false，从环境变量读取

场景 3: pi
  → visited["api-key"] = false，环境变量也为空
  → apiKey == ""，报错退出
```

### 3. 组装流程

```go
func main() {
    flag.Parse()

    // ... 参数处理 ...

    // 1. 创建 Provider
    prov := openai.New(openai.Config{
        APIKey:  apiKey,
        BaseURL: baseURL,
    })

    // 2. 创建 Agent
    ag := agent.New(agent.Config{
        Provider:     prov,
        ModelID:      model,
        SystemPrompt: systemPrompt,
        MaxSteps:     maxSteps,
        Temperature:  temperature,
        MaxTokens:    maxTokens,
    })

    // 3. 创建 ChatUI 并运行
    ui := tui.NewChatUI(ag)
    if err := ui.Run(context.Background()); err != nil {
        fmt.Fprintf(os.Stderr, "pi: %v\n", err)
        os.Exit(1)
    }
}
```

不到 10 行核心代码，把 5 个模块串起来。

### 4. 为什么不注册工具？

首版（v0.1.0）是纯文本对话，不注册任何工具。工具将在后续迭代中加入。

原因：
1. **先跑通**：确保核心对话链路正常
2. **工具需要安全工作**：Shell 执行、文件读写等工具需要仔细设计安全边界
3. **按需添加**：用户可以根据需要加载不同的工具集

---

## 完整使用方式

### 编译

```bash
go build -o pi ./cmd/pi/
```

### 运行

```bash
# 方式 1: 命令行参数
./pi -api-key="sk-xxx" -model="gpt-4o-mini"

# 方式 2: 环境变量
export OPENAI_API_KEY="sk-xxx"
export PI_MODEL="gpt-4o-mini"
./pi

# 方式 3: 使用自定义 API（兼容 OpenAI 格式）
./pi -api-key="sk-xxx" -base-url="https://your-api.com/v1"

# 方式 4: 查看版本
./pi -version
```

### 运行时交互

```
🤖 Pi Agent v0.1.0
输入消息后按 Enter，Ctrl+C 退出

> 帮我写一个 Go HTTP server
💬 You: 帮我写一个 Go HTTP server
🤖 Assistant: 好的，帮你写一个简单的 HTTP server...

package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello, World!")
    })
    http.ListenAndServe(":8080", nil)
}

> 
```

---

## 项目依赖全图

```
cmd/pi/main.go
├── import "ai/providers/openai"   → 创建 Provider
├── import "agent"                 → 创建 Agent
└── import "tui"                   → 创建 ChatUI

依赖方向（自顶向下）:
cmd/pi → tui → agent → ai → lg
cmd/pi ─────→ ai/providers/openai
```

一切在 `main` 函数完成组装。所有库模块不知道彼此的存在，也不依赖 `cmd/pi`。

---

## 思想总结

| 思想 | 体现 |
|------|------|
| **胶水代码** | main.go 不包含业务逻辑，只做对象创建和组装 |
| **依赖注入** | Provider 和 Agent 通过构造函数传入，不在 main 里 new |
| **最小依赖** | 仅用 `flag` + `os` 标准库，不引入 cobra/viper |
| **环境变量回退** | `flag.Visit` 巧妙区分"显式设置"和"默认值" |
| **先跑通再完善** | v0.1.0 纯文本对话，工具系统后续迭代加入 |
