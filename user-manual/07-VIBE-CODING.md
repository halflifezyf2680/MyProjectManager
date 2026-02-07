# 第7章 Vibe Coding 最佳实践

> **"MPM 不是工具集合，而是一套工程方法论。"** 本章是 MPM 的完整使用指南，帮助你在 AI 辅助开发中保持心流。

> **更新日期**: 2026-02-03
> **所属章节**: 第7章
> **版本**: Go MCP Server v2.0

---

## 7.1 核心工作流

### 新对话启动

**每次新开对话时**，阅读 `dev-log.md` 快速恢复上下文，然后开始任务：

| 步骤 | 操作 | 说明 |
|------|------|------|
| 1 | 阅读 `dev-log.md` | 快速获取短期上下文（最近 100 条操作） |
| 2 | `manager_analyze()` 或 `prompt_enhance()` | 开始任务 |

### 重启后初始化

**只有在重启了 MCP/IDE/终端后**，才需要先执行初始化：

| 步骤 | 工具 | 触发词 | 说明 |
|------|------|--------|------|
| 1 | `initialize_project()` | 初始化、绑定项目 | 加载数据库和项目配置 |
| 2 | 阅读 `dev-log.md` | - | 快速获取短期上下文 |
| 3 | `manager_analyze()` | 分析任务、规划、mg | 大型任务规划 |
| 4 | `prompt_enhance()` | 增强、pe、小任务 | 小任务快速启动 |

### 工具分级

| 级别 | 工具 | 使用场景 |
|------|------|----------|
| **轻量级** | `prompt_enhance` | 小任务、快速问答 |
| **重量级** | `manager_analyze` | 大型任务、需要规划 |
| **感知层** | `code_search`, `code_impact`, `project_map` | 代码搜索、定位、影响分析 |
| **专家层** | `wiki_writer` | 生成 Wiki 大纲与写作指南（基于项目地图），由 LLM/用户按需继续深入阅读与编写 |
| **记录层** | `memo` | 操作完成后归档 |

### 标准循环

```
┌─────────────────────────────────────────────────────────────┐
│                    MPM 标准工作循环                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌──────────┐     ┌──────────┐     ┌──────────┐           │
│   │  规划    │ ──▶ │  执行    │ ──▶ │  记录    │           │
│   │ manager  │     │ 代码修改 │     │  memo    │           │
│   └──────────┘     └──────────┘     └──────────┘           │
│        │                                   │                │
│        │           ┌──────────┐            │                │
│        └────────── │  感知    │ ◀──────────┘                │
│                    │ code_... │                             │
│                    └──────────┘                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 模型选择建议

MPM 通过定向信息清洗与注入，对不同类型的模型和用户群体效果差异显著。

> 从左到右：推荐 → 不推荐

```
Claude Sonnet 4.5 → Claude Opus 4.5 → GLM-4.7 → Gemini 3 Flash → Claude Haiku 4.5 

- Claude Opus 4.5 不在第一位有成本原因，实际上有mpm在，大多数时候不需要它。
- GPT系列没有提及是因为速度太慢无法忍受。
- kimi和minimax实际使用并不便宜，而且5h限额较低对重度个人用户不友好，有这个等待和试错成本不如直接上sonnet 4.5。
- GLM则是因为真的便宜，而且指令遵守能力极强，它不会很聪明，但是它听话。
- Gemini 3 Flash 快，聪明，但是它自我权限给的太高，老是自己发现“问题”跑去处理。搭配mpm进行约束，实际能力有惊喜。
```

**适用人群说明**：

对于无法准确描述需求、对技术栈缺乏了解的技术新手，往往需要依赖高算力、高思考强度的模型来弥补 **Semantic Synchrony**（语义同步）的差距。MPM 对这类人群的加成效果存在较大波动，无法保证始终产生正面收益。

---

## 7.2 工具速查表

### 系统工具

| 工具 | 触发词 | 用途 |
|------|--------|------|
| `initialize_project()` | 初始化、绑定项目 | 每次新对话必调用 |
| `open_hud()` | 打开面板、HUD | 可视化控制面板 |
| `open_timeline()` | 时间线、Timeline | 项目演进可视化 |

### 调度工具

| 工具 | 触发词 | 用途 |
|------|--------|------|
| `manager_analyze()` | 分析任务、规划、mg | 大型任务规划与调度 |
| `prompt_enhance()` | 增强、pe、小任务 | 小任务意图增强 |
| `manager_create_hook()` | 挂起、断点、待办 | 任务中断时保存状态 |
| `manager_list_hooks()` | 待办列表、Hook | 查看挂起的任务 |
| `manager_release_hook()` | 释放、完成Hook | 任务完成时关闭 |

### 感知工具

| 工具 | 触发词 | 用途 |
|------|--------|------|
| `project_map()` | 项目地图、结构 | 生成项目结构总览 |
| `code_search()` | 搜索、找函数 | 搜索符号定位 |
| `code_impact()` | 影响分析、依赖 | 修改前评估风险 |

### 专家工具

| 工具 | 触发词 | 用途 |
|------|--------|------|
| `wiki_writer()` | 写文档、Wiki | 生成项目 Wiki 大纲与写作指南（支持 style） |

### 记录工具

| 工具 | 触发词 | 用途 |
|------|--------|------|
| `memo()` | 记录、备忘、memo | 操作完成后归档 |
| `system_recall()` | 回忆、历史、recall | 检索过去的决策 |
| `known_facts()` | 铁律、避坑、记住 | 存档长期有效规则 |

### 技能工具

| 工具 | 触发词 | 用途 |
|------|--------|------|
| `skill_list()` | 技能列表、有什么技能 | 查看可用领域知识 |
| `skill_load()` | 加载技能、使用技能 | 获取专家指南 |

### 人格工具

| 工具 | 触发词 | 用途 |
|------|--------|------|
| `persona(mode="list")` | mpm 人格、mpm persona | 查看可用人格 |
| `persona(mode="activate", name="xxx")` | mpm 人格 xxx | 激活指定人格 |

**Context 流失度设计**：

人格 Buff 不持久化，会随对话长度自然衰减。这是有意的设计：

| 机制 | 作用 |
|------|------|
| **时效性** | 人格效果随对话长度自然衰减 |
| **信号功能** | AI 风格变化 = Context 太长的信号 |
| **用户决策** | 提示你：压缩 context 或新开对话 |

**为什么这样设计**：
- 借鉴游戏 Buff 机制：有时间限制
- 自然提示：通过 AI 风格变化感知 context 状态
- 避免僵化：防止人格在超长对话中"固化"

---

## 7.3 场景指南

### 场景 1：首次接触新项目

### 场景 1：首次接触新项目

```javascript
// "初始化当前项目"
initialize_project()

// "生成项目地图"
project_map()

// (阅读 _MPM_PROJECT_RULES.md 了解项目规范)

// "查看有哪些专家技能"
skill_list()
```

### 场景 2：修复一个 Bug

### 场景 2：修复一个 Bug

```javascript
// "分析并规划这个修复任务"
manager_analyze(task_description="修复 xxx 的 bug", symbols=["xxx"])

// "搜索目标代码定义"
code_search(query="xxx")

// "评估重构影响范围"
code_impact(symbol_name="xxx")

// (执行代码修改)

// "记录本次改动备忘"
memo(items=[{...}])
```

### 场景 3：大型重构

### 场景 3：大型重构

```javascript
// "开启大型重构任务规划"
manager_analyze(task_description="重构 xxx 模块")

// "获取详细的项目符号结构"
project_map(detail="full")

// "创建一个重构断点"
manager_create_hook(description="重构进行中")

// (分步执行，每步调用 memo())

// "结束重构，释放断点"
manager_release_hook(hook_id="xxx")
```

### 场景 4：任务中断/恢复

**中断时**：
```
manager_create_hook(description="当前进度描述", priority="high")
```

**恢复时**：
```
1. initialize_project()
2. 阅读 dev-log.md
3. manager_list_hooks()
4. 继续任务...
5. manager_release_hook(hook_id="xxx", result_summary="完成描述")
```

### 场景 5：需要领域专家知识

### 场景 5：需要领域专家知识

```javascript
// "列出所有可用专家技能"
skill_list()

// "加载特定的前端设计指南"
skill_load(name="frontend-design")

// "查阅技能包内的特定参考资源"
skill_load(name="xxx", resource="references/guide.md")
```

### 场景 6：自动化代码审查

利用 `project_map` + `code_search` + `code_impact` 进行项目级代码审查；`wiki_writer` 用于先生成 Wiki 大纲与写作指南，作为审查/写作的“任务指引”。

### 场景 6：自动化代码审查

利用 `wiki_writer` 快速生成审查/写作指引（项目地图 + 风格规范），再按需深入关键模块。

```javascript
// "生成项目 Wiki 大纲与审查指引（可选指定风格）"
wiki_writer(style="technical")

// 接下来按指引执行（示例）：
project_map()
code_search(query="关键符号")
code_impact(symbol_name="关键符号")
// read_file(...) / 然后记录发现到你的审查文档
```

> **优势**: 指引先行、再定点深挖；避免盲目通读与漏查，且更贴合当前 `wiki_writer` 的无状态设计。

---

## 7.4 命名规范

> "在 AI 时代，变量名不是写给人看的注释，而是写给 LLM 看的 Prompt。"

### 为什么命名很重要？

Vibe Coding 的核心是**心流**。为了保持心流，必须减少与 AI 的"摩擦力"。最大的摩擦力来源就是**幻觉**——AI 猜错了你的意图。

如何消除幻觉？答案是**上下文密度**。代码中密度最高的地方，就是**命名**。

### 三大命名法则

**1. 符号锚定**
- 原则：拒绝通用词，拥抱全称
- 反例: `data = get_data()` (信息量为 0)
- 正例: `verified_payload = auth_service.fetch_verified_payload()`

**2. 前缀即领域**
- 原则：使用 `domain_action_target` 结构
- 示例：`ui_btn_submit`、`api_req_login`、`db_conn_main`

**3. 可检索性优先**
- 原则：名字越长，冲突越少，越容易被搜索定位
- 示例：`id` 可能有 10 万个匹配，但 `transaction_unique_trace_id` 全局唯一

### 新旧项目策略

| 项目类型 | MPM 行为 |
|---------|---------|
| **新项目** | 推荐采用 Vibe Coding 命名规范 |
| **旧项目** | **自动分析现有风格并生成规则文件** |

### 智能命名指纹分析

这是 MPM 最贴心的功能之一：

当你对一个**旧项目**执行 `initialize_project()` 时，MPM 会自动：

1. **AST 扫描** - 使用 Rust 引擎快速扫描项目里的函数名、变量名
2. **风格统计** - 统计命名风格（如 `get_user_name` 下划线风格 vs `getUserName` 驼峰风格）
3. **前缀识别** - 提取常用命名前缀（如 `_`、`get_`、`set_`）
4. **自动生成** - 创建 `_MPM_PROJECT_RULES.md` 规则文件

**示例输出**：
```markdown
## 检测结果

| 项目类型 | 旧项目 (检测到 158 个源码文件，503 个符号) |
|---------|------|
| **函数/变量风格** | `snake_case` (83.7%) |
| **类名风格** | `PascalCase` |
| **常见前缀** | `_`、`get_`、`set_`、`internal_` |
```

### 使用步骤

1. 执行 `initialize_project()` 后，查看项目根目录的 `_MPM_PROJECT_RULES.md`
2. **将内容复制到你的 IDE rules 配置中**：
   - Cursor: `.cursorrules`
   - Windsurf: `.windsurfrules`
   - Claude Code: `.claude/CLAUDE.md`
3. LLM 会在每次对话开始时读取这些规则，自动适配项目风格

> **效果**：你不需要教 LLM "这个项目用驼峰还是下划线"，它通过读取规则文件，**自动变身为这个项目的资深开发者**。

---

## 7.5 代码定位黄金组合

> **这是效率翻倍的核心技巧！**

### 低效做法：大海捞针

先用 IDE 自带的搜索盲目查找，发现找不到或找错了，再换方法。

```
grep_search("some_function")  → 搜出 50 个结果，不知道哪个是对的
view_file("random_file.py")   → 打开后发现不是想要的
...反复试错...
```

### 高效做法：先定位，再精查

**第一步**：用 `code_search` 在语义级别定位目标

```
code_search(query="target_function")
→ 返回：物理路径、行号、逻辑上下文
```

**第二步**：用 IDE 工具精确查看

```
view_file("path/to/file.py", StartLine=100, EndLine=150)
→ 直接看到上下文
```

### 工具能力对比

| 工具 | 视角 | 底层技术 | 强项 | 适用阶段 |
|------|------|----------|------|----------|
| `project_map` | 宏观架构 | Tree-sitter AST | 理解文件关系 | 探索/接手 |
| `code_search` | 精准定位 | Ripgrep + AST | 找定义 | 定位问题 |
| `code_impact` | 逻辑依赖 | AST + 调用分析 | 找依赖、影响范围 | 重构评估 |

### 推荐流程

```
1. code_search(query="xxx")            # 语义搜索，找到候选
2. view_file("path", StartLine, EndLine)  # 阅读代码
3. code_impact(symbol_name="xxx")      # 评估影响（如需修改）
```

---

## 7.6 黄金法则

### 规则 1：变更即记录

**任何代码/文档变更后，必须调用 `memo()`**

```javascript
// "记录本次改动备忘"
memo(items=[{
    "category": "开发",
    "entity": "skill_tool.go",
    "act": "refactor",
    "path": "internal/tools/skill_tools.go",
    "content": "移除 extractRules 函数"
}])
```

### 规则 2：修改前必定位

**严禁盲改！修改代码前必须先定位。**

```javascript
// "定位目标代码位置"
code_search(query="target_function")
```

### 规则 3：大改动必评估

**重构前必须评估影响范围。**

```javascript
// "分析此项修改的影响范围"
code_impact(symbol_name="module_name")
```

### 规则 4：新对话读日志

**新开对话时，阅读 `dev-log.md` 快速获取上下文。**

> **注意**：`initialize_project()` 只在以下情况需要调用：
> - 重启了 MCP Server
> - 重启了 IDE (Cursor/Windsurf/Claude Code)
> - 首次使用 MPM
>
> 如果只是新开一个对话（MCP 没重启），直接读 `dev-log.md` 即可。

---

## 7.7 Anti-Patterns（禁忌行为）

| 禁忌 | 说明 | 正确做法 |
|------|------|----------|
| **上帝文件修改** | 一次性重写超过 300 行 | 分块进行原子修改 |
| **僵尸代码** | 保留注释掉的旧代码 | 删除或归档 |
| **静默失败** | 工具调用失败后装作成功 | 报告并分析原因 |
| **盲目修改** | 不定位就直接改代码 | 先 `code_search` 再修改 |
| **忘记记录** | 修改后不调用 `memo` | 每次修改后必记录 |

---

## 7.8 健壮性检查清单

开始写代码前，问自己三个问题：

1. **如果不看上下文，光看这个变量名，我知道它是干嘛的吗？**
2. **如果我要在全局搜这个变量，能搜到精确的结果吗？**
3. **如果把这个函数名喂给 LLM，它能猜出返回值的结构吗？**

如果答案都是 YES，恭喜你，你已经掌握了 Vibe Coding 的真谛。

---

*End of Vibe Coding Manual*
