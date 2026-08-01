#!/bin/bash
# 手动打包脚本 — 不依赖 goreleaser/CI，本地交叉编译 + 打包
# 用法: ./pack.sh [版本号]
#   ./pack.sh           # 从 git tag 取版本号
#   ./pack.sh v1.0.0    # 手动指定版本号

set -e

BINARY="pg"
VERSION="${1:-$(git describe --tags --abbrev=0 2>/dev/null || echo dev)}"
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
OUT="dist"

echo "打包 $BINARY $VERSION (commit: $COMMIT)"

# 交叉编译
LDFLAGS="-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.buildDate=$DATE"

rm -rf "$OUT"
mkdir -p "$OUT"

# darwin/arm64
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o "$OUT/$BINARY" ./cmd/pg/
tar czf "$OUT/${BINARY}_${VERSION}_darwin_arm64.tar.gz" -C "$OUT" "$BINARY"
echo "  → darwin_arm64"

# windows/amd64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o "$OUT/${BINARY}.exe" ./cmd/pg/
zip -j "$OUT/${BINARY}_${VERSION}_windows_amd64.zip" "$OUT/${BINARY}.exe"
echo "  → windows_amd64"

# 清理临时二进制
rm -f "$OUT/$BINARY" "$OUT/${BINARY}.exe"

# checksums
cd "$OUT"
shasum -a 256 *.tar.gz *.zip > checksums.txt
cd - >/dev/null

echo ""
echo "完成: $OUT/"
ls -lh "$OUT"/*.tar.gz "$OUT"/*.zip "$OUT"/checksums.txt
