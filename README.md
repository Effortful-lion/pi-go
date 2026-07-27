# Pi Agent — AI Coding Assistant in Go

Pi Agent 是一个**交互式 AI 编程助手 CLI**，同时也是可嵌入 Go 项目的 **Agent 开发库**。

核心能力：
- 多 LLM 提供商统一接口（OpenAI / Anthropic Claude / Google Gemini）
- Agent 运行时：LLM ↔ 工具调用自动循环
- 内置工具：文件读写、目录列表、文件搜索、Shell 执行
- 终端 UI：多行输入、历史记录、语法高亮、Markdown 渲染、路径补全
- 配置管理：YAML 配置文件 + 环境变量 + 命令行参数三级优先级
- Session 持久化：JSONL 格式保存/加载对话历史

---

## 目录

- [快速开始](#快速开始)
- [使用方式](#使用方式)
  - [交互式对话 / 对话内命令](#交互式对话--对话内命令)
  - [CLI 子命令](#cli-子命令)
  - [配置文件 / 环境变量](#配置文件--环境变量)
- [作为库引用](#作为库引用)
- [项目架构](#项目架构)
- [二次开发指南](#二次开发指南)
  - [如何新增 LLM Provider](#如何新增-llm-provider)
  - [如何新增 Tool](#如何新增-tool)
  - [如何新增对话内命令](#如何新增对话内命令)
  - [如何新增 pi 命令行](#如何新增-pi-命令行)
  - [扩展 TUI 能力](#扩展-tui-能力)
- [API 参考](#api-参考)
- [命令参考](#命令参考)

---

## 快速开始

### 安装

```bash
go install github.com/Effortful-lion/pi-go/cmd/pi@latest
```

### 配置 API Key

```bash
# 方式 1：环境变量（临时）
export OPENAI_API_KEY="sk-..."

# 方式 2：配置文件写入（永久）
pi config init
pi config set api_key "sk-..."
```

### 开始对话

```bash
pi
```

用 `pi -provider anthropic` 切换到 Claude，用 `pi -provider google` 切换到 Gemini。

---

## 使用方式

### 交互式对话 / 对话内命令

启动 `pi` 后在 `> ` 提示符下直接输入问题即可。对话内支持以下命令：

| 命令 | 作用 |
|------|------|
| `/clear` | 清屏 |
| `/reset` | 重置对话（清空历史，保留 system prompt） |
| `/export [路径]` | 将当前对话导出为 Markdown 文件 |
| `/help` | 显示帮助 |

**快捷键：**

| 按键 | 作用 |
|------|------|
| `↑` `↓` | 浏览历史记录 |
| `Alt+Enter` | 多行输入 |
| `Tab` | 文件路径补全 |
| `Ctrl+L` | 清屏 |
| `Ctrl+C` | 取消当前输入 |
| `Ctrl+D` | 退出 |

### CLI 子命令

```bash
pi                              # 启动交互式对话（默认）
pi chat [flags]                 # 同上，带更多参数
pi config init                  # 创建默认配置文件 (~/.pi-go/config.yaml)
pi config show                  # 打印当前配置
pi config set <key> <value>     # 修改配置项
pi version                      # 打印版本号
pi help                         # 打印命令行帮助
```

**Chat Flags：**

| Flag | 默认值 | 说明 |
|------|--------|------|
| `-provider` | `openai` | LLM 提供商 (`openai` / `anthropic` / `google`) |
| `-model` | `gpt-4o` | 模型 ID |
| `-api-key` | — | API Key |
| `-base-url` | — | 自定义 API 地址 |
| `-system-prompt` | — | 系统提示词 |
| `-max-steps` | `10` | 工具调用最大轮数 |
| `-temperature` | — | 采样温度 |
| `-max-tokens` | — | 最大输出 token |
| `-session` | — | Session 名称（恢复对话） |

参数优先级：**命令行 `>` 环境变量 `>` 配置文件 `>` 默认值**。

### 配置文件 / 环境变量

配置文件位于 `~/.pi-go/config.yaml`（或当前目录 `.pi-go.yaml`）：

```yaml
provider: openai
model: gpt-4o
api_key: sk-xxx
base_url: https://api.openai.com/v1
system_prompt: You are Pi, a coding assistant.
max_steps: 10
temperature: 0.7
max_tokens: 2048
```

环境变量对应关系：

| 环境变量 | 对应 flag | 说明 |
|----------|-----------|------|
| `OPENAI_API_KEY` | `-api-key` | API Key |
| `PI_PROVIDER` | `-provider` | 提供商 |
| `PI_MODEL` | `-model` | 模型 ID |
| `PI_BASE_URL` | `-base-url` | API 地址 |
| `PI_SYSTEM_PROMPT` | `-system-prompt` | 系统提示词 |
| `PI_MAX_STEPS` | `-max-steps` | 最大轮数 |
| `PI_TEMPERATURE` | `-temperature` | 温度 |
| `PI_MAX_TOKENS` | `-max-tokens` | 最大 token |

---

## 作为库引用

如果你的 Go 项目需要嵌入 AI Agent 能力，可以引用本项目的开发库：

```
go get github.com/Effortful-lion/pi-go
```

### 最小示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/Effortful-lion/pi-go/agent"
    "github.com/Effortful-lion/pi-go/ai/providers/openai"
    "github.com/Effortful-lion/pi-go/tool/builtin"
)

func main() {
    prov := openai.New(openai.Config{APIKey: "sk-..."})

    ag := agent.New(agent.Config{
        Provider:     prov,
        ModelID:      "gpt-4o",
        SystemPrompt: "You are a coding assistant.",
        Tools:        builtin.NewFileTools(builtin.FileConfig{}),
        MaxSteps:     10,
    })

    ctx := context.Background()
    for evt := range ag.Run(ctx, "读一下 main.go 的前 10 行") {
        switch evt.Type {
        case agent.EventTextDelta:
            fmt.Print(evt.Text)
        case agent.EventToolCall:
            fmt.Printf("[调用工具 %s]\n", evt.ToolCall.Name)
        case agent.EventDone:
            fmt.Println("\n--- 对话结束 ---")
        }
    }
}
```

在你的 main.go 中组合 `agent.Config`、`ai.Provider` 和 `tool.Tool` 即可使用。

---

## 项目架构

```
pi-go/
├── cmd/pi/             # CLI 入口：命令路由、配置解析
│   ├── main.go         #   子命令分发 (chat/config/version/help)
│   ├── chat.go         #   chat 命令：组装 Provider + Agent + TUI
│   ├── config.go       #   配置结构体 + YAML 加载/保存
│   └── config_cmd.go   #   config 子命令 (init/show/set)
│
├── agent/              # Agent 运行时 —— 核心调度层
│   ├── agent.go        #   Agent 结构体、Run/Continue/Reset/Messages
│   ├── event.go        #   高级事件系统 (Event/Stream)
│   └── session.go      #   Session 持久化 (Save/Load/List/Delete)
│
├── ai/                 # LLM 统一抽象 —— 多 Provider 接口
│   ├── provider.go     #   Provider / Model 接口
│   ├── types.go        #   Message / Context / ToolCall / ToolDefinition
│   ├── event.go        #   流式事件 (EventType / Stream)
│   └── providers/      #   具体 Provider 实现
│       ├── openai/     #     OpenAI (+ 兼容协议)
│       ├── anthropic/  #     Anthropic Claude Messages API
│       └── google/     #     Google Gemini API
│
├── tool/               # 工具系统 —— Agent 可调用的能力
│   ├── tool.go         #   Tool 接口 (Name/Definition/Execute)
│   └── builtin/        #   内置工具
│       ├── file.go     #     read_file / write_file / list_dir / search_file
│       └── shell.go    #     shell 执行
│
├── tui/                # 终端 UI —— 交互界面
│   ├── chat.go         #   ChatUI + 对话内命令处理 + 对话导出
│   ├── terminal.go     #   终端原语 + LineEditor (多行/历史/补全)
│   ├── highlight.go    #   语法高亮 (Go/Python/JS/Shell/JSON/YAML)
│   └── markdown.go     #   Markdown → ANSI 渲染
│
├── lg/                 # 日志库
├── docs/               # 设计文档 & 学习总结
├── change-log/         # 变更日志
└── go.mod
```

**依赖方向：** `cmd/pi` → `agent` + `tui` → `ai` + `tool` → `lg`

---

## 二次开发指南

以下指南面向希望扩展 pi-go 能力的开发者。每个章节说明需要改动哪些文件，并给出范例代码。

### 如何新增 LLM Provider

**目标：** 接入一个新的 LLM 提供商（如 Azure、DeepSeek、Ollama 等）。

需要实现的接口是 `ai.Provider` 和 `ai.Model`，定义在 `ai/provider.go`：

```go
type Provider interface {
    ID() string
    Name() string
    Models() []ModelInfo
    Model(modelID string) (Model, error)
    Chat(ctx context.Context, modelID string, context Context) Stream
}

type Model interface {
    Info() ModelInfo
    Chat(ctx context.Context, context Context) Stream
}
```

#### 步骤

**1.** 在 `ai/providers/` 下新建子包，例如 `azure/`：

```
ai/providers/azure/
└── provider.go
```

**2.** 实现 `provider` 和 `model` 结构体，参考 `openai/provider.go` 为模板：

```go
package azure

import (
    "context"
    "github.com/Effortful-lion/pi-go/ai"
)

type Config struct {
    APIKey  string
    BaseURL string
}

type provider struct {
    cfg    Config
    models []ai.ModelInfo
}

func New(cfg Config) ai.Provider {
    if cfg.BaseURL == "" {
        cfg.BaseURL = "https://YOUR.openai.azure.com"
    }
    return &provider{
        cfg: cfg,
        models: []ai.ModelInfo{
            {ID: "gpt-4o", Name: "Azure GPT-4o", ProviderID: "azure", ContextWindow: 128000, MaxTokens: 16384},
        },
    }
}

func (p *provider) ID() string               { return "azure" }
func (p *provider) Name() string             { return "Azure OpenAI" }
func (p *provider) Models() []ai.ModelInfo   { return p.models }
func (p *provider) Model(modelID string) (ai.Model, error) { /* ... */ }
func (p *provider) Chat(ctx context.Context, modelID string, ctx2 ai.Context) ai.Stream { /* ... */ }
```

核心工作在于 `model.Chat` 方法：
- 将 `ai.Context`（统一的 Message/Tool 表示）转换为该提供商的 HTTP 请求体。
- 发起 HTTP SSE 请求，逐行解析服务端响应。
- 将解析结果映射为 `ai.Event` 流式发送。

**3.** 在 `cmd/pi/chat.go` 的 `newProvider` 中注册：

```go
import "github.com/Effortful-lion/pi-go/ai/providers/azure"

func newProvider(name, apiKey, baseURL string) ai.Provider {
    switch name {
    case "openai":
        return openai.New(openai.Config{APIKey: apiKey, BaseURL: baseURL})
    case "azure":                              // ← 新增
        return azure.New(azure.Config{APIKey: apiKey, BaseURL: baseURL})
    // ...
    }
}
```

**4.** 可选：在 `cmd/pi/main.go` 的 help 中更新 provider 列表。

**5.** 编译测试：

```bash
go build ./...
pi -provider azure -model gpt-4o
```

---

### 如何新增 Tool

**目标：** 为 Agent 增加新的可调用工具（如 HTTP 请求、数据库查询、天气查询等）。

需要实现的接口是 `tool.Tool`，定义在 `tool/tool.go`：

```go
type Tool interface {
    Name() string
    Definition() ai.ToolDefinition
    Execute(ctx context.Context, argsJSON string) (string, error)
}
```

#### 步骤

**1.** 在 `tool/builtin/` 下新建工具文件，例如 `http.go`：

```go
package builtin

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "strings"

    "github.com/Effortful-lion/pi-go/ai"
    "github.com/Effortful-lion/pi-go/tool"
)

// --- 工具实现 ---

type httpTool struct{}

func (t *httpTool) Name() string { return "http_get" }

func (t *httpTool) Definition() ai.ToolDefinition {
    return ai.ToolDefinition{
        Name:        "http_get",
        Description: "发送 HTTP GET 请求并返回响应体",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "url": map[string]any{
                    "type":        "string",
                    "description": "请求 URL",
                },
            },
            "required": []string{"url"},
        },
    }
}

func (t *httpTool) Execute(ctx context.Context, argsJSON string) (string, error) {
    var args struct {
        URL string `json:"url"`
    }
    if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
        return "", err
    }
    req, _ := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    return string(body), nil
}

// --- 工厂函数 ---

func NewHTTPTool() tool.Tool {
    return &httpTool{}
}
```

**2.** 在 `cmd/pi/chat.go` 的 `runChat` 中将新工具注入 Agent：

```go
import "github.com/Effortful-lion/pi-go/tool/builtin"

func runChat(cfg *PiConfig, cliFlags *ChatFlags) error {
    // ... 创建 provider ...

    // 组合所需工具
    tools := []tool.Tool{}
    tools = append(tools, builtin.NewFileTools(builtin.FileConfig{})...)
    tools = append(tools, builtin.NewShellTool(builtin.ShellConfig{}))
    tools = append(tools, builtin.NewHTTPTool())  // ← 新增

    ag := agent.New(agent.Config{
        Provider:     prov,
        ModelID:      model,
        SystemPrompt: systemPrompt,
        Tools:        tools,       // ← 传入工具列表
        MaxSteps:     maxSteps,
        Temperature:  temperature,
        MaxTokens:    maxTokens,
    })
    // ...
}
```

**3.** 编译测试，LLM 会根据 definition 中的 JSON Schema 自动判断何时调用该工具。

**安全提示：** 任何涉及外部 I/O 的工具都应考虑权限控制和安全限制（参考 `file.go` 的 `AllowedRoots` 和 `shell.go` 的 `BlockedPatterns` 白/黑名单模式）。

---

### 如何新增对话内命令

**目标：** 在交互式对话 `> ` 提示符下增加自定义斜杠命令（如 `/save <name>`、`/model gpt-4o` 等）。

对话内命令在 `tui/chat.go` 的 `handleCommand` 方法中注册。

#### 步骤

**1.** 在 `handleCommand` 的 `switch` 中新增 case：

```go
func (ui *ChatUI) handleCommand(ctx context.Context, input string, le *LineEditor) {
    parts := strings.Fields(input)
    if len(parts) == 0 {
        return
    }
    switch parts[0] {
    case "/clear":
        ClearScreen()
        ui.printWelcome()
    case "/reset":
        ui.agent.Reset()
        le.AddHistory(input)
        fmt.Fprintln(ui.out, Dim("对话已重置"))
    case "/model":                                                     // ← 新增
        if len(parts) > 1 {
            fmt.Fprintln(ui.out, Dim("手动切换模型需要重建 Agent，暂不支持"))
        } else {
            fmt.Fprintln(ui.out, Dim("当前模型: ..."))
        }
    case "/save":                                                      // ← 新增
        name := "default"
        if len(parts) > 1 {
            name = parts[1]
        }
        if err := ui.agent.SaveSession(name); err != nil {
            fmt.Fprintln(ui.out, Red(fmt.Sprintf("保存失败: %v", err)))
        } else {
            fmt.Fprintln(ui.out, Green(fmt.Sprintf("Session 已保存: %s", name)))
        }
    // ...
    }
}
```

**2.** 可选：在 `printHelp` 中添加新命令的帮助说明。

#### 需要更复杂的 UI 交互？

如果命令需要额外的交互（如选择提示、分页列表等），可以结合 `tui/terminal.go` 的 `LineEditor` 和终端控制函数（`CursorUp`/`ClearLine` 等）来实现。现有的 `/clear`、`/reset`、`/export` 已经展示了基本的模式。

---

### 如何新增 pi 命令行

**目标：** 增加如 `pi sessions list`、`pi tools` 这样的新 CLI 子命令或子命令组。

CLI 主干在 `cmd/pi/main.go` 的 `run` 函数和 `switch args[0]` 中。

#### 新增子命令组（如 `pi sessions`）

**1.** 创建新的命令实现文件，如 `cmd/pi/sessions.go`：

```go
package main

import (
    "fmt"
    "os"
    "github.com/Effortful-lion/pi-go/agent"
)

func runSessionsCmd(args []string) {
    if len(args) == 0 {
        args = []string{"list"}
    }
    switch args[0] {
    case "list":
        sessions, err := agent.ListSessions()
        if err != nil {
            fmt.Fprintf(os.Stderr, "列出 session 失败: %v\n", err)
            return
        }
        for _, s := range sessions {
            fmt.Printf("  %s (%d 条消息, %s)\n", s.ID, s.MsgCount, s.UpdatedAt.Format("01-02 15:04"))
        }
    case "delete":
        if len(args) < 2 {
            fmt.Fprintln(os.Stderr, "用法: pi sessions delete <name>")
            return
        }
        if err := agent.DeleteSession(args[1]); err != nil {
            fmt.Fprintf(os.Stderr, "删除失败: %v\n", err)
        }
    default:
        fmt.Fprintf(os.Stderr, "未知 sessions 子命令: %s\n", args[0])
    }
}
```

**2.** 在 `main.go` 的 `run()` 的 `switch` 中添加路由：

```go
switch args[0] {
case "chat":
    return runChatCmd(args[1:])
case "config":
    runConfig(args[1:])
    return nil
case "sessions":               // ← 新增
    runSessionsCmd(args[1:])
    return nil
case "version", "-version", "--version":
    // ...
}
```

**3.** 在 `printHelp()` 中添加使用说明，编译后即可使用：

```bash
go build ./cmd/pi/
./pi sessions list
./pi sessions delete mychat
```

---

### 扩展 TUI 能力

这部分指南面向希望增强终端 UI 的能力（语法高亮语言支持、Markdown 渲染规则、终端组件等）。

#### 增加语法高亮语言

修改 `tui/highlight.go`：

1. 在已有的语言常量后新增语言标识。
2. 在 `highlightLine` 函数的 `switch lang` 中添加新的 `case`，实现关键词/函数/注释的 token 匹配逻辑。参考已有的 Go/Python/JS/Shell/JSON/YAML 实现。

```go
func highlightLine(line, lang string) string {
    var tokens []token
    switch lang {
    case "go":
        tokens = tokenizeGo(line)
    case "rust", "rs":    // ← 新增
        tokens = tokenizeRust(line)
    // ...
    }
    return renderTokens(tokens)
}
```

#### 扩展 Markdown 渲染

修改 `tui/markdown.go`：

- 新增 `*` 或 `_` 包裹的**斜体**渲染规则。
- 新增 `###` 三级标题、列表缩进等。
- 新增代码块内行号显示。

核心函数 `MarkdownLine(line string) string` 对每行文本进行正则匹配并包装 ANSI 转义码。

#### 扩展 LineEditor 功能

`tui/terminal.go` 中的 `LineEditor` 结构体已经在内部封装了所有编辑状态。如果要增加 Ctrl+R 反向搜索、语法感知补全等高级功能，需要：

1. 在 `readKey()` 的按键序列匹配中添加新按键。
2. 在 `ReadLine()` 主循环中添加新 case 处理。
3. 利用 `le.renderAll()` 重绘更新屏幕。

---

## API 参考

### `ai` 包

```go
type Provider interface {
    ID() string
    Name() string
    Models() []ModelInfo
    Model(modelID string) (Model, error)
    Chat(ctx context.Context, modelID string, context Context) Stream
}

type Context struct {
    Messages     []Message
    Tools        []ToolDefinition
    SystemPrompt string
    MaxTokens    int
    Temperature  float64
}

type Message struct {
    Role       Role          // RoleSystem / RoleUser / RoleAssistant / RoleTool
    Content    string
    Blocks     []ContentBlock
    ToolCallID string
    ToolName   string
}

type ToolDefinition struct {
    Name        string
    Description string
    Parameters  map[string]any  // JSON Schema obj
}

type ToolCall struct {
    ID        string
    Name      string
    Arguments string  // JSON 字符串
}
```

### `agent` 包

```go
type Config struct {
    Provider     ai.Provider
    ModelID      string
    SystemPrompt string
    Tools        []tool.Tool
    MaxSteps     int
    Temperature  float64
    MaxTokens    int
}

func New(cfg Config) *Agent

// 对话
func (a *Agent) Run(ctx context.Context, userInput string) Stream
func (a *Agent) Continue(ctx context.Context, extraContext string) Stream
func (a *Agent) Reset()

// 状态
func (a *Agent) Messages() []ai.Message
func (a *Agent) SaveSession(sessionID string) error

// Session
func LoadSession(sessionID string) ([]ai.Message, error)
func ListSessions() ([]SessionInfo, error)
func DeleteSession(sessionID string) error
```

### `tool` 包

```go
type Tool interface {
    Name() string
    Definition() ai.ToolDefinition
    Execute(ctx context.Context, argsJSON string) (string, error)
}
```

### `tui` 包

```go
func NewChatUI(ag *agent.Agent, opts ...ChatUIOption) *ChatUI
func (ui *ChatUI) Run(ctx context.Context) error
func (ui *ChatUI) ExportConversation(path string) error
```

---

## 命令参考

| 命令 | 描述 |
|------|------|
| `pi` | 启动交互式对话 |
| `pi chat` | 同上（可带 flags） |
| `pi config init` | 创建默认配置文件 |
| `pi config show` | 显示当前配置 |
| `pi config set <k> <v>` | 设置配置项 |
| `pi version` | 显示版本 |
| `pi help` | 显示帮助 |
