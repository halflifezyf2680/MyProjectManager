# 第8章 工具快速参考

> 输入 工具名称/触发词  给参数或者让LLM自己填参数调用工具

---

## 🔍 代码定位 (3个)

### 8.1 project_map

**触发词**: `mpm 地图`, `mpm 结构`

**参数**:

| 参数           | 类型     | 默认值       | 说明                           |
| ------------ | ------ | --------- | ---------------------------- |
| `scope`      | string | -         | 目录路径（可选）                     |
| `level`      | string | `symbols` | 视图层级 (`structure`/`symbols`) |
| `core_paths` | string | -         | 核心目录列表（JSON 数组字符串）           |

### 8.2 code_search

**触发词**: `mpm 搜索`, `mpm 定位`

**参数**:

| 参数            | 类型     | 默认值   | 说明                              |
| ------------- | ------ | ----- | ------------------------------- |
| `query`       | string | 必填    | 搜索关键词                           |
| `scope`       | string | -     | 限定范围                            |
| `search_type` | string | `any` | 符号类型 (`any`/`function`/`class`) |

### 8.3 code_impact

**触发词**: `mpm 影响`, `mpm 依赖`

**参数**:

| 参数            | 类型     | 默认值        | 说明                                 |
| ------------- | ------ | ---------- | ---------------------------------- |
| `symbol_name` | string | 必填         | 要分析的符号名（函数名或类名）                    |
| `direction`   | string | `backward` | 分析方向 (`backward`/`forward`/`both`) |

---

## 🧠 任务管理 (5个)

### 8.4 manager_analyze

**触发词**: `mpm 分析`, `mpm mg`

**参数**:

| 参数                 | 类型     | 默认值 | 说明                                                      |
| ------------------ | ------ | --- | ------------------------------------------------------- |
| `task_description` | string | 必填  | 用户的原始指令/任务详情                                            |
| `intent`           | string | 必填  | 任务意向 (`DEBUG`/`DEVELOP`/`REFACTOR`/`DESIGN`/`RESEARCH`) |
| `symbols`          | array  | 必填  | 提取的代码符号列表                                               |
| `read_only`        | bool   | -   | 是否为只读分析模式                                               |
| `scope`            | string | -   | 任务范围描述                                                  |
| `step`             | int    | `1` | 执行步骤（1=分析, 2=生成策略）                                      |
| `task_id`          | string | -   | 步骤2时必填，步骤1返回的 task_id                                   |

- **复杂度评估**: High → 建议先 code_impact 分析影响范围
- **约束提醒**: 自动列出所有 Critical 约束
- **工具策略**: 根据实际情况给出针对性建议

> **与 prompt_enhance 的区别**:
> 
> - `manager_analyze`: 两步自迭代，strategic_handoff **基于真实代码分析动态生成**
> - `prompt_enhance`: 单步硬编码模板注入

### 8.5 task_chain

**触发词**: `mpm 任务链`, `mpm 续传`, `mpm chain`
**动态任务链执行器**。不仅仅是"顺序执行"，更支持**运行时动态调整**。

> **核心特性**: **自适应修正 (Adaptive Correction)**
> 在执行过程中，如果遇到预期外的情况（如编译失败、缺少文件、逻辑冲突），`task_chain` 允许 LLM 使用 `mode="insert"` 动态插入修复步骤，或使用 `mode="delete"` 移除过时步骤。
> 
> **实测数据**: 在 Claude Code + Sonnet/GLM-4 环境下，曾成功零干预跑通 1 小时级别的长流程复杂任务。

**⚠️ 重要**: 虽然支持动态调整，但严禁"跳步"。请务必严格按照 `Step 1 -> Next -> Next` 的节奏推进，每一步都确保验证通过再执行下一步。

**参数**:

| 参数             | 类型     | 默认值 | 说明                   |
| -------------- | ------ | --- | -------------------- |
| `mode`         | string | 必填  | 操作模式                 |
| `task_id`      | string | -   | 任务 ID（continue 模式除外） |
| `plan`         | array  | -   | 任务计划列表（step 模式）      |
| `insert_plan`  | array  | -   | 插入计划（insert 模式）      |
| `subtask_id`   | string | -   | 子任务 ID（delete 模式）    |
| `step_order`   | int    | -   | 步骤序号（delete 模式）      |
| `delete_scope` | string | -   | 删除范围（remaining）      |

**模式 (mode)**:

| 模式         | 说明          | 必需参数                                           |
| ---------- | ----------- | ---------------------------------------------- |
| `step`     | 初始化任务链      | task_id + plan                                 |
| `next`     | 执行下一步       | task_id                                        |
| `continue` | 继续执行（无状态模式） | -                                              |
| `resume`   | 恢复中断的任务     | task_id                                        |
| `insert`   | 动态插入新步骤     | task_id + insert_plan                          |
| `delete`   | 删除步骤        | task_id + (step_order/subtask_id/delete_scope) |
| `finish`   | 完成任务链       | task_id                                        |

**plan 格式** (step 模式):

| 字段       | 类型     | 必填  | 说明                       |
| -------- | ------ | --- | ------------------------ |
| `name`   | string | ✅   | 步骤名称                     |
| `expert` | string | -   | 可选的专家工具（如 `code_search`） |

**真实使用示例** (展示 task_chain 在复杂长流程中的价值):

> **用户**: "帮我把项目的日志系统从 log4j 迁移到 zap，需要更新所有 20+ 个服务模块，还要更新配置和文档"

> **LLM**: 这是一个跨 20+ 模块的大型迁移任务，步骤多、容易遗漏。我用 `task_chain` 建立完整执行计划：
> 
> ```javascript
> task_chain(mode="step", task_id="LOG_MIGRATION_001", plan=[
>   {"name": "搜索所有使用 log4j 的代码位置", "expert": "code_search"},
>   {"name": "分析日志调用模式和数据流"},
>   {"name": "设计 zap 迁移方案（保持兼容性）"},
>   {"name": "创建 zap 封装层"},
>   {"name": "迁移 core 模块"},
>   {"name": "迁移 api 模块"},
>   {"name": "迁移 service 模块"},
>   {"name": "迁移 dao 模块"},
>   {"name": "迁移 utils 模块"},
>   {"name": "迁移 config 模块"},
>   {"name": "更新配置文件（log4j2.xml → zap.yaml）"},
>   {"name": "更新依赖 pom.xml"},
>   {"name": "更新部署脚本"},
>   {"name": "编写单元测试验证日志输出"},
>   {"name": "集成测试验证各模块日志"},
>   {"name": "性能对比测试（log4j vs zap）"},
>   {"name": "更新 API 文档中的日志示例"},
>   {"name": "更新运维文档"},
>   {"name": "全量回归测试"},
>   {"name": "清理旧 log4j 代码和依赖"}
> ])
> ```

> *(执行第 3 步时，发现某个遗留模块有特殊的日志格式需求)*

> **LLM**: 检测到 `legacy` 模块有自定义日志格式器，需要在计划中插入适配步骤：
> 
> ```javascript
> task_chain(mode="insert", task_id="LOG_MIGRATION_001", insert_plan=[
>   {"name": "为 legacy 模块设计 zap 自定义 encoder"},
>   {"name": "验证 legacy 模块日志格式兼容性"}
> ])
> ```

> *(执行到第 12 步，更新配置文件时，发现 zap 配置语法与 log4j 差异很大)*

> **LLM**: zap 配置语法完全不同，需要插入配置转换步骤：
> 
> ```javascript
> task_chain(mode="insert", task_id="LOG_MIGRATION_001", insert_plan=[
>   {"name": "编写 log4j2.xml 到 zap.yaml 的转换脚本"},
>   {"name": "验证转换后的配置文件正确性"}
> ])
> ```

> *(执行到第 18 步，性能测试发现 zap 在高并发下内存占用异常)*

> **LLM**: 性能测试发现内存问题，需要插入排查优化步骤：
> 
> ```javascript
> task_chain(mode="insert", task_id="LOG_MIGRATION_001", insert_plan=[
>   {"name": "使用 pprof 分析内存占用"},
>   {"name": "优化 zap buffer pool 配置"},
>   {"name": "重新进行性能测试验证"}
> ])
> ```

> *(所有 20+ 步骤完成)*

> **LLM**: 日志系统迁移完成，总结：
> 
> - ✅ 20+ 模块全部迁移完成
> - ✅ 配置文件已更新
> - ✅ 文档已同步更新
> - ✅ 性能测试通过（zap 比 log4j 性能提升 40%）
> - ✅ 所有测试通过
>   
>   ```javascript
>   task_chain(mode="finish", task_id="LOG_MIGRATION_001")
>   ```

> **💡 为什么必须用 task_chain**:
> 
> - 20+ 步骤，人工管理容易遗漏
> - 执行中发现 3 次意外情况，需要动态插入步骤
> - 如果不用 task_chain，很可能：漏掉 legacy 模块适配、配置错误、内存问题未发现
> - task_chain 保证了每一步都有记录、可追溯、可恢复

### 8.6 manager_create_hook

**触发词**: `mpm 挂起`, `mpm 待办`, `mpm hook`

> **设计历史**：最初设计为 Manager 的"断点续传"机制，用于跨会话恢复任务。实际使用后发现用作待办事项更实用，所以保留了"Hook"这个名字。

创建任务钩子（待办事项）。

**参数**:

| 参数                 | 类型     | 默认值      | 说明                          |
| ------------------ | ------ | -------- | --------------------------- |
| `description`      | string | 必填       | 描述                          |
| `priority`         | string | `medium` | 优先级 (`low`/`medium`/`high`) |
| `task_id`          | string | -        | 关联任务 ID                     |
| `tag`              | string | -        | 自定义标签                       |
| `expires_in_hours` | int    | 0        | 过期时间（0=永不过期）                |

### 8.7 manager_list_hooks

**触发词**: `mpm 待办列表`, `mpm listhooks`
列出所有待办。

**参数**:

| 参数       | 类型     | 默认值    | 说明                   |
| -------- | ------ | ------ | -------------------- |
| `status` | string | `open` | 状态 (`open`/`closed`) |

### 8.8 manager_release_hook

**触发词**: `mpm 释放`, `mpm 完成`
标记待办已完成。

**参数**:

| 参数               | 类型     | 默认值 | 说明                    |
| ---------------- | ------ | --- | --------------------- |
| `hook_id`        | string | 必填  | Hook 编号（支持 `#001` 格式） |
| `result_summary` | string | 必填  | 完成总结                  |

---

## 📝 文档与记忆系列 (4个)

### 8.9 wiki_writer

**触发词**: `mpm wiki`, `mpm 文档`
为当前项目生成一份 **Wiki 大纲与写作指南**，不再管理多轮状态，而是一次性给出「项目地图 + 写作任务 + 风格规范」。

**参数**:

| 参数          | 类型     | 默认值             | 说明                                                         |
| ----------- | ------ | ---------------- | ---------------------------------------------------------- |
| `output_file` | string | `wiki_outline.md` | 输出文件名（逻辑上的目标文件名，实际内容由 LLM 根据指引生成并写入） |
| `style`     | string | -                | 书写风格：`technical` / `tutorial` / `reference` / `blog`，或任意自定义说明 |

**工作流程**:

1. 自动调用 `project_map` 生成完整的项目地图（`symbols` 级别），并读取 `.mcp-data/project_map_symbols.md` 作为参考资料。
2. 在工具返回中附带：项目统计信息、目录结构摘要，以及「基于项目地图编写 Wiki 大纲」的详细任务说明。
3. 提供 4 种预置风格模板（技术文档 / 教程 / 参考手册 / 博客），并根据 `style` 生成对应的「书写指南」附在结果末尾。
4. 由 LLM/用户根据这份指引，自主组合 `code_search` / `read_file` 等工具，完成实际的 Wiki 文档编写与保存。

> **说明**：旧版的 `init/next/add/status/review` 多模式、章节状态机和自动审查会话已废弃。现在 wiki_writer 聚焦于「提供高质量的大纲与风格规范」，代码审查与章节推进建议由 `project_map` + `code_search` + `task_chain` 组合完成。

### 8.10 memo

**触发词**: `mpm memo`, `mpm 记录`, `mpm 存档`

**参数**:

| 参数      | 类型     | 默认值  | 说明               |
| ------- | ------ | ---- | ---------------- |
| `items` | array  | 必填   | 录入事项列表（见下方结构）    |
| `lang`  | string | `zh` | 记录语言 (`zh`/`en`) |

**items 结构**:

| 字段         | 类型     | 必填  | 说明                        |
| ---------- | ------ | --- | ------------------------- |
| `category` | string | ✅   | 分类（如：修改、开发、决策、重构、避坑）      |
| `entity`   | string | ✅   | 改动的实体（文件名、函数名、模块名）        |
| `act`      | string | ✅   | 具体的行动（如：修复Bug、新增功能、技术选型）  |
| `path`     | string | ✅   | 文件路径                      |
| `content`  | string | ✅   | 详细内容，解释"为什么这么改"而非只说"改了什么" |

### 8.11 system_recall

**触发词**: `mpm 召回`, `mpm 历史`, `mpm recall`

**参数**:

| 参数         | 类型     | 默认值       | 说明                        |
| ---------- | ------ | --------- | ------------------------- |
| `keywords` | string | 必填        | 检索关键词                     |
| `category` | string | -         | 过滤类型（如：开发、重构、避坑等）         |
| `scope`    | string | `project` | 搜索范围 (`project`/`global`) |
| `limit`    | int    | `20`      | 返回条数                      |

### 8.12 known_facts

**触发词**: `mpm 铁律`, `mpm 避坑`, `mpm fact`
存档已验证的死规则。

**参数**:

| 参数          | 类型     | 默认值 | 说明                  |
| ----------- | ------ | --- | ------------------- |
| `type`      | string | 必填  | 类型 (`避坑`/`规则`/`铁律`) |
| `summarize` | string | 必填  | 简要说明（中文）            |

---

## 🎭 人格与技能系列 (3个)

### 8.13 persona

**触发词**: mpm 人格、mpm persona
人格管理工具（支持思维协议 Buff）。

**模式 (mode)**:

| 模式         | 说明     | 参数        |
| ---------- | ------ | --------- |
| `list`     | 列出可用人格 | -         |
| `activate` | 激活人格   | name (必填) |

**name 支持**: 英文 ID / 显示名称 / 别名

### 8.14 skill_list

**触发词**: `mpm 技能列表`, `mpm skills`
列出所有可用技能。

**返回**: 技能名称 + 描述 + 触发词列表

### 8.15 skill_load

**触发词**: `mpm 加载技能`, `mpm skill`, `mpm loadskill`
加载指定技能或资源。

**参数**:

| 参数         | 类型     | 默认值        | 说明                       |
| ---------- | ------ | ---------- | ------------------------ |
| `name`     | string | 必填         | 技能名称                     |
| `level`    | string | `standard` | 加载级别 (`standard`/`full`) |
| `resource` | string | -          | 子资源路径                    |
| `refresh`  | bool   | `false`    | 强制刷新缓存                   |

**level="standard"**: 返回正文
**level="full"**: 返回 Frontmatter + 正文

---

## 🖥️ 系统交互系列 (5个)

### 8.16 initialize_project

**触发词**: `mpm 初始化`, `mpm init`
项目环境绑定与数据库初始化。

**参数**:

| 参数             | 类型     | 默认值  | 说明    |
| -------------- | ------ | ---- | ----- |
| `project_root` | string | 自动探测 | 项目根路径 |

**执行动作**:

1. 路径安全校验
2. 创建 `.mcp-data/` 目录
3. 初始化 SQLite 数据库
4. 植入 `visualize_history.py`
5. 生成 `_MPM_PROJECT_RULES.md`
6. 启动服务发现心跳（向 HUD 注册）

### 8.17 open_hud

**触发词**: `mpm hud`, `mpm 监控`
打开 Rust Tauri 悬浮窗。

**无需参数**。自动检测 HUD 进程，避免重复启动。

### 8.18 open_timeline

**触发词**: `mpm 时间线`, `mpm timeline`
打开项目演进可视化界面。

**无需参数**。自动调用浏览器打开 `project_timeline.html`

### 8.19 save_prompt_from_context

**触发词**: `mpm 保存提示词`, `mpm saveprompt`
从对话上下文提炼提示词到数据库。

**参数**:

| 参数          | 类型     | 默认值       | 说明                        |
| ----------- | ------ | --------- | ------------------------- |
| `title`     | string | 必填        | 提示词标题                     |
| `content`   | string | 必填        | 提示词内容                     |
| `tag_names` | string | -         | 标签（逗号分隔）                  |
| `scope`     | string | `project` | 存储范围 (`project`/`global`) |

### 8.20 prompt_enhance

**触发词**: `mpm 增强`, `mpm pe`, `mpm enhance`
注入战术执行协议，强制精确执行。

**参数**:

| 参数                 | 类型     | 默认值      | 说明                        |
| ------------------ | ------ | -------- | ------------------------- |
| `task_description` | string | -        | 原始任务描述                    |
| `mode`             | string | `inject` | 操作模式 (`inject`/`explain`) |

**协议内容**:

1. 建立任务边界
2. 历史探测 (system_recall)
3. 意图解析与拆分
4. 现状映射
5. 任务规划输出
6. 立即执行

---

## 💡 最佳实践组合

### 1. 修改代码

```javascript
// "帮我定位目标函数"
code_search(query="目标函数")

// "分析修改这个函数会有什么影响"
code_impact(symbol_name="目标函数")

// "执行代码修改" (由 IDE 或 edit_file 完成)

// "记录刚才的变更备忘"
memo(items=[...])
```

### 2. 大型任务

```javascript
// "帮我分析并规划这个大任务"
manager_analyze(task_description="任务描述")

// "按计划执行下一步"
task_chain(mode="next", task_id="...")

// "记录进度"
memo(...)

// "任务完成，释放断点"
manager_release_hook(...)
```

### 3. 生成文档 ⭐ **自动审核**

```javascript
// "生成 Wiki 大纲与写作指南（可选指定风格）"
wiki_writer(style="technical")

// 之后按指引，自主组合定位/阅读工具补全章节内容：
project_map()
code_search(query="关键符号")
read_file(...)
```

### 4. 遇到困难

```javascript
// "看看有哪些可以帮我的技能"
skill_list()

// "加载特定的技术指南"
skill_load(name="go-expert")

// "搜一搜以前有没有解决过类似问题"
system_recall(keywords="database connection timeout")
```

### 5. 任务中断

```javascript
// "我要离开一会，先保存当前任务断点"
manager_create_hook(description="完成登录校验逻辑一半")

// (中断并开启新会话)

// "列出我还没做完的事情"
manager_list_hooks()

// "继续执行刚才的任务"
task_chain(mode="resume", task_id="...")

// "任务彻底搞定了，释放钩子"
manager_release_hook(hook_id="#001")
```

---

## 📊 工具分类矩阵

| 分类       | 工具数 | 核心价值           |
| -------- | --- | -------------- |
| **代码定位** | 3   | 精准搜索 + 影响分析    |
| **任务管理** | 5   | 任务拆解 + 断点续传    |
| **文档记忆** | 4   | SSOT 记录 + 历史召回 |
| **人格技能** | 3   | 风格定制 + 专家知识    |
| **系统交互** | 5   | 项目初始化 + 可视化    |

**总计**: 20 个工具

---

*本章完*
