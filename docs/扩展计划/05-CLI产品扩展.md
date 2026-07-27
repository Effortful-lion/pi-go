# 扩展计划：CLI 产品

> 对应设计文档：`docs/cli-entry-design.md`
> 源文件"不做什么"章节（这些"不做"的正是后续需要做的）

---

## 已实现

| 模块 | 文件 | 说明 |
|------|------|------|
| CLI 入口 | `cmd/pi/main.go` | flag 参数解析 + 环境变量回退 + 组装启动 |

---

## 扩展项

### P0 — 配置文件支持

这里我更希望使用：yaml文件 + .env文件

**描述**：支持从配置文件读取参数，不再仅靠命令行 + 环境变量。

**为什么需要**：
- 当前每次启动都要传 `-api-key` 或设环境变量，很麻烦
- 有经验用户的配置项多（API Key、模型、BaseURL、System Prompt），全部靠命令参数太冗长

**配置文件格式**：YAML（易读、Go 原生支持 `gopkg.in/yaml.v3`）

```yaml
# ~/.pi-go/config.yaml
provider: openai
model: gpt-4o
api_key: sk-xxx
base_url: ""                    # 空字符串 = 使用默认
system_prompt: |
  You are Pi Agent...
max_steps: 10
temperature: 0.7
max_tokens: 4096

# 可选：自定义日志配置
log:
  level: info
  file: ~/.pi-go/logs/pi.log

# 可选：安全策略
safety:
  allowed_roots: ["~/workspace/"]
  blocked_cmds: ["rm -rf", "sudo"]
  cmd_timeout: 30s
  max_file_size: 1MB
```

**参数优先级**：命令行 > 环境变量 > 配置文件 > 默认值

**文件查找顺序**：
1. `-config` 参数指定的路径
2. `./.pi-go.yaml`（项目级配置）
3. `~/.pi-go/config.yaml`（用户级配置）

**文件变更**：
```
cmd/pi/
├── main.go         # 增加 -config 参数 + 配置文件加载
└── config.go       # Config 结构体 + LoadConfig 函数
```

**依赖**：引入 `gopkg.in/yaml.v3`

---

### P1 — 子命令系统

还需要 pi help 命令做命令手册

**描述**：从单一 `pi` 命令扩展为 `pi chat`/`pi config`/`pi version` 子命令。

**为什么需要**：
- 配置文件（P0）需要 `pi config init` 来生成初始配置
- 未来还有 `pi export`（导出对话）、`pi list-models`（列出可用模型）
- 单一命令负担太重，子命令是 CLI 工具成熟的标志

**命令设计**：
```
pi
├── chat          # 启动交互式对话（当前功能，默认命令）
│   └── pi chat   # 或直接 pi（默认行为）
├── config        # 配置管理
│   ├── pi config init      # 生成默认配置文件
│   ├── pi config show      # 显示当前配置
│   └── pi config set key value  # 设置单个配置项
├── export        # 对话导出
│   └── pi export --last   # 导出最后一次对话
├── list-models   # 列出可用模型
│   └── pi list-models
└── version       # 版本信息
    └── pi version
```

**框架选择**：
- 选项一（推荐）：Go 1.26 开始 `flag` 支持子命令，标准库即可
- 选项二：引入 `cobra`。cobra 生态成熟，但早期过度设计

**文件变更**：
```
cmd/pi/
├── main.go         # 命令路由
├── chat.go         # chat 子命令
├── config_cmd.go   # config 子命令
└── config.go       # 配置文件读写
```

---

### P1 — Session 持久化

**描述**：自动保存和恢复对话历史。

**为什么需要**：
- 当前关闭终端后对话丢失，无法"继续上次的对话"
- 编程任务经常跨越多个会话

**存储格式**：JSONL（每行一个 JSON，逐行追加）

```
~/.pi-go/sessions/
├── 2024-01-15-1430.jsonl   # 按时间命名
├── 2024-01-15-1530.jsonl
└── latest -> 2024-01-15-1530.jsonl
```

```json
{"role":"system","content":"You are Pi Agent..."}
{"role":"user","content":"帮我写个 HTTP server"}
{"role":"assistant","content":"好的..."}
```

**CLI 使用方式**：
```bash
pi chat                     # 开始新对话（自动创建 session 文件）
pi chat --continue          # 继续上次对话
pi chat --session 2024-01-15-1430  # 继续指定对话
pi export --session latest  # 导出对话
```

**实现要点**：
- 每条消息实时追加写入（不等到对话结束，防止崩溃丢失）
- autosave 定时刷新（写 buffer）
- 依赖 `PI_SESSION_DIR` 环境变量指定存储目录

**文件变更**：
```
agent/
├── session.go       # Session 管理器
└── session_test.go
cmd/pi/
├── main.go          # --continue / --session 参数
```

---

### P2 — [待考虑]多 Agent 协作

**描述**：支持启动多个 Agent 协作完成任务（子 Agent 模式）。

**场景**：
```
主 Agent: "这个项目需要重构"
  ├── 子 Agent 1: 分析代码结构
  ├── 子 Agent 2: 重构 API 层
  └── 子 Agent 3: 更新测试
```

**选择**：P2 优先级，等核心循环完全稳定后再考虑。目前单 Agent + Tool 模式已覆盖大部分场景。

---

### P2 — [待考虑]RPC/HTTP Server 模式

**描述**：除了交互式终端模式，还支持 HTTP Server 模式供外部调用。

**场景**：
- IDE 插件通过 HTTP API 调用 pi-agent
- CI/CD 中集成 Agent 做代码审查

**命令**：
```bash
pi serve --port 9090    # 启动 HTTP Server
```

**文件变更**：
```
cmd/pi/serve.go     # HTTP Server 模式
```

---

### P2 — [待考虑]插件系统

**描述**：允许第三方以 plugin 形式提供 Tool 和 Provider。

**实现思路**：
- Go plugin（`plugin` 包）：编译为 `.so` 动态加载，但平台兼容性差
- WASM：更现代、跨平台、安全沙箱化
- 进程模式：通过 MCP 协议通信（推荐，与 MCP Tool 合并）

---

## 优先级总结

| 优先级 | 项目 | 工作量 | 新增依赖 |
|--------|------|--------|----------|
| P0 | 配置文件支持 | 中 | `gopkg.in/yaml.v3` |
| P1 | 子命令系统 | 中 | 无（Go 1.26 flag 子命令） |
| P1 | Session 持久化 | 中 | 无 |
| P2 | 多 Agent 协作 | 大 | 无 |
| P2 | RPC/HTTP Server | 中 | 无 |
| P2 | 插件系统 | 大 | MCP 协议 |
