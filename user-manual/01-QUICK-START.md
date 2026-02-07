# 第1章 快速开始

> 本章帮助你在5分钟内完成 MyProjectManager (MPM) 的安装与首次使用。

> **更新日期**: 2026-02-04
> **所属章节**: 第1章
> **版本**: Go MCP Server v2.0

---

## 1.1 MPM 是什么？

**MyProjectManager (MPM)** 是一个基于 MCP (Model Context Protocol) 协议的智能开发辅助系统。它专为**编程场景**设计，运行在 **所有支持 MCP 的编程环境**（如 Windsurf、Cursor、Claude Code）中，为你提供：

- 🎯 **智能任务调度**：自动分析任务类型，选择合适的专家工具
- 🔍 **上下文提取与清洗**：AST 级别的符号搜索、依赖分析、影响评估
- 📚 **项目记忆**：持久化项目知识，跨会话复用经验
- 🎭 **人格系统**：可自定义的 AI 回复风格
- 🖥️ **HUD 状态面板**：实时显示系统状态

---

## 1.2 系统要求

| 组件   | 要求                                                |
| ---- | ------------------------------------------------- |
| Go   | 1.21 或更高版本（编译用）                                   |
| 操作系统 | Windows 10/11（推荐）、Linux、macOS                     |
| 运行环境 | Windsurf / Cursor / Claude Code 等支持 MCP 的**编程工具** |
| 磁盘空间 | 至少 100MB                                          |

---

## 1.3 安装步骤

### 步骤 1：获取代码

```bash
# 克隆仓库到本地
git clone <repository-url> MyProjectManager
cd MyProjectManager
```

### 步骤 2：编译 Go Server

```bash
# 进入 MCP Server 目录
cd mcp-server-go

# 编译
go build -o bin/mpm-go.exe ./cmd/server
```

### 步骤 3：配置 IDE

在你的 AI IDE 中添加 MCP Server 配置：

**配置文件路径**（Windsurf/Claude Code）：

```json
{
  "mcpServers": {
    "MyProjectManager": {
      "command": "C:\\path\\to\\MyProjectManager\\mcp-server-go\\bin\\mpm-go.exe",
      "args": [],
      "env": {},
      "disabled": false
    }
  }
}
```

> **注意**：请将路径替换为你实际的安装路径。

### 步骤 4：重启 IDE

配置完成后，重启 IDE 使 MCP Server 生效。

---

## 1.4 验证安装

### 方法 1：查看工具列表

在 IDE 中，MCP 面板应该显示多个可用工具，包括：

- `initialize_project` - 项目初始化
- `manager_analyze` - 任务分析
- `code_search` - 精确符号定位
- `code_impact` - 影响分析
- `project_map` - 项目结构映射

### 方法 2：初始化项目

在对话中输入：

```
mpm mg
```

或者：

```
初始化项目
```

系统应返回初始化成功的消息。

> **代码基因感知**
> 初始化时，MPM 会自动扫描你的代码风格（命名规范），并为你动态生成专属的工程规则文件 `_MPM_PROJECT_RULES.md`。
> 
> * **旧项目**: 自动分析并沿用既有风格
> * **新项目**: 推荐采用 Vibe Coding 高上下文规范

---

## 1.5 第一个任务

让我们尝试一个更能体现 MPM **"上帝视角"** 的任务：生成项目的认知地图。

**输入**：

```
帮我梳理一下项目的核心结构，我想知道哪里最复杂
```

MPM 会：

1. **意图识别**：识别出这是"架构探索"任务
2. **工具调度**：调用 `project_map(detail='overview')`
3. **复杂度热力图**：不仅列出目录，还计算复杂度分布，标记"Top 复杂符号"
4. **智能折叠**：自动折叠低价值目录，防止刷屏

**示例输出**：

```text
### 🗺️ 项目地图 (Overview)

**📊 统计**: 40 文件 | 311 符号
**🔥 复杂度**: High: 10 | Med: 33 | Low: 239 | Avg: 10.5

**🎯 Top复杂符号**:
  1. `run_indexer` [HIGH:181.5]
  2. `run_analyze` [HIGH:111.5]
  3. `nextChapter` [HIGH:92.5]

**📁 目录结构** (按复杂度排序):
- **internal/services/** (3 files) [Avg:45.2] 
  - **ast_indexer/** (2 files)
- **internal/tools/** (11 files) [Avg:12.3]
- **internal/core/** (5 files) [Avg:8.7]
- ... (还有 10 个低复杂度目录)
```

**为什么用 MPM？**
IDE 的目录树只能看结构，而 MPM 能透视代码本质，告诉你 **"这 5 个函数占据了项目 80% 的复杂度"**，**"这个目录是核心深水区"**。它自带"降噪"光环，只让你关注最重要的 5%。

---

## 1.6 核心概念速览

### 触发词机制

MPM 使用"触发词"来激活不同功能：

| 触发词                 | 功能              |
| ------------------- | --------------- |
| `mpm mg` / `mpm 分析` | 启动 Manager，分析任务 |
| `初始化项目`             | 绑定项目路径，创建数据库    |
| `切换到哆啦A梦`           | 切换 AI 人格风格      |

### 任务分析流程

复杂任务建议使用 `manager_analyze` 进行分析：

1. **分析阶段**：识别任务意图，提取相关代码符号，生成约束建议
2. **执行阶段**：基于分析结果，按需调用具体工具完成任务

### 项目级数据库

每个项目在 `.mcp-data/` 目录下有独立的数据库：

```
project-root/
├── .mcp-data/
│   ├── mcp_memory.db      # SQLite 数据库
│   └── project_config.json # 项目配置
```

数据库存储：任务历史、已知事实、操作日志、待办钩子

---

## 1.7 常见问题

### Q: 提示"项目未初始化"？

**原因**：MPM 需要绑定到具体项目才能工作。

**解决**：

```javascript
// "初始化当前项目"
initialize_project(project_root='D:/path/to/your-project')
```

### Q: 工具调用失败？

**可能原因**：

1. 可执行文件路径错误
2. MCP 配置有误
3. 数据库文件被锁定

**解决**：检查 IDE 的 MCP 日志

### Q: 人格没有生效？

**原因**：人格切换需要在任务开始前进行。

**解决**：先说"切换到孔明"，再描述任务。

---

## 1.8 下一步

恭喜！你已经完成了 MPM 的基本安装和首次使用。

接下来建议阅读：

- [第2章 系统架构](02-ARCHITECTURE.md) - 理解各模块的关系
- [第3章 Manager 调度核心](03-MANAGER.md) - 深入了解任务分析流程
- [第4章 代码解析器](04-CODE-PARSER.md) - 了解 AST 解析能力

---

*本章完*
