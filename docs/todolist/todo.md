# 待完成事项

## 近期
- [ ] TUI 代码块状态机：`MarkdownLine` 改为有状态版本，正确处理跨行代码块（```）
- [ ] my-openai provider 工具调用解析补全（当前标记 TODO）
- [ ] Agent 错误处理和中断重连机制
- [ ] 输入历史持久化到文件（重启后保留历史记录）

## 远期
- [ ] 多 Agent 协作
- [ ] RPC/HTTP Server 模式
- [ ] MCP 协议支持
- [ ] 插件系统

## 已完成（2026-08-01）
- [x] 日志分级存储（info/error 分目录，根目录无汇总文件）
- [x] 日志库架构简化（writerSrc + SetDefaultWriter）
- [x] 去掉用户输入重复回显
- [x] Markdown 渲染集成（### 前缀修复 + 逐行转换）
- [x] 光标位置修复（CursorBack 替代 CursorForward）
- [x] 去掉输出背景色
- [x] 产品名称修正（Pi Agent → Pi-Go Agent）
