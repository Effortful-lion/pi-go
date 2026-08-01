# Pi-Go Agent — AI Coding Assistant in Go

Pi-Go Agent 是一个**交互式 AI 编程助手 CLI**，同时也是可嵌入 Go 项目的 **Agent 开发库**。

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
  - [如何新增 pg 命令行](#如何新增-pg-命令行)
  - [扩展 TUI 能力](#扩展-tui-能力)
- [API 参考](#api-参考)
- [命令参考](#命令参考)
- [贡献指南](#贡献指南)

---

## 快速开始

### 安装

**方式 A：一键安装（推荐，无需 Go 环境）**

自动检测 OS/Arch，从 GitHub Releases 下载预编译二进制。

```bash
curl -fsSL https://raw.githubusercontent.com/Effortful-lion/pi-go/main/install.sh | bash
```

**方式 B：go install（需 Go 环境）**

```bash
go install github.com/Effortful-lion/pi-go/cmd/pg@latest
```

**方式 C：本地编译**

```bash
git clone https://github.com/Effortful-lion/pi-go.git
cd pi-go
make install      # 编译安装到 $GOPATH/bin
```

> **PATH 配置：** 如果 `pg` 命令未找到，需要确保 `$GOPATH/bin` 在 PATH 中：
> ```bash
> echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
> source ~/.zshrc
> ```

### 卸载

```bash
./uninstall.sh
```

一键移除 `pg` 二进制、PATH 配置项、`~/.pi-go/` 数据目录。

### 配置 API Key

**方式 A：环境变量（临时、不落盘）**

```bash
export OPENAI_API_KEY="sk-..."
```

**方式 B：配置文件（永久、推荐）**

```bash
pg config init                         # 创建 ~/.pi-go/config.yaml
pg config set api_key "sk-..."         # 写入 API Key
pg config set provider anthropic       # 可选：切换默认 Provider
pg config show                         # 查看当前配置
```

**方式 C：命令行参数（一次性、优先级最高）**

```bash
pg -api-key "sk-..." -provider openai -model gpt-4o
```

> 参数优先级：**命令行 > 环境变量 > 配置文件 > 默认值**

### 开始对话

```bash
pg                                     # 默认 openai + gpt-4o
pg -provider anthropic                 # 切换到 Claude
pg -provider google                    # 切换到 Gemini
pg -session mychat                     # 恢复之前的对话
```

### 快速测试核心功能

进入对话后（`> ` 提示符），可以依次体验以下能力：

| 输入 | 验证目标 |
|------|---------|
| `写一段 Go hello world` | LLM 基本对话 |
| `帮我看看当前目录有哪些文件` | File Tool（`list_dir`） |
| `/clear` | 清屏 |
| `/reset` | 重置对话上下文 |
| `/export test.md` | 导出对话为 Markdown |
| 按 `↑` `↓` | 历史记录浏览 |
| 按 `Tab` | 文件路径补全 |
| 按 `Ctrl+D` | 退出程序 |

---

## 使用方式

### 交互式对话 / 对话内命令

启动 `pg` 后在 `> ` 提示符下直接输入问题即可。对话内支持以下命令：

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
pg                              # 启动交互式对话（默认）
pg chat [flags]                 # 同上，带更多参数
pg config init                  # 创建默认配置文件 (~/.pi-go/config.yaml)
pg config show                  # 打印当前配置
pg config set <key> <value>     # 修改配置项
pg version                      # 打印版本号
pg help                         # 打印命令行帮助
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
├── cmd/pg/             # CLI 入口：命令路由、配置解析
│   ├── main.go         #   公共代码 (printVersion/printHelp)
│   ├── main_unix.go    #   Unix 子命令路由 (chat/config/version/help)
│   ├── main_windows.go #   Windows 子命令路由（基础命令）
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
├── tui/                # 终端 UI —— 交互界面 (!windows)
│   ├── chat.go         #   ChatUI + 对话内命令处理 + 对话导出
│   ├── terminal.go     #   终端原语 + LineEditor (多行/历史/补全)
│   ├── highlight.go    #   语法高亮 (Go/Python/JS/Shell/JSON/YAML)
│   └── markdown.go     #   Markdown → ANSI 渲染
│
├── lg/                 # 日志库
├── docs/               # 设计文档 & 学习总结
├── change-log/         # 变更日志
├── Makefile            # 开发/构建/发布命令
├── pack.sh             # 本地手动打包
├── install.sh          # 一键安装脚本
├── .goreleaser.yaml    # goreleaser 发布配置
└── go.mod
```

**依赖方向：** `cmd/pg` → `agent` + `tui` → `ai` + `tool` → `lg`

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

**3.** 在 `cmd/pg/chat.go` 的 `newProvider` 中注册：

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

**4.** 可选：在 `cmd/pg/main.go` 的 help 中更新 provider 列表。

**5.** 编译测试：

```bash
go build ./...
pg -provider azure -model gpt-4o
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

**2.** 在 `cmd/pg/chat.go` 的 `runChat` 中将新工具注入 Agent：

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

### 如何新增 pg 命令行

**目标：** 增加如 `pg sessions list`、`pg tools` 这样的新 CLI 子命令或子命令组。

CLI 主干在 `cmd/pg/main.go` 的 `run` 函数和 `switch args[0]` 中。

#### 新增子命令组（如 `pg sessions`）

**1.** 创建新的命令实现文件，如 `cmd/pg/sessions.go`：

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
            fmt.Fprintln(os.Stderr, "用法: pg sessions delete <name>")
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
go build ./cmd/pg/
./pg sessions list
./pg sessions delete mychat
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

### pg 命令

| 命令 | 描述 |
|------|------|
| `pg` | 启动交互式对话 |
| `pg chat` | 同上（可带 flags） |
| `pg config init` | 创建默认配置文件 |
| `pg config show` | 显示当前配置 |
| `pg config set <k> <v>` | 设置配置项 |
| `pg version` | 显示版本 |
| `pg help` | 显示帮助 |

### Makefile（开发用）

**编译、打包、发布**

| 命令 | 说明 |
|------|------|
| `make build` | 本地发布构建（ldflags 注入 tag） |
| `make pack` | 手动交叉编译打包 darwin/arm64 + windows/amd64 |
| `make release` | 推送 tag 触发 CI 发布 GitHub Release |

**代码检查**

| 命令 | 说明 |
|------|------|
| `make lint` | 运行 golangci-lint |
| `make test` | 全量测试 + 竞态检测 |

**发布 tag 版本管理**

| 命令 | 说明 |
|------|------|
| `make version` | 查看最新 tag |
| `make next-version` | 交互式输入新版本号并打 tag |

**其他**

| 命令 | 说明 |
|------|------|
| `make run` | 本地编译并运行 |
| `make clean` | 清理构建产物 |
| `make help` | 显示帮助 |

---

## 贡献指南

欢迎 PR！最简流程：

```bash
# 1. fork + clone
git clone https://github.com/YOUR_USERNAME/pi-go.git
cd pi-go

# 2. 创建分支
git checkout -b feat/my-feature

# 3. 开发 + 验证
# - 开发后本地运行进行功能点测试
# - 然后测试
make lint          # 代码检查
make test          # 全量测试

# 4. 提交（conventional commits 格式）
git add <files>
git commit -m "feat(module): 描述你的改动" 

# 5. 推送
# - 第一次push:
git push origin --set-upstream feat/my-feature
# - 之后直接push:
git push

# 6. 提 PR
# - 先切到main，pull到最新代码
git switch main
git pull / git pull upstream main
# - 提PR前：先合并代码到开发分支
git merge main
# - 如果存在冲突处理冲突后 commit & push；不存在冲突就直接 push
git commit -m "处理冲突"
git push
# - 提PR
1. 打开个人 fork 仓库页面
2. 点击 Compare & pull request
3. 目标分支选择官方仓库 `main`
4. 填写改动说明，提交 PR 等待审核
```

**PR 自动检查：** CI 会跑 `golangci-lint` + `go test -race`（Go 1.21 / 1.23），全部通过才能合并。

**Commit 格式：** `feat|fix|docs|chore(模块): 描述`

**新增 Provider：** 参考 [如何新增 LLM Provider](#如何新增-llm-provider)，在 `ai/providers/` 下新建子包，`chat.go` 中注册。

**新增 Tool：** 参考 [如何新增 Tool](#如何新增-tool)，在 `tool/builtin/` 下新建工具文件。

### SOP 示范模块：`lg/` 日志库

`lg/` 作为本项目的标准开发流程（SOP）示范模块，所有新模块或功能开发请参考此流程：

```
分支新建 → 本地开发 → 单包测试 → 全量测试 → lint 检查 → 提 PR
```

| 步骤 | 命令 | 说明 |
|------|------|------|
| 1. 分支 | `git checkout -b feat/lg-xxx` | 从 main 新建功能分支 |
| 2. 开发 | 写代码 + 写测试（`lg/xxx_test.go`） | Go 标准 testing 包，不引入第三方框架 |
| 3. 单包测试 | `go test -v -run TestXxx ./lg/` | 验证新功能的单元测试 |
| 4. 全量测试 | `go test -race ./...` | 确保不破坏已有功能，竞态检测 |
| 5. lint | `golangci-lint run ./lg/` | 静态检查：errcheck/unused/gosimple |
| 6. 提交 | `git add lg/xxx.go lg/xxx_test.go` | 精确 stage 修改文件，不用 `git add -A` |
| 7. commit | `git commit -m "feat(lg): 描述"` | 格式：`feat|fix|docs(chore)(包名): 简短描述` |
| 8. 提 PR | `git push origin feat/lg-xxx` | 在 GitHub 打开 PR → main，CI 自动验证 |

**SOP 要点：**

- **测试先行**：每个公开函数至少一个测试用例（`TestXxx`），覆盖正常路径
- **不追求覆盖率**：验证核心路径即可，不过度测试
- **命名常量**：魔法数字、固定字符串定义为命名常量
- **函数尺度**：<10 行且 1-2 处调用可直接内联，多处重复或明确语义才提取
- **go vet 零报错**：`go vet ./lg/` 必须通过
