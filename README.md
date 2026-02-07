# <img src="docs/images/mpm_logo.png" height="60" style="vertical-align:middle;" /> MyProjectManager (MPM)

> **Agentic Coding 的外骨骼。让 AI 拥有资深架构师的灵魂。**

![License](https://img.shields.io/badge/license-MIT-blue.svg) ![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg) ![MCP](https://img.shields.io/badge/MCP-v1.0-FF4F5E.svg) ![Status](https://img.shields.io/badge/status-active-success.svg) ![Version](https://img.shields.io/badge/release-v1.0.0-brightgreen.svg)

---

## 📖 起因

**MPM 最初只是我在自己开发项目过程中随手搭建的一个 MCP 工具集**，名字也是随便起的（MyProjectManager）。它一开始只是个玩具，直到有一天怼一个10年前的C#项目bug，opus给我跑限额了。
后来它就越来越不像玩具了，就这么回事。
如果你想让gemini、glm什么LLM的达到claude的项目实际水平，那可以试试。
PS：文档都是Gemini写的我自己都没看完。

---

## 上下文工程

提示词工程发展到今天，已经形成了相当成熟的方法论。从日常对话到复杂 Agent 开发，精心设计的 Prompt 几乎是所有 AI 应用的基础——它确实有用，也确实全面。

但随着实际场景的深入，一个问题逐渐清晰：仅靠 Prompt，不足以支撑工程化落地。不是技巧不够细，而是单次交互的信息承载能力，本身就有边界。

Context Engineering（上下文工程）因此成为近年的关注焦点。它不是要替代 Prompt Engineering，而是站在更高维度重新组织信息流：

- **Prompt 关注"怎么问"，Context 关注"准备什么"**  
  与其反复调整提示词，不如直接提供结构化的上下文输入

- **Prompt 是单次交互，Context 是持续状态**  
  Prompt 每次都独立执行，Context 可以贯穿整个任务周期

- **Prompt 依赖人工设计，Context 可以工程化**  
  Prompt 需要反复打磨，Context 可以自动提取、更新和维护

**实际场景**：
- 代码分析：直接提供项目结构、调用关系、复杂度分布，而不是让模型自行推断
- Bug 修复：附带历史决策记录、已知问题、相关改动，避免每次重复背景说明
- 多轮对话：维护完整的上下文状态，而不是依赖模型从聊天记录中提取关键信息

---

### MPM 是什么

MPM 完全基于 Context Engineering 理念构建。它的核心目标是：**将代码库转化为 AI 可直接理解的结构化上下文**，使模型具备架构级视角——不仅知道代码写了什么，还能理解设计决策、历史演进和潜在风险点。

在 Prompt Engineering 领域，**Claude Skill** 是目前较为成熟的实践形态——通过标准化的 Skill 定义，让提示词能力可以复用和组合。

**MPM 完全兼容 Claude Skill 的调用机制**。你可以直接从 Claude 官方文档或社区获取现成 Skill，无需改动即可在 MPM 中使用。

工程化的本质：不是用各种各样的提示词，而是提供一套可持续积累、跨项目复用的系统架构。

---

## 📥 怎么用

### 下载

**[Windows 版下载](https://github.com/halflifezyf2680/MyProjectManager/releases/download/v1.0.0/MyProjectManager-v1.0.0.zip)** (31 MB)

> 目前仅提供 Windows 预编译版本，其他平台请从源码编译

解压就能用，不用折腾环境。

### 从源码编译

**Windows:**
```powershell
git clone https://github.com/halflifezyf2680/MyProjectManager.git
cd MyProjectManager
powershell -ExecutionPolicy Bypass -File scripts\build-windows.ps1
```

**Linux/macOS:**
```bash
git clone https://github.com/halflifezyf2680/MyProjectManager.git
cd MyProjectManager
chmod +x scripts/build-unix.sh
./scripts/build-unix.sh
```

**跨平台编译 (仅 Go 组件):**
```bash
./scripts/build-cross-platform.sh
```

### 支持的开发环境

- ✅ **Cursor** - 完美适配
- ✅ **Antigravity** - 完美适配
- ✅ **GitHub Copilot** - 完美适配
- ⭐ **Claude Code** - **推荐**，最稳定（虽非IDE但使用体验最佳）
- ✅ **Codex** - 完美适配
- ⚠️ **Windsurf** - 因次数计费限制，禁止大量自定义MCP，连顺序思考都不支持
- ⚠️ **其他** - 理论上支持 MCP 协议的都行

### 文档

想深入了解？→ [查看完整文档](user-manual/00-INDEX.md)

---

## 📋 版本历史

### [v1.0.0] - 2025-02-05

**首次正式发布**

#### 🐛 修复
- 修复 `manager_analyze` 工具 Step 2 状态丢失问题
- 更新 `package_product.py` 二进制校验路径
- 更新 `PersonaEditor.bat` 启动脚本

#### ✨ 功能
- 🗺️ Project Map - 项目结构可视化
- 🔍 Code Search - 精确符号定位
- 🛡️ Code Impact - 影响范围分析
- 🧠 Manager - 任务智能调度
- 🔗 Task Chain - 分步执行管理
- 📝 Memo - 开发记录持久化
- 🎭 Persona - AI 人格切换
- 📖 Wiki Writer - 自动文档生成
- 📊 Timeline - 项目演进可视化

#### 📦 包含
- mpm-go.exe (13.3 MB) - MCP 服务器
- mcp-cockpit-hud.exe (9.8 MB) - HUD 监控界面
- ast_indexer.exe (18.4 MB) - AST 索引器
- persona-editor.exe (9.6 MB) - 人格编辑器
- 完整文档和用户手册

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

---

<p align="center">
  <i>Built with ❤️ for the Agentic Future.</i>
</p>
