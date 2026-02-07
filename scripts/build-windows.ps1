# MPM 一键编译脚本 (Windows)
# 用法: powershell -ExecutionPolicy Bypass -File build-windows.ps1

$ErrorActionPreference = "Stop"

Write-Host "=== MPM 编译脚本 (Windows) ===" -ForegroundColor Cyan
Write-Host ""

# 1. 检测 Go
Write-Host "[1/4] 检测 Go 环境..." -ForegroundColor Yellow
try {
    $goVersion = go version 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  ✓ Go: $goVersion" -ForegroundColor Green
    } else {
        throw "Go 未安装"
    }
} catch {
    Write-Host "  ✗ 未检测到 Go，请先安装: https://go.dev/dl/" -ForegroundColor Red
    exit 1
}

# 2. 检测 Rust
Write-Host "[2/4] 检测 Rust 环境..." -ForegroundColor Yellow
try {
    $rustVersion = rustc --version 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  ✓ Rust: $rustVersion" -ForegroundColor Green
    } else {
        throw "Rust 未安装"
    }
} catch {
    Write-Host "  ✗ 未检测到 Rust，将跳过 Rust 组件编译" -ForegroundColor Red
    $rustInstalled = $false
} `
if ($rustInstalled -eq $null) { $rustInstalled = $true }

# 3. 检测 Node.js (Tauri 需要)
Write-Host "[3/4] 检测 Node.js 环境..." -ForegroundColor Yellow
try {
    $nodeVersion = node --version 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  ✓ Node.js: $nodeVersion" -ForegroundColor Green
    } else {
        throw "Node.js 未安装"
    }
} catch {
    Write-Host "  ✗ 未检测到 Node.js，将跳过 Tauri 组件编译" -ForegroundColor Red
    $nodeInstalled = $false
} `
if ($nodeInstalled -eq $null) { $nodeInstalled = $true }

# 4. 开始编译
Write-Host ""
Write-Host "[4/4] 开始编译..." -ForegroundColor Yellow
Write-Host ""

$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptRoot
$binDir = Join-Path $projectRoot "mcp-server-go\bin"

# 创建 bin 目录
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

# 编译 mpm-go.exe (Go)
Write-Host "  → 编译 mpm-go.exe..." -ForegroundColor Cyan
Push-Location (Join-Path $projectRoot "mcp-server-go")
go build -o bin\mpm-go.exe .\cmd\server
if ($LASTEXITCODE -eq 0) {
    $size = [math]::Round((Get-Item bin\mpm-go.exe).Length / 1MB, 1)
    Write-Host "    ✓ mpm-go.exe ($size MB)" -ForegroundColor Green
} else {
    Write-Host "    ✗ 编译失败" -ForegroundColor Red
}
Pop-Location

# 编译 ast_indexer.exe (Rust)
if ($rustInstalled) {
    Write-Host "  → 编译 ast_indexer.exe..." -ForegroundColor Cyan
    Push-Location (Join-Path $projectRoot "mcp-server-go\ast_indexer_rust")
    cargo build --release
    if ($LASTEXITCODE -eq 0) {
        $src = "target\release\ast_indexer.exe"
        if (Test-Path $src) {
            Copy-Item $src (Join-Path $binDir "ast_indexer.exe") -Force
            $size = [math]::Round((Get-Item (Join-Path $binDir "ast_indexer.exe")).Length / 1MB, 1)
            Write-Host "    ✓ ast_indexer.exe ($size MB)" -ForegroundColor Green
        }
    } else {
        Write-Host "    ✗ 编译失败" -ForegroundColor Red
    }
    Pop-Location
}

# 编译 Tauri 应用
if ($nodeInstalled -and $rustInstalled) {
    Write-Host "  → 编译 mcp-cockpit-hud.exe..." -ForegroundColor Cyan
    $hudDir = Join-Path $projectRoot "mcp-server-go\mcp-cockpit-hud"
    if (Test-Path $hudDir) {
        Push-Location $hudDir
        npm install
        if ($LASTEXITCODE -eq 0) {
            npm run tauri build
            if ($LASTEXITCODE -eq 0) {
                $src = "src-tauri\target\release\mcp-cockpit-hud.exe"
                if (Test-Path $src) {
                    Copy-Item $src (Join-Path $binDir "mcp-cockpit-hud.exe") -Force
                    $size = [math]::Round((Get-Item (Join-Path $binDir "mcp-cockpit-hud.exe")).Length / 1MB, 1)
                    Write-Host "    ✓ mcp-cockpit-hud.exe ($size MB)" -ForegroundColor Green
                }
            }
        }
        Pop-Location
    }
}

Write-Host ""
Write-Host "=== 编译完成 ===" -ForegroundColor Cyan
Write-Host "输出目录: $binDir" -ForegroundColor Gray
