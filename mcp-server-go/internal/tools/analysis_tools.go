package tools

import (
	"context"
	"fmt"
	"mcp-server-go/internal/services"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ImpactArgs 影响分析参数
type ImpactArgs struct {
	SymbolName string `json:"symbol_name" jsonschema:"required,description=要分析的符号名 (函数名或类名)"`
	Direction  string `json:"direction" jsonschema:"default=backward,enum=backward,enum=forward,enum=both,description=分析方向"`
}

// ProjectMapArgs 项目地图参数
type ProjectMapArgs struct {
	Scope     string `json:"scope" jsonschema:"description=限定范围 (目录或文件路径，留空=整个项目)"`
	Level     string `json:"level" jsonschema:"default=symbols,enum=structure,enum=symbols,description=视图层级"`
	CorePaths string `json:"core_paths" jsonschema:"description=核心目录列表 (JSON 数组字符串)"`
}

// RegisterAnalysisTools 注册分析类工具
func RegisterAnalysisTools(s *server.MCPServer, sm *SessionManager, ai *services.ASTIndexer) {
	s.AddTool(mcp.NewTool("code_impact",
		mcp.WithDescription(`code_impact - 代码修改影响分析

用途：
  分析修改函数或类时的影响范围，识别需要同步修改的位置

参数：
  symbol_name (必填)
    要分析的符号名（函数名或类名）
    注意：必须是精确的代码符号，不支持字符串搜索
  
  direction (默认: backward)
    - backward: 谁调用了我（影响上游）
    - forward: 我调用了谁（影响下游）
    - both: 双向分析

返回：
  - 风险等级（low/medium/high）
  - 直接调用者列表（前10个）
  - 间接调用者数量
  - 修改检查清单

示例：
  code_impact(symbol_name="Login", direction="backward")
    -> 分析谁在调用 Login 函数

触发词：
  "mpm 影响", "mpm 依赖", "mpm impact"`),
		mcp.WithInputSchema[ImpactArgs](),
	), wrapImpact(sm, ai))

	s.AddTool(mcp.NewTool("project_map",
		mcp.WithDescription(`project_map - 你的项目导航仪 (当不知道代码在哪时)

用途：
  【宏观视角】当你迷路了，或者不知道该改哪个文件时，用我。我会给你一张带导航的地图。

决策指南：
  level (默认: symbols)
    - 刚接手/想看架构？ -> "structure" (只看目录树，不看代码)
    - 找代码/准备修改？ -> "symbols" (列出更详细的函数/类)
  
  scope (可选)
    如果不填，默认看整个项目（可能会很长）。建议填入你感兴趣的目录。

返回：
  一张 ASCII 格式的项目地图 + 复杂度热力图。

触发词：
  "mpm 地图", "mpm 结构", "mpm map"`),
		mcp.WithInputSchema[ProjectMapArgs](),
	), wrapProjectMap(sm, ai))
}

func wrapImpact(sm *SessionManager, ai *services.ASTIndexer) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args ImpactArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数格式错误: %v", err)), nil
		}

		if sm.ProjectRoot == "" {
			return mcp.NewToolResultError("项目尚未初始化，请先执行 initialize_project。"), nil
		}

		// 默认方向
		if args.Direction == "" {
			args.Direction = "backward"
		}

		// 1. AST 静态分析 (硬调用)
		astResult, err := ai.Analyze(sm.ProjectRoot, args.SymbolName, args.Direction)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("AST 分析失败: %v", err)), nil
		}

		if astResult == nil || astResult.Status != "success" {
			errorMessage := fmt.Sprintf("⚠️ `%s` 不是代码函数/类定义。\n\n", args.SymbolName)
			errorMessage += "> 如果要搜索**字符串**，用 **Grep** 工具\n"
			errorMessage += "> 如果要查找**函数定义**，用 **code_search** 工具"
			return mcp.NewToolResultText(errorMessage), nil
		}

		// 2. 精简输出 (面向 LLM 决策)
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("## `%s` 影响分析\n\n", args.SymbolName))
		sb.WriteString(fmt.Sprintf("**风险**: %s | **复杂度**: %.0f | **影响节点**: %d\n\n",
			astResult.RiskLevel, astResult.ComplexityScore, astResult.AffectedNodes))

		// 直接调用者列表
		if len(astResult.DirectCallers) > 0 {
			sb.WriteString("### 直接调用者（修改前必须检查）\n")
			limit := 10
			if len(astResult.DirectCallers) < limit {
				limit = len(astResult.DirectCallers)
			}
			for i := 0; i < limit; i++ {
				c := astResult.DirectCallers[i]
				sb.WriteString(fmt.Sprintf("- `%s` @ %s:%d\n", c.Node.Name, c.Node.FilePath, c.Node.LineStart))
			}
			if len(astResult.DirectCallers) > limit {
				sb.WriteString(fmt.Sprintf("- ... 还有 %d 个\n", len(astResult.DirectCallers)-limit))
			}
		} else {
			sb.WriteString("✅ 无直接调用者，可安全修改\n")
		}

		// 间接调用总数
		if len(astResult.IndirectCallers) > 0 {
			sb.WriteString(fmt.Sprintf("\n_间接影响: %d 个函数_\n", len(astResult.IndirectCallers)))
		}

		// JSON：直接调用者 + 间接调用者（按距离，前20个）
		sb.WriteString("\n```json\n")
		sb.WriteString(fmt.Sprintf(`{"risk":"%s","direct_count":%d,"indirect_count":%d,"callers":[`,
			astResult.RiskLevel, len(astResult.DirectCallers), len(astResult.IndirectCallers)))

		// 直接调用者
		for i, c := range astResult.DirectCallers {
			if i >= 10 {
				break
			}
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf(`"%s"`, c.Node.Name))
		}

		// 间接调用者（前20个，BFS已按距离排序）
		indirectLimit := 20
		if len(astResult.IndirectCallers) < indirectLimit {
			indirectLimit = len(astResult.IndirectCallers)
		}
		for i := 0; i < indirectLimit; i++ {
			c := astResult.IndirectCallers[i]
			if i > 0 || len(astResult.DirectCallers) > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf(`"%s"`, c.Node.Name))
		}

		sb.WriteString("]}\n```\n")

		return mcp.NewToolResultText(sb.String()), nil
	}
}

func wrapProjectMap(sm *SessionManager, ai *services.ASTIndexer) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args ProjectMapArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数错误: %v", err)), nil
		}

		if sm.ProjectRoot == "" {
			return mcp.NewToolResultError("项目未初始化，请先执行 initialize_project"), nil
		}

		// 🆕 【关键】先刷新索引，确保数据最新
		_, _ = ai.Index(sm.ProjectRoot)

		level := args.Level
		if level == "" {
			level = "symbols"
		}

		// 调用 AST 服务生成数据
		// 注意：如果 scope 为空，底层会自动处理为整个项目
		result, err := ai.MapProjectWithScope(sm.ProjectRoot, level, args.Scope)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("生成地图失败: %v", err)), nil
		}

		// 🆕 收集所有符号名并分析复杂度
		var symbolNames []string
		for _, nodes := range result.Structure {
			for _, node := range nodes {
				// 只分析函数、方法和类
				if node.NodeType == "function" || node.NodeType == "method" || node.NodeType == "class" {
					symbolNames = append(symbolNames, node.Name)
				}
			}
		}

		// 调用复杂度分析
		if len(symbolNames) > 0 {
			complexityReport, err := ai.AnalyzeComplexity(sm.ProjectRoot, symbolNames)
			if err == nil && complexityReport != nil {
				// 构建复杂度映射
				result.ComplexityMap = make(map[string]float64)
				for _, risk := range complexityReport.HighRiskSymbols {
					result.ComplexityMap[risk.SymbolName] = risk.Score
				}
			}
		}

		// 使用 MapRenderer 渲染结果
		mr := NewMapRenderer(result, sm.ProjectRoot)

		var content string
		switch level {
		case "structure":
			content = mr.RenderOverview()
		default: // symbols
			content = mr.RenderStandard()
		}

		// 🆕 主动接管大输出：如果 > 2000 字符，保存到文件
		if len(content) > 2000 {
			mcpDataDir := filepath.Join(sm.ProjectRoot, ".mcp-data")
			_ = os.MkdirAll(mcpDataDir, 0755)

			// 按模式固定命名，每次直接覆盖（不保留历史版本）
			filename := fmt.Sprintf("project_map_%s.md", level)
			outputPath := filepath.Join(mcpDataDir, filename)

			if err := os.WriteFile(outputPath, []byte(content), 0644); err == nil {
				return mcp.NewToolResultText(fmt.Sprintf(
					"⚠️ Map 内容较长 (%d chars)，已自动保存到项目文件：\n👉 `%s`\n\n请使用 view_file 查看。",
					len(content), outputPath)), nil
			}
			// 如果保存失败，降级回直接返回
		}

		return mcp.NewToolResultText(content), nil
	}
}
