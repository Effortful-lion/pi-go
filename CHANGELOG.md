# Changelog

## 2026-08-10

### 架构重构：简化日志库设计，统一输出与格式职责

#### 核心设计变更

- **删除 Router 模块**：`lg.Module()` 不再动态跟随 `defaultLogger`，Module 仅负责标记模块名，输出路由由用户显式控制
- **删除 SetPath 全家桶**：移除 `SetPath`、`SetDefault`、`SetDefaultWriter`、`LogOption` 等魔法函数，配置输出唯一路径：`lg.New(writer)`
- **删除 Router 文件**：`lg/router.go` 已清空并删除

#### 新增功能

- **Formatter 接口**：`ConsoleWriter` / `FileWriter` 支持自定义输出格式
- **JSONFormatter**：内置 JSON 格式化器，通过 `writer.SetFormatter(lg.JSONFormatter)` 使用
- **SetRotateSize**：`FileWriter` 支持按大小轮转，静态路径自动生成 `.001.log` 序号文件
- **DisableCaller**：`Logger.DisableCaller()` 关闭 `runtime.Caller` 采集，降低热路径开销
- **SetFatalHook**：`Logger.SetFatalHook()` 支持 Fatal 时拦截退出，便于测试

#### 修复问题

- **Fields 输出顺序**：`Fields.format()` 按键字典序排序，日志输出稳定
- **JSONFormatter 错误处理**：JSON 序列化失败时返回可识别的降级字符串，不再静默吞错
- **测试全局状态污染**：`TestPackageLevelFunctions` 保存/恢复 `defaultLogger`，避免影响其他测试
- **FileWriter 静态路径轮转**：新增 `path` 字段，`SetRotateSize` 在静态路径下正确创建带序号的新文件

#### 删除的 API

| 删除项 | 替代方案 |
|--------|---------|
| `Router` / `NewRouter` / `Route` / `Unroute` | 独立创建多个 `Logger` |
| `SetPath` / `SetDefault` / `SetDefaultWriter` | `lg.New(writer)` 显式配置 |
| `WithLevelDir` / `WithRotateByInterval` / `WithRotateBySize` / `WithRetention` | 自行组合 `FileWriter` |
| `JSONWriter` / `NewJSONWriter` / `NewJSONConsoleWriter` | `ConsoleWriter` + `SetFormatter(JSONFormatter)` |
| `writerSrc` 字段 | 直接拷贝 `writer` |
| `TestModule_FollowsSetDefault` | 移除 writerSrc 后不再适用 |
| `TestSetPath_*` / `TestRotateBy*` / `TestRetention_*` | 依赖已删除的 API |

#### 文件变更

```
修改：  lg/entry.go      - Fields 排序、FormatJSON、导入调整
修改：  lg/logger.go     - 删除 writerSrc/SetDefault/SetPath、新增 DisableCaller/SetFatalHook
修改：  lg/writer.go     - 新增 Formatter/JSONFormatter、FileWriter.SetFormatter/SetRotateSize
修改：  lg/logger_test.go - 新增 6 个测试、修复全局状态污染
删除：  lg/router.go     - 空文件已删除
```

#### 测试覆盖

- 总计 31 个测试，全部通过
- 新增：`TestFieldsFormat_Sorted`、`TestJSONFormatter`、`TestFatalHook_Intercept`、`TestDisableCaller`、`TestSetFatalHook_Chain`、`TestFileWriter_SetRotateSize`、`TestFileWriter_SetRotateSize_Disabled`

#### 使用方式示例

```go
// 文本 → 文件（默认）
fw, _ := lg.NewFileWriter("logs/app.log", lg.LevelInfo)
lg.New(fw).Module("user").Info("登录")

// JSON → 文件
fw, _ := lg.NewFileWriter("logs/app.json", lg.LevelInfo)
fw.SetFormatter(lg.JSONFormatter)
lg.New(fw).Module("user").Info("登录", lg.Fields{"uid": 123})

// 多模块独立输出
userLog := lg.New(lg.NewFileWriter("logs/user.log", lg.LevelDebug)).Module("user")
shopLog := lg.New(lg.NewFileWriter("logs/shop.log", lg.LevelWarn)).Module("shop")
```
