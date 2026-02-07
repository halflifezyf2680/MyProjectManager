package tools

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	_ "modernc.org/sqlite"
)

// PromptArgs 保存提示词参数
type PromptArgs struct {
	Title    string `json:"title" jsonschema:"required,description=提示词标题"`
	Content  string `json:"content" jsonschema:"required,description=提示词内容"`
	TagNames string `json:"tag_names" jsonschema:"description=标签 (逗号分隔)"`
	Scope    string `json:"scope" jsonschema:"default=project,enum=project,enum=global,description=存储范围 (project/global)"`
}

// RegisterPromptTools 注册提示词相关工具
func RegisterPromptTools(s *server.MCPServer, sm *SessionManager) {
	s.AddTool(mcp.NewTool("save_prompt_from_context",
		mcp.WithDescription(`save_prompt_from_context - 保存提示词到上下文库

用途：
  将当前对话中生成的优秀 Prompt 或代码片段存入本地/全局库。这些积累的 Prompt 可以在未来的任务中被复用，提升响应质量。

参数：
  title (必填)
    提示词的标题，描述其核心用途。
  
  content (必填)
    提示词的具体内容。
  
  tag_names (可选)
    标签名称，用逗号分隔，方便后续检索。
  
  scope (默认: project)
    - project: 保存到当前项目专用的数据库。
    - global: 保存到全局通用数据库，跨项目共享。

说明：
  - 建议将通用的系统提示（System Prompt）保存为 global，且打上领域标签。

示例：
  save_prompt_from_context(title="Go 错误处理指南", content="...", scope="global")
    -> 将指南保存到全局库

触发词：
  "mpm 保存提示词", "mpm saveprompt"`),
		mcp.WithInputSchema[PromptArgs](),
	), wrapSavePrompt(sm))
}

func wrapSavePrompt(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args PromptArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数错误: %v", err)), nil
		}

		// 1. 确定数据库路径
		var dbPath string
		if args.Scope == "global" {
			// Global: mcp-expert-server/.mcp-data/prompt_snippets.db
			// 注意：这里需要定位到 Go server 对应的 global data 目录
			// 假设我们在 mcp-server-go/bin/ 下运行，那么 global data 在 ../.mcp-data ?
			// 为了兼容 Python 版路径，我们尽量复用 mcp-expert-server 的结构
			// 或者，我们可以定义一个新的 global 路径，比如 UserHome/.mcp-data
			// 这里假设与 SessionManager.ProjectRoot 同级的 .mcp-data (如果是 global)
			// 但这很不安全。
			// 让我们暂时用项目内的 global (即 fallback 到 project 吧，或者 hardcode 一个路径)
			// 为了简单起见，如果 scope 是 global，我们尝试找父级目录的 .mcp-data，或者就在项目下

			// 更好的策略：Go Server 应该知道自己的 Root。
			// 暂时: 只是用 Project DB，以后再处理 Global。
			// 但用户明确要求迁移，所以 Global 必须支持。
			// 假设 Global DB 在 EXEC_DIR/../.mcp-data/ (即 mcp-server-go/.mcp-data)
			execPath, _ := os.Executable()
			globalDir := filepath.Join(filepath.Dir(filepath.Dir(execPath)), ".mcp-data")
			_ = os.MkdirAll(globalDir, 0755)
			dbPath = filepath.Join(globalDir, "prompt_snippets.db")
		} else {
			// Project Scope
			if sm.ProjectRoot == "" {
				return mcp.NewToolResultError("项目尚未初始化，无法保存到项目 Scope。请先运行 initialize_project。"), nil
			}
			mcpData := filepath.Join(sm.ProjectRoot, ".mcp-data")
			_ = os.MkdirAll(mcpData, 0755)
			dbPath = filepath.Join(mcpData, "prompt_snippets.db")
		}

		// 2. 初始化数据库
		if err := initPromptDB(dbPath); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("初始化数据库失败: %v", err)), nil
		}

		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("打开数据库失败: %v", err)), nil
		}
		defer db.Close()

		tx, err := db.Begin()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("开启事务失败: %v", err)), nil
		}

		// 3. 插入提示词
		res, err := tx.Exec("INSERT INTO prompt_snippets (title, content, category) VALUES (?, ?, ?)",
			args.Title, args.Content, "general")
		if err != nil {
			tx.Rollback()
			return mcp.NewToolResultError(fmt.Sprintf("插入提示词失败: %v", err)), nil
		}

		snippetID, _ := res.LastInsertId()

		// 4. 处理标签
		if args.TagNames != "" {
			tags := strings.Split(args.TagNames, ",")
			for _, tagName := range tags {
				tagName = strings.TrimSpace(tagName)
				if tagName == "" {
					continue
				}

				// 查找或创建标签
				var tagID int64
				err := tx.QueryRow("SELECT id FROM snippet_tags WHERE name = ?", tagName).Scan(&tagID)
				if err == sql.ErrNoRows {
					res, err := tx.Exec("INSERT INTO snippet_tags (name) VALUES (?)", tagName)
					if err != nil {
						// 忽略标签插入错误? 最好还是报错
						tx.Rollback()
						return mcp.NewToolResultError(fmt.Sprintf("创建标签失败: %v", err)), nil
					}
					tagID, _ = res.LastInsertId()
				} else if err != nil {
					tx.Rollback()
					return mcp.NewToolResultError(fmt.Sprintf("查询标签失败: %v", err)), nil
				}

				// 关联
				_, err = tx.Exec("INSERT OR IGNORE INTO snippet_tag_relations (snippet_id, tag_id) VALUES (?, ?)", snippetID, tagID)
				if err != nil {
					tx.Rollback()
					return mcp.NewToolResultError(fmt.Sprintf("关联标签失败: %v", err)), nil
				}

				// 更新引用计数
				_, _ = tx.Exec("UPDATE snippet_tags SET use_count = use_count + 1 WHERE id = ?", tagID)
			}
		}

		if err := tx.Commit(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("提交事务失败: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("✅ 提示词已保存 (ID: %d)\n📂 Scope: %s\n🏷️ Tags: %s", snippetID, args.Scope, args.TagNames)), nil
	}
}

func initPromptDB(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	schema := `
	CREATE TABLE IF NOT EXISTS prompt_snippets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		category TEXT DEFAULT 'general',
		is_favorite INTEGER DEFAULT 0,
		use_count INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS snippet_tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		color TEXT DEFAULT '#4285F4',
		use_count INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS snippet_tag_relations (
		snippet_id INTEGER,
		tag_id INTEGER,
		PRIMARY KEY (snippet_id, tag_id),
		FOREIGN KEY (snippet_id) REFERENCES prompt_snippets(id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES snippet_tags(id) ON DELETE CASCADE
	);
	`
	_, err = db.Exec(schema)
	return err
}
