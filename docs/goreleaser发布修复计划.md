# goreleaser 发布修复计划

## 背景

当前仓库在发布阶段出现 `goreleaser release` 报错，需要重点排查：

- `goreleaser` 配置语法是否与 CI 中固定版本兼容
- GitHub Actions 中 action 版本与语法是否有效
- 发布环境中的 Go 代理/依赖下载是否会影响 release

## 已确认事实

- 本地已安装 `goreleaser 2.17.1`
- CI 工作流固定 `goreleaser/goreleaser-action@v7`，并指定 `version: '~> v2.8'`
- 使用本地 `goreleaser 2.17.1` 执行 `goreleaser check` 通过
- 使用本地 `goreleaser 2.17.1` 执行 `goreleaser release --snapshot --clean` 通过
- 使用 `go run github.com/goreleaser/goreleaser/v2@v2.8.0 check` 通过
- 使用 `go run github.com/goreleaser/goreleaser/v2@v2.8.0 release --snapshot --clean` 通过
- 当前机器默认 `GOPROXY=https://goproxy.cn,direct`，在拉取 `goreleaser v2.8.0` 依赖时出现过 `504 Gateway Timeout`，改用 `https://proxy.golang.org,direct` 后恢复

## 当前判断

现有 `.goreleaser.yaml` 本身不是一个可稳定复现的语法错误源，至少对 `2.8.0` 和 `2.17.1` 都成立。

更可能的风险点有两类：

1. CI 运行环境问题
   - GitHub Actions runner 对 action 主版本或 Node runtime 的兼容性
   - 依赖拉取、缓存或代理行为差异
2. 发布链路稳定性问题
   - `goreleaser` 固定到较老次版本，后续维护成本较高
   - 文档、配置和工作流描述存在不一致，容易误导后续排障

## 拟修改内容

1. 收敛 release workflow 版本声明
   - 评估是否将 `version: '~> v2.8'` 调整为 `version: '~> v2'`
   - 保留 `goreleaser-action@v7`
   - 视需要补充更明确的 Go 依赖缓存/版本读取配置
2. 对齐文档与实际配置
   - 修正文档中仍然描述旧版 `setup-go 1.23`、`release.header_file` 等已过时内容
3. 补充变更日志
   - 记录本次 `goreleaser` / release workflow 修复内容

## 预期结果

- GitHub Release 工作流配置更稳，不再依赖过窄的 `goreleaser` 次版本范围
- `goreleaser` 配置、workflow、设计文档三者一致
- 后续再排查 release 失败时，能更快区分“配置错误”和“环境问题”
