.PHONY: help dev build test lint clean install pack version next-version release

# ============================================================================
# 开发
# ============================================================================

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

dev: ## 本地编译 + 显示版本
	@go build -o pg ./cmd/pg/ && ./pg version

test: ## 运行全部测试（含竞态检测）
	@go test -race -count=1 ./...

lint: ## 运行 golangci-lint
	@golangci-lint run ./...

clean: ## 清理构建产物
	@rm -rf pg pi dist/

install: ## 安装到 $GOPATH/bin
	@go install ./cmd/pg/

# ============================================================================
# 构建 & 打包
# ============================================================================

build: ## 本地发布构建（ldflags 注入最新 tag）
	@go build -ldflags "$(LDFLAGS)" -o pg ./cmd/pg/ && ./pg version

pack: ## 手动打包（交叉编译 + tar.gz/zip）
	@./pack.sh

# ============================================================================
# 版本 & 发布
# ============================================================================

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

release: ## 推送 tag 触发 CI 发布
	@echo "pushing tags..."
	@git push origin main --tags

# ============================================================================
# ldflags
# ============================================================================

LDFLAGS := -s -w \
	-X main.version=$(shell git describe --tags --abbrev=0 2>/dev/null || echo dev) \
	-X main.commit=$(shell git rev-parse --short HEAD) \
	-X main.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
