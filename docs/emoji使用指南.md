# Emoji 主题系统使用指南

## 概述

`pi-go` 提供了一套可插拔的 emoji 主题系统，用于结构化 Agent 回复和终端展示。

## 内置主题

系统内置 3 个主题：

### 1. default（默认）
使用完整 emoji 字符，提供丰富的视觉反馈：
```
💬 用户输入
🤖 助手回复
🔧 read_file   # 工具调用
📋 文件内容    # 工具结果
```

### 2. minimal
仅用 ASCII 符号，适合不支持 emoji 的终端：
```
> 用户输入
> 助手回复
[tool] read_file
[result] 文件内容
```

### 3. monochrome
使用简单几何符号，保持单色风格：
```
[U] 用户输入
[A] 助手回复
[T] read_file
[R] 文件内容
```

## 配置方式

### 1. 命令行参数（最高优先级）

```bash
pg -emoji-theme=minimal
pg -emoji-theme=monochrome
```

### 2. 环境变量

```bash
export PIGO_EMOJI_THEME=minimal
pg
```

### 3. 配置文件

编辑 `~/.pi-go/config.yaml`：

```yaml
provider: openai
model: gpt-4o
api_key: sk-...
emoji_theme: minimal
```

或使用命令设置：

```bash
pg config set emoji_theme minimal
```

### 4. 默认值

未配置时自动使用 `default` 主题。

## 主题槽位

每个主题定义以下 8 个语义槽位：

| 槽位 | 说明 | default | minimal | monochrome |
|------|------|---------|---------|------------|
| `assistant` | 助手标识 | 🤖 | > | [A] |
| `user` | 用户标识 | 💬 | > | [U] |
| `tool_call` | 工具调用 | 🔧 | [tool] | [T] |
| `tool_result` | 工具结果 | 📋 | [result] | [R] |
| `step` | 步骤 | ▶ | -> | [>] |
| `success` | 成功 | ✅ | [ok] | [+] |
| `warning` | 警告 | ⚠️ | [!] | [!] |
| `error` | 错误 | ✖ | [x] | [-] |

## 自定义主题

可以通过代码注册自定义主题：

```go
import "github.com/Effortful-lion/pi-go/emoji"

// 创建自定义主题
customTheme := emoji.NewTheme("my-theme", map[emoji.Slot]string{
    emoji.SlotAssistant:  "🤔",
    emoji.SlotUser:       "👤",
    emoji.SlotToolCall:   "⚙️",
    emoji.SlotToolResult: "📄",
    // ... 其他槽位
})

// 注册到全局注册表
emoji.DefaultRegistry.Register(customTheme)

// 使用自定义主题
resolver := emoji.NewResolver(emoji.DefaultRegistry, "my-theme")
```

## 技术细节

### 回退链

1. 用户指定的主题名称
2. Registry 中的同名主题
3. 内置 `default` 主题
4. 纯文本回退（`[A]`, `[U]` 等）

### 包依赖

- `emoji` 包独立，不依赖 `tui` 或 `agent`
- `tui` 通过 `emoji.Resolver` 获取符号
- `agent` 事件协议完全不受影响

### API 参考

```go
// 核心类型
type Slot string
type Theme struct{ Name string; Slots map[Slot]string }
type Registry struct{ /* ... */ }
type Resolver struct{ /* ... */ }

// 主要方法
registry.Register(theme)
registry.Get(name) (Theme, bool)
resolver.Resolve(slot) string
resolver.SetTheme(name)

// 全局实例
emoji.DefaultRegistry  // 内置所有主题
emoji.DefaultResolver  // 使用 default 主题
```

## 兼容性

- 终端不支持 emoji 时，使用 `minimal` 或 `monochrome` 主题
- 主题缺失槽位时，自动回退到纯文本
- 主题解析失败时，直接使用内置回退链
