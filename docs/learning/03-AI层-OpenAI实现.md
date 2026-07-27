# 学习总结：AI 层 — OpenAI Provider 实现

## 开发时序：第 3 步

### 为什么选择 OpenAI 作为第一个实现？

有了核心接口定义（上一步），需要一个"试金石"来验证接口是否合理。

选择 OpenAI 的理由：
1. **最成熟的 LLM API**，文档完善，社区参考多
2. **SSE（Server-Sent Events）** 是标准 HTTP 协议，无特殊依赖
3. 这是 pi-agent 最重要的 Provider，用户最常用

---

## 开发思路：渐进实现

```
Step 1: 理解 OpenAI Chat Completions Streaming API 的工作方式
Step 2: 用 Go 标准库 net/http 发送请求
Step 3: 解析 SSE 响应流
Step 4: 将 SSE 数据映射为 ai.Event 流
```

核心原则：**只用 Go 标准库，不引入任何第三方 HTTP/SSE 包**。

---

## SSE 协议简释

### 什么是 SSE？

SSE（Server-Sent Events）是 HTTP 的一种使用方式，服务器不断推送数据：

```
HTTP Response:
Content-Type: text/event-stream

data: {"id":"chatcmpl-123","choices":[{"delta":{"content":"你"}}]}

data: {"id":"chatcmpl-123","choices":[{"delta":{"content":"好"}}]}

data: {"id":"chatcmpl-123","choices":[{"delta":{"content":"！"}}]}

data: [DONE]
```

每条 `data:` 行是一个 JSON 事件，`[DONE]` 表示流结束。

### OpenAI 的 SSE 负载结构

```json
{
  "id": "chatcmpl-xxx",
  "choices": [{
    "index": 0,
    "delta": {
      "role": "assistant",
      "content": "你好",
      "tool_calls": [{
        "index": 0,
        "id": "call_xxx",
        "function": {
          "name": "get_weather",
          "arguments": "{\"city\":"
        }
      }]
    }
  }],
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 200,
    "total_tokens": 300
  }
}
```

**注意**：`tool_calls[].function.arguments` 是一个 **JSON 字符串**（二次编码），需要解码：

```
"{\"city\":" ← 一个字符串，内容是 JSON
需要 json.Unmarshal 解码成: {"city": ...}
```

这是 OpenAI API 的一个"怪癖"——arguments 的增量是 JSON 文本，只有当所有片段拼接完成后，才是一个合法的 JSON 对象。

---

## 核心流程

### 1. Provider 创建

```go
type Config struct {
    APIKey  string
    BaseURL string // 空字符串 = 使用默认域名
}

func New(cfg Config) *Provider {
    if cfg.BaseURL == "" {
        cfg.BaseURL = "https://api.openai.com/v1"
    }
    return &Provider{
        config: cfg,
        client: &http.Client{},
    }
}
```

**关键决策**：为什么有 `BaseURL` 可配置？
→ 不仅支持 OpenAI 官方 API，还能兼容所有 OpenAI 格式的第三方 API（如 Azure、本地 vLLM、Ollama 等）。只要提供方实现了 `/v1/chat/completions` 端点，就能无缝替换。

### 2. Chat 方法：请求 → 流式响应

```go
func (m *Model) Chat(ctx context.Context, context ai.Context) ai.Stream {
    ch := make(chan ai.Event, 10) // 带缓冲的 channel

    go func() {
        defer close(ch)

        // 1. 构建请求体
        body := m.buildRequest(context)

        // 2. 发送 HTTP POST
        req, _ := http.NewRequestWithContext(ctx, "POST", url, body)
        req.Header.Set("Authorization", "Bearer "+apiKey)
        req.Header.Set("Content-Type", "application/json")
        resp, _ := m.client.Do(req)
        defer resp.Body.Close()

        // 3. 解析 SSE 流 → ai.Event
        ch <- ai.Event{Type: ai.EventStart}
        m.parseSSE(resp.Body, ch)
    }()

    return ch
}
```

**关键设计**：
- `goroutine + channel` 是 Go 的天然并发模式：主 goroutine 返回 channel 立即返回，子 goroutine 在后台上传解析。
- `chan ai.Event` 带缓冲 10：避免子 goroutine 因为主 goroutine 消费慢而阻塞。
- `defer close(ch)`：确保 stream 的 `for...range` 循环能正常退出。

### 3. SSE 解析：按行读取

```go
func (m *Model) parseSSE(body io.Reader, ch chan<- ai.Event) {
    scanner := bufio.NewScanner(body)
    for scanner.Scan() {
        line := scanner.Text()
        if line == "" {
            continue // 跳过空行
        }
        if line == "data: [DONE]" {
            ch <- ai.Event{Type: ai.EventDone}
            return
        }
        if strings.HasPrefix(line, "data: ") {
            data := strings.TrimPrefix(line, "data: ")
            m.processChunk(data, ch)
        }
    }
}
```

`bufio.Scanner` 是 Go 标准库的按行读取器，非常适合 SSE 协议（每个事件换行分隔）。

### 4. SSE → Event 映射

这是整个实现中最核心的逻辑。需要从 OpenAI 的 JSON 数据中提取信息，映射为 `ai.Event` 类型。

```go
func (m *Model) processChunk(raw string, ch chan<- ai.Event) {
    var chunk openAISSEChunk
    json.Unmarshal([]byte(raw), &chunk)

    for _, choice := range chunk.Choices {
        delta := choice.Delta

        // 文本增量
        if delta.Content != "" {
            ch <- ai.Event{
                Type:  ai.EventTextDelta,
                Delta: delta.Content,
            }
        }

        // 工具调用
        for _, tc := range delta.ToolCalls {
            // 首次出现：发送 ToolCallStart
            // 后续出现：发送 ToolCallDelta
            // arguments 完成：发送 ToolCallEnd
        }
    }
}
```

**关键决策**：为什么不一次性解析完再发送，而是每个增量都发一个 Event？

→ 上层（TUI/Agent）需要 **实时** 展示打字效果。如果等所有 chunk 合并完了再发，用户会看到一片空白然后突然出现一堆文字——体验很差。

### 5. arguments 二次解码

```go
func decodeArg(raw string) (map[string]any, error) {
    var result map[string]any
    if err := json.Unmarshal([]byte(raw), &result); err != nil {
        return nil, err
    }
    return result, nil
}
```

OpenAI 的 `arguments` 字段是 JSON 字符串里面套 JSON 对象，需要先取出字符串，再 `json.Unmarshal` 一次。

---

## 测试策略

测试使用 **mock HTTP server**（`httptest.NewServer`），不依赖真实 API：

```go
func TestOpenAIProvider_TextStreaming(t *testing.T) {
    // 1. 创建 mock server，返回预设的 SSE 流
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"你好"}}]}`)
        fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"世界"}}]}`)
        fmt.Fprintln(w, "data: [DONE]")
    }))
    defer server.Close()

    // 2. 创建 Provider，指向 mock server
    prov := openai.New(openai.Config{
        APIKey:  "test-key",
        BaseURL: server.URL,
    })

    // 3. 消费 stream，验证事件
    stream := prov.Chat(context.Background(), context)
    var text string
    for event := range stream {
        if event.Type == ai.EventTextDelta {
            text += event.Delta
        }
    }
    assert.Equal(t, "你好世界", text)
}
```

4 个测试场景：文本流、工具调用流、错误响应、模型不存在。

---

## 思想总结

| 思想 | 体现 |
|------|------|
| **纯标准库** | `net/http` + `bufio.Scanner`，零外部依赖 |
| **goroutine 异步** | 解析在子 goroutine，主 goroutine 通过 channel 消费 |
| **流式增量** | 每个 SSE chunk 立即映射为 Event，不等待合并 |
| **可替换性** | BaseURL 可配置，兼容任何 OpenAI 兼容 API |
| **mock 测试** | `httptest.NewServer` 提供精确的 HTTP 模拟 |
