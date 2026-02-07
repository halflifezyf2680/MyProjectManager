# 📦 MPM 打包和分发指南

## 🎯 概述

MyProjectManager 提供了多种打包和分发方式,类似于 npm/npx 的便捷体验:

1. **NPM/NPX** - 最推荐的安装方式
2. **GitHub Releases** - 直接下载预编译二进制
3. **Go Install** - Go 开发者专用
4. **手动编译** - 完全自定义

---

## 🚀 方式一: NPM/NPX (推荐)

### 用户安装方式

```bash
# 全局安装
npm install -g @myprojectmanager/mpm-cli

# 或使用 npx (无需安装)
npx @myprojectmanager/mpm-cli --help
```

### 发布到 NPM 的步骤

1. **构建所有平台的二进制**
```powershell
.\scripts\build-release.ps1 -Version "1.0.0"
```

2. **上传到 GitHub Releases**
```bash
# 使用 GitHub Actions 自动化 (推荐)
git tag v1.0.0
git push origin v1.0.0

# 或手动上传到 GitHub Releases
```

3. **准备 NPM 包**
```bash
cd npm-package
npm install
npm version 1.0.0 --no-git-tag-version
```

4. **发布到 NPM**
```bash
# 首次发布需要登录
npm login

# 发布
npm publish --access public
```

---

## 📦 方式二: GitHub Releases

### 自动化发布 (推荐)

使用 GitHub Actions 自动化构建和发布:

```bash
# 创建并推送 tag
git tag v1.0.0
git push origin v1.0.0

# GitHub Actions 会自动:
# 1. 编译所有平台二进制
# 2. 生成校验和
# 3. 创建 GitHub Release
# 4. 发布到 NPM
```

### 手动发布

```powershell
# 1. 构建发布版本
.\scripts\build-release.ps1 -Version "1.0.0"

# 2. 查看构建产物
ls .\dist

# 3. 手动创建 GitHub Release 并上传文件
```

---

## 🛠️ 方式三: Go Install

### 配置 Go Module

在 `mcp-server-go/cmd/server/main.go` 中添加版本信息:

```go
package main

var Version = "dev"

func main() {
    fmt.Printf("MPM Server v%s\n", Version)
    // ...
}
```

### 用户安装方式

```bash
go install github.com/halflifezyf2680/MyProjectManager/mcp-server-go/cmd/server@latest
```

---

## 📋 本地构建脚本说明

### `scripts\build-release.ps1`

完整的发布构建脚本,功能包括:
- ✅ 清理旧构建产物
- ✅ 运行测试 (可选)
- ✅ 编译 6 个平台的二进制
- ✅ 创建发布包结构
- ✅ 生成 SHA256 校验和
- ✅ 创建压缩包

**使用方法:**

```powershell
# 基本用法
.\scripts\build-release.ps1 -Version "1.0.0"

# 跳过测试
.\scripts\build-release.ps1 -Version "1.0.0" -SkipTests

# 查看帮助
Get-Help .\scripts\build-release.ps1 -Detailed
```

**输出结构:**

```
dist/
├── mpm-v1.0.0/                 # 发布包
│   ├── bin/                    # 所有平台二进制
│   ├── docs/                   # 文档
│   ├── configs/                # 配置
│   ├── README.md
│   └── install.ps1
├── mpm-v1.0.0-windows.zip      # Windows 专用包
├── mpm-v1.0.0-full.tar.gz      # 完整包
└── checksums.txt               # SHA256 校验和
```

---

## 🔄 完整发布流程

### 1. 准备发布

```bash
# 更新版本号
# 编辑 mcp-server-go/cmd/server/main.go
# 编辑 npm-package/package.json

# 更新 CHANGELOG
# 编辑 CHANGELOG.md
```

### 2. 本地测试

```powershell
# 构建所有平台
.\scripts\build-release.ps1 -Version "1.0.0"

# 测试 Windows 版本
.\dist\bin\mpm-server-windows-amd64.exe --version

# 测试 NPM 包 (本地)
cd npm-package
npm pack
npm install -g .\myprojectmanager-mpm-cli-1.0.0.tgz
mpm-server --version
```

### 3. 创建 Git Tag

```bash
git add .
git commit -m "Release v1.0.0"
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin main
git push origin v1.0.0
```

### 4. 等待 CI/CD

GitHub Actions 会自动:
1. ✅ 编译所有平台
2. ✅ 创建 GitHub Release
3. ✅ 发布到 NPM (需要配置 `NPM_TOKEN`)

### 5. 验证发布

```bash
# 检查 GitHub Releases
# https://github.com/halflifezyf2680/MyProjectManager/releases

# 检查 NPM
npm view @myprojectmanager/mpm-cli

# 测试安装
npx @myprojectmanager/mpm-cli@latest --version
```

---

## 🔐 配置 NPM Token

在 GitHub 仓库设置中添加 Secret:

1. 登录 NPM: https://www.npmjs.com
2. 创建 Access Token (Automation)
3. 在 GitHub 仓库: Settings → Secrets → Actions
4. 添加 `NPM_TOKEN`

---

## 📝 版本号管理

遵循语义化版本 (Semantic Versioning):

- **主版本 (Major)**: 不兼容的 API 变更 (1.0.0 → 2.0.0)
- **次版本 (Minor)**: 向后兼容的功能新增 (1.0.0 → 1.1.0)
- **修订版本 (Patch)**: 向后兼容的问题修正 (1.0.0 → 1.0.1)

---

## 🐛 常见问题

### Q: GitHub Actions 构建失败?

A: 检查:
1. Go 版本是否正确 (1.25+)
2. 依赖是否都能正常下载
3. 运行 `go mod tidy` 清理依赖

### Q: NPM 发布权限错误?

A: 确保:
1. `NPM_TOKEN` Secret 已正确配置
2. Token 有发布权限 (Automation token)
3. 包名 `@myprojectmanager/mpm-cli` 可用

### Q: 二进制文件太大?

A: 已使用 `-ldflags "-s -w"` 去除调试信息,如需进一步压缩:
```bash
# 使用 UPX 压缩 (可选)
upx --best mpm-server.exe
```

---

## ✨ 下一步

- [ ] 配置 Homebrew tap (macOS/Linux)
- [ ] 配置 Chocolatey (Windows)
- [ ] Docker 镜像发布
- [ ] 自动化 changelog 生成

---

*最后更新: 2026-02-05*
