package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

// ============================================================================
// 数据结构 - 与 Rust ast_indexer 输出格式匹配
// ============================================================================

// Node 符号节点
type Node struct {
	ID            string   `json:"id"`
	NodeType      string   `json:"type"`
	Name          string   `json:"name"`
	QualifiedName string   `json:"qualified_name"`
	FilePath      string   `json:"file_path"`
	LineStart     int      `json:"line_start"`
	LineEnd       int      `json:"line_end"`
	Signature     string   `json:"signature,omitempty"`
	Calls         []string `json:"calls,omitempty"`
}

// Stats 统计信息
type Stats struct {
	TotalFiles   int `json:"total_files"`
	TotalSymbols int `json:"total_symbols"`
}

// MapResult 项目地图结果 (--mode map)
type MapResult struct {
	Statistics    Stats              `json:"statistics"`
	Structure     map[string][]Node  `json:"structure"`
	Elapsed       string             `json:"elapsed"`
	ComplexityMap map[string]float64 `json:"complexity_map,omitempty"` // 符号名 -> 复杂度分数
}

// CandidateMatch 候选匹配
type CandidateMatch struct {
	Node      Node    `json:"node"`
	MatchType string  `json:"match_type"`
	Score     float32 `json:"score"`
}

// CallerInfo 调用者信息
type CallerInfo struct {
	Node     Node   `json:"node"`
	CallType string `json:"call_type"`
}

// QueryResult 查询结果 (--mode query)
type QueryResult struct {
	Status       string           `json:"status"`
	Query        string           `json:"query"`
	FoundSymbol  *Node            `json:"found_symbol"`
	MatchType    string           `json:"match_type,omitempty"`
	Candidates   []CandidateMatch `json:"candidates"`
	RelatedNodes []CallerInfo     `json:"related_nodes"`
}

// ImpactResult 影响分析结果 (--mode analyze)
type ImpactResult struct {
	Status                string       `json:"status"`
	NodeID                string       `json:"node_id"`
	ComplexityScore       float64      `json:"complexity_score"`
	ComplexityLevel       string       `json:"complexity_level"`
	RiskLevel             string       `json:"risk_level"`
	AffectedNodes         int          `json:"affected_nodes"`
	DirectCallers         []CallerInfo `json:"direct_callers"`
	IndirectCallers       []CallerInfo `json:"indirect_callers"`
	ModificationChecklist []string     `json:"modification_checklist"`
	Message               string       `json:"message,omitempty"`
}

// IndexResult 索引结果 (--mode index)
type IndexResult struct {
	Status     string `json:"status"`
	TotalFiles int    `json:"total_files"`
	ElapsedMs  int64  `json:"elapsed_ms"`
}

// NamingAnalysis 命名风格分析结果
type NamingAnalysis struct {
	FileCount      int      `json:"file_count"`
	SymbolCount    int      `json:"symbol_count"`
	DominantStyle  string   `json:"dominant_style"` // snake_case / camelCase / mixed
	SnakeCasePct   string   `json:"snake_case_pct"`
	CamelCasePct   string   `json:"camel_case_pct"`
	ClassStyle     string   `json:"class_style"` // PascalCase
	CommonPrefixes []string `json:"common_prefixes"`
	SampleNames    []string `json:"sample_names"` // 样例 函数名
	IsNewProject   bool     `json:"is_new_project"`
}

// ============================================================================
// ASTIndexer 核心服务
// ============================================================================

// ASTIndexer AST 索引器服务
type ASTIndexer struct {
	BinaryPath string
}

// NewASTIndexer 创建 AST 索引器
func NewASTIndexer() *ASTIndexer {
	exeName := "ast_indexer.exe"
	if runtime.GOOS != "windows" {
		exeName = "ast_indexer"
	}

	// 获取当前可执行文件所在目录
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		// 尝试在同级 bin 目录查找
		binPath := filepath.Join(execDir, "bin", exeName)
		if fileExists(binPath) {
			return &ASTIndexer{BinaryPath: binPath}
		}
		// 尝试同级目录
		sameDirPath := filepath.Join(execDir, exeName)
		if fileExists(sameDirPath) {
			return &ASTIndexer{BinaryPath: sameDirPath}
		}
	}

	// 兜底：尝试相对路径
	paths := []string{
		filepath.Join("bin", exeName),
		filepath.Join("mcp-server-go", "bin", exeName),
	}

	for _, p := range paths {
		abs, _ := filepath.Abs(p)
		if fileExists(abs) {
			return &ASTIndexer{BinaryPath: abs}
		}
	}

	return &ASTIndexer{BinaryPath: exeName}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// getDBPath 获取数据库路径
func getDBPath(projectRoot string) string {
	// 【修复】确保返回绝对路径,防止Rust引擎将文件写到错误位置
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		// 如果转换失败,使用原路径(但可能有风险)
		absRoot = projectRoot
	}
	return filepath.Join(absRoot, ".mcp-data", "symbols.db")
}

// getOutputPath 获取临时输出路径
func getOutputPath(projectRoot string, mode string) string {
	// 【修复】确保返回绝对路径,防止缓存文件跑到C盘
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		// 如果转换失败,使用原路径(但可能有风险)
		absRoot = projectRoot
	}
	mcpData := filepath.Join(absRoot, ".mcp-data")
	_ = os.MkdirAll(mcpData, 0755)
	return filepath.Join(mcpData, fmt.Sprintf(".ast_result_%s.json", mode))
}

// ============================================================================
// 技术栈检测与过滤配置 (移植自 ast_indexer_helper.py)
// ============================================================================

// detectTechStackAndConfig 智能检测技术栈，返回(允许的扩展名, 忽略的目录)
func detectTechStackAndConfig(projectRoot string) (extensions string, ignoreDirs string) {
	var stackDetected []string
	var exts []string

	// 基础忽略目录
	ignores := []string{
		".git", "__pycache__", "node_modules", ".venv", "venv",
		"dist", "build", ".idea", ".vscode",
		"release", "releases", "archive", "backup", "old",
	}

	// 从 .gitignore 解析额外的忽略目录
	gitignoreDirs := parseGitignoreDirs(projectRoot)
	ignores = append(ignores, gitignoreDirs...)

	// 1. 检测 Python
	if fileExists(filepath.Join(projectRoot, "requirements.txt")) ||
		fileExists(filepath.Join(projectRoot, "pyproject.toml")) ||
		hasFilesWithExt(projectRoot, ".py") {
		stackDetected = append(stackDetected, "python")
		exts = append(exts, ".py")
		ignores = append(ignores, "site-packages", "htmlcov", ".pytest_cache")
	}

	// 2. 检测 Frontend (Node/React/Vue)
	if fileExists(filepath.Join(projectRoot, "package.json")) {
		stackDetected = append(stackDetected, "frontend")
		exts = append(exts, ".js", ".jsx", ".ts", ".tsx", ".vue", ".svelte", ".css", ".html")
		ignores = append(ignores, "coverage", ".next", ".nuxt", "out")
	}

	// 3. 检测 Go
	if fileExists(filepath.Join(projectRoot, "go.mod")) {
		stackDetected = append(stackDetected, "go")
		exts = append(exts, ".go")
		ignores = append(ignores, "vendor", "bin", "pkg")
	}

	// 4. 检测 Rust (递归搜索)
	if hasRustProject(projectRoot) {
		stackDetected = append(stackDetected, "rust")
		exts = append(exts, ".rs")
		ignores = append(ignores, "target")
	}

	// 5. 检测 C/C++
	if hasFilesWithExt(projectRoot, ".c") || hasFilesWithExt(projectRoot, ".cpp") ||
		hasFilesWithExt(projectRoot, ".h") || fileExists(filepath.Join(projectRoot, "CMakeLists.txt")) {
		stackDetected = append(stackDetected, "cpp")
		exts = append(exts, ".c", ".h", ".cpp", ".hpp", ".cc")
		ignores = append(ignores, "cmake-build-debug", "obj")
	}

	// 6. 检测 Java
	if hasFilesWithExt(projectRoot, ".java") || fileExists(filepath.Join(projectRoot, "pom.xml")) ||
		fileExists(filepath.Join(projectRoot, "build.gradle")) {
		stackDetected = append(stackDetected, "java")
		exts = append(exts, ".java")
		ignores = append(ignores, ".gradle")
	}

	// 如果没有检测到特定栈，不限制扩展名
	if len(stackDetected) == 0 {
		return "", uniqueJoin(ignores)
	}

	return uniqueJoin(exts), uniqueJoin(ignores)
}

// parseGitignoreDirs 解析 .gitignore 文件，提取目录忽略规则
func parseGitignoreDirs(projectRoot string) []string {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		return nil
	}

	var ignoredDirs []string
	fileExtensions := map[string]bool{
		"txt": true, "md": true, "json": true, "yml": true, "yaml": true,
		"toml": true, "lock": true, "log": true, "py": true, "js": true,
		"ts": true, "rs": true, "go": true, "java": true, "c": true,
		"cpp": true, "h": true, "hpp": true, "sql": true, "db": true,
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过注释和空行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 跳过否定规则
		if strings.HasPrefix(line, "!") {
			continue
		}

		// 优先级 1: 以 / 结尾的明确目录
		if strings.HasSuffix(line, "/") {
			dirName := strings.TrimSuffix(line, "/")
			dirName = strings.TrimPrefix(dirName, "/")
			dirName = strings.ReplaceAll(dirName, "**/", "")
			if dirName != "" && !strings.HasPrefix(dirName, "**") {
				ignoredDirs = append(ignoredDirs, dirName)
			}
			continue
		}

		// 优先级 2: 包含 / 的路径模式
		if strings.Contains(line, "/") {
			parts := strings.Split(line, "/")
			pathPart := strings.ReplaceAll(parts[len(parts)-1], "*", "")
			if pathPart != "" && !strings.HasPrefix(pathPart, "**") {
				// 检查是否是纯文件名（包含扩展名）
				if strings.Contains(pathPart, ".") {
					extParts := strings.Split(pathPart, ".")
					ext := strings.ToLower(extParts[len(extParts)-1])
					if fileExtensions[ext] {
						continue
					}
				}
				ignoredDirs = append(ignoredDirs, pathPart)
			}
		}
	}

	return ignoredDirs
}

// hasFilesWithExt 检查目录下是否有指定扩展名的文件
func hasFilesWithExt(dir string, ext string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			return true
		}
	}
	return false
}

// hasRustProject 递归检查是否有 Rust 项目
func hasRustProject(projectRoot string) bool {
	if fileExists(filepath.Join(projectRoot, "Cargo.toml")) {
		return true
	}
	// 递归搜索子目录（最多6层）
	return hasCargoTomlRecursive(projectRoot, 0, 6)
}

func hasCargoTomlRecursive(dir string, depth, maxDepth int) bool {
	if depth >= maxDepth {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			subdir := filepath.Join(dir, e.Name())
			if fileExists(filepath.Join(subdir, "Cargo.toml")) {
				return true
			}
			if hasCargoTomlRecursive(subdir, depth+1, maxDepth) {
				return true
			}
		}
	}
	return false
}

// uniqueJoin 去重并用逗号连接
func uniqueJoin(items []string) string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		item = strings.TrimPrefix(item, ".")
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return strings.Join(result, ",")
}

// ============================================================================
// 核心方法

// ============================================================================

// MapProject 绘制项目地图 (--mode map)
func (ai *ASTIndexer) MapProject(projectRoot string, detail string) (*MapResult, error) {
	return ai.MapProjectWithScope(projectRoot, detail, "")
}

// MapProjectWithScope 带范围的项目地图
func (ai *ASTIndexer) MapProjectWithScope(projectRoot string, detail string, scope string) (*MapResult, error) {
	dbPath := getDBPath(projectRoot)
	outputPath := getOutputPath(projectRoot, "map")

	// 清理旧文件
	_ = os.Remove(outputPath)

	// 智能技术栈检测
	_, ignoreDirs := detectTechStackAndConfig(projectRoot)

	// 如果 scope 是 "." 或 "./"，清理掉，让 Rust 引擎执行全量扫描
	if scope == "." || scope == "./" {
		scope = ""
	}

	args := []string{
		"--mode", "map",
		"--project", projectRoot,
		"--db", dbPath,
		"--output", outputPath,
		"--detail", detail,
	}
	if scope != "" {
		args = append(args, "--scope", scope)
	}
	// 允许 Rust 引擎自动探测所有语言，除非明确指定（暂不自动限定）
	// if exts != "" {
	// 	args = append(args, "--extensions", exts)
	// }
	if ignoreDirs != "" {
		args = append(args, "--ignore-dirs", ignoreDirs)
	}

	cmd := exec.Command(ai.BinaryPath, args...)
	cmd.Dir = projectRoot // 设置工作目录

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("项目地图生成失败: %v", err)
	}

	// 读取输出文件
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("读取地图结果失败: %v", err)
	}

	var result MapResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析地图结果失败: %v", err)
	}

	return &result, nil
}

// SearchSymbol 搜索符号 (--mode query)
func (ai *ASTIndexer) SearchSymbol(projectRoot string, query string) (*QueryResult, error) {
	return ai.SearchSymbolWithScope(projectRoot, query, "")
}

// SearchSymbolWithScope 带范围的符号搜索
func (ai *ASTIndexer) SearchSymbolWithScope(projectRoot string, query string, scope string) (*QueryResult, error) {
	dbPath := getDBPath(projectRoot)
	outputPath := getOutputPath(projectRoot, "query")

	// 清理旧文件
	_ = os.Remove(outputPath)

	args := []string{
		"--mode", "query",
		"--project", projectRoot,
		"--db", dbPath,
		"--output", outputPath,
		"--query", query,
	}
	if scope != "" {
		args = append(args, "--scope", scope)
	}

	cmd := exec.Command(ai.BinaryPath, args...)
	cmd.Dir = projectRoot

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("符号搜索失败: %v", err)
	}

	// 读取输出文件
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("读取搜索结果失败: %v", err)
	}

	var result QueryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %v", err)
	}

	return &result, nil
}

// GetSymbolAtLine 获取指定文件行号处的符号信息 (--mode query --file --line)
func (ai *ASTIndexer) GetSymbolAtLine(projectRoot string, filePath string, line int) (*Node, error) {
	dbPath := getDBPath(projectRoot)
	outputPath := getOutputPath(projectRoot, fmt.Sprintf("line_%d", line))

	// 清理所有旧的 line_*.json 临时文件（避免泄漏）
	mcpData := filepath.Join(projectRoot, ".mcp-data")
	if entries, err := os.ReadDir(mcpData); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasPrefix(e.Name(), ".ast_result_line_") && strings.HasSuffix(e.Name(), ".json") {
				_ = os.Remove(filepath.Join(mcpData, e.Name()))
			}
		}
	}

	// 清理当前文件
	_ = os.Remove(outputPath)

	args := []string{
		"--mode", "query",
		"--project", projectRoot,
		"--db", dbPath,
		"--output", outputPath,
		"--file", filePath,
		"--line", fmt.Sprintf("%d", line),
	}

	cmd := exec.Command(ai.BinaryPath, args...)
	cmd.Dir = projectRoot

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("定位符号失败: %v", err)
	}

	// 读取输出文件
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("读取定位结果失败: %v", err)
	}

	var result QueryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析定位结果失败: %v", err)
	}

	return result.FoundSymbol, nil
}

// Analyze 执行影响分析 (--mode analyze)
func (ai *ASTIndexer) Analyze(projectRoot string, symbol string, direction string) (*ImpactResult, error) {
	// 先确保索引是最新的
	_, _ = ai.Index(projectRoot)

	dbPath := getDBPath(projectRoot)
	outputPath := getOutputPath(projectRoot, "analyze")

	// 清理旧文件
	_ = os.Remove(outputPath)

	args := []string{
		"--mode", "analyze",
		"--project", projectRoot,
		"--db", dbPath,
		"--output", outputPath,
		"--query", symbol,
	}
	if direction != "" {
		args = append(args, "--direction", direction)
	}

	cmd := exec.Command(ai.BinaryPath, args...)
	cmd.Dir = projectRoot

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("影响分析执行失败: %v", err)
	}

	// 读取输出文件
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("读取分析结果失败: %v", err)
	}

	var result ImpactResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析分析结果失败: %v", err)
	}

	return &result, nil
}

// Index 刷新索引 (--mode index)
func (ai *ASTIndexer) Index(projectRoot string) (*IndexResult, error) {
	dbPath := getDBPath(projectRoot)
	outputPath := getOutputPath(projectRoot, "index")

	// 确保 .mcp-data 目录存在
	mcpData := filepath.Join(projectRoot, ".mcp-data")
	_ = os.MkdirAll(mcpData, 0755)
	// 清理旧文件
	_ = os.Remove(outputPath)

	// 智能技术栈检测
	extensions, ignoreDirs := detectTechStackAndConfig(projectRoot)

	args := []string{
		"--mode", "index",
		"--project", projectRoot,
		"--db", dbPath,
		"--output", outputPath,
	}
	// 🆕 传递扩展名，避免 Rust 引擎触发 TypeScript Query Bug
	if extensions != "" {
		args = append(args, "--extensions", extensions)
	}
	if ignoreDirs != "" {
		args = append(args, "--ignore-dirs", ignoreDirs)
	}

	cmd := exec.Command(ai.BinaryPath, args...)
	cmd.Dir = projectRoot

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("索引刷新失败: %v", err)
	}

	// 读取输出文件
	data, err := os.ReadFile(outputPath)
	if err != nil {
		// 索引可能不输出文件，返回默认结果
		return &IndexResult{Status: "success"}, nil
	}

	var result IndexResult
	if err := json.Unmarshal(data, &result); err != nil {
		return &IndexResult{Status: "success"}, nil
	}

	return &result, nil
}

// AnalyzeNamingStyle 分析项目命名风格
func (ai *ASTIndexer) AnalyzeNamingStyle(projectRoot string) (*NamingAnalysis, error) {
	// 1. 确保索引存在 (且尝试刷新)
	if _, err := ai.Index(projectRoot); err != nil {
		// 如果索引失败，尝试直接读取现有数据库
		// 什么也不做
	}

	dbPath := getDBPath(projectRoot)
	if !fileExists(dbPath) {
		return &NamingAnalysis{IsNewProject: true}, nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %v", err)
	}
	defer db.Close()

	// 2. 统计文件数
	var fileCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM files").Scan(&fileCount); err != nil {
		// 可能表不存在
		return &NamingAnalysis{IsNewProject: true}, nil
	}

	if fileCount < 3 {
		return &NamingAnalysis{IsNewProject: true, FileCount: fileCount}, nil
	}

	// 3. 提取所有函数名
	rows, err := db.Query("SELECT name FROM symbols WHERE symbol_type IN ('function', 'method') LIMIT 1000")
	if err != nil {
		return nil, fmt.Errorf("查询符号失败: %v", err)
	}
	defer rows.Close()

	var funcNames []string
	var snakeCount, camelCount int
	// reSnake := regexp.MustCompile(`^[a-z0-9_]+$`) // Unused
	reCamel := regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)

	prefixCounts := make(map[string]int)

	for rows.Next() {
		var name string
		rows.Scan(&name)
		funcNames = append(funcNames, name)

		// 风格判定
		if strings.Contains(name, "_") && strings.ToLower(name) == name {
			snakeCount++
		} else if reCamel.MatchString(name) && !strings.Contains(name, "_") {
			camelCount++
		}

		// 前缀提取 (如 get_, set_, on_)
		parts := strings.Split(name, "_")
		if len(parts) > 1 {
			prefixCounts[parts[0]+"_"]++
		} else if strings.HasPrefix(name, "get") && len(name) > 3 && name[3] >= 'A' && name[3] <= 'Z' {
			prefixCounts["get"]++ // camelCase get
		}
	}

	// 4. 计算结果
	totalFuncs := len(funcNames)
	if totalFuncs == 0 {
		return &NamingAnalysis{IsNewProject: true, FileCount: fileCount}, nil
	}

	snakePct := float64(snakeCount) / float64(totalFuncs) * 100
	camelPct := float64(camelCount) / float64(totalFuncs) * 100

	style := "snake_case"
	if camelCount > snakeCount {
		style = "camelCase"
	} else if snakeCount == 0 && camelCount == 0 {
		style = "mixed"
	}

	// 提取Top前缀
	var prefixes []string
	for p, c := range prefixCounts {
		if c > max(2, totalFuncs/20) { // 至少出现2次且占比>5%
			prefixes = append(prefixes, p)
		}
	}
	// 简单取前5个作为展示
	if len(prefixes) > 5 {
		prefixes = prefixes[:5]
	}

	// 样例数据 (取前10个)
	var samples []string
	if totalFuncs > 10 {
		samples = funcNames[:10]
	} else {
		samples = funcNames
	}

	return &NamingAnalysis{
		FileCount:      fileCount,
		SymbolCount:    totalFuncs,
		DominantStyle:  style,
		SnakeCasePct:   fmt.Sprintf("%.1f%%", snakePct),
		CamelCasePct:   fmt.Sprintf("%.1f%%", camelPct),
		ClassStyle:     "PascalCase", // 默认假设
		CommonPrefixes: prefixes,
		SampleNames:    samples,
		IsNewProject:   false,
	}, nil
}

// RiskInfo 风险信息
type RiskInfo struct {
	SymbolName string  `json:"symbol_name"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
}

// ComplexityReport 复杂度报告
type ComplexityReport struct {
	HighRiskSymbols []RiskInfo `json:"high_risk_symbols"`
	TotalAnalyzed   int        `json:"total_analyzed"`
}

// AnalyzeComplexity 分析符号复杂度 (基于调用关系)
// 简单的中心度分析：Fan-out (出度) 高代表依赖复杂，Fan-in (入度) 高代表影响范围广/责任重
func (ai *ASTIndexer) AnalyzeComplexity(projectRoot string, symbolNames []string) (*ComplexityReport, error) {
	if len(symbolNames) == 0 {
		return &ComplexityReport{}, nil
	}

	dbPath := getDBPath(projectRoot)
	if !fileExists(dbPath) {
		return nil, nil // No DB, no analysis
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var report ComplexityReport
	report.TotalAnalyzed = len(symbolNames)

	for _, name := range symbolNames {
		// 1. 获取 Symbol ID (可能会有重名，这里简单处理取第一个，或汇总)
		rows, err := db.Query("SELECT symbol_id, symbol_type FROM symbols WHERE name = ?", name)
		if err != nil {
			continue
		}

		var ids []int
		for rows.Next() {
			var id int
			var sType string
			rows.Scan(&id, &sType)
			if sType == "function" || sType == "method" || sType == "class" {
				ids = append(ids, id)
			}
		}
		rows.Close()

		if len(ids) == 0 {
			continue
		}

		// 聚合所有同名符号的指标
		var maxFanIn, maxFanOut int

		for _, id := range ids {
			// Fan-out: 我调用了谁 (caller_id = id)
			var fanOut int
			db.QueryRow("SELECT COUNT(*) FROM calls WHERE caller_id = ?", id).Scan(&fanOut)
			if fanOut > maxFanOut {
				maxFanOut = fanOut
			}

			// Fan-in: 谁调用了我 (callee_name = name) -> 注意这里 callee_name 是字符串
			// 严格来说应该关联 caller_id 对应的 symbol，但这里 callee_name 已经是名字了
			// 另外 Rust 存的是 callee_name (被调用的函数名)，不是 ID，因为静态解析很难解析动态调用的 ID
			var fanIn int
			db.QueryRow("SELECT COUNT(*) FROM calls WHERE callee_name = ?", name).Scan(&fanIn)
			if fanIn > maxFanIn {
				maxFanIn = fanIn
			}
		}

		// 简单的评分模型
		// FanOut > 10 -> Complex Logic
		// FanIn > 20 -> High Impact Core
		score := float64(maxFanOut)*1.0 + float64(maxFanIn)*0.5

		var reasons []string
		if maxFanOut > 10 {
			reasons = append(reasons, fmt.Sprintf("High Coupling (Calls: %d)", maxFanOut))
		}
		if maxFanIn > 20 {
			reasons = append(reasons, fmt.Sprintf("Core Module (Ref by: %d)", maxFanIn))
		}

		// 🆕 始终添加到报告，即使复杂度很低
		report.HighRiskSymbols = append(report.HighRiskSymbols, RiskInfo{
			SymbolName: name,
			Score:      score,
			Reason:     strings.Join(reasons, ", "),
		})
	}

	return &report, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
