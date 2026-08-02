# TODO — Pi-Go 待办清单

## Coding Agent CLI（当前主线）

> **背景**：产品定位为 coding agent CLI，必须具备"读项目、改文件"的 coding 能力，且改动应支持 dry-run（先预览再落盘）。
>
> **现状**：`tool/builtin/` 的 4 个文件工具（`read_file`/`write_file`/`list_dir`/`search_file`）+ `shell` 工具已在库层面实现完毕，但 `cmd/pg/chat.go` 创建 Agent 时未传入 `Tools`——CLI 产品实际运行时 Agent 无任何工具可用（库已就绪、产品未组装）。同时缺少结构化编辑工具（只有全量覆盖的 `write_file`），且完全没有 dry-run 能力。

### 一、基础目标（让产品真正具备 coding 能力）

#### 目标 1：工具接入产品，Agent 真正能干活（P0）

- [ ] **1.1 注入文件工具**：在 `cmd/pg/chat.go` 的 `agent.New(...)` 中传入 `builtin.NewFileTools(...)` 返回的 4 个工具（`read_file`/`write_file`/`list_dir`/`search_file`）
  - `FileConfig.MaxReadSize` 使用默认值 1MB
  - 工具切片统一在一处组装后传入 `agent.Config.Tools`，不要分散多处
- [ ] **1.2 注入 shell 工具**：同处传入 `builtin.NewShellTool(...)`
  - 使用默认危险命令黑名单（`rm -rf`/`sudo`/`chmod 777`/`curl|sh` 等）
  - `CmdTimeout` 使用默认值 30s
- [ ] **1.3 AllowedRoots 绑定工作目录**：启动时取 `os.Getwd()` 作为 `FileConfig.AllowedRoots` 的首个根目录，限制文件工具只能操作当前项目目录；同时新增 CLI flag `-allow-root`（可重复传入多个），允许用户显式追加可访问的根目录
- [ ] **1.4 system prompt 加入工具使用规范**：在 `cmd/pg/main.go` 的 `defaultSystemPrompt` 中追加一段编码工作流约束，明确告知 LLM：
  - 改文件前必须先用 `read_file` 读取目标文件，禁止盲改
  - 用 `list_dir`/`search_file` 探索项目结构，不要猜测文件路径
  - 改动完成后用 `shell` 运行构建/测试命令验证（如 `go build ./...`）
- [ ] **1.5 端到端验证**：`pg chat` 中输入"在当前目录创建一个 hello.go，打印 Hello，然后用 go run 运行验证"，确认 Agent 能自主完成 写文件 → 执行命令 → 汇报结果 的完整闭环

#### 目标 2：结构化代码编辑能力（P0）

> 现状只有 `write_file`（全量覆盖），改一行代码需要 LLM 重新输出整个文件，浪费 token 且容易出错。

- [ ] **2.1 实现 `replace_in_file` 工具**（新文件 `tool/builtin/edit.go`）
  - 参数：`path`（必填）、`old_string`（必填，被替换的原文）、`new_string`（必填，新内容）、`replace_all`（可选布尔，默认 false）
  - 语义：`old_string` 在文件中**唯一匹配**才执行替换；匹配到多处时返回错误并列出各匹配位置的行号，让 LLM 补充更多上下文后重试；匹配不到时返回错误
  - 必须走 `AllowedRoots` 校验，与现有文件工具一致
- [ ] **2.2 实现 `insert_text` 工具**（同文件）
  - 参数：`path`（必填）、`line`（必填，1-based 行号，表示插入到该行之前）、`content`（必填）
  - 用于"在第 N 行插入 import/函数"等精确插入场景；行号越界时返回错误
- [ ] **2.3 工厂函数**：新增 `builtin.NewEditTools(cfg FileConfig) []tool.Tool`，与 `NewFileTools` 平级，返回上述 2 个工具
- [ ] **2.4 接入产品**：在 `cmd/pg/chat.go` 组装工具处一并注入编辑工具
- [ ] **2.5 单元测试**（`tool/builtin/edit_test.go`）：覆盖 唯一匹配替换成功 / 多处匹配报错含行号 / 无匹配报错 / replace_all 全量替换 / 路径越出 AllowedRoots 报错 / insert_text 正常插入与行号越界

#### 目标 3：dry-run 变更预览（P0）

> 现状所有写入操作直接落盘，没有任何"先看再改"的预览机制。coding agent 必须支持 dry-run。

- [ ] **3.1 设计先行**：在 `docs/spec/` 写 dry-run 设计文档并与用户确认，明确：
  - dry-run 的作用范围（`write_file`/`replace_in_file`/`insert_text`，`shell` 是否纳入待定）
  - 交互方案选型：A) 写入工具返回 diff 文本由 LLM 自行决策；B) Agent 层两阶段执行（先 preview 等确认再 commit）
- [ ] **3.2 实现 unified diff 生成**（新包 `tool/diff/`）
  - 签名：`func Unified(old, new, path string) string`，输出 git 风格 unified diff（带 `---`/`+++`/`@@` 头，`+`/`-` 前缀行）
  - 不引入第三方 diff 库，行级对比即可，第一版可用最简单的逐行 LCS
- [ ] **3.3 CLI 新增 `-dry-run` flag**：开启后所有写入类工具不实际落盘，返回 "dry-run: 未写入，变更预览如下" + unified diff 文本；TUI 中正常展示该 diff
- [ ] **3.4 diff 着色**：TUI 渲染工具结果时，对 diff 内容按行着色（`+` 行绿色、`-` 行红色、`@@` 行青色）
- [ ] **3.5 单元测试**：diff 输出格式正确性；dry-run 开启时文件内容不被修改

### 二、扩展目标（产品体验与工程能力）

#### 目标 4：项目感知与上下文提供（P1）

- [ ] **4.1 启动时项目结构注入**：`runChat` 启动时自动对工作目录做一次浅层 `list_dir`（仅顶层），将结果作为项目上下文附加到 system prompt 或首条 user 消息，避免 Agent 每次从零探索浪费 token
- [ ] **4.2 项目类型自动识别**：检测工作目录下的标志性文件并在上下文中追加对应工作流提示——`go.mod`→Go（`go build`/`go test`）、`package.json`→Node、 `Cargo.toml`→Rust、`pyproject.toml`/`requirements.txt`→Python
- [ ] **4.3 `read_files` 批量读取工具**（可选）：参数 `paths` 数组，一次读多个文件，减少 tool call 轮数

#### 目标 5：Agent 工程能力增强（P1）

- [ ] **5.1 高危操作交互确认**：`shell` 工具执行命中"写操作"特征（如 `rm`/`mv`/`>`重定向/git push）的命令前，通过 TUI 向用户请求确认（y/n），确认后才执行；需 Agent 事件流新增"等待确认"事件类型
- [ ] **5.2 Subscribe() 多事件监听器**：Agent 事件从单 channel 改为观察者模式，TUI 与 logger 独立订阅（原远期 #8）
- [ ] **5.3 ToolChoice 策略**：支持 forced/none/auto，透传至 `ai.Context`（原远期 #9）
- [ ] **5.4 Parallel Tool Calls**：单步内多个工具调用并行执行（原远期 #10）

#### 目标 6：变更可追溯（P1）

- [ ] **6.1 工具事件持久化**：Session 记录补充 `EventToolCall`/`EventToolResult`，事后可回放"Agent 改了哪些文件、执行了什么命令"
- [ ] **6.2 `/undo` 命令**：撤销最后一次写入操作——每次 `write_file`/`replace_in_file`/`insert_text` 落盘前备份原内容（内存或临时文件），TUI 中 `/undo` 还原

#### 目标 7：测试与文档（P1）

- [ ] **7.1 集成测试**：用 mock Provider 验证 Agent 调用文件工具的完整链路（读→改→验证）
- [ ] **7.2 `产品使用文档.md`**：安装、配置、基本使用、工具列表、dry-run 模式说明
- [ ] **7.3 `docs/learning/` 学习总结**：coding agent 架构设计总结（为什么这套架构构成 coding agent）

### 三、推进顺序（MVP 关键路径）

| 阶段 | 目标 | 完成后的能力 |
|------|------|-------------|
| 第 1 阶段 | 目标 1：工具接入 | Agent 能读写文件、执行命令，但改一行也要重写整个文件 |
| 第 2 阶段 | 目标 2：结构化编辑 | Agent 能精准替换/插入代码，效率大幅提升 |
| 第 3 阶段 | 目标 3：dry-run | 改动前可预览 diff，变更可控 |
| 第 4 阶段 | 目标 4-7：体验增强 | 更好用、更可追溯 |

**P0 关键路径**：1.1 → 1.2 → 1.3 → 1.4 → 1.5 → 2.1 → 2.4 → 2.5 → 3.2 → 3.3 → 3.5

---

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
