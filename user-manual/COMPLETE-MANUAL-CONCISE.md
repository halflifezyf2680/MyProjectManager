# MyProjectManager 完整用户手册（精炼版）

> **更新日期**: 2026-02-04  
> **版本**: Go MCP Server v2.0

---

## 📚 目录

- [快速开始](#快速开始) - MPM 是什么、为什么需要、安装使用
- [核心架构](#核心架构) - 设计哲学、系统架构、Manager 调度
- [核心能力](#核心能力) - 代码解析器、记忆系统
- [高级功能](#高级功能) - HUD、Persona、Skill、Timeline
- [实践指南](#实践指南) - 工作流、命名规范、黄金法则
- [工具速查](#工具速查) - 20 个工具快速参考
- [效能验证](#效能验证) - 实战案例与数据对比

---

## 快速开始

### MPM 是什么？

**MyProjectManager (MPM)** 是基于 MCP 协议的智能开发辅助系统，运行在所有支持 MCP 的编程环境（Windsurf、Cursor、Claude Code）中，提供：

- 🎯 **智能任务调度**：自动分析任务类型，选择合适的专家工具
- 🔍 **上下文提取与清洗**：AST 级别的符号搜索、依赖分析、影响评估
- 📚 **项目记忆**：持久化项目知识，跨会话复用经验
- 🎭 **人格系统**：可自定义的 AI 回复风格
- 🖥️ **HUD 状态面板**：实时显示系统状态

### 为什么需要 MPM？

**Vibe Coding 的困境**：AI 时代的新范式让编程效率提升 10 倍，但也带来了**黑箱问题**——LLM 内部推理不可见、批量生成不可控、每次对话从零开始、决策过程无法追溯。

**MPM 的使命**：让 Vibe Coding 从「闭眼信任」变成「睁眼协作」。

| 维度 | 用户痛点 | MPM 解决方案 |
|------|----------|--------------|
| **可预测** | "LLM 的编码风格每次都不一样" | `initialize_project` 自动分析项目风格，生成规则文件 |
| **可观测** | "不知道 LLM 在想什么" | `manager_analyze` 输出【情报包】，展示代码锚点 |
| **可追溯** | "找不到之前做了什么改动" | `memo` 记录每次操作，同步到 `dev-log.md` |
| **可控制** | "任务做到一半中断了" | `manager_create_hook` 创建断点，支持断点续传 |
| **上下文提取** | "LLM 靠猜测修改代码" | `code_search` 精确符号定位，`code_impact` 清洗影响上下文，大幅减少 Token 消耗 |

### 安装与使用

**系统要求**：
- **编译环境**：Go 1.21+（编译主程序）、Rust（编译 AST 索引器和 HUD）
- **运行环境**：Windows 10/11/Linux/macOS、支持 MCP 的编程工具
- **跨平台**：核心代码采用 Go、Rust 编写，不同平台自行编译即可使用

**安装步骤**：
1. 克隆仓库：`git clone <repository-url> MyProjectManager`
2. 编译：`cd mcp-server-go && go build -o bin/mpm-go.exe ./cmd/server`
3. 配置 IDE：在 MCP 配置中添加可执行文件路径
4. 重启 IDE

**验证安装**：在对话中输入 `mpm mg` 或 `初始化项目`，系统应返回初始化成功消息。

**第一个任务**：输入"帮我梳理一下项目的核心结构，我想知道哪里最复杂"，MPM 会调用 `project_map` 生成复杂度热力图，标记 Top 复杂符号。

**适用范围**：
- 🟢 **单人开发 + 长期项目**：最佳
- 🟢 **AI-First 工作流**：最佳
- 🟡 **小团队 (2-5人)**：适中
- 🔴 **实时多人协作**：待完善

---

## 核心架构

### 设计哲学

MPM 采用**分层架构**：AI IDE → Go MCP Server → 工具层 → 核心服务层 → 持久化层（SQLite + 文件系统）

**设计原则**：
1. **单一职责**：每个工具只负责特定领域
2. **延迟初始化**：数据库和服务按需创建
3. **项目隔离**：每个项目有独立的数据库
4. **两阶段执行**：先分析、后执行
5. **幂等设计**：重复调用同一工具不会产生副作用

### Manager 调度核心

> **"Manager 不是指挥官，而是战术情报官。"**

Manager 是 LLM 的**即时上下文与提示注入器**，通过**情报包 (Intel Package)** 将代码实体、架构约束和历史记忆聚合为单一的 JSON 对象。

**认知增强框架**：

传统模式：`用户 → 模型自由思考 → 输出结果`（黑盒推理，信息过载）

MPM 模式：`用户 → Manager 清洗信息 → 锚定上下文 → 模型推理 → 结构化输出`（质量控制 + 注意力引导）

**OODA 循环**：

- **步骤1：真实分析** - 符号定位、记忆加载、复杂度评估 → 返回分析结果 + task_id
- **步骤2：动态策略** - 基于真实分析，动态生成 strategic_handoff → 返回完整 Mission Briefing

**意图分类**：DEBUG、DEVELOP、REFACTOR、DESIGN、RESEARCH、PERFORMANCE、REFLECT

**情报包结构**：
```json
{
  "mission_control": { "intent": "...", "user_directive": "..." },
  "context_anchors": [{ "symbol": "...", "file": "...", "line": ..., "type": "..." }],
  "verified_facts": [...],
  "guardrails": { "critical": [...], "advisory": [...] },
  "strategic_handoff": "..."
}
```

**工程禁令**：根据意图动态生成（如 RESEARCH → READ_ONLY，DEBUG → VERIFY_FIRST）

---

## 核心能力

### 代码解析器

**为什么选择 AST 而非 LSP？**

- **LSP**：实时编辑辅助，主动监听（Push 模式）
- **MPM**：AI Context 提取，被动触发刷新（Pull 模式），基于 SHA256 哈希对比，只解析变更文件

**技术栈**：Tree-sitter（多语言 AST）+ Rust（零 GC）+ SQLite（增量索引）

**核心优势**：
1. 输出优化：一次性提取所有信息，格式化为 LLM 友好结构
2. 统一接口：单一工具处理多种语言，行为一致
3. 零依赖：静态链接的单一可执行文件
4. 特殊功能：项目地图、影响分析等 LSP 不提供的功能

**三大工具**：
- **project_map**：结构解析，生成项目层级地图 + 复杂度热点
- **code_search**：符号定位，AST 精确匹配 + 5层降级搜索
- **code_impact**：影响分析，调用链追踪 + 智能折叠

**技术实现**：
- 索引模式：SHA256 哈希对比，只重解析变更文件
- 查询模式：5层降级（精确匹配 → 前缀匹配 → 后缀匹配 → 编辑距离 → 词根匹配）
- 分析模式：DICE 复杂度算法（覆盖节点数 × 0.5 + 调用外出度 × 2.0 + 被调用入度 × 1.0）

### 记忆系统

**双层记忆系统**：

| 组件 | 技术载体 | 作用域 |
|------|----------|--------|
| **LTS** | SQLite (`.mcp-data/mcp_memory.db`) | Project |
| **Global** | SQLite (`.mcp-data/.../prompt_snippets.db`) | Global |
| **Memo Log** | `dev-log.md` | Project |
| **Skill Index** | 文件系统 (`skills/`) | Global |

**四种记忆单元**：

1. **Memos (工程日志) - SSOT**：项目的唯一真理来源
   - Category（操作类型）、Entity（操作对象）、Act（关键动作）、Content（深度上下文）
   - 同步机制：每次写入自动同步到 `dev-log.md`，保持倒序显示（最新的在上面），最多保留最近 100 条记录

2. **Known Facts (经验铁律)**：经过验证的规则和避坑指南
   - 避坑 (Pitfall)：记录曾经踩过的坑
   - 铁律 (Rule)：项目特定的硬性规定

3. **Pending Hooks (待办事项)**：跨越 Session 的待办事项系统
   - **注意**：最初设计为断点续传机制，实际使用中主要用作待办事项（断点续传由 `task_chain` 提供）
   - 支持优先级、过期时间、关联任务 ID
   - 状态循环：Open → Released

4. **Tasks (任务链)**：复杂任务的执行状态、计划和进度

**工具映射**：

| 记忆类型 | 写工具 | 读工具 |
|----------|--------|--------|
| **Memo** | `memo` | `system_recall` ⭐ |
| **Fact** | `known_facts` | `manager_analyze` (自动), `system_recall` |
| **待办事项 (Hook)** | `manager_create_hook` | `manager_list_hooks` |
| **Task** | `task_chain` | `task_chain` |

> **system_recall 的独特价值**：采用"宽进严出"策略，在 Entity/Act/Content 多字段中模糊匹配，通过 category/scope/limit 精细过滤，分类展示 + 时间戳 + 完整上下文。

---

## 高级功能

### HUD (Heads-Up Display)

**工具**: `open_hud()`

可视化监控终端，实时心跳监控、状态可视化、待办事项显示、当前人格显示。

### Persona System (人格矩阵)

**工具**: `persona(mode="activate", name="zhuge")`

切换 LLM 的思维协议和注意力偏置：

| 人格 | 代号 | 适用场景 |
|------|------|----------|
| **孔明** | `zhuge` | 架构设计、代码审查、复杂逻辑诊断 |
| **懂王** | `trump` | 头脑风暴、快速原型、打破僵局 |
| **哆啦** | `doraemon` | 学习新技术、新手引导、编写教程 |
| **柯南** | `detective_conan` | Bug 排查、日志分析、根因定位 |

### Skill System (技能系统)

**工具**: `skill_list()`, `skill_load(name="swe-bench")`

动态知识挂载机制，完全兼容 Claude Desktop 的 MCP Skill 规范。每个 Skill 包含：`SKILL.md`（核心操作指南）、`scripts/`（自动化脚本）、`templates/`（代码模板）。

### Timeline (演进视图)

**工具**: `open_timeline()`

将 Git Log、Memos 和关键决策点融合为可交互的 HTML 演进图谱，可视化决策链，识别频繁重构的"热点"。

### Prompt Manager

**工具**: `save_prompt_from_context(title="...", content="...", scope="global")`

**提示词库机制**：在 HUD 人格编辑器中集成了 Prompt 库功能，支持常用提示词模板（初始化、项目约定、标准开发、6步工作流等）一键应用。支持 Project Scope 和 Global Scope 双作用域。

---

## 实践指南

### Vibe Coding 核心思路

**Vibe Coding** 是 AI 时代的新编程范式：用自然语言描述需求，LLM 帮你写代码。这种模式让编程效率提升 10 倍，但也带来了黑箱问题。

**MPM 的完整生态**：整个 MPM 体系围绕 Vibe Coding 生态建立，通过工程化手段让 Vibe Coding 从「闭眼信任」变成「睁眼协作」：

**核心机制**：
1. **自动规则生成**：`initialize_project()` 自动分析项目代码风格，生成 `_MPM_PROJECT_RULES.md`，包含：
   - MPM 强制协议（死规则、工具使用时机、禁止事项）
   - 项目命名规范（自动检测函数/变量/类名风格）
   - 代码语言约束（基于项目现有代码提取）
   - 这些规则直接提高 LLM 调用效率，减少风格不一致和错误

2. **提示词库机制**：在 HUD 人格编辑器中集成了 Prompt 库功能，支持：
   - 常用提示词模板（初始化、项目约定、标准开发、6步工作流等）
   - Global/Project 双作用域
   - 一键应用，简化 LLM 调用流程

3. **跨平台编译**：核心代码采用 Go、Rust 编写，不同平台自行编译即可使用，无需依赖特定运行时环境。

**四大保障维度**：
- **可预测**：项目风格自动分析，生成规则文件
- **可观测**：情报包展示代码锚点，透明化推理过程
- **可追溯**：memo 记录每次操作，同步到 dev-log.md
- **可控制**：task_chain 支持断点续传，manager_create_hook 创建待办事项

**核心工作循环**：
```
规划（manager_analyze）→ 执行（代码修改）→ 记录（memo）→ 感知（code_search/impact/map）
```

### 核心工作流

**新对话启动**：
1. 阅读 `dev-log.md` 快速恢复上下文
2. `manager_analyze()` 或 `prompt_enhance()` 开始任务

**重启后初始化**：
1. `initialize_project()` 加载数据库和项目配置
2. 阅读 `dev-log.md`
3. `manager_analyze()` 或 `prompt_enhance()` 开始任务

**工具分级**：
- **轻量级**：`prompt_enhance`（小任务、快速问答）
- **重量级**：`manager_analyze`（大型任务、需要规划）
- **感知层**：`code_search`, `code_impact`, `project_map`
- **记录层**：`memo`

**标准循环**：
```
规划（manager_analyze）→ 执行（代码修改）→ 记录（memo）→ 感知（code_search/impact/map）
```

### 命名规范

**三大命名法则**：

1. **符号锚定**：拒绝通用词，拥抱全称
   - 反例: `data = get_data()`
   - 正例: `verified_payload = auth_service.fetch_verified_payload()`

2. **前缀即领域**：使用 `domain_action_target` 结构
   - 示例：`ui_btn_submit`、`api_req_login`、`db_conn_main`

3. **可检索性优先**：名字越长，冲突越少，越容易被搜索定位
   - 示例：`transaction_unique_trace_id` 全局唯一

### 黄金法则

1. **变更即记录**：任何代码/文档变更后，必须调用 `memo()`
2. **修改前必定位**：严禁盲改！修改代码前必须先 `code_search()`
3. **大改动必评估**：重构前必须 `code_impact()` 评估影响范围
4. **新对话读日志**：新开对话时，阅读 `dev-log.md` 快速获取上下文

---

## 工具速查

### 代码定位 (3个)

| 工具 | 触发词 | 核心参数 |
|------|--------|----------|
| `project_map` | `mpm 地图` | `scope`, `level` (`structure`/`symbols`) |
| `code_search` | `mpm 搜索` | `query` (必填), `scope`, `search_type` |
| `code_impact` | `mpm 影响` | `symbol_name` (必填), `direction` (`backward`/`forward`/`both`) |

### 任务管理 (5个)

| 工具 | 触发词 | 核心参数 |
|------|--------|----------|
| `manager_analyze` | `mpm 分析` | `task_description`, `intent`, `symbols`, `step` (1=分析, 2=策略) |
| `task_chain` | `mpm 任务链` | `mode` (`step`/`next`/`insert`/`finish`), `task_id`, `plan` |
| `manager_create_hook` | `mpm 挂起` | `description`, `priority` (`low`/`medium`/`high`) |
| `manager_list_hooks` | `mpm 待办列表` | `status` (`open`/`closed`) |
| `manager_release_hook` | `mpm 释放` | `hook_id`, `result_summary` |

### 文档与记忆 (4个)

| 工具 | 触发词 | 核心参数 |
|------|--------|----------|
| `wiki_writer` | `mpm wiki` | `output_file`, `style` (`technical`/`tutorial`/`reference`/`blog`) |
| `memo` | `mpm memo` | `items` (array: category, entity, act, path, content) |
| `system_recall` | `mpm 召回` | `keywords`, `category`, `scope`, `limit` |
| `known_facts` | `mpm 铁律` | `type` (`避坑`/`规则`/`铁律`), `summarize` |

### 人格与技能 (3个)

| 工具 | 触发词 | 核心参数 |
|------|--------|----------|
| `persona` | `mpm 人格` | `mode` (`list`/`activate`), `name` |
| `skill_list` | `mpm 技能列表` | 无参数 |
| `skill_load` | `mpm 加载技能` | `name`, `level` (`standard`/`full`) |

### 系统交互 (5个)

| 工具 | 触发词 | 核心参数 |
|------|--------|----------|
| `initialize_project` | `mpm 初始化` | `project_root` (可选，自动探测) |
| `open_hud` | `mpm hud` | 无参数 |
| `open_timeline` | `mpm 时间线` | 无参数 |
| `save_prompt_from_context` | `mpm 保存提示词` | `title`, `content`, `tag_names`, `scope` |
| `prompt_enhance` | `mpm 增强` | `task_description`, `mode` (`inject`/`explain`) |

**总计**: 20 个工具

---

## 效能验证

### Case Study 1: 符号定位与情报包

**任务**: 分析 `memo` 工具的实现逻辑

| 指标 | With MPM | No MPM | 效能提升 |
|------|----------|--------|----------|
| **步骤数** | **3 步** | **12+ 步** | **+300%** |
| **工具调用** | **2 次** | **10+ 次** | **+400%** |
| **首步命中率** | **100%** | **0%** | **∞** |

**核心洞察**：MPM 改变了 LLM 的认知模式，从 Search → Recall → Read → Reason 变为 **Briefing → Read → Reason**。

### Case Study 2: 认知重力系统

**任务**: "我要修改 `session.go` 的逻辑，这安全吗？"

| 维度 | No MPM | With MPM |
|------|--------|----------|
| **风险感知** | 低 (基于局部信息) | **高** (基于 AST 分析) |
| **Token 消耗** | 高 (通读文件) | **低** (Map 摘要) |
| **决策依据** | 经验判断 | 数据支持 |
| **行动建议** | 模糊反问 | 精确清单 |

**核心洞察**：从"文本猜测"到"结构推理"的范式转移。

### Case Study 3: 光速认知

**测试场景**: 新项目冷启动认知（300+ 文件，2000+ 符号）

| 指标 | With MPM | Without MPM | MPM 优势 |
|------|----------|-------------|----------|
| **总耗时** | **15 秒** | **40 秒** | **2.67x 加速** |
| **工具调用** | **1 次** | **4+ 次** | **4x+ 减少** |
| **Token 输入** | **显著减少** (结构化 JSON) | **大量** (原始文件内容) | **显著优化** |

**核心洞察**：Rust AST 引擎实现了高效的信息压缩，把 300+ 个文件的物理复杂度压缩成结构化的逻辑地图，大幅减少 Token 消耗。

### Case Study 4: 数据库备份机制

**事故**: 用户执行 `git reset --hard`，导致未提交代码被删除

| 维度 | Git | MPM DB |
|------|-----|--------|
| **记录触发** | 显式 Commit | **原子化 Memo (修改即记录)** |
| **覆盖范围** | 物理文本 | **意图 + 语义** |
| **抗灾能力** | 弱 (对未提交无效) | **强 (独立于源码)** |

**核心洞察**：Git 保护的是代码，而 MPM 保护的是开发过程和决策记录。

### Case Study 5: 模糊搜索的认知导航

**任务**: 查找代码中的符号搜索机制（只知道模糊概念）

| 搜索输入 | grep 结果 | code_search 结果 |
|----------|-----------|------------------|
| `符号搜索` | 0 行 | 找到 `CodeSearch` |
| `symbol` | 500+ 行噪音 | 找到相关函数簇 |

**核心洞察**：5 层降级搜索不是"备选方案"，而是"核心能力"。**模糊输入 → 精确输出**，这才是真正的智能。

### 总结

这五个案例验证了 MPM 的核心价值：
1. 符号定位与情报包 — **300% 效率提升**
2. 认知重力系统 — **风险感知能力显著提升**
3. 光速认知 — **2.67x 加速，Token 消耗显著减少**
4. 数据库备份机制 — **Git 之外的补充方案**
5. 认知导航 — **从精确匹配到语义理解**

**MPM 的本质**: 通过工程化手段构建可靠的基础设施，让 AI 从"对话助手"向"开发工具"转变。

---

*文档完*

