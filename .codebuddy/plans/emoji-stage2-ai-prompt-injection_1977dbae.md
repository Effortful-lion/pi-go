---
name: emoji-stage2-ai-prompt-injection
overview: 补全 emoji 可插拔系统的阶段 2（AI 层提示词注入），打通 emojiTheme → agent.Config → ai.Context 的透传链，并可选地将主题约束注入 system prompt。
todos:
  - id: update-agent-config
    content: 修改 agent/agent.go：Config 增加 EmojiTheme 字段，新增 emojiThemeHint 常量，New() 中注入提示词到 system prompt，buildContext() 透传到 ai.Context
    status: completed
  - id: update-cmd-chat
    content: 修改 cmd/pg/chat.go：创建 agent.Config 时传入 EmojiTheme 字段
    status: completed
    dependencies:
      - update-agent-config
  - id: update-changelog
    content: 新建 change-log/2026-8-1.md 记录阶段 2 补全
    status: completed
    dependencies:
      - update-agent-config
      - update-cmd-chat
  - id: verify-build-test
    content: 运行 go build ./... 和 go test ./... 验证编译与测试通过
    status: completed
    dependencies:
      - update-cmd-chat
---

## 用户需求

根据 `docs/emoji设计.md` 和 `docs/emoji实现计划.md`，补全 emoji 可插拔功能阶段 2：将 EmojiTheme 从配置层透传到 AI 层，并在 system prompt 中注入轻量结构化表达约束，引导模型使用语义化输出方式。

## 核心功能

- **Config 透传**：`agent.Config` 新增 `EmojiTheme` 字段，沿 `cmd`→`agent`→`ai` 链路传递主题名称
- **System Prompt 注入**：当配置了 emoji 主题时，自动在 system prompt 尾部追加一段轻量结构化表达规范，引导模型使用短段落、明确小标题和一致的语义标记
- **零侵入**：不动 agent 事件协议（`agent/event.go`），不动 Provider 实现（`ai/providers/`），只沿现有透传链补一个字段

## 技术栈

- Go 1.x（项目现用）
- 仅涉及现有包：`agent`、`ai`、`cmd/pg`

## 实现方案

### 策略

沿已有配置透传模式（`Temperature`、`MaxTokens` 同样的路径），将 `EmojiTheme` 从 CLI 入口传入 Agent，Agent 在构建 `ai.Context` 时透传，并在初始化 system prompt 时注入结构化表达提示。

### 透传链

```
cmd/pg/chat.go (emojiTheme 变量, 已有)
    ↓ 加 1 行：agent.Config.EmojiTheme
agent/agent.go Config (新增字段)
    ↓ New() 中：拼入 SystemPrompt 文本尾部
    ↓ buildContext() 中：赋值到 ai.Context
ai.Context.EmojiTheme (字段已存在)
```

### 提示词注入设计

不引入复杂的 prompt 模板系统。在 `agent/agent.go` 中定义一个包级常量 `emojiThemeHint`，内容是设计文档要求的轻量规范。当 `Config.EmojiTheme` 非空时，在 `New()` 中将该常量追加到用户配置的 system prompt 尾部。

提示词内容遵循设计文档（`docs/emoji设计.md` 第 83-88 行）：

- 回复优先使用短段落和明确小标题
- 工具调用前后使用一致的语义标记
- 不强制模型生成 emoji 字符本身，只要求语义化表达

### 关键决策

- **提示词放在 agent 层而非 ai 层**：因为 ai 层是纯粹的 LLM 接口抽象，不应携带业务含义的 prompt 内容；agent 层负责对话策略，是注入输出风格约束的正确位置
- **不做主题相关的差异化提示**：所有主题共用同一段结构化表达规范，因为提示词的目的是引导模型的行为模式（结构化），而不是要求模型输出特定字符；特定字符由 tui 层的 resolver 负责
- **不提取为独立函数**：提示词注入逻辑 < 5 行，只有 `New()` 一处调用，直接内联即可（遵循 AGENTS.md 准则 5）

## 实现细节

### 改动清单

| 文件 | 操作 | 内容 |
| --- | --- | --- |
| `agent/agent.go` | MODIFY | `Config` 加 `EmojiTheme` 字段；新增 `emojiThemeHint` 常量；`New()` 中当字段非空时追加提示到 system prompt；`buildContext()` 透传字段 |
| `cmd/pg/chat.go` | MODIFY | `agent.Config{}` 字面量中加 `EmojiTheme: emojiTheme` |
| `change-log/2026-8-1.md` | NEW | 变更日志 |


### 向后兼容

- `EmojiTheme` 为空（默认值）时行为完全不变：system prompt 不追加任何内容，`ai.Context.EmojiTheme` 保持空字符串
- 不影响任何测试：现有 `agent_test.go` 使用零值 Config 即可通过

### 验证方案

- 不配置 emoji_theme：system prompt 不变，输出行为不变
- 配置 `emoji_theme=minimal`：system prompt 尾部出现结构化表达提示，`ai.Context.EmojiTheme = "minimal"`