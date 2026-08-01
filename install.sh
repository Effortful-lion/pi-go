#!/bin/bash
set -e

# Pi-Go 一键安装脚本
# curl -fsSL https://raw.githubusercontent.com/Effortful-lion/pi-go/main/install.sh | bash

REPO="Effortful-lion/pi-go"
BINARY="pg"
INSTALL_DIR="${GOPATH:-$HOME/go}/bin"
VERSION="${PI_GO_VERSION:-latest}"

# 检测 OS/Arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *)       echo "不支持的架构: $ARCH"; exit 1 ;;
esac

echo "检测到: $OS/$ARCH"

# 构建下载 URL
if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/$REPO/releases/latest/download/${BINARY}_${OS}_${ARCH}.tar.gz"
else
    URL="https://github.com/$REPO/releases/download/$VERSION/${BINARY}_${OS}_${ARCH}.tar.gz"
fi

echo "下载: $URL"

# 创建临时目录
TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

# 下载并解压
if command -v curl &>/dev/null; then
    curl -fsSL "$URL" -o "$TMP/pg.tar.gz"
elif command -v wget &>/dev/null; then
    wget -q "$URL" -O "$TMP/pg.tar.gz"
else
    echo "需要 curl 或 wget"; exit 1
fi

tar xzf "$TMP/pg.tar.gz" -C "$TMP"

# 安装
mkdir -p "$INSTALL_DIR"
cp "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

echo "安装完成: $INSTALL_DIR/$BINARY"
echo "版本: $($INSTALL_DIR/$BINARY version 2>/dev/null || echo 'dev')"

# PATH 提示
if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
    echo ""
    echo "请将以下路径加入 PATH:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
fi
