.PHONY: help build test lint clean install dev version next-version release

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## 本地编译（带版本注入）
	@go build -ldflags "$(LDFLAGS)" -o pg ./cmd/pg/

test: ## 运行全部测试（含竞态检测）
	@go test -race -count=1 ./...

lint: ## 运行 golangci-lint
	@golangci-lint run ./...

clean: ## 清理构建产物
	@rm -f pg pi dist/

dev: ## 本地开发构建（dev 版本号）
	@go build -o pg ./cmd/pg/
	@./pg version

install: ## 安装到 $GOPATH/bin
	@go install ./cmd/pg/

version: ## 显示当前最新 tag
	@echo "latest tag: $$(git tag --sort=-v:refname | head -1 || echo 'none')"

next-version: ## 交互式确定下一个版本号
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

LDFLAGS := -s -w \
	-X main.version=$(shell git describe --tags --abbrev=0 2>/dev/null || echo dev) \
	-X main.commit=$(shell git rev-parse --short HEAD) \
	-X main.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
