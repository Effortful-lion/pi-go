# 学习总结：Tool 接口

## 开发时序：第 4 步

### 为什么 Tool 排在 Agent 之前？

Agent 运行时依赖 Tool 接口。必须在写 Agent 的"大脑"之前，先定义"手"能做什么。

**核心原则**：依赖方向 = 开发顺序。被依赖的先写。

---

## 概念解释

### Agent 的工具是什么？

当用户说"帮我查一下今天北京的天气"，Agent 不能靠自己的知识回答（LLM 的知识截止于训练日期），它需要调用一个外部工具：

```
用户: 北京今天天气怎么样？
Agent: 我需要调用天气查询工具...
       → get_weather(city="北京")
       ← 晴，25°C，湿度 60%
Agent: 北京今天天气晴朗，温度 25°C...
```

这个 `get_weather` 就是一个 **Tool**。它可以调用外部 API、读写文件、执行 Shell 命令、查询数据库等等。

### 工具的完整生命周期

```
1. Agent 告诉 LLM："你可以用这些工具"（发送 ToolDefinition）
2. LLM 决定调用某个工具（返回 ToolCall）
3. Agent 执行工具（Execute）
4. Agent 把结果告诉 LLM（把 ToolResult 注入对话历史）
5. LLM 根据结果生成最终回复
```

---

## Tool 接口设计

### 最小契约

```go
type Tool interface {
    Name() string
    Definition() ai.ToolDefinition
    Execute(ctx context.Context, args map[string]any) (string, error)
}
```

只有 3 个方法：

| 方法 | 用途 | 谁用 |
|------|------|------|
| `Name()` | 返回工具标识 | Agent 自己做索引 |
| `Definition()` | 返回工具的"使用说明书" | 发送给 LLM |
| `Execute()` | 真正执行工具逻辑 | Agent 调用 |

### 为什么放在独立的 `tool/` 包？

```
┌────────────────┐
│   agent/       │  import →  "tool"
│   Agent 运行时  │     ↓
├────────────────┤
│   tool/        │  import →  "ai"
│   Tool 接口     │     ↓
├────────────────┤
│   ai/          │
│   核心类型      │
└────────────────┘
```

`tool` 包只依赖 `ai`（因为 `Definition()` 返回 `ai.ToolDefinition`），不依赖 `agent`。这使得：
- `tool/` 可以被其他项目独立引用（"我只需要你的 Tool 接口，不需要你的 Agent"）
- 避免循环依赖

### ToolDefinition：给 LLM 看的说明书

```go
type ToolDefinition struct {
    Name        string         // "get_weather"
    Description string         // "查询指定城市的实时天气"
    Parameters  map[string]any // {"type":"object", "properties":{"city":{"type":"string"}},...}
}
```

LLM 通过 `Description` 理解工具的用途，通过 `Parameters`（JSON Schema）知道怎么传参数。**Description 写得越好，LLM 调用工具越准确。**

---

## 一个简单的工具实例

```go
type WeatherTool struct{}

func (t *WeatherTool) Name() string {
    return "get_weather"
}

func (t *WeatherTool) Definition() ai.ToolDefinition {
    return ai.ToolDefinition{
        Name:        "get_weather",
        Description: "查询指定城市的实时天气，返回温度、湿度、天气状况",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "city": map[string]any{
                    "type":        "string",
                    "description": "城市名称，如 '北京'、'上海'",
                },
            },
            "required": []string{"city"},
        },
    }
}

func (t *WeatherTool) Execute(ctx context.Context, args map[string]any) (string, error) {
    city, _ := args["city"].(string)
    // 调用真实的天气 API...
    return fmt.Sprintf("晴，25°C，湿度 60%%"), nil
}
```

---

## 设计哲学：接口要小

| 设计 | 好/坏 | 理由 |
|------|-------|------|
| 3 个方法 | ✅ 好 | 一个工具开发者只需实现 3 个方法，门槛极低 |
| 带 Initialize/Close | ❌ 坏 | 增加了心智负担，大部分工具不需要 |
| 带 Tool 注册表 | ❌ 坏 | 注册表是 Agent 的职责，不是 Tool 包的职责 |

**Go 的接口哲学**：定义最少的契约，给实现者最大的自由。如果接口只有 3 个方法，实现者只需要关注"我用什么工具、怎么定义它、怎么执行它"。

---

## 思想总结

| 思想 | 体现 |
|------|------|
| **依赖方向决定开发顺序** | Tool 被 Agent 依赖 → Tool 先写 |
| **最小接口契约** | 3 个方法，够用即可 |
| **包独立性** | `tool/` 独立于 `agent/`，可被其他项目引用 |
| **关注点分离** | Tool 只管自己的逻辑，不知道在什么上下文中被调用 |
