#!/bin/bash
# quickstart.sh — 从源码构建 Pi Agent 并配置 PATH
# 适用于已经 clone 了 pi-go 仓库的用户

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

echo ""
echo "========================================="
echo "   Pi Agent Quickstart — 源码快速启动"
echo "========================================="
echo ""

# 1. 检查 Go 版本
log_info "检查 Go 环境..."
if ! command -v go &>/dev/null; then
    log_error "未找到 go 命令，请先安装 Go 1.21+: https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
log_info "检测到 $GO_VERSION"

# 2. 确认当前在 pi-go 根目录
if [[ ! -f "go.mod" ]] || ! grep -q "pi-go" go.mod 2>/dev/null; then
    log_error "请在 pi-go 仓库根目录下运行此脚本"
    exit 1
fi

# 3. 下载依赖
log_info "下载依赖..."
go mod download

# 4. 编译
log_info "编译 pg 二进制..."
go build -o pg ./cmd/pg/

# 5. 安装到 $GOPATH/bin
log_info "安装到 \$GOPATH/bin..."
go install ./cmd/pg/

# 6. 检查 PATH 中是否包含 $GOPATH/bin
GOPATH=$(go env GOPATH)
GOPATH_BIN="${GOPATH}/bin"

if ! echo "$PATH" | tr ':' '\n' | grep -Fxq "$GOPATH_BIN"; then
    log_warn "\$GOPATH/bin 不在 PATH 中，正在配置..."

    CONFIG_FILE=""
    case "$SHELL" in
        */zsh)  CONFIG_FILE="$HOME/.zshrc" ;;
        */bash) CONFIG_FILE="$HOME/.bashrc" ;;
        *)      CONFIG_FILE="$HOME/.profile" ;;
    esac

    if ! grep -q "GOPATH/bin" "$CONFIG_FILE" 2>/dev/null; then
        echo "" >> "$CONFIG_FILE"
        echo "# Pi Agent (pg) — added by quickstart.sh" >> "$CONFIG_FILE"
        echo "export PATH=\"\$PATH:$GOPATH_BIN\"" >> "$CONFIG_FILE"
        log_info "已写入 $CONFIG_FILE"
    else
        log_info "$CONFIG_FILE 中已存在 GOPATH/bin 配置"
    fi

    PATH_ADDED=true
else
    log_info "\$GOPATH/bin 已在 PATH 中"
    PATH_ADDED=false
fi

# 7. 验证（用完整路径，避免 PATH 未生效的问题）
log_info "验证安装..."
PG_BIN="${GOPATH}/bin/pg"
if [[ -x "$PG_BIN" ]]; then
    "$PG_BIN" version
else
    log_error "pg 二进制未找到: $PG_BIN"
    exit 1
fi

echo ""
echo "========================================="
log_info "安装完成！"
echo ""

if [[ "${PATH_ADDED:-false}" == "true" ]]; then
    echo -e "  ${YELLOW}!!! 请执行以下命令使 PATH 配置在当前会话生效:${NC}"
    echo ""
    echo "      source $CONFIG_FILE"
    echo ""
fi

echo "  启动命令: pg"
echo "  配置命令: pg config init"
echo "  帮助命令: pg help"
echo ""
echo "  如需切换到 Anthropic: pg -provider anthropic"
echo "  如需切换到 Google:    pg -provider google"
echo "========================================="
