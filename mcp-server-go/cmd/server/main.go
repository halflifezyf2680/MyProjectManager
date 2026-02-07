package main

import (
	"fmt"
	"os"

	"mcp-server-go/internal/core"
	"mcp-server-go/internal/services"
	"mcp-server-go/internal/tools"

	"github.com/mark3labs/mcp-go/server"
)

func init() {
	// 设置 UTF-8 编码，确保中文正常显示
	os.Setenv("LANG", "zh_CN.UTF-8")
	os.Setenv("LC_ALL", "zh_CN.UTF-8")
}

func main() {
	// 初始化会话管理器与内部服务
	sm := &tools.SessionManager{}
	ai := services.NewASTIndexer()

	// 🆕 无条件清理 servers.json（移除死亡进程）
	services.CleanupServersJSON()

	// 🚀 [LifeCycle] 探测并尝试自动绑定项目
	projectRoot := core.DetectProjectRoot()
	var discovery *services.DiscoveryService
	if projectRoot != "" {
		fmt.Fprintf(os.Stderr, "[MCP-Go] 已锁定项目根目录: %s\n", projectRoot)
		m, err := core.NewMemoryLayer(projectRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[MCP-Go][ERROR] 记忆层初始化受阻: %v\n", err)
		} else {
			sm.Memory = m
			sm.ProjectRoot = projectRoot
			fmt.Fprintf(os.Stderr, "[MCP-Go] 记忆层（SSOT）与项目上下文已就绪。\n")

			// 🆕 启动服务发现心跳（向 HUD 注册自身）
			discovery = services.NewDiscoveryService(projectRoot)
			discovery.Start()
			fmt.Fprintf(os.Stderr, "[MCP-Go] 服务发现心跳已启动，HUD 可识别此实例。\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "[MCP-Go][WARN] 无法探测项目根目录，请检查环境变量或在项目目录下运行。\n")
	}

	// 注：HUD 自动启动已移至 initialize_project 工具，不再在 server 启动时触发

	// 启动 MCP Server (StdIO)
	s := server.NewMCPServer(
		"MyProjectManager-Go",
		"1.0.0",
	) // 注册工具
	tools.RegisterSystemTools(s, sm, ai)       // 系统初始化
	tools.RegisterMemoryTools(s, sm)           // 备忘与检索
	tools.RegisterSearchTools(s, sm, ai)       // 项目地图与搜索
	tools.RegisterIntelligenceTools(s, sm, ai) // 任务分析与事实存档
	tools.RegisterAnalysisTools(s, sm, ai)     // 影响分析工具
	tools.RegisterSkillTools(s, sm)            // 技能库工具
	tools.RegisterTaskTools(s, sm)             // 任务管理工具
	tools.RegisterEnhanceTools(s, sm)          // 增强工具 (prompt_enhance, persona)
	tools.RegisterDocTools(s, sm, ai)          // 文档工具 (wiki_writer)
	tools.RegisterPromptTools(s, sm)           // 提示词工具 (save_prompt)

	fmt.Fprintf(os.Stderr, "[MCP-Go] MyProjectManager 正在启动...\n")

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "服务运行错误: %v\n", err)
		os.Exit(1)
	}
}
