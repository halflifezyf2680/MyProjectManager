#!/bin/bash
# MPM 一键编译脚本 (Linux/macOS)
# 用法: chmod +x build-unix.sh && ./build-unix.sh

set -e

echo "=== MPM 编译脚本 (Linux/macOS) ==="
echo ""

# 检测操作系统
OS="$(uname -s)"
case "${OS}" in
    Linux*)     MACHINE=Linux;;
    Darwin*)    MACHINE=Mac;;
    *)          MACHINE="UNKNOWN:${OS}"
esac

# 1. 检测 Go
echo "[1/4] 检测 Go 环境..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version)
    echo "  ✓ ${GO_VERSION}"
else
    echo "  ✗ 未检测到 Go，请先安装: https://go.dev/dl/"
    exit 1
fi

# 2. 检测 Rust
echo "[2/4] 检测 Rust 环境..."
if command -v rustc &> /dev/null; then
    RUST_VERSION=$(rustc --version)
    echo "  ✓ ${RUST_VERSION}"
    RUST_INSTALLED=true
else
    echo "  ✗ 未检测到 Rust，将跳过 Rust 组件编译"
    RUST_INSTALLED=false
fi

# 3. 检测 Node.js (Tauri 需要)
echo "[3/4] 检测 Node.js 环境..."
if command -v node &> /dev/null; then
    NODE_VERSION=$(node --version)
    echo "  ✓ Node.js: ${NODE_VERSION}"
    NODE_INSTALLED=true
else
    echo "  ✗ 未检测到 Node.js，将跳过 Tauri 组件编译"
    NODE_INSTALLED=false
fi

# 4. 开始编译
echo ""
echo "[4/4] 开始编译..."
echo ""

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BIN_DIR="${PROJECT_ROOT}/mcp-server-go/bin"

# 创建 bin 目录
mkdir -p "${BIN_DIR}"

# 确定可执行文件后缀
if [ "${MACHINE}" = "Windows_NT" ]; then
    EXE_EXT=".exe"
else
    EXE_EXT=""
fi

# 编译 mpm-go (Go)
echo "  → 编译 mpm-go${EXE_EXT}..."
cd "${PROJECT_ROOT}/mcp-server-go"
if go build -o "bin/mpm-go${EXE_EXT}" ./cmd/server; then
    SIZE=$(du -h "bin/mpm-go${EXE_EXT}" | cut -f1)
    echo "    ✓ mpm-go${EXE_EXT} (${SIZE})"
else
    echo "    ✗ 编译失败"
fi

# 编译 ast_indexer (Rust)
if [ "${RUST_INSTALLED}" = true ]; then
    echo "  → 编译 ast_indexer${EXE_EXT}..."
    cd "${PROJECT_ROOT}/mcp-server-go/ast_indexer_rust"
    if cargo build --release; then
        SRC="target/release/ast_indexer${EXE_EXT}"
        if [ -f "${SRC}" ]; then
            cp "${SRC}" "${BIN_DIR}/ast_indexer${EXE_EXT}"
            SIZE=$(du -h "${BIN_DIR}/ast_indexer${EXE_EXT}" | cut -f1)
            echo "    ✓ ast_indexer${EXE_EXT} (${SIZE})"
        fi
    else
        echo "    ✗ 编译失败"
    fi
fi

# 编译 Tauri 应用
if [ "${NODE_INSTALLED}" = true ] && [ "${RUST_INSTALLED}" = true ]; then
    echo "  → 编译 mcp-cockpit-hud..."
    HUD_DIR="${PROJECT_ROOT}/mcp-server-go/mcp-cockpit-hud"
    if [ -d "${HUD_DIR}" ]; then
        cd "${HUD_DIR}"
        if npm install; then
            if npm run tauri build; then
                # Tauri 输出路径因平台而异
                if [ "${MACHINE}" = "Mac" ]; then
                    SRC="src-tauri/target/release/mcp-cockpit-hud"
                else
                    SRC="src-tauri/target/release/mcp-cockpit-hud"
                fi
                if [ -f "${SRC}" ]; then
                    cp "${SRC}" "${BIN_DIR}/mcp-cockpit-hud${EXE_EXT}"
                    SIZE=$(du -h "${BIN_DIR}/mcp-cockpit-hud${EXE_EXT}" | cut -f1)
                    echo "    ✓ mcp-cockpit-hud${EXE_EXT} (${SIZE})"
                fi
            fi
        fi
    fi
fi

echo ""
echo "=== 编译完成 ==="
echo "输出目录: ${BIN_DIR}"
