#!/bin/bash
# uninstall.sh — 卸载 Pi Agent，清理二进制、PATH 配置和 ~/.pi-go 数据目录

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

echo ""
echo "========================================="
echo "   Pi Agent Uninstaller — 卸载脚本"
echo "========================================="
echo ""

REMOVED=0

# 1. 删除 pg 二进制文件
log_info "查找并删除 pg 二进制..."

GOPATH=$(go env GOPATH 2>/dev/null || echo "$HOME/go")
PG_BIN="${GOPATH}/bin/pg"

if [[ -f "$PG_BIN" ]]; then
    rm -f "$PG_BIN"
    log_info "已删除: $PG_BIN"
    REMOVED=1
else
    log_info "未找到 $PG_BIN（可能已删除或位于其他路径）"
fi

# 2. 从 shell 配置文件中删除 GOPATH/bin 的 pg 相关行
log_info "清理 shell 配置中的 PATH..."

for CONFIG_FILE in "$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.profile"; do
    if [[ -f "$CONFIG_FILE" ]]; then
        # 删除 Pi Agent 相关的 PATH 行
        if grep -q "# Pi Agent (pg)" "$CONFIG_FILE" 2>/dev/null; then
            # macOS 用 sed '' 兼容
            if [[ "$(uname)" == "Darwin" ]]; then
                sed -i '' '/# Pi Agent (pg)/d' "$CONFIG_FILE"
                sed -i '' "/export PATH.*GOPATH.*bin.*pg/d" "$CONFIG_FILE"
            else
                sed -i '/# Pi Agent (pg)/d' "$CONFIG_FILE"
                sed -i "/export PATH.*GOPATH.*bin.*pg/d" "$CONFIG_FILE"
            fi
            log_info "已清理 $CONFIG_FILE 中的 pg 相关配置"
            REMOVED=1
        fi
    fi
done

# 3. 删除 ~/.pi-go 数据目录
PI_GO_DIR="$HOME/.pi-go"
if [[ -d "$PI_GO_DIR" ]]; then
    rm -rf "$PI_GO_DIR"
    log_info "已删除: $PI_GO_DIR"
    REMOVED=1
else
    log_info "未找到 $PI_GO_DIR（已清理或从未创建）"
fi

echo ""
if [[ $REMOVED -eq 1 ]]; then
    echo "========================================="
    log_info "卸载完成！"
    echo ""
    log_warn "请重新打开终端，或执行: unset pg 2>/dev/null; hash -r"
    echo "========================================="
else
    log_info "未找到任何需清理的内容，可能已卸载。"
fi
