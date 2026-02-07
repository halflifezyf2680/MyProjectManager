# 📦 MPM 发布包内容清单

## 📋 完整打包内容

### 1. 核心二进制文件 (`bin/`)

包含所有平台的预编译二进制：

```
bin/
├── mpm-server-windows-amd64.exe    # Windows (Intel/AMD 64位)
├── mpm-server-windows-arm64.exe    # Windows (ARM64)
├── mpm-server-darwin-amd64         # macOS (Intel)
├── mpm-server-darwin-arm64         # macOS (Apple Silicon)
├── mpm-server-linux-amd64          # Linux (x86_64)
└── mpm-server-linux-arm64          # Linux (ARM64)
```

### 2. Skills 技能包 (`skills/`)

**✅ 已包含** - 所有预置的技能包都会打包到发布版本中：

```
skills/
├── README.md                       # Skills 使用指南
├── algorithmic-art/                # 算法艺术生成
├── architecture/                   # 架构设计
├── brand-guidelines/               # 品牌指南
├── canvas-design/                  # Canvas 设计
├── doc-coauthoring/                # 文档协作
├── docx/                           # Word 文档处理
├── frontend-design/                # 前端设计
├── go-game-dev/                    # Go 游戏开发
├── internal-comms/                 # 内部通讯
├── mcp-builder/                    # MCP 构建器
├── pdf/                            # PDF 处理
├── performance-analysis/           # 性能分析
├── pptx/                           # PowerPoint 处理
├── skill-creator/                  # 技能创建器
├── slack-gif-creator/              # Slack GIF 创建
├── swe-bench/                      # SWE-Bench 工程基准
├── theme-factory/                  # 主题工厂
├── web-artifacts-builder/          # Web 工件构建器
├── webapp-testing/                 # Web应用测试
└── xlsx/                           # Excel 处理
```

**Skills 说明:**
- 📁 每个 skill 都是独立的专家指导包
- 📝 包含完整的 SKILL.md + 资源文件
- 🔄 用户可以通过 `skill_list` 和 `skill_load` 工具使用
- 💾 Skills 存储在 `<release>/skills/` 目录下

### 3. 文档 (`docs/`)

用户手册和开发文档：

```
docs/
└── user-manual/                    # 用户手册
    ├── 00-INDEX.md                 # 目录索引
    ├── 01-QUICK-START.md           # 快速开始
    ├── 02-INSTALLATION.md          # 安装指南
    ├── 03-MANAGER.md               # Manager 工具
    ├── 04-FINDER.md                # Finder 工具
    ├── 05-MEMO.md                  # Memo 系统
    ├── 06-SKILLS.md                # Skills 使用
    ├── 07-WORKFLOW.md              # 工作流程
    ├── 08-TOOLS.md                 # 工具参考
    ├── 09-FAQ.md                   # 常见问题
    └── CASE_STUDY.md               # 案例研究
```

### 4. 配置文件 (`configs/`)

示例配置和模板：

```
configs/
└── (配置模板文件)
```

### 5. 根目录文件

```
.
├── README.md                       # 项目介绍
├── install.ps1                     # Windows 一键安装脚本
└── LICENSE                         # 许可证 (如果有)
```

---

## 📊 打包方式对比

### 方式 1: `build-release.ps1` (PowerShell)

**包含内容:**
- ✅ 所有平台二进制
- ✅ Skills 目录 (单独复制)
- ✅ 用户手册
- ✅ 配置文件
- ✅ README 和安装脚本

**输出:**
```
dist/
├── mpm-v1.0.0/                     # 完整发布包
│   ├── bin/                        # 二进制文件
│   ├── skills/                     # ✅ Skills 包
│   ├── docs/                       # 文档
│   ├── configs/                    # 配置
│   └── README.md
├── mpm-v1.0.0-windows.zip          # Windows 专用包
├── mpm-v1.0.0-full.tar.gz          # 完整包
└── checksums.txt                   # 校验和
```

### 方式 2: `package_product.py` (Python)

**包含内容:**
- ✅ 完整的 mcp-server-go 目录 (自动包含 skills/)
- ✅ 用户手册
- ✅ 文档
- ❌ 过滤掉编译产物 (`target/`, `__pycache__/`, 等)

**输出:**
```
release_vYYYYMMDD/
└── MyProjectManager/
    ├── mcp-server-go/              # ✅ 包含 skills/
    ├── docs/
    ├── user-manual/
    └── README.md
```

---

## 🔍 验证 Skills 是否打包

### 本地验证

```powershell
# 1. 运行打包脚本
.\scripts\build-release.ps1 -Version "1.0.0"

# 2. 检查 skills 目录
Get-ChildItem .\dist\mpm-v1.0.0\skills

# 3. 验证内容
Get-ChildItem .\dist\mpm-v1.0.0\skills -Recurse | Measure-Object
```

### 安装后验证

```powershell
# 用户安装后可以查看
mpm-server skill_list

# 或手动检查安装目录
ls $INSTALL_PATH/skills/
```

---

## 📦 发布包大小估算

| 组件 | 预估大小 | 说明 |
|------|---------|------|
| 二进制 (单个) | ~15-20 MB | Go 编译后的可执行文件 |
| 二进制 (全部6个) | ~90-120 MB | 所有平台二进制 |
| Skills 目录 | ~5-10 MB | 20个技能包的文档和脚本 |
| 用户手册 | ~1-2 MB | Markdown 文档 |
| **总计 (完整包)** | **~100-135 MB** | 未压缩大小 |
| **总计 (压缩包)** | **~30-40 MB** | .tar.gz 压缩后 |

---

## ⚙️ 自定义打包

### 只打包特定 Skills

如果想减小包体积，可以修改 `build-release.ps1`:

```powershell
# 复制所有 skills
@{ Src = "mcp-server-go\skills"; Target = "skills" }

# 或只复制部分 skills
# 需要手动创建过滤逻辑
```

### 排除大文件

在 `package_product.py` 的 `ignore_patterns` 中添加：

```python
shutil.copytree(src_dir, target_dir, ignore=shutil.ignore_patterns(
    "__pycache__", ".mcp-data", ".git", "*.pyc", 
    "target", "*.log",
    # 排除特定 skills
    "skills/large-skill-name"
))
```

---

## 🎯 最佳实践

1. **完整打包** - 推荐包含所有 skills，方便用户开箱即用
2. **按需下载** - 未来可考虑实现 skills 的在线下载机制
3. **版本管理** - Skills 也应该有版本号，便于更新
4. **文档完整** - 确保每个 skill 都有 SKILL.md 说明文件

---

*最后更新: 2026-02-05*
