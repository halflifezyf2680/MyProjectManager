package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"mcp-server-go/internal/core"
	"mcp-server-go/internal/services"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// InitArgs 初始化参数
type InitArgs struct {
	ProjectRoot string `json:"project_root" jsonschema:"description=项目根路径 (绝对路径)"`
}

// SessionManager 管理项目上下文（项目根路径与记忆层）
type SessionManager struct {
	Memory        *core.MemoryLayer
	ProjectRoot   string
	TaskChains    map[string]*TaskChain    // V1 版本（向后兼容）
	TaskChainsV2  map[string]*TaskChainV2  // V2 自适应版本
	Discovery     *services.DiscoveryService // 服务发现心跳
	AnalysisState map[string]*AnalysisState  // manager_analyze 两步调用的中间状态
}

// TaskChain 任务链状态（V1 版本，向后兼容）
type TaskChain struct {
	TaskID      string   `json:"task_id"`
	Plan        []string `json:"plan"`
	CurrentStep int      `json:"current_step"`
	Status      string   `json:"status"` // running, paused, finished
}

// AnalysisState 第一步分析结果（临时存储）
type AnalysisState struct {
	Intent         string                 `json:"intent"`
	UserDirective  string                 `json:"user_directive"`
	ContextAnchors []CodeAnchor           `json:"context_anchors"`
	VerifiedFacts  []string               `json:"verified_facts"`
	Telemetry      map[string]interface{} `json:"telemetry"`
	Guardrails     Guardrails             `json:"guardrails"`
	Alerts         []string               `json:"alerts"`
}

// CodeAnchor 代码锚点
type CodeAnchor struct {
	Symbol string `json:"symbol"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Type   string `json:"type"`
}

// Guardrails 约束规则
type Guardrails struct {
	Critical []string `json:"critical"`
	Advisory []string `json:"advisory"`
}

// SavePromptArgs 保存提示词参数
type SavePromptArgs struct {
	Title    string `json:"title" jsonschema:"required,description=提示词标题"`
	Content  string `json:"content" jsonschema:"required,description=提示词内容"`
	TagNames string `json:"tag_names" jsonschema:"description=标签名称（可选），用逗号分隔"`
	Scope    string `json:"scope" jsonschema:"default=project,enum=project,enum=global,description=保存范围"`
}

// SystemRecallArgs 历史召回参数
type SystemRecallArgs struct {
	Keywords string `json:"keywords" jsonschema:"required,description=检索关键词"`
	Category string `json:"category" jsonschema:"description=过滤类型 (开发/重构/避坑等)"`
	Scope    string `json:"scope" jsonschema:"default=project,description=范围"`
	Limit    int    `json:"limit" jsonschema:"default=20,description=返回条数"`
}

// RegisterSystemTools 注册系统工具
func RegisterSystemTools(s *server.MCPServer, sm *SessionManager, ai *services.ASTIndexer) {
	s.AddTool(mcp.NewTool("initialize_project",
		mcp.WithDescription(`initialize_project - 初始化项目环境与数据库

用途：
  任何其他 MPM 操作前，必须先调用此工具初始化项目环境。它会建立数据库索引、检测技术栈并生成项目规则。

参数：
  project_root (必填)
    项目根目录的绝对路径。如果留空，工具会尝试自动探测。

说明：
  - 手动指定 project_root 时必须使用绝对路径。
  - 初始化成功后，会生成 _MPM_PROJECT_RULES.md 供 LLM 参考。

示例：
  initialize_project(project_root="D:/AI_Project/MyProject")
    -> 初始化指定路径的项目

触发词：
  "mpm 初始化", "mpm init"`),
		mcp.WithInputSchema[InitArgs](),
	), wrapInit(sm, ai))

	s.AddTool(mcp.NewTool("open_timeline",
		mcp.WithDescription(`open_timeline - 项目演进可视化界面

用途：
  生成并展示交互式时间线，可视化项目的开发历史和决策演进。

参数：
  无

说明：
  - 基于 memo 记录生成 project_timeline.html。
  - 会尝试自动在默认浏览器中打开生成的文件。

示例：
  open_timeline()
    -> 在浏览器中打开项目演进时间线

触发词：
  "mpm 时间线", "mpm timeline"`),
	), wrapOpenTimeline(sm))

	s.AddTool(mcp.NewTool("open_hud",
		mcp.WithDescription(`open_hud - 项目实时监控悬浮窗 (Cockpit HUD)

用途：
  启动可视化监控界面，查看 MCP 服务状态、项目心跳及代码复杂度分布。

参数：
  无

说明：
  - 显示 Rust 编写的 Cockpit HUD 悬浮窗。
  - 支持多项目切换管理，并实时展示代码热力图。

示例：
  open_hud()
    -> 启动 HUD 监控界面

触发词：
  "mpm hud", "mpm 监控"`),
	), wrapOpenHUD(sm))

	// 注：save_prompt_from_context 已在 RegisterPromptTools 中注册,此处删除重复注册

	s.AddTool(mcp.NewTool("system_recall",
		mcp.WithDescription(`system_recall - 你的记忆回溯器 (少走弯路)

用途：
  【下手前推荐】想改某个功能，但不确定以前有没有类似的逻辑？或者怕踩到以前的坑？
  用此工具查一下记忆库，避免重复造轮子或重蹈覆辙。

参数策略：
  keywords (必填)
    想查什么就填什么，支持模糊匹配（空格拆分）。
  
  category (可选)
    缩小范围：如 "避坑" / "开发" / "决策"

触发词：
  "mpm 召回", "mpm 历史", "mpm recall"`),
		mcp.WithInputSchema[SystemRecallArgs](),
	), wrapSystemRecall(sm))
}

func wrapInit(sm *SessionManager, ai *services.ASTIndexer) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args InitArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数格式错误： %v", err)), nil
		}

		root := args.ProjectRoot

		// 1. 危险路径过滤：拒绝可能导致路径漂移的输入
		dangerousRoots := []string{"", ".", "..", "/", "\\", "./", ".\\"}
		for _, d := range dangerousRoots {
			if root == d {
				root = "" // 强制触发自动探测
				break
			}
		}

		if root == "" {
			// 自动探测
			root = core.DetectProjectRoot()
		}

		if root == "" {
			return mcp.NewToolResultText("❌ 无法自动识别项目路径，请手动指定 project_root（需为绝对路径）。"), nil
		}

		// 1. 路径统一化 (Path Normalization)
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("路径解析失败： %v", err)), nil
		}

		absRoot = filepath.ToSlash(filepath.Clean(absRoot))
		if len(absRoot) > 1 && absRoot[1] == ':' {
			drive := strings.ToUpper(string(absRoot[0]))
			absRoot = drive + absRoot[1:]
		}

		// 2. 校验路径安全性
		if !core.ValidateProjectPath(absRoot) {
			return mcp.NewToolResultError(fmt.Sprintf("⛔ 敏感路径（系统或 IDE 目录），禁止在此初始化项目： %s", absRoot)), nil
		}

		// 3. 确保 .mcp-data 存在
		mcpDataDir := filepath.Join(absRoot, ".mcp-data")
		if err := os.MkdirAll(mcpDataDir, 0755); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("创建数据目录失败： %v", err)), nil
		}

		// 4. 持久化项目配置
		configPath := filepath.Join(mcpDataDir, "project_config.json")
		configContent := fmt.Sprintf(`{
  "project_root": "%s",
  "initialized_at": "%s"
}`, absRoot, time.Now().Format(time.RFC3339))
		_ = os.WriteFile(configPath, []byte(configContent), 0644)

		// 5. 初始化记忆层
		mem, err := core.NewMemoryLayer(absRoot)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("初始化记忆层失败： %v", err)), nil
		}

		sm.Memory = mem
		sm.ProjectRoot = absRoot
		if sm.TaskChains == nil {
			sm.TaskChains = make(map[string]*TaskChain)
		}
		if sm.TaskChainsV2 == nil {
			sm.TaskChainsV2 = make(map[string]*TaskChainV2)
		}

		// 6. 🆕 【关键】刷新 AST 索引数据库
		// 确保 symbols.db 是最新的，否则所有代码工具都会查询到旧数据
		_, indexErr := ai.Index(absRoot)
		indexStatus := "✅"
		if indexErr != nil {
			indexStatus = fmt.Sprintf("⚠️ (索引失败: %v)", indexErr)
		}

		// 7. 启动服务发现心跳（向 HUD 注册自身）
		if sm.Discovery != nil {
			sm.Discovery.Stop() // 停止旧的心跳
		}
		sm.Discovery = services.NewDiscoveryService(absRoot)
		sm.Discovery.Start()

		// 8. 植入 visualize_history.py (Timeline 生成脚本)
		// 写入到项目根目录，如果不存在或强制更新（这里简化为覆盖）
		scriptPath := filepath.Join(absRoot, "visualize_history.py")
		if err := os.WriteFile(scriptPath, []byte(VisualizeHistoryScript), 0644); err != nil {
			// 记录警告但不阻断
			fmt.Printf("Warning: Failed to inject visualize_history.py: %v\n", err)
		}

		// 9. 规则生成 (_MPM_PROJECT_RULES.md)
		var rulesMsg string
		rulesPath := filepath.Join(absRoot, "_MPM_PROJECT_RULES.md")

		analysis, err := ai.AnalyzeNamingStyle(absRoot)
		if err == nil {
			if err := generateProjectRules(rulesPath, analysis); err == nil {
				rulesMsg = "\n\n[NEW] 已同步项目规则模板: _MPM_PROJECT_RULES.md\nIDE 将自动加载更新后的规则。"
			}
		}

		// 注：HUD 自动启动逻辑（带进程检查和配置检查）
		var hudMsg string
		hudMsgStr, hudErr := tryLaunchHUD(sm)
		if hudErr != nil {
			// HUD 启动失败不阻断初始化，只记录原因
			hudMsg = fmt.Sprintf("\n\n[HUD] %s", hudErr.Error())
		} else if hudMsgStr != "" {
			hudMsg = fmt.Sprintf("\n\n%s", hudMsgStr)
		} else {
			hudMsg = "\n\n[HUD] 启动逻辑返回空消息"
		}

		return mcp.NewToolResultText(fmt.Sprintf("✅ 项目初始化成功！\n\n项目目录: %s\n数据库已准备就绪。\nAST 索引: %s%s%s", absRoot, indexStatus, rulesMsg, hudMsg)), nil
	}
}

func generateProjectRules(path string, analysis *services.NamingAnalysis) error {
	mpmProtocol := `# MPM 强制协议

## 🚨 死规则 (违反即失败)

1. **复杂任务前** → 必须先 ` + "`manager_analyze`" + ` (主动填 Intent)，获取战术简报
2. **改代码前** → 必须先 ` + "`code_search`" + ` 或 ` + "`project_map`" + ` 定位，严禁凭记忆改
3. **预计任务很长** → 必须使用 ` + "`task_chain`" + ` 分步执行，禁止单次并发操作
4. **改代码后** → 必须立即 ` + "`memo`" + ` 记录
5. **准备改函数时** → 必须先 ` + "`code_impact`" + ` 分析谁在调用它
6. **code_search 失败** → 必须换词重试（同义词/缩写/驼峰变体），禁止放弃

---

## 🔧 工具使用时机

| 场景 | 必须使用的工具 |
|------|---------------|
| **任务复杂/模糊** | ` + "`manager_analyze`" + ` (必填 Intent) |
| **任务 > 2 步** | ` + "`task_chain`" + ` (防止搞砸) |
| 刚接手项目 / 宏观探索 | ` + "`project_map`" + ` |
| 找具体函数/类的定义 | ` + "`code_search`" + ` |
| 准备修改某函数 | ` + "`code_impact`" + ` |
| 代码改完了 | ` + "`memo`" + ` (SSOT) |

---

## 🚫 禁止

- 禁止凭记忆修改代码
- 禁止 code_search 失败后直接放弃
- 禁止修改代码后不调用 memo
- 禁止并发调用工具
`

	var namingRules string
	if analysis.IsNewProject {
		namingRules = fmt.Sprintf(`
# 项目命名规范 (由 MPM 自动分析生成)

> **检测到新项目** (文件数: %d)
> 这是您的新项目，请建立良好的命名习惯。推荐使用 Pythonic 风格。

## 推荐规范

- **函数/变量**: snake_case (e.g., get_user, total_count)
- **类名**: PascalCase (e.g., UserHandler, DataModel)
- **私有成员**: 使用 _ 前缀 (e.g., _internal_state)

---
`, analysis.FileCount)
	} else {
		funcExample := "`get_task`, `session_manager`"
		classExample := "`TaskContext`, `SessionManager`"
		if analysis.DominantStyle == "camelCase" {
			funcExample = "`getTask`, `sessionManager`"
		}

		prefixesStr := "无特殊前缀"
		if len(analysis.CommonPrefixes) > 0 {
			prefixesStr = strings.Join(analysis.CommonPrefixes, ", ")
		}

		samplesStr := strings.Join(analysis.SampleNames, ", ")

		namingRules = fmt.Sprintf(`
# 项目命名规范 (由 MPM 自动分析生成)

> **重要**: 此规范基于项目现有代码自动提取。LLM 必须严格遵守以确保风格一致。

## 检测结果

| 项目类型 | 旧项目 (检测到 %d 个源码文件，%d 个符号) |
|---------|------|
| **函数/变量风格** | %s (%s) |
| **类名风格** | %s |
| **常见前缀** | %s |

## 命名约定

-   **函数/变量**: 使用 %s，示例: %s
-   **类名**: 使用 %s，示例: %s
-   **禁止模糊修改**: 修改前必须用 code_search 确认目标唯一性。

## 代码示例 (从项目中提取)

%s

---

> **提示**: 如需修改规范，请直接编辑此文件。IDE 会自动读取更新后的内容。
`,
			analysis.FileCount,
			analysis.SymbolCount,
			analysis.DominantStyle,
			analysis.SnakeCasePct,
			analysis.ClassStyle,
			prefixesStr,
			analysis.DominantStyle,
			funcExample,
			analysis.ClassStyle,
			classExample,
			samplesStr,
		)
	}

	content := mpmProtocol + "\n" + namingRules
	return os.WriteFile(path, []byte(content), 0644)
}

func wrapOpenTimeline(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		root := sm.ProjectRoot
		if root == "" {
			return mcp.NewToolResultError("❌ 项目未初始化，请先调用 initialize_project"), nil
		}

		// 1. 定位脚本 (优先 scripts/, 其次 root)
		scriptPath := filepath.Join(root, "scripts", "visualize_history.py")
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			scriptPath = filepath.Join(root, "visualize_history.py")
			if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
				return mcp.NewToolResultError(fmt.Sprintf("❌ 找不到生成脚本: %s (checked scripts/ and root)", "visualize_history.py")), nil
			}
		}

		// 2. 生成 HTML (Python)
		cmd := exec.Command("python", scriptPath)
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("❌ 生成 Timeline 失败:\n%s\nOutput: %s", err, string(output))), nil
		}

		// 3. 定位 HTML
		htmlPath := filepath.Join(root, "project_timeline.html")
		if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
			return mcp.NewToolResultError("❌ 脚本执行成功但未生成 project_timeline.html"), nil
		}

		// 4. 打开浏览器
		htmlURL := "file:///" + filepath.ToSlash(htmlPath)
		edgeCmd := exec.Command("cmd", "/c", "start", "msedge", fmt.Sprintf("--app=%s", htmlURL))
		if err := edgeCmd.Start(); err != nil {
			fallbackCmd := exec.Command("cmd", "/c", "start", htmlURL)
			if err := fallbackCmd.Start(); err != nil {
				return mcp.NewToolResultText(fmt.Sprintf("⚠️ Timeline 已生成但无法自动打开。\n路径: %s", htmlPath)), nil
			}
		}

		return mcp.NewToolResultText(fmt.Sprintf("✅ Timeline 已生成并尝试打开。\n文件: %s", htmlPath)), nil
	}
}

// tryLaunchHUD 尝试启动 HUD（带进程检查和配置检查）
// 返回: (成功消息, 错误)
func tryLaunchHUD(sm *SessionManager) (string, error) {
	// 1. 检查用户配置
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法获取用户目录: %v", err)
	}
	configPath := filepath.Join(home, ".mcp-cockpit", "config.json")
	shouldOpen := true // 默认开启
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg struct {
			AutoOpen bool `json:"auto_open"`
		}
		if json.Unmarshal(data, &cfg) == nil {
			shouldOpen = cfg.AutoOpen
		}
	}

	if !shouldOpen {
		return "", fmt.Errorf("HUD 自动启动已禁用 (config.auto_open=false)")
	}

	// 2. 改进的进程检查（更可靠的方法）
	// 检查 HUD 可执行文件是否被锁定（运行中的文件无法被删除/重写）
	projectRoot := sm.ProjectRoot
	cwd, _ := os.Getwd()
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	candidates := []string{
		filepath.Join(projectRoot, "mcp-server-go", "bin", "mcp-cockpit-hud.exe"),
		filepath.Join(projectRoot, "bin", "mcp-cockpit-hud.exe"),
		filepath.Join(exeDir, "bin", "mcp-cockpit-hud.exe"),
		filepath.Join(exeDir, "mcp-cockpit-hud.exe"),
		filepath.Join(cwd, "bin", "mcp-cockpit-hud.exe"),
		filepath.Join(cwd, "mcp-cockpit-hud.exe"),
	}

	var hudPath string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			hudPath = c
			break
		}
	}

	if hudPath == "" {
		return "", fmt.Errorf("HUD 可执行文件未找到")
	}

	// 3. 检查 HUD 进程是否已在运行（使用 tasklist + 输出解析）
	checkCmd := exec.Command("tasklist", "/FI", "IMAGENAME eq mcp-cockpit-hud.exe", "/NH", "/FO", "CSV")
	output, err := checkCmd.Output()
	if err == nil {
		// tasklist CSV 格式输出: "mcp-cockpit-hud.exe","12345","Console","1","150,000 K"
		outputStr := string(output)
		// 检查是否包含有效的进程行（不是 "INFO: No tasks..."）
		if strings.Contains(outputStr, "mcp-cockpit-hud.exe") && !strings.Contains(outputStr, "INFO:") {
			return "", fmt.Errorf("HUD 已在运行，跳过启动")
		}
	}

	// 4. 启动 HUD
	cmd := exec.Command("cmd", "/c", "start", "", hudPath)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("启动 HUD 失败: %v", err)
	}

	return fmt.Sprintf("✅ HUD 已自动启动\n路径: %s", hudPath), nil
}

func wrapOpenHUD(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 获取基准路径
		projectRoot := sm.ProjectRoot
		cwd, _ := os.Getwd()
		exePath, _ := os.Executable()
		exeDir := filepath.Dir(exePath)

		// 候选路径列表（按优先级排序）
		candidates := []string{
			// 项目根目录下
			filepath.Join(projectRoot, "mcp-server-go", "bin", "mcp-cockpit-hud.exe"),
			filepath.Join(projectRoot, "bin", "mcp-cockpit-hud.exe"),
			// Go 服务的 bin 目录
			filepath.Join(exeDir, "bin", "mcp-cockpit-hud.exe"),
			filepath.Join(exeDir, "mcp-cockpit-hud.exe"),
			// 当前工作目录
			filepath.Join(cwd, "bin", "mcp-cockpit-hud.exe"),
			filepath.Join(cwd, "mcp-cockpit-hud.exe"),
			// Python 服务的副本
			filepath.Join(projectRoot, "mcp-expert-server", "src", "services", "bin", "mcp-cockpit-hud.exe"),
			filepath.Join(projectRoot, "mcp-expert-server", "src", "services", "cockpit_hud", "target", "release", "mcp-cockpit-hud.exe"),
		}

		var hudPath string
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				hudPath = c
				break
			}
		}

		if hudPath == "" {
			return mcp.NewToolResultError(fmt.Sprintf("❌ HUD 可执行文件未找到 (mcp-cockpit-hud.exe)\n\n已尝试路径:\n- %s", strings.Join(candidates[:4], "\n- "))), nil
		}

		cmd := exec.Command("cmd", "/c", "start", "", hudPath)
		if err := cmd.Start(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("启动 HUD 失败: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("✅ Rust HUD 已在后台启动\n路径: %s", hudPath)), nil
	}
}

func wrapSavePromptFromContext(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args SavePromptArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数错误: %v", err)), nil
		}

		if args.Title == "" || args.Content == "" {
			return mcp.NewToolResultError("标题和内容不能为空"), nil
		}

		// 确定数据库路径
		var dbPath string

		if args.Scope == "global" {
			ex, err := os.Executable()
			if err != nil {
				ex, _ = os.Getwd()
			}
			dbPath = filepath.Join(filepath.Dir(ex), ".mcp-data", "prompt_snippets.db")
		} else {
			if sm.ProjectRoot == "" {
				return mcp.NewToolResultError("项目未初始化，无法保存到 project scope。请先调用 initialize_project"), nil
			}
			dbPath = filepath.Join(sm.ProjectRoot, ".mcp-data", "mcp_memory.db")
		}

		// 确保目录存在
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("无法创建数据库目录: %v", err)), nil
		}

		// 连接数据库
		db, err := core.NewDatabaseManager(dbPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("数据库连接失败: %v", err)), nil
		}
		defer db.Close()

		// 初始化 Schema
		statements := []string{
			`CREATE TABLE IF NOT EXISTS prompt_snippets (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				title TEXT NOT NULL,
				content TEXT NOT NULL,
				is_favorite INTEGER DEFAULT 0,
				use_count INTEGER DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS snippet_tags (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT UNIQUE NOT NULL,
				color TEXT DEFAULT '#4285F4',
				use_count INTEGER DEFAULT 0
			)`,
			`CREATE TABLE IF NOT EXISTS snippet_tag_relations (
				snippet_id INTEGER,
				tag_id INTEGER,
				PRIMARY KEY (snippet_id, tag_id),
				FOREIGN KEY (snippet_id) REFERENCES prompt_snippets(id) ON DELETE CASCADE,
				FOREIGN KEY (tag_id) REFERENCES snippet_tags(id) ON DELETE CASCADE
			)`,
		}

		for _, stmt := range statements {
			if _, err := db.Exec(stmt); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Schema 初始化失败: %v", err)), nil
			}
		}

		// 插入 Prompt
		res, err := db.Exec("INSERT INTO prompt_snippets (title, content, is_favorite) VALUES (?, ?, 0)", args.Title, args.Content)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("保存提示词失败: %v", err)), nil
		}
		snippetID, _ := res.LastInsertId()

		// 处理标签
		if args.TagNames != "" {
			tags := strings.Split(args.TagNames, ",")
			for _, tag := range tags {
				tagName := strings.TrimSpace(tag)
				if tagName == "" {
					continue
				}

				colors := []string{"#E53935", "#1976D2", "#43A047", "#FBC02D", "#8E24AA"}
				randColor := colors[int(time.Now().UnixNano())%len(colors)]

				// 插入标签
				db.Exec("INSERT OR IGNORE INTO snippet_tags (name, color) VALUES (?, ?)", tagName, randColor)

				// 获取 Tag ID
				var tagID int64
				row := db.QueryRow("SELECT id FROM snippet_tags WHERE name = ?", tagName)
				if err := row.Scan(&tagID); err != nil {
					continue
				}

				// 插入关联
				db.Exec("INSERT OR IGNORE INTO snippet_tag_relations (snippet_id, tag_id) VALUES (?, ?)", snippetID, tagID)

				// 更新计数
				db.Exec("UPDATE snippet_tags SET use_count = use_count + 1 WHERE id = ?", tagID)
			}
		}

		return mcp.NewToolResultText(fmt.Sprintf("✅ 提示词已保存到 %s 库 (ID: %d)\n\nTitle: %s\nTags: %s", strings.ToUpper(args.Scope), snippetID, args.Title, args.TagNames)), nil
	}
}

func wrapSystemRecall(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args SystemRecallArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数错误: %v", err)), nil
		}

		if sm.ProjectRoot == "" {
			return mcp.NewToolResultError("项目未初始化"), nil
		}

		// 1. 查询 Memos（历史修改记录）
		memos, err := sm.Memory.SearchMemos(ctx, args.Keywords, args.Category, args.Limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("检索 memos 失败: %v", err)), nil
		}

		// 2. 查询 Known Facts（铁律/避坑经验）
		facts, err := sm.Memory.QueryFacts(ctx, args.Keywords, args.Limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("检索 known_facts 失败: %v", err)), nil
		}

		// 3. 检查是否有结果
		if len(memos) == 0 && len(facts) == 0 {
			return mcp.NewToolResultText("未找到相关记录"), nil
		}

		// 4. 构建返回结果
		var sb strings.Builder

		// 输出 Known Facts
		if len(facts) > 0 {
			sb.WriteString(fmt.Sprintf("## 📌 Known Facts (%d)\n\n", len(facts)))
			for _, f := range facts {
				sb.WriteString(fmt.Sprintf("- **[%s]** %s _(ID: %d, %s)_\n",
					f.Type,
					f.Summarize,
					f.ID,
					f.CreatedAt.Format("2006-01-02")))
			}
			sb.WriteString("\n")
		}

		// 输出 Memos
		if len(memos) > 0 {
			sb.WriteString(fmt.Sprintf("## 📝 Memos (%d)\n\n", len(memos)))
			for _, m := range memos {
				sb.WriteString(fmt.Sprintf("- **[%d] %s** (%s) %s: %s\n",
					m.ID,
					m.Timestamp.Format("2006-01-02 15:04"),
					m.Category,
					m.Act,
					m.Content))
			}
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}
