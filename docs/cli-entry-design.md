# CLI 产品入口设计

## 概述

`cmd/pi/main.go` 将 ai/agent/tui 三个库层组装成可执行的 CLI 产品 `pi`。这是 pi-go 项目的最终产品层。

## 架构

```
cmd/pi/main.go
  ├── ai/providers/openai/  → 创建 OpenAI Provider
  ├── agent/                → 创建 Agent（注入 Provider）
  └── tui/                  → 创建 ChatUI（注入 Agent），启动交互循环
```

## 命令行参数

| 参数 | 类型 | 默认值 | 环境变量 | 说明 |
|------|------|--------|----------|------|
| `-provider` | string | `openai` | `PI_PROVIDER` | LLM 提供商 |
| `-model` | string | `gpt-4o` | `PI_MODEL` | 模型 ID |
| `-api-key` | string | `$OPENAI_API_KEY` | `OPENAI_API_KEY` | API Key |
| `-base-url` | string | `""`（OpenAI 默认） | `PI_BASE_URL` | 自定义 API Base URL |
| `-system-prompt` | string | 见下方默认提示词 | `PI_SYSTEM_PROMPT` | 系统提示词 |
| `-max-steps` | int | `10` | `PI_MAX_STEPS` | 工具调用最大轮数 |
| `-temperature` | float64 | `0.7` | `PI_TEMPERATURE` | 随机性参数 |
| `-max-tokens` | int | `0`（不限制） | `PI_MAX_TOKENS` | 最大输出 tokens |

## 默认系统提示词

```
You are Pi Agent, an AI coding assistant.
You help with writing code, debugging, answering technical questions, and more.
Be concise, helpful, and use tools when appropriate.
```

## 参数优先级

命令行参数 > 环境变量 > 默认值。例如：

- `-api-key "sk-xxx"` 优先于 `$OPENAI_API_KEY`
- `-model "gpt-4o-mini"` 优先于 `$PI_MODEL`

通过 `flag.Visit` 判断参数是否显式设置，未设置时回退到环境变量。

## Agent 创建流程

```go
func main() {
    // 1. 解析参数
    // 2. 创建 Provider
    prov := openai.New(openai.Config{
        APIKey:  apiKey,
        BaseURL: baseURL,
    })
    // 3. 创建 Agent（不传 Tools，纯文本对话）
    ag := agent.New(agent.Config{
        Provider:     prov,
        ModelID:      modelID,
        SystemPrompt: systemPrompt,
        MaxSteps:     maxSteps,
        Temperature:  temperature,
        MaxTokens:    maxTokens,
    })
    // 4. 创建 ChatUI 并运行
    ui := tui.NewChatUI(ag)
    if err := ui.Run(context.Background()); err != nil {
        // ...
    }
}
```

## 版本号

版本号硬编码为 `0.1.0`，定义在文件顶部常量 `version`。

## 不做什么

- 不实现子命令（如 `pi start`/`pi config`）
- 不实现配置文件（仅命令行参数 + 环境变量）
- 不实现 session 持久化
- 不引入 cobra/viper 等三方 CLI 框架，纯 `flag` 包
- 不注册工具（首个版本为纯文本对话，工具通过后续迭代加入）

## 依赖方向

```
cmd/pi → tui → agent → ai → lg
cmd/pi → ai/providers/openai
```

符合 AGENTS.md 中的依赖规范：`cmd/pi → agent → ai → lg` + `cmd/pi → tui`。

## 编译产物

```bash
go build -o pi ./cmd/pi/
./pi -api-key="sk-..."
```
