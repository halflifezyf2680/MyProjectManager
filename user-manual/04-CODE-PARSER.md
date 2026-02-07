# 第4章 代码解析器 (Code Parser)

> **"不识庐山真面目，只缘身在此山中。"**

代码解析器是 MPM 的 **Context 工程化核心**，专注于 **AST 静态解析** 的上下文提取。

**核心能力**：Project Map（结构解析）| Code Search（符号定位）| Code Impact（影响分析）

**技术实现**：AST 静态解析 + 文本搜索 + 调用图分析

> **更新日期**: 2026-02-04
> **所属章节**: 第4章
> **版本**: Go MCP Server v2.0

> **与 IDE 原生工具的关系**：代码解析器是 IDE 内置搜索的**补充而非替代**。
> 
> - **IDE 工具**：文本级精确匹配，适合查找字符串
> - **代码解析器**：AST 级结构理解，解析符号定义与调用关系
> - **协同使用**：先用代码解析器定位符号，再用 IDE 工具清理残留引用

---

## 4.0 设计决策：为什么选择 AST 扫描而非 LSP？

MPM 采用 **快速 AST 扫描** 而非 **LSP**，因为**两种方案针对不同场景**。

### 4.0.1 场景差异

**LSP**：实时编辑辅助（代码补全、错误诊断），主动监听文件变化（Push 模式）

**MPM**：AI Context 提取，被动触发刷新（Pull 模式）
- 工具调用时自动刷新索引（`code_search`、`code_impact`、`project_map`）
- 基于 SHA256 哈希对比，只解析变更文件，速度很快
- 无需常驻进程监听

### 4.0.2 核心优势

**技术栈**：Tree-sitter（多语言 AST）+ Rust（零 GC）+ SQLite（增量索引）

**核心优势**：
1. **输出优化**：一次性提取所有信息（符号、调用关系、复杂度），格式化为 LLM 友好结构
2. **统一接口**：单一工具处理多种语言，行为一致
3. **零依赖**：静态链接的单一可执行文件，无需安装语言服务器
4. **特殊功能**：项目地图、影响分析等 LSP 不提供的功能

**这不是对 LSP 的否定，而是针对 MPM 特定场景（AI Context 工程化）的最优选择。**

---

## 4.1 三大独立工具

代码解析器提供三个**独立使用**的工具，可按需单独调用或组合使用：

| 工具              | 用途   | 典型场景           |
| --------------- | ---- | -------------- |
| **project_map** | 结构解析 | 刚接手项目，需要宏观视图   |
| **code_search** | 符号定位 | 知道函数名，不知道在哪个文件 |
| **code_impact** | 影响分析 | 修改前评估影响范围      |

**组合使用示例**（非强制流程）：

```
场景1: 接手新项目
  → project_map 了解结构 
  → code_search 定位关键符号

场景2: 修改已知函数
  → code_impact 评估影响
  → 直接修改

场景3: 搜索未知符号
  → code_search 定位
  → code_impact 查看调用关系
```

**核心能力**：

1. **Map**: 生成项目层级地图 + 复杂度热点 (DICE)
2. **Search**: AST 精确匹配 + Deep Context 反查
3. **Impact**: 调用链追踪 + 智能折叠（防 token 爆炸）

---

## 4.2 核心工具详解

### 4.2.0 触发词快速调用

为提高调用效率，所有解析工具均支持触发词唤醒：

**project_map**:

- 触发词: `mpm 地图`, `mpm 结构`, `mpm map`
- 示例: "mpm 地图 standard src/core"

**code_search**:

- 触发词: `mpm 搜索`, `mpm 定位`, `mpm 符号`, `mpm find`
- 示例: "mpm 搜索 SessionManager"

**code_impact**:

- 触发词: `mpm 影响`, `mpm 依赖`, `mpm impact`
- 示例: "mpm 影响 update_task backward"

### 4.2.1 project_map - 结构上下文提取器

**Context 工程价值**：将项目结构清洗成 LLM 可理解的层级地图，节省 LLM "理解项目" 的 Token。

**用途**: 接手新项目时的第一步，快速建立项目认知。

**调用示例**：`mpm 地图 symbols src/core`

**参数**:
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `scope` | 限定范围（目录或文件路径） | 整个项目 |
| `level` | 视图层级（`structure`/`symbols`） | `symbols` |
| `core_paths` | 核心目录列表（JSON 数组字符串） | - |

**两种视图层级**:
| 级别 | 内容 | 适用场景 |
|------|------|---------|
| `structure` | 目录树 + 复杂度统计（宏观结构） | 快速浏览项目架构 |
| `symbols` | 文件列表 + 符号详情 + 智能折叠（Top 10 详细展开） | 查看代码细节 |

**热力图标准**:

- 🔴 **HIGH** (≥50): 核心模块/高耦合
- 🟡 **MED** (20-49): 中等复杂度
- 🟢 **LOW** (<20): 简单函数

### 4.2.2 code_search - 精确符号定位器

**用途**: 定位函数/类定义，0 幻觉的精确定位。

**参数**:
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `query` | 搜索关键词（支持模糊匹配） | 必填 |
| `scope` | 限定搜索范围 | 整个项目 |
| `search_type` | 符号类型（`any`/`function`/`class`） | `any` |

**搜索策略**：AST 精准搜索 → AST 候选搜索 → Ripgrep 文本搜索 + Deep Context 反查

### 4.2.3 code_impact - 影响上下文分析器

**用途**: **修改前必做**，分析影响范围，避免改一漏十。

**参数**:
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `symbol_name` | 要分析的符号名（必填） | - |
| `direction` | 分析方向（`backward`/`forward`/`both`） | `both` |

**核心特性**:
- **多层级传播**：展示直接调用者和间接调用者的完整调用链
- **智能折叠**：间接调用者仅显示 Top 20，防止 Token 爆炸
- **可执行清单**：生成带行号的修改检查清单，比自行阅读源代码节省 90% Token

---

## 4.3 技术实现：AST 解析引擎

代码解析器的核心能力由 **Rust AST Indexer** 提供，Go 版本通过调用外部可执行文件使用其功能。

### 核心架构

```
┌─────────────────────────────────────────────────────────────┐
│                    Rust AST Indexer                         │
├─────────────────────────────────────────────────────────────┤
│  [索引模式] → [查询模式] → [地图模式] → [分析模式]         │
│       ↓           ↓           ↓           ↓                │
│  增量索引    渐进式搜索    结构扫描    DICE复杂度            │
│  SHA256对比   5层降级     Scope过滤   影响分析              │
└─────────────────────────────────────────────────────────────┘
                            ↓
                    Go MCP Server (调用)
```

### 1. 索引模式 - 被动增量更新

**增量策略**: SHA256 哈希对比，只重解析变更文件。工具调用时自动刷新（`code_search`、`code_impact`、`project_map`）。

**核心流程**: 文件发现 → 哈希对比 → 并行解析变更文件 → SQLite WAL 写入

### 2. 查询模式 - 渐进式容错搜索

**5层降级**：精确匹配 → 前缀匹配 → 后缀匹配 → 编辑距离 → 词根匹配

### 3. 地图模式 - Scope-Aware 结构扫描

根据 scope 参数自适应过滤，支持全项目扫描或指定目录深度扫描。

### 4. 分析模式 - DICE 复杂度算法

**计算公式**：覆盖节点数 × 0.5 + 调用外出度 × 2.0 + 被调用入度 × 1.0

**复杂度等级**: Simple (0-20) | Medium (20-50) | High (50-80) | Extreme (80+)

**风险评级**: low (0-3) | medium (4-10) | high (>10)

---

## 4.4 多语言支持

| 语言                    | 扩展名          | Tree-sitter 语法                              |
| --------------------- | ------------ | ------------------------------------------- |
| Python                | .py          | `function_definition`, `class_definition`   |
| Go                    | .go          | `function_declaration`, `type_spec`         |
| Rust                  | .rs          | `function_item`, `struct_item`, `impl_item` |
| JavaScript/TypeScript | .js, .ts     | `function_declaration`, `class_declaration` |
| Java                  | .java        | `method_declaration`, `class_declaration`   |
| C/C++                 | .c, .cpp, .h | `function_definition`, `struct_specifier`   |

---

## 4.5 最佳实践

### 1. 修改代码标准流程

```
code_search → code_impact → read_file → edit_file
```

### 2. 大型项目战术

```
project_map → code_search(scope="目标目录") → code_impact
```

### 3. 容错搜索

支持拼写错误容错：`createtask` → `create_task`、`loginhandler` → `LoginHandler`

---

## 4.6 下一步

深入了解代码解析器后，建议阅读：

- [第3章 Manager 调度核心](./03-MANAGER.md) - 了解解析工具如何被 Manager 调度
- [第5章 数据库与记忆层](./05-DATABASE-MEMORY.md) - 了解符号索引的存储机制

---

*本章完*
