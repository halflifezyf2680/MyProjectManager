# MPM 效能对比实战 (MPM Effectiveness Case Studies)

> **"从大海捞针到按图索骥。"**

本文档记录了 MPM 架构（Manager + 符号探测）与原生 LLM 操作模式的一次真实盲测对比。测试旨在验证 MPM 核心理念——**"情报包 (Intel Package)"** 与 **"认知重力 (Cognitive Gravity)"** 在实际工程中的效能。

> **更新日期**: 2026-02-03
> **文档类型**: 实战案例
> **版本**: Go MCP Server v1.0

---

## Case Study 1: 符号定位与情报包 (Symbol Anchoring & Intel Package)

### 1.1 测试背景 (Experiment Setup)

* **任务**: 分析 `memo` 工具的实现逻辑，并找出其与 `dev-log.md` 的同步机制。
* **测试对象**:
  * **Group A (With MPM)**: 启用 `manager_analyze`，搭载 MPM 架构。
  * **Group B (No MPM)**: 禁用 Manager，仅使用基础文件操作工具 (List/Read/Search)。
* **评估指标**: 步骤数 (Turn Count)、工具调用效率、定位精准度。

---

### 1.2 核心数据对比 (The Metrics)

| 指标              | Group A (With MPM)       | Group B (No MPM)                  | 效能提升      |
|------------------ |------------------------ |--------------------------------- |------------ |
| **步骤数 (Turns)** | **3 步**                  | **12+ 步**                         | **+300%**    |
| **工具调用**       | **2 次** (Manager, View) | **10+ 次** (List, Search, Grep...) | **+400%**    |
| **首步命中率**     | **100%** (直接命中目标)   | **0%** (首轮搜索返回大量噪音)       | **∞**       |
| **定位方式**       | 战术坐标引导 (Anchoring) | 暴力地毯式搜索 (Brute Force)      | -            |

#### 可视化流程对比

```mermaid
graph TD
    subgraph MPM ["With MPM (3 Steps)"]
        M1[User: Ask Task] --> M2[Manager: Mission Briefing]
        M2 -->|Anchors: memory.go, dev-log.md| M3[LLM: View Specific Code]
        M3 --> M4[LLM: Perfect Insight]
    end

    subgraph Native ["No MPM (10+ Steps)"]
        N1[User: Ask Task] --> N2[LLM: Search '*memo*']
        N2 -->|Too many results| N3[LLM: Search 'memo' code]
        N3 -->|Noise| N4[LLM: View Directory]
        N4 --> N5[LLM: Grep 'func memo']
        N5 -->|Failed| N6[LLM: Try Shell Grep]
        N6 -->|Found| N7[LLM: Verify File]
        N7 --> N8[LLM: Read Code]
        N8 --> N9[LLM: Final Insight]
    end
```

---

### 1.3 详细复盘 (Playback)

#### 1.3.1 Group A: 专家的自信 (With MPM)

仅需 **1 个工具调用** 即可完成情报构建。

* **Step 1 (Manager Analyze)**:
  
  * LLM 调用 `manager_analyze`。
  
  * Manager 使用 `code_search` 工具从任务描述中提取出符号 `memo`。
  
  * **核心返回** (精简版):
    
    ```json
    {
      "mission_control": {
        "intent": "RESEARCH",
        "user_directive": "分析 memo 工具..."
      },
      "context_anchors": [
        {
          "symbol": "wrapMemo",
          "file": "internal/tools/memory_tools.go",
          "line": 46,
          "type": "function"
        }
      ],
      "guardrails": {
        "critical": ["READ_ONLY: 严禁修改任何文件"]
      }
    }
    ```
  
  * **关键点**：
    
    * `context_anchors`: **精准给出文件路径 + 行号**
    * `symbol` + `type`: 明确符号名和类型
    * `guardrails.critical`: 自动识别只读任务，加上防护

* **Step 2 (Execution)**:
  
  * LLM 收到情报包，**不再进行任何搜索**。
  * 直接调用 `read_file` 阅读 `memory_tools.go`的指定行。

* **Step 3 (Conclusion)**:
  
  * 输出分析结果：识别出 SSOT 模式和归档逻辑。

**评价**: 整个过程行云流水，LLM 表现得像一个在这个代码库工作了 5 年的资深架构师。

#### 1.3.2 Group B: 迷雾中的探索 (No MPM)

LLM 陷入了典型的 **"冷启动迷茫" (Cold Start Confusion)**。

* **Step 1-3 (Panic Search)**:
  * 先搜文件名 `*memo*` -> 出来一堆无关的 `.md` 文件。
  * 再搜代码 `memo` -> 几百个匹配结果，全是噪音。
* **Step 4-6 (Tool Struggle)**:
  * 试图列出目录找灵感。
  * 试图用 `grep` 搜定义，结果忘了加行号参数。
  * 被逼无奈，切到 Shell 模式暴力搜索。
* **Step 7-10 (Validation)**:
  * 最终找到了 `memory_tools.go`，但需要反复确认。
  * 谨慎地读取文件，验证是否为目标。

**评价**: LLM 虽然最终完成了任务，但过程效率较低。**大量的工作集中在定位代码位置，而非分析代码本身。**

---

### 1.4 核心洞察 (Insights)

#### 1.4.1 符号定位的核心力量

Manager 内部的符号定位机制（基于 Rust AST Indexer 的 `code_search`），解决了大模型工程化最大的痛点——**上下文定位**。

**对比 grep**:

| 维度         | code_search       | grep          | 胜者         |
|------------- |------------------ |-------------- |------------ |
| 精确符号匹配  | ✅ 符号级 + 行号    | ❌ 文本级      | code_search |
| 模糊查询      | ✅ 5 层降级         | ❌ 必须精确字符 | code_search |
| 上下文信息    | ✅ 调用关系         | ❌ 原始文本     | code_search |
| 纯文本搜索    | ❌                 | ✅ 原始文本    | grep        |

#### 1.4.2 从 "Search" 到 "Review"

MPM 改变了 LLM 的认知模式：

* **Old**: Search (搜) -> Recall (记) -> Read (读) -> Reason (想)
* **New**: **Briefing (览)** -> Read (读) -> Reason (想)

#### 1.4.3 隐形护栏的价值

在 `Group A` 的情报包中，风险提示让 LLM 在后续阅读代码时带有更强的**风险意识**。而 `Group B` 的 LLM 直到读完代码都不知道自己正处于一个高复杂度模块中。

---

### 1.5 Case Summary

**"情报包 -> 移交" (Intel -> Handoff)** 模式在实践中证明是高效的。

它既保留了 LLM 的自主性（不强制指令），又提供了精确的上下文信息（Context Anchors）。

**MPM 为 LLM 提供了结构化的工作环境。**

---

## Case Study 2: Finder 认知重力系统

> **"Agent 也有认知盲区。"**

本案例通过双盲实验，验证了 `project_map` (项目地图 + 热力图) 与 `code_impact` (影响分析) 对 Agent 决策安全性的影响。

### 2.1 实验背景

* **任务**: "我是新来的，我要修改 `session.go` 的逻辑，这安全吗？"
* **测试对象**:
  * **Group A (With MPM)**: 启用 `project_map` + `code_impact`。
  * **Group B (No MPM)**: 禁用 MPM，仅使用 grep/read_file (基线水平)。
* **场景**: 冷启动环境 (Cold Start)。

### 2.2 详细对比过程

#### Round 1: 认知建立

* **No MPM (Group B)**:
  
  * Agent 试图 `read_file("session.go")`，但被文件长度劝退。
  * **实际情况**: "文件较长，先读取部分内容查看..."
  * **结果**: 只能判断这是个"重要的文件"，但**无法准确评估风险程度**。

* **With MPM (Group A)**:
  
  * Agent 调用 `project_map()`。
  
  * **系统返回复杂度警报**：
    
    ```markdown
    ## Hotspots (Top 5 Complexity)
    - `session.go:GetSession` — Score: 85
    - `manager.go:Analyze` — Score: 92
    ```
  
  * **系统响应**: "检测到高复杂度区域，需要谨慎分析。"
  
  * **关键优势**: **快速识别高风险区域**，在未打开文件前就获得了复杂度信息。

#### Round 2: 影响评估

* **No MPM (Group B)**:
  
  * **操作**: 搜索 "session_manager"
  * **结果**: 找到了几个显式引用。
  * **Agent 结论**: "看起来影响范围有限。**修改风险可控。**"
  * **问题**: 完全漏掉了隐式调用关系。

* **With MPM (Group A)**:
  
  * **操作**: `code_impact(symbol_name="session_manager", direction="both")`
  
  * **系统返回的详细分析报告**:
    
    ```text
    CODE_IMPACT_REPORT: session_manager
    RISK_LEVEL: high
    COMPLEXITY: 85.0
    AFFECTED_NODES: 15
    
    #### POLLUTION_PROPAGATION_GRAPH
    LAYER_1_DIRECT:
      - [session.go:154-182] SYMBOL: GetSession (function)
    LAYER_2_INDIRECT:
      - [intelligence_tools.go:418-520] SYMBOL: wrapAnalyze (function)
      ... and 12 more nodes
    
    #### ACTION_REQUIRED_CHECKLIST
    - [ ] MODIFY_TARGET: [session.go:100-150] NAME: session_manager
    - [ ] VERIFY_CALLER: [session.go:154-182] NAME: GetSession
    - [ ] VERIFY_CALLER: [intelligence_tools.go:418-520] NAME: wrapAnalyze
    ```    * **Agent 结论**: **"高风险区域，建议按照清单进行严格的验证和修改。"**

### 2.3 效能对比

| 维度           | No MPM            | With MPM            | 备注       |
|--------------- |----------------- |-------------------- |---------- |
| **风险感知**    | 低 (基于局部信息)  | **高** (基于 AST 分析)| 显著差异  |
| **Token 消耗** | 高 (通读文件)     | **低** (Map 摘要)   | 成本优化   |
| **决策依据**    | 经验判断          | 数据支持             | 可解释性   |
| **行动建议**    | 模糊反问          | 精确清单             | 可执行性   |

### 2.4 核心洞察 (Core Insights)

**从 "文本猜测" 到 "结构推理" 的范式转移**

这次双盲实验揭示了当前 Agentic Coding 的核心痛点：**大多数 Agent 仍停留在"文本处理"阶段，而非真正的"软件工程"阶段。**

* **Old Way (The Guesser)**: Agent 依赖 grep 在海量文本中寻找线索。
* **New Way (The Engineer)**: MPM 为 Agent 提供了一套高精度的 **"认知导航系统"**。
  * **Map** 提供了空间坐标，消除了迷茫。
  * **Impact** 提供了因果链条，消除了盲视。

这也正是 MPM 存在的终极意义：**让 AI 拥有与人类资深架构师同等的"直觉"，从而将"Vibe Coding"的高效与"Engineering"的严谨完美融合。**

---

## Case Study 3: 光速认知与 Rust AST

> **"15 秒 vs 40 秒。1 次调用 vs 4+ 次调用。显著的效率优势。"**

本案例通过**对照实验**，验证了 `project_map` 在新项目冷启动场景下的高效性能。

### 3.1 对照实验设计

* **测试场景**: 新项目冷启动认知（300+ 文件，2000+ 符号）。
* **测试对象**:
  * **Group A (With MPM)**: 使用 `project_map()` + Claude Sonnet 4.5。
  * **Group B (No MPM)**: 使用原生 IDE 工具 (Read files, Search, List dirs) + Claude Sonnet 4.5。

### 3.2 效能数据对比

| 指标            | With MPM                   | Without MPM                | MPM 优势        |
|--------------- |-------------------------- |-------------------------- |--------------- |
| **总耗时**      | **15 秒**                  | **40 秒**                  | **2.67x 加速**  |
| **工具调用**    | **1 次**                   | **4+ 次**                  | **4x+ 减少**    |
| **Token 输入**  | **~800** (结构化 JSON)     | **4000+** (原始文件内容)   | **5x 减少**     |
| **认知路径**    | 直达 (JSON → 解读)         | 迂回 (配置 → 源码 → 拼装)   | 质的差异        |

### 3.3 详细过程对比

#### With MPM —— 快速方式 (15 秒)

**Action**: `project_map(detail="standard")`

**关键时刻**: 工具返回的瞬间，Agent 获得了完整的项目结构信息。Rust AST 引擎已经提取出了代码骨架。

**Agent 响应**:

> "代码地图已生成。这是一个 Go 语言的 MCP Server，采用模块化架构..."

基于结构化数据进行分析，而非基于猜测。

#### Without MPM —— 常规方式 (40 秒)

Agent 需要通过多个步骤逐步理解项目：

1. **查找入口**: 先读 `main.go` 找入口。
2. **查看依赖**: 读了 `go.mod` 才知道依赖。
3. **理解结构**: 查看目录结构，逐步拼凑出模块概念。
4. **验证理解**: "项目规模较大，需要确认更多细节..."

虽然最终结论正确，但消耗了**4倍的操作**和**5倍的 Token** 才达到同样的认知水平。

### 3.4 核心洞察 (Insights)

#### 3.4.1 信息压缩的优势

Rust AST 引擎不仅速度快 (<1000ms)，更重要的是实现了**高效的信息压缩**。

MPM 把 300+ 个文件的**物理复杂度**，压缩成了一张几百 Token 的**逻辑地图**。

#### 3.4.2 上下文工程探索 (Context Engineering)

有人可能会问：*"这不就是个 AST Map 吗？IDE 早就有了。"*

是的，AST 是旧技术，但 **MPM 对 AST 的使用方式是革命性的**。

* **Prompt Engineering (低维)**: 试图用花哨的话术去"催眠"大模型。
* **Context Engineering (高维, MPM)**: 用硬核的代码（Rust/Go）去清洗、压缩、结构化输入数据，直接喂给模型**高质量的信息熵**。

MPM 不是在写 Prompt，而是在**构建 Context**。它不试图教大模型"如何思考"，而是通过 Rust 引擎的高速预处理，直接提供**高质量的结构化信息**。

**结论**: 相比让 Agent 自行探索项目结构，MPM 直接提供了完整的项目地图，这就是 40 秒变 15 秒的关键。

---

## Case Study 4: 数据库备份机制 (Database Backup Resiliency)

> **"代码可能会消失，但记录（Memo）是持久的。"**

本案例源自一次真实的工程事故：用户因操作失误执行了 `git reset --hard`，导致整整一天的 **Uncommitted（未提交）** 代码被物理删除。

### 4.1 事故现场

* **情况**: 一整天的开发成果完全丢失。
* **Git 局限**: 由于代码未 commit，`git reflog` 无法找回任何内容。
* **MPM 表现**: 通过 `system_recall` 发现，MPM 数据库（SQLite）完整记录了每一项修改意图、影响路径和核心逻辑说明。

### 4.2 韧性对比

| 维度         | Git (Version Control)    | MPM DB (SSOT)                        |
|------------- |------------------------ |------------------------------------- |
| **记录触发** | 显式 Commit / Stash      | **原子化 Memo (修改即记录)**           |
| **覆盖范围** | 物理文本 (Snapshot)      | **意图 + 语义 (Decision Memo)**       |
| **抗灾能力** | 弱 (对未提交修改无效)     | **强 (独立于源码目录同步)**           |
| **恢复成本** | 全量回滚或手动重写        | **指导性恢复 (Guided Recovery)**        |

### 4.3 为什么数据库存储是"救命稻草"？

在 Vibe Coding 模式下，程序员往往会连续工作数小时而不 commit。这时，每一个 `memo()` 调用都是在向 **"工程黑匣子"** 写入生存补丁。

* **意图恢复 (Intent Restoration)**: 数据库记录的不只是 "改了什么"，更有 "为什么要这么改"。
* **独立于物理结构**: MPM 数据库采用了独立的持久化策略，即使你做了一个灾难性的 `rm -rf *`，Agent 的认知依然是连续的。

### 4.4 核心洞察 (Insights)

#### 4.4.1 SSOT (唯一真理源) 的真正威力

SSOT 不仅是用来给 AI 读的，更是给系统做 **"意志持久化"** 的。

#### 4.4.2 认知的持久化

物理世界的代码是脆弱且容易丢失的。而基于数据库的结构化 Memos 构建了一套**持久化系统**。它将碎片化的、易失的代码修改，固化成了有序的、持久的**工程认知链**。

**结论**: **Git 保护的是代码，而 MPM 保护的是开发过程和决策记录。**

---

## Case Study 5: 模糊搜索的认知导航威力 (Cognitive Navigation)

> **"从'精确匹配'到'语义理解'的范式跨越。"**

本案例验证了 Rust AST 引擎的 **5 层降级搜索** 不仅仅是"容错机制"，更是实现 **"认知导航"** 的核心能力。

### 5.1 探索背景

* **任务**: 查找代码中的符号搜索机制
* **已知信息**: 只知道一个模糊的概念 —— "符号搜索"
* **未知信息**: 不确定具体函数名

### 5.2 搜索实验记录

#### 实验 1：模糊概念搜索

调用：

```
code_search(query="symbol search", search_type="function")
```

**实际结果**：

```json
{
  "found_symbol": {
    "file": "internal/tools/search_tools.go",
    "name": "CodeSearch",
    "line": 25,
    "type": "function"
  }
}
```

**关键发现**：

- AST 层找不到名为"符号搜索"的函数
- 但通过**语义关联**，找到了 `CodeSearch`
- 即使输入是模糊概念，也能匹配到正确的函数

### 5.3 搜索方式对比

| 搜索输入     | grep 结果    | code_search 结果   | 差异         |
|------------- |------------- |------------------- |------------ |
| `符号搜索`   | 0 行         | 找到 `CodeSearch`  | **语义推断**  |
| `symbol`    | 500+ 行噪音  | 找到相关函数簇      | **语义关联**  |
| `search`    | 200+ 行噪音  | 找到精确匹配        | **上下文推理** |

### 5.4 核心洞察

#### 5.4.1 超越"文本匹配"的"语义理解"

传统搜索工具只能在**字面层面**匹配，而 Rust AST 引擎进行的是**符号层**的匹配。

#### 5.4.2 认知的"联想能力"

这 5 层降级搜索，本质上是模拟了**人类的联想思维**：

```
人类思维: "符号搜索机制..."
  ↓
联想 1: 搜 "symbol" → 找到相关函数
  ↓
联想 2: 看注释 → 理解设计意图
  ↓
联想 3: 看调用链 → 理解实际用法
```

**Rust AST 引擎自动完成了这些联想**，将"搜索"升级为"理解"。

### 5.5 结论

**5 层降级搜索不是"备选方案"，而是"核心能力"**。

它体现了 MPM 的设计哲学：

- **LLM 不需要成为专家**，只需要描述需求
- **系统应该理解意图**，而不是等待精确指令
- **模糊输入 → 精确输出**，这才是真正的智能

**这就是"认知导航"的威力：你只需要指向大致方向，系统会带你到达精确位置。**

---

## 总结

这五个案例从不同角度验证了 MPM 的核心价值：

1. **Case Study 1**: 符号定位与情报包 — **300% 效率提升**
2. **Case Study 2**: 认知重力系统 — **风险感知能力显著提升**
3. **Case Study 3**: 光速认知 — **2.67x 加速，5x Token 减少**
4. **Case Study 4**: 数据库备份机制 — **Git 之外的补充方案**
5. **Case Study 5**: 认知导航 — **从精确匹配到语义理解**

**MPM 的本质**: 通过工程化手段构建可靠的基础设施，让 AI 从"对话助手"向"开发工具"转变。

---

*文档完*
