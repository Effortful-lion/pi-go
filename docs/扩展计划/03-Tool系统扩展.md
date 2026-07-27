# 扩展计划：Tool 系统

> 对应设计文档：`docs/agent-layer-design.md`（Tool 接口为 Agent 层的子模块）
> 当前状态：`tool/tool.go` 仅有 Tool 接口定义，无内置工具实现

---

## 已实现

| 模块 | 文件 | 说明 |
|------|------|------|
| Tool 接口 | `tool/tool.go` | Name / Definition / Execute，3 方法最小契约 |

---

## 概述

Tool 系统是 Agent 的"手"。没有工具的 Agent 只能纯文本对话，有了工具才能：
- 读写文件
- 执行 Shell 命令
- 搜索 Web
- 查询数据库
- 调用外部 API

这是 pi-go 从"聊天机器人"到"编程助手"的关键跨越。

---

## 扩展项

### P0 — File Tool：文件读写

**描述**：提供文件读/写/列表工具，让 Agent 能操作文件系统。

**工具定义**：

| 工具名 | 功能 | 参数 |
|--------|------|------|
| `read_file` | 读取文件内容 | path, [offset], [limit] |
| `write_file` | 写入/覆盖文件 | path, content |
| `list_dir` | 列出目录内容 | path |
| `search_file` | 按通配符搜索文件 | pattern, [recursive] |

**文件变更**：
```
tool/builtin/
├── file.go           # FileTool 实现
└── file_test.go
```

**安全考虑**：
- 沙箱化：限制 Agent 能访问的目录范围（`AllowedRoots []string`）
- 大小限制：读文件时限制最大读取字节数，防止大文件撑爆上下文
- 敏感路径过滤：`/etc/passwd`、`~/.ssh/` 等禁止访问

---

### P0 — Shell Tool：命令执行

**描述**：让 Agent 执行 Shell 命令并获取输出。

**工具定义**：

| 工具名 | 功能 | 参数 |
|--------|------|------|
| `shell_exec` | 执行 Shell 命令 | command, [working_dir], [timeout_ms] |

**设计要点**：
- 命令超时控制（默认 30s）
- stdout + stderr 合并返回，附带 exit code
- 工作目录隔离（默认在项目根目录）

**安全考虑**（非常重要）：
- ❌ 不允许用户确认：Agent 是自动循环的，不应每次命令都弹确认框
- ✅ 命令白名单/黑名单：禁止 `rm -rf /`、`curl | sh` 等危险模式
- ✅ 命令审计日志：每条执行的命令记录到日志
- ✅ 沙箱化：每个命令在受限环境中运行

**文件变更**：
```
tool/builtin/
├── shell.go
└── shell_test.go
```

---

### P1 — [待考虑]Web Search Tool

**描述**：让 Agent 搜索互联网获取最新信息。

**实现方式**：
- 方案一：调用搜索引擎 API（Bing/Google API Key）
- 方案二：调用 SerpAPI/DuckDuckGo 等封装服务
- 方案三：直接 HTTP 抓取（适合特定 URL 场景）

**工具定义**：

| 工具名 | 功能 | 参数 |
|--------|------|------|
| `web_search` | 搜索互联网 | query, [max_results] |
| `web_fetch` | 抓取网页内容 | url |

---

### P1 — [待考虑]MCP 工具协议支持

**描述**：支持 [Model Context Protocol (MCP)](https://modelcontextprotocol.io/)，Agent 可以连接 MCP Server 获取动态工具。

**为什么需要**：
- MCP 是 Anthropic 提出的标准协议，大型生态正在形成
- 很多工具已经提供了 MCP Server（文件系统、数据库、GitHub API 等）
- 通过 MCP 接入比每个工具都自己实现更可持续

**实现思路**：
```go
type MCPTool struct {
    client *mcp.Client
    tool   mcp.Tool
}

func (t *MCPTool) Execute(ctx context.Context, args map[string]any) (string, error) {
    return t.client.CallTool(ctx, t.tool.Name, args)
}
```

**文件变更**：
```
tool/mcp/
├── client.go          # MCP 客户端封装
├── tool.go            # MCPTool 适配器
└── client_test.go
```

---

### P2 — [待考虑]工具安全框架

**描述**：统一的工具安全策略层。

**实现**：
```go
type SafetyPolicy struct {
    AllowedRoots []string      // 文件工具：允许访问的目录
    BlockedCmds  []string       // Shell 工具：禁止的命令模式
    MaxFileSize  int64          // 文件工具：最大读取字节数
    CmdTimeout   time.Duration  // Shell 工具：命令超时
    RequireConfirm bool         // 全局：是否需要用户确认（默认 false）
}

// Apply 包装任意 Tool，注入安全检查
func Apply(tool Tool, policy SafetyPolicy) Tool {
    return &safeTool{inner: tool, policy: policy}
}
```

---

## 优先级总结

| 优先级 | 项目 | 工作量 | 风险 |
|--------|------|--------|------|
| P0 | File Tool | 中 | 低 |
| P0 | Shell Tool | 中 | **高**（安全敏感） |
| P1 | Web Search Tool | 中 | 低 |
| P1 | MCP 协议支持 | 大 | 中 |
| P2 | 工具安全框架 | 中 | 低 |
