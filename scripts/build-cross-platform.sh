#!/bin/bash
# MPM 跨平台编译脚本 (仅 Go 组件)
# 用法: chmod +x build-cross-platform.sh && ./build-cross-platform.sh
#
# 说明: Rust 和 Tauri 组件需要目标平台工具链，请用户在对应平台自行编译
# 此脚本仅编译 mpm-go 到各平台

set -e

echo "=== MPM 跨平台编译脚本 (仅 Go 组件) ==="
echo ""

# 检测 Go
echo "检测 Go 环境..."
if ! command -v go &> /dev/null; then
    echo "✗ 未检测到 Go，请先安装: https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version)
echo "✓ ${GO_VERSION}"
echo ""

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RELEASE_DIR="${PROJECT_ROOT}/release_cross_platform"
mkdir -p "${RELEASE_DIR}"

cd "${PROJECT_ROOT}/mcp-server-go"

echo "开始编译..."
echo ""

# 定义平台
PLATFORMS=(
    "windows/amd64"
    "linux/amd64"
    "darwin/amd64"
    "darwin/arm64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS="${PLATFORM%/*}"
    GOARCH="${PLATFORM#*/}"

    # 确定输出文件名和后缀
    if [ "${GOOS}" = "windows" ]; then
        OUTPUT="mpm-go.exe"
    else
        OUTPUT="mpm-go"
    fi

    echo "  → 编译 ${GOOS}/${GOARCH}..."

    # 编译
    GOOS=${GOOS} GOARCH=${GOARCH} go build -o "${RELEASE_DIR}/${GOOS}-${GOARCH}-${OUTPUT}" ./cmd/server

    if [ $? -eq 0 ]; then
        SIZE=$(du -h "${RELEASE_DIR}/${GOOS}-${GOARCH}-${OUTPUT}" | cut -f1)
        echo "    ✓ ${OUTPUT} (${SIZE})"
    else
        echo "    ✗ 编译失败"
    fi
done

echo ""
echo "=== 编译完成 ==="
echo "输出目录: ${RELEASE_DIR}"
echo ""
echo "说明："
echo "  - mpm-go 已编译到各平台"
echo "  - Rust/Tauri 组件 (ast_indexer, mcp-cockpit-hud, persona-editor)"
echo "    需要目标平台工具链，请在对应平台运行 build-windows.ps1 或 build-unix.sh"
