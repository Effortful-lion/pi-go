# AGENTS.md This file provides guidance to CodeBuddy when working with code in this repository.

## 项目概述

本项目是从 TypeScript 版 [Pi](https://github.com/earendil-works/pi-mono)（位于 `../pi`）重新实现的 **Go 语言版 Pi Agent 开发库 + 产品**。

目标产物分两层：
1. **agent 开发库**：可供其他 Go 项目引用的标准库，提供 LLM 多提供商统一接口、Agent 运行时、工具调用、状态管理等能力
2. **pi-agent 产品**：基于该库构建的交互式编程 Agent CLI 产品

**核心原则**：不是单纯的代码翻译，而是用 Go 最佳实践重新设计实现，保留 Pi 的架构理念。

## 参考代码源

| 源 | 路径 | 用途 |
|---|------|------|
| Pi TypeScript 源码 | `../pi/packages/` | 架构参考：ai(多提供商LLM)、agent-core(Agent运行时)、tui(终端UI) |
| llmLib Go 实现 | `../agent-study/llmLib/` | Go 语言 Agent 开发参考实现 |
| lg 日志库 | `../agent-study/llmLib/lg/` | 日志库实现参考（Writer/Router/ConsoleWriter/FileWriter/MultiWriter） |

## 开发准则

### 1. Go 语言风格
- 代码必须是地道的 Go 风格，不要照搬 TypeScript 的 OOP/函数式混合模式
- 优先使用 Go 的 interface、struct、error 返回等惯用模式
- 不使用 `any`，除非与外部接口交互且无法避免
- 不使用 Go generate、enum 模拟等非必要技巧

### 2. 标准库设计
- 项目输出一套 **agent 开发库**（可被 import 引用）+ **pi-agent 产品**（CLI 可执行文件）
- 库的 package 设计要模块化、可组合，每个 package 提供清晰的公开 API
- 产品层在库之上构建，不污染库的 API

### 3. Git 记录关键节点
- 每个知识点、完整功能点完成后提交，commit message 使用 `{feat,fix,docs}({package}): <简短描述>` 格式
- 不要使用 `git add -A` / `git add .`，精确 stage 修改的文件
- 不得在未要求的情况下提交

### 4. 日志规范
- 如果 `../agent-study/llmLib/lg/` 有成熟的日志库，可以直接复制（vendor）使用
- 如果没有，参考其实现（Writer 接口 + Router 路由 + ConsoleWriter/FileWriter/MultiWriter）
- 日志输出格式整洁：`[LEVEL] [module] message file:line key=value`
- 提供包级别便捷函数和模块化 Logger

### 5. 函数抽象尺度
- 如果函数很小（<10行）且只有 1-2 处调用：直接内联，不要提取为函数
- 如果逻辑在多处重复、或抽象后有明确的语义：提取
- 不要为了"看起来整洁"而过度抽象

### 6. 常量常量化
- 所有魔法数字、固定字符串必须定义为命名常量
- 根据作用域选择 const 位置：仅当前文件使用则放文件顶部，跨文件使用则放公共常量文件

### 7. 遵循 Pi 工程最佳实践
- 保持与 Pi TypeScript 版本一致的架构分层（ai → agent → 产品）
- Provider 模式、事件驱动、流式处理等核心设计保留
- 不盲目引入不属于 Pi 设计理念的模式

### 8. 测试策略
- 一个完整功能点开发完毕后写简单测试即可，不要过度测试
- 不追求覆盖率指标，测试以验证核心路径为主
- 使用 Go 标准 testing 包，不引入第三方测试框架

### 9. 工程组织
- 目录嵌套不超过 3 层
- 功能按知识分类模块化：`ai/`（LLM多提供商）、`agent/`（Agent运行时）、`tool/`（工具系统）、`tui/`（终端UI）、`cmd/`（产品入口）
- 每个一级目录对应一个核心知识域

### 10. 不做兼容
- 纯 Go 实现，不兼容 TypeScript 的任何 API 设计
- 不为了与 Pi TypeScript 版保持一致而牺牲 Go 风格
- 用 Go 的最佳实践实现相同的功能目标

### 11. Change Log
- 每次代码修改必须更新 `CHANGELOG.md`
- 格式：
  ```
  ## YYYY-MM-DD
  
  ### 标题（简短描述本次修改主题）
  - 修改内容1
  - 修改内容2
  ```
- 同一天的多次修改合并到同一个日期下

### 12. 设计文档归档
- 关键设计决策必须留档，放在 `./docs/` 目录下
- 包括但不限于：架构设计、接口设计、模块职责说明
- 文档必须是 Markdown 格式（.md）

### 13. 从零开始开发
- 从最底层开始构建（通用基础库 → ai → agent → 产品）
- 第一步：项目初始化（go.mod、目录结构、基础工具库如 log/errors）
- 第二步：AI 层（多提供商统一接口）
- 第三步：Agent 运行时层
- 第四步：工具系统
- 第五步：产品 CLI
- 第五步：终端 UI

### 14. 学习总结文档
- 每个知识大类开发结束并测试通过后，输出总结文档
- 放在 `./docs/learning/` 目录下
- 面向零基础/后端转型人员，内容包含：概念解释、设计思路、关键代码示例
- 文件名自选，但需要能体现知识大类名称

### 15. 产品使用文档
- 最终产品完成后，输出 `产品使用文档.md`
- 包含：安装、配置、基本使用、高级功能、常见问题
- 易用化、面向终端用户

### 16. 文档格式
- 所有输出文档必须是 Markdown 格式（.md）
- 使用标准 Markdown 语法，代码块需标注语言

### 17. 开发流程
- **每次编码前，先写计划，用户确认后再编写**
- 计划内容包括：要做什么、改动哪些文件、API 设计、预期结果
- 不要未经确认直接开始写代码

## 架构约定

### 目录结构（计划）
```
pi-go/
├── cmd/                    # 产品 CLI 入口
│   └── pi/
│       └── main.go
├── ai/                     # 多提供商 LLM 统一接口
│   ├── provider.go          # Provider 接口定义
│   ├── model.go             # Model 接口定义
│   ├── message.go           # 消息/上下文类型
│   └── providers/           # 具体提供商实现（openai, anthropic,...）
├── agent/                  # Agent 运行时
│   ├── agent.go             # Agent 核心
│   ├── state.go             # 状态管理
│   └── event.go             # 事件系统
├── tool/                   # 工具系统
│   ├── tool.go              # Tool 接口
│   └── builtin/             # 内置工具
├── tui/                    # 终端 UI
├── lg/                     # 日志库
├── docs/                   # 设计文档
│   └── learning/           # 学习总结
├── CHANGELOG.md
├── go.mod
└── go.sum
```

### 包依赖方向
```
cmd/pi → agent → ai → lg
cmd/pi → tui
agent → tool
```

## 命令

```bash
# 构建
go build ./...

# 运行所有测试
go test ./...

# 运行单个包的测试
go test ./ai/...

# 运行单个测试函数
go test -run TestFunctionName ./package/...

# 代码格式化
go fmt ./...

# 静态检查
go vet ./...

# 运行产品
go run ./cmd/pi/
```

## 参考：Pi TypeScript 版核心架构

Pi TypeScript 版有 4 个核心包：

| 包 | 职责 |
|----|------|
| `pi-ai` | 统一多提供商 LLM API，支持 30+ 提供商（OpenAI、Anthropic、Google 等）。核心抽象：`Provider` → `Models` → `Model`。流式事件模型。 |
| `pi-agent-core` | Agent 运行时：工具调用循环、状态管理、事件订阅。核心类 `Agent`，包含状态机和事件流。 |
| `pi-coding-agent` | 交互式编程 Agent CLI，支持多种交互模式（交互/打印/JSON/RPC），扩展系统，会话管理。 |
| `pi-tui` | 终端 UI 框架，差异渲染 + 组件系统。 |

Go 版保留此架构的分层思想，但每个层用 Go 惯用方式重新实现。
