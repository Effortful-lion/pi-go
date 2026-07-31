# Emoji 主题系统设计

## 目标

为 `pi-go` 提供一套可插拔、可替换的 emoji 主题系统，用于两件事：

1. 让 Agent 回复更结构化
2. 让终端展示更生动，但不把 emoji 写死在业务代码里

## 设计原则

- 不改 `agent` 的核心事件协议
- `ai` 负责“怎么引导模型表达”
- `tui` 负责“怎么渲染给用户看”
- emoji 必须可替换、可降级、可测试

## 总体架构

新增独立 `emoji/` 包，作为语义映射层。

```text
ai.Context -> system prompt 里的 emoji/style 约束
agent.Event -> 保持不变
emoji.Theme -> 语义到符号的映射
tui -> 读取主题并渲染
```

## 核心组件

### `emoji.Theme`

定义一组语义槽位，而不是单个字符。

建议槽位：

- `assistant`
- `user`
- `toolCall`
- `toolResult`
- `step`
- `success`
- `warning`
- `error`

### `emoji.Registry`

管理主题注册、默认主题、用户自定义主题和回退链。

### `emoji.Resolver`

根据当前场景解析最终显示值：

1. 用户指定主题
2. 项目默认主题
3. 内置默认主题
4. 纯文本回退

## 数据流

1. `agent` 产出标准事件
2. `tui` 读取事件
3. `tui` 通过 `emoji.Resolver` 获取符号
4. `tui` 统一渲染前缀、状态、结果
5. `ai.Context.SystemPrompt` 注入输出约束，提示模型使用结构化表达

## 配置方式

优先级建议：

1. 命令行参数
2. 配置文件
3. 环境变量
4. 默认主题

建议支持：

- `emoji_theme: default`
- `emoji_theme: minimal`
- `emoji_theme: monochrome`

## 输出约束

system prompt 中补充一段轻量规范：

- 回复优先使用短段落和明确小标题
- 工具调用前后使用一致的语义标记
- 不强制模型生成 emoji 字符本身，只要求语义化表达

这样可以避免模型过度依赖具体字符，也方便主题替换。

## 兼容策略

- 终端不支持彩色或 emoji 时，自动降级为 ASCII 前缀
- 主题缺失槽位时，使用默认值补齐
- 主题解析失败时，直接回退到内置主题

## 建议目录

```text
emoji/
  theme.go
  registry.go
  resolver.go
  builtin/
    default.go
    minimal.go
    monochrome.go
```

## 预期改动点

- `ai/types.go`：为上下文增加可选输出风格入口
- `agent/`：保持事件不变，只在构建上下文时透传风格信息
- `tui/chat.go`：移除硬编码 emoji 前缀，改为主题解析
- `tui/markdown.go`：不直接依赖 emoji，保持纯渲染职责

## 测试策略

- 主题解析测试
- 回退链测试
- `tui` 渲染前缀测试
- system prompt 片段测试

## 范围边界

本设计只解决“结构化表达的 emoji 主题化”。

不做：

- 富表情包系统
- 复杂动画
- 网络化主题下载
- 运行时在线切换整套 UI 风格

