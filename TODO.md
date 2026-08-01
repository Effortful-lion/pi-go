# TODO — Pi-Go 待办清单

## 代码 TODO（需立即修复）

| # | 文件 | 行号 | 内容 |
|---|------|------|------|
| 1 | `ai/providers/my-openai/provider.go` | 114 | `chat()` 方法中 `buildOpenAIRuquestBody` 未实现 |
| 2 | `ai/providers/my-openai/provider.go` | 257 | 流式解析中工具调用的累积处理不完整 |
| 3 | `ai/providers/my-openai/provider.go` | 306 | 请求体 message 组合顺序待验证 |

## 近期开发

| # | 模块 | 内容 | 优先级 |
|---|------|------|--------|
| 4 | TUI | 代码块状态机：`MarkdownLine` 改为有状态版本，正确处理跨行 ``` 代码块 | P0 |
| 5 | TUI | 差异渲染时清除多余行的逻辑完善 | P1 |
| 6 | Agent | 错误处理和中断重连机制 | P1 |
| 7 | CLI | 输入历史持久化到文件（重启后保留历史记录） | P1 |

## 远期规划

| # | 模块 | 内容 | 优先级 |
|---|------|------|--------|
| 8 | Agent | Subscribe() 多事件监听器（从单 channel 改为观察者模式） | P0 |
| 9 | Agent | ToolChoice 策略（forced/none/auto） | P1 |
| 10 | Agent | Parallel Tool Calls 并行工具执行 | P1 |
| 11 | AI | Provider 自动注册与发现 | P2 |
| 12 | AI | 流式处理优化（sync.Pool 复用、json.Decoder 流式解码） | P2 |
| 13 | AI | 请求重试与错误恢复（指数退避重试 429/5xx） | P2 |
| 14 | Tool | Web Search Tool（搜索引擎集成） | P1 |
| 15 | Tool | MCP 工具协议支持 | P1 |
| 16 | Tool | 工具安全框架（统一安全策略层） | P2 |
| 17 | CLI | 多 Agent 协作（子 Agent 模式） | P2 |
| 18 | CLI | RPC/HTTP Server 模式（`pg serve`） | P2 |
| 19 | CLI | 插件系统（Go plugin / WASM / MCP） | P2 |

## 文档缺失

| # | 内容 | 说明 |
|---|------|------|
| 20 | `产品使用文档.md` | 面向终端用户：安装、配置、基本使用、高级功能、常见问题 |
| 21 | `agent/state.go` | 设计文档中规划但未实现的状态管理模块 |

## 测试缺失

| # | 包 | 说明 |
|---|------|------|
| 22 | `ai/` | AI 层核心类型和接口 |
| 23 | `ai/providers/anthropic/` | Anthropic Provider |
| 24 | `ai/providers/google/` | Google Gemini Provider |
| 25 | `tool/` + `tool/builtin/` | Tool 接口 + File/Shell 内置工具 |
| 26 | `cmd/pg/` | CLI 产品入口 |
| 27 | `agent/session.go` | Session 持久化管理器 |
| 28 | `tui/highlight.go`、`tui/markdown.go`、`tui/box.go` | TUI 子模块 |
