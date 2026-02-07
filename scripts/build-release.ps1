# MyProjectManager 跨平台打包脚本
# 用途: 编译多平台二进制文件并打包为发布版本

param(
    [string]$Version = "1.0.0",
    [switch]$SkipTests = $false
)

$ErrorActionPreference = "Stop"

# 颜色输出函数
function Write-Step { param($msg) Write-Host "🚀 $msg" -ForegroundColor Cyan }
function Write-Success { param($msg) Write-Host "✅ $msg" -ForegroundColor Green }
function Write-Error { param($msg) Write-Host "❌ $msg" -ForegroundColor Red }
function Write-Info { param($msg) Write-Host "ℹ️  $msg" -ForegroundColor Yellow }

$rootDir = Split-Path -Parent $PSScriptRoot
$goDir = Join-Path $rootDir "mcp-server-go"
$distDir = Join-Path $rootDir "dist"
$releaseName = "mpm-v$Version"

Write-Step "开始构建 MyProjectManager v$Version"
Write-Info "项目根目录: $rootDir"

# 1. 清理旧的构建产物
Write-Step "[1/6] 清理旧构建产物..."
if (Test-Path $distDir) {
    Remove-Item $distDir -Recurse -Force
}
New-Item -ItemType Directory -Path $distDir -Force | Out-Null
Write-Success "构建目录已准备就绪: $distDir"

# 2. 运行测试 (可选)
if (-not $SkipTests) {
    Write-Step "[2/6] 运行测试..."
    Push-Location $goDir
    try {
        go test ./... -v
        if ($LASTEXITCODE -ne 0) {
            Write-Error "测试失败，终止构建"
            exit 1
        }
        Write-Success "所有测试通过"
    }
    finally {
        Pop-Location
    }
}
else {
    Write-Info "[2/6] 跳过测试 (SkipTests)"
}

# 3. 定义编译目标平台
$platforms = @(
    @{ OS = "windows"; Arch = "amd64"; Ext = ".exe" }
    @{ OS = "windows"; Arch = "arm64"; Ext = ".exe" }
    @{ OS = "darwin"; Arch = "amd64"; Ext = "" }      # macOS Intel
    @{ OS = "darwin"; Arch = "arm64"; Ext = "" }      # macOS Apple Silicon
    @{ OS = "linux"; Arch = "amd64"; Ext = "" }
    @{ OS = "linux"; Arch = "arm64"; Ext = "" }
)

Write-Step "[3/6] 编译多平台二进制文件..."
Push-Location $goDir

foreach ($platform in $platforms) {
    $env:GOOS = $platform.OS
    $env:GOARCH = $platform.Arch
    $outputName = "mpm-server-$($platform.OS)-$($platform.Arch)$($platform.Ext)"
    $outputPath = Join-Path $distDir $outputName
    
    Write-Info "  编译 $($platform.OS)/$($platform.Arch)..."
    
    go build -o $outputPath -ldflags "-s -w -X main.Version=$Version" ./cmd/server
    
    if ($LASTEXITCODE -ne 0) {
        Write-Error "编译失败: $($platform.OS)/$($platform.Arch)"
        Pop-Location
        exit 1
    }
    
    Write-Success "  ✓ $outputName ($('{0:N2}' -f ((Get-Item $outputPath).Length / 1MB)) MB)"
}

Pop-Location
Write-Success "所有平台编译完成"

# 4. 创建发布包
Write-Step "[4/6] 打包发布文件..."

$releaseDir = Join-Path $distDir $releaseName
New-Item -ItemType Directory -Path $releaseDir -Force | Out-Null

# 复制文档和配置
$filesToCopy = @(
    @{ Src = "README.md"; Target = "" }
    @{ Src = "install.ps1"; Target = "" }
    @{ Src = "user-manual"; Target = "docs" }
    @{ Src = "configs"; Target = "" }
    @{ Src = "mcp-server-go\skills"; Target = "skills" }
)

foreach ($file in $filesToCopy) {
    $srcPath = Join-Path $rootDir $file.Src
    if ($file.Target) {
        $targetDir = Join-Path $releaseDir $file.Target
        New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
        $targetPath = Join-Path $targetDir (Split-Path $file.Src -Leaf)
    }
    else {
        $targetPath = Join-Path $releaseDir (Split-Path $file.Src -Leaf)
    }
    
    if (Test-Path $srcPath) {
        if (Test-Path $srcPath -PathType Container) {
            Copy-Item $srcPath $targetPath -Recurse -Force
        }
        else {
            Copy-Item $srcPath $targetPath -Force
        }
        Write-Info "  ✓ 已复制: $($file.Src)"
    }
}

# 复制二进制到 bin 目录
$binDir = Join-Path $releaseDir "bin"
New-Item -ItemType Directory -Path $binDir -Force | Out-Null
Copy-Item (Join-Path $distDir "mpm-server-*") $binDir
Write-Success "发布包结构已创建"

# 5. 生成校验和
Write-Step "[5/6] 生成文件校验和..."
$checksumFile = Join-Path $distDir "checksums.txt"
Get-ChildItem (Join-Path $distDir "mpm-server-*") | ForEach-Object {
    $hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash
    "$hash  $($_.Name)" | Out-File -Append -Encoding UTF8 $checksumFile
}
Write-Success "校验和已生成: checksums.txt"

# 6. 创建压缩包
Write-Step "[6/6] 创建压缩包..."

# Windows ZIP
if (Get-Command Compress-Archive -ErrorAction SilentlyContinue) {
    $zipPath = Join-Path $distDir "$releaseName-windows.zip"
    
    # 只打包 Windows 二进制 + 文档
    $tempWinDir = Join-Path $distDir "temp-win"
    New-Item -ItemType Directory -Path $tempWinDir -Force | Out-Null
    Copy-Item (Join-Path $releaseDir "*") $tempWinDir -Recurse
    Remove-Item (Join-Path $tempWinDir "bin" "mpm-server-darwin-*") -Force -ErrorAction SilentlyContinue
    Remove-Item (Join-Path $tempWinDir "bin" "mpm-server-linux-*") -Force -ErrorAction SilentlyContinue
    
    Compress-Archive -Path "$tempWinDir\*" -DestinationPath $zipPath -Force
    Remove-Item $tempWinDir -Recurse -Force
    Write-Success "  ✓ Windows 包: $releaseName-windows.zip"
}

# 使用 tar 创建跨平台包 (如果有 tar 命令)
if (Get-Command tar -ErrorAction SilentlyContinue) {
    Push-Location $distDir
    tar -czf "$releaseName-full.tar.gz" $releaseName
    Pop-Location
    Write-Success "  ✓ 完整包: $releaseName-full.tar.gz"
}

# 7. 最终报告
Write-Step "==========================================`n"
Write-Success "✨ 构建完成！版本: v$Version`n"
Write-Host "📦 发布产物:"
Get-ChildItem $distDir -File | ForEach-Object {
    Write-Host "   - $($_.Name) ($('{0:N2}' -f ($_.Length / 1MB)) MB)" -ForegroundColor Gray
}
Write-Host "`n📂 发布目录: $distDir"
Write-Host "`n💡 下一步:"
Write-Host "   1. 测试二进制: .\dist\bin\mpm-server-windows-amd64.exe --version"
Write-Host "   2. 创建 GitHub Release 并上传压缩包"
Write-Host "   3. 发布到包管理器 (npm/homebrew)"
Write-Step "=========================================="
