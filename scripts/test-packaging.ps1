# MPM 打包系统快速测试
# 用途: 验证打包流程是否正常工作

$ErrorActionPreference = "Stop"

function Test-Step {
    param($Name, $ScriptBlock)
    Write-Host "`n🧪 测试: $Name" -ForegroundColor Cyan
    try {
        & $ScriptBlock
        Write-Host "✅ 通过" -ForegroundColor Green
        return $true
    }
    catch {
        Write-Host "❌ 失败: $($_.Exception.Message)" -ForegroundColor Red
        return $false
    }
}

$rootDir = Split-Path -Parent $PSScriptRoot
$results = @()

# 测试 1: 检查 Go 环境
$results += Test-Step "Go 环境" {
    $goVer = go version
    if ($LASTEXITCODE -ne 0) { throw "Go not found" }
    Write-Host "   $goVer" -ForegroundColor Gray
}

# 测试 2: 检查项目结构
$results += Test-Step "项目结构" {
    $requiredPaths = @(
        "mcp-server-go\cmd\server",
        "scripts\build-release.ps1",
        "npm-package\package.json"
    )
    foreach ($p in $requiredPaths) {
        $fullPath = Join-Path $rootDir $p
        if (-not (Test-Path $fullPath)) {
            throw "Missing: $p"
        }
    }
    Write-Host "   所有必需文件存在" -ForegroundColor Gray
}

# 测试 3: 快速编译测试 (仅当前平台)
$results += Test-Step "快速编译" {
    Push-Location (Join-Path $rootDir "mcp-server-go")
    try {
        $output = Join-Path $rootDir "test-binary.exe"
        go build -o $output ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw "Build failed" }
        if (-not (Test-Path $output)) { throw "Binary not created" }
        
        $size = (Get-Item $output).Length / 1MB
        Write-Host "   二进制大小: $([math]::Round($size, 2)) MB" -ForegroundColor Gray
        
        Remove-Item $output -Force
    }
    finally {
        Pop-Location
    }
}

# 测试 4: NPM package.json 验证
$results += Test-Step "NPM 包配置" {
    $packageJson = Get-Content (Join-Path $rootDir "npm-package\package.json") | ConvertFrom-Json
    if (-not $packageJson.name) { throw "Missing package name" }
    if (-not $packageJson.version) { throw "Missing version" }
    if (-not $packageJson.bin) { throw "Missing bin config" }
    Write-Host "   包名: $($packageJson.name)" -ForegroundColor Gray
    Write-Host "   版本: $($packageJson.version)" -ForegroundColor Gray
}

# 测试 5: GitHub Actions 工作流验证
$results += Test-Step "GitHub Actions 配置" {
    $workflowFile = Join-Path $rootDir ".github\workflows\release.yml"
    if (-not (Test-Path $workflowFile)) { throw "Workflow file not found" }
    
    $content = Get-Content $workflowFile -Raw
    if ($content -notmatch "GOOS|GOARCH") { throw "Missing platform config" }
    Write-Host "   工作流配置完整" -ForegroundColor Gray
}

# 汇总报告
Write-Host "`n" + "="*50 -ForegroundColor Cyan
Write-Host "📊 测试报告" -ForegroundColor Cyan
Write-Host "="*50 -ForegroundColor Cyan

$passed = ($results | Where-Object { $_ -eq $true }).Count
$total = $results.Count

Write-Host "`n通过: $passed / $total" -ForegroundColor $(if ($passed -eq $total) { "Green" } else { "Yellow" })

if ($passed -eq $total) {
    Write-Host "`n✨ 所有测试通过! 打包系统已就绪。" -ForegroundColor Green
    Write-Host "`n💡 下一步:" -ForegroundColor Cyan
    Write-Host "   1. 运行完整构build: .\scripts\build-release.ps1 -Version `"1.0.0`"" -ForegroundColor Gray
    Write-Host "   2. 测试本地安装: cd npm-package && npm link" -ForegroundColor Gray
    Write-Host "   3. 创建 Git tag: git tag v1.0.0 && git push origin v1.0.0" -ForegroundColor Gray
}
else {
    Write-Host "`n⚠️ 部分测试失败,请检查上述错误信息。" -ForegroundColor Yellow
    exit 1
}
