# <img src="../docs/images/mpm_logo.png" height="50" style="vertical-align:middle;" /> MyProjectManager 用户手册

> **更新日期**: 2026-02-04  
> **文档类型**: 用户手册  
> **版本**: Go MCP Server v2.0

### 适用范围

| 场景 | 适配度 | 说明 |
|------|--------|------|
| 🟢 **单人开发 + 长期项目** | 最佳 | Memory 完整发挥，跨会话知识沉淀 |
| 🟢 **AI-First 工作流** | 最佳 | 结构化约束显著提升 LLM 可控性 |
| 🟡 **小团队 (2-5人)** | 适中 | `.mcp-data/` 目前项目内隔离，团队成员各自维护 |
| 🔴 **实时多人协作** | 待完善 | Memory 同步机制开发中 |

---

## 📚 文档导航

本手册采用单目录多文档结构，每个核心模块独立成篇，方便快速查阅。

### 入门篇

| 章节 | 文档 | 说明 |
|------|------|------|
| 第1章 | [快速开始](01-QUICK-START.md) | 5分钟上手指南，从安装到第一个任务 |
| 第2章 | [系统架构](02-ARCHITECTURE.md) | 整体架构图解，理解各模块关系 |

### 核心功能篇

| 章节 | 文档 | 说明 |
|------|------|------|
| 第3章 | [Manager 调度核心](03-MANAGER.md) | 智能任务调度中心的工作原理 |
| 第4章 | [代码解析器](04-CODE-PARSER.md) | AST 静态解析：Project Map / Code Search / Code Impact |
| 第5章 | [工具完整参考](08-TOOLS.md) | 所有 20 个 MPM 工具的参数、用法、示例 |

### 数据与状态篇

| 章节 | 文档 | 说明 |
|------|------|------|
| 第6章 | [数据库与记忆](05-DATABASE-MEMORY.md) | 项目级数据持久化与知识管理 |

### 高级功能篇

| 章节 | 文档 | 说明 |
|------|------|------|
| 第7章 | [额外功能](06-ADVANCED.md) | dev-log、Hook、Timeline、Persona、Skill、HUD ⭐ |
| 第8章 | [Vibe Coding 最佳实践](07-VIBE-CODING.md) | **[推荐]** AI 时代的命名规范与工程方法论 |

### 实战案例篇

| 章节 | 文档 | 说明 |
|------|------|------|
| 案例 | [MPM 效能实战](CASE_STUDY.md) | **[必读]** 双盲实验：MPM 工具与原生模式的效能对比 |

---

## 🎯 快速入口

### 我想...

- **开始使用 MPM** → [快速开始](01-QUICK-START.md)
- **理解系统工作原理** → [系统架构](02-ARCHITECTURE.md)
- **分析一个任务** → [Manager 调度核心](03-MANAGER.md)
- **定位代码位置** → [代码解析器](04-CODE-PARSER.md)
- **查看工具参数** → [工具完整参考](08-TOOLS.md)
- **保存项目知识** → [数据库与记忆](05-DATABASE-MEMORY.md)
- **使用高级功能** → [额外功能](06-ADVANCED.md)

---

## 📋 项目信息

- **项目名称**: MyProjectManager (MPM)
- **技术栈**: Go 1.21+ / MCP Protocol / SQLite / Rust (AST Indexer) / Tauri (HUD)
- **支持的 IDE**: Windsurf, Cursor, Claude Code
- **核心模块数量**: 11+
- **MCP 工具数量**: 20

---

## 🔗 相关资源

- [MCP 协议规范](https://modelcontextprotocol.io/)
- [开发日志](../dev-log.md)

---

*本手册由 MPM 系统辅助生成。*
