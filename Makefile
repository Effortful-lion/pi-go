# ============================================================================
# 编译、打包、发布
# ============================================================================
# 1. 本地发布构建（ldflags 注入最新 tag）
# 2. 手动交叉编译打包 darwin/arm64 + windows/amd64
# 3. 推送 tag 触发 CI 自动发布 GitHub Release

.PHONY: build pack release

build: ## 本地发布构建
	@go build -ldflags "$(LDFLAGS)" -o pg ./cmd/pg/ && ./pg version

pack: ## 手动打包（交叉编译 + tar.gz/zip + checksums）
	@./pack.sh

release: ## 推送 tag 触发 CI 发布
	@echo "pushing tags..."
	@git push origin main --tags

# ============================================================================
# 代码检查
# ============================================================================
# 1. golangci-lint 静态检查
# 2. 全量测试（含竞态检测）

.PHONY: lint test

lint: ## 运行 golangci-lint
	@golangci-lint run ./...

test: ## 运行全部测试（含竞态检测）
	@go test -race -count=1 ./...

# ============================================================================
# 发布 tag 版本管理
# ============================================================================
# 1. 查看当前最新 tag
# 2. 交互式输入新版本号并打 tag

.PHONY: version next-version

version: ## 显示当前最新 tag
	@echo "latest tag: $$(git tag --sort=-v:refname | head -1 || echo 'none')"

next-version: ## 交互式输入新版本号并打 tag
	@echo "latest tag: $$(git tag --sort=-v:refname | head -1 || echo 'none')"
	@echo ""
	@echo "bump 规则:"
	@echo "  fix      → patch (v1.0.0 → v1.0.1)"
	@echo "  feat     → minor (v1.0.0 → v1.1.0)"
	@echo "  breaking → major (v1.0.0 → v2.0.0)"
	@echo ""
	@read -p "输入新版本号 (如 v1.0.1): " ver; \
		git tag "$$ver" && \
		echo "tag $$ver 已创建，运行 'make release' 推送并发布" || \
		echo "tag 创建失败（可能已存在）"

# ============================================================================
# 本地编译运行
# ============================================================================

.PHONY: run

run: ## 本地编译并运行（dev 版本号）
	@go build -o pg ./cmd/pg/ && ./pg

# ============================================================================
# 清理
# ============================================================================

.PHONY: clean

clean: ## 清理构建产物
	@rm -rf pg pi dist/

# ============================================================================
# 帮助
# ============================================================================

.PHONY: help

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ============================================================================
# ldflags
# ============================================================================

LDFLAGS := -s -w \
	-X main.version=$(shell git describe --tags --abbrev=0 2>/dev/null || echo dev) \
	-X main.commit=$(shell git rev-parse --short HEAD) \
	-X main.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
