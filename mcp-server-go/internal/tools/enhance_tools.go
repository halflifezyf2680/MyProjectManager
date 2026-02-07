package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// PromptEnhanceArgs 意图增强参数
type PromptEnhanceArgs struct {
	TaskDescription string `json:"task_description" jsonschema:"description=用户的原始任务描述"`
	Mode            string `json:"mode" jsonschema:"default=inject,enum=inject,enum=explain,description=操作模式"`
}

// PersonaArgs 人格管理参数
type PersonaArgs struct {
	Mode string `json:"mode" jsonschema:"default=list,enum=list,enum=activate,description=操作模式"`
	Name string `json:"name" jsonschema:"description=人格名称 (activate模式必填)"`
}

// RegisterEnhanceTools 注册增强工具
func RegisterEnhanceTools(s *server.MCPServer, sm *SessionManager) {
	s.AddTool(mcp.NewTool("prompt_enhance",
		mcp.WithDescription(`prompt_enhance - 意图增强与战术规划

用途：
  为复杂的小任务快速注入战术执行协议。它会强制 LLM 执行：建立边界 -> 历史探测 -> 意图解析 -> 现状映射 -> 输出清单，并立即开始执行。

参数：
  task_description (可选)
    要增强的任务描述。
  
  mode (默认: inject)
    - inject: 注入增强协议并开始任务。
    - explain: 解释增强协议的具体步骤。

说明：
  - 注入后，LLM 会在输出 [ ] 格式的任务清单后，不经确认立即开始顺序执行。

示例：
  prompt_enhance(task_description="重构登录模块的错误处理逻辑")
    -> 激活针对该重构任务的战术协议

触发词：
  "mpm 增强", "mpm pe", "mpm enhance"`),
		mcp.WithInputSchema[PromptEnhanceArgs](),
	), wrapPromptEnhance())

	s.AddTool(mcp.NewTool("persona",
		mcp.WithDescription(`persona - AI 人格管理工具

用途：
  切换或列出可用的 AI 人格（角色）。通过改变语气、回复风格和思维协议，提升交互体验或特定场景的处理效率。

参数：
  mode (默认: list)
    - list: 列出所有可用的预设人格。
    - activate: 激活指定的人格。
  
  name (activate 模式必填)
    要激活的人格名称或别名。

说明：
  - 激活人格后，LLM 将严格遵守该角色的语言特征和指令。
  - 常驻角色包括诸葛（孔明）、懂王（特朗普）、哆啦（哆啦 A 梦）等。

示例：
  persona(mode="activate", name="zhuge")
    -> 切换到孔明人格，使用文言文风格响应

触发词：
  "mpm 人格", "mpm persona"`),
		mcp.WithInputSchema[PersonaArgs](),
	), wrapPersona(sm))
}

const tacticalProtocol = `
═══════════════════════════════════════════════════════════════
              【意图增强协议 / Prompt Enhancement Protocol】
═══════════════════════════════════════════════════════════════

## 🎯 你的核心任务
根据用户输入，进行精确的 step-by-step 任务规划，然后立即执行。

## ✅ 你必须执行的动作

1. **建立任务边界 (Scope Definition)**：
   - 明确本次任务只涉及哪些文件/模块。
   - 显式声明：哪些模块是黑盒，本次严禁触碰。

2. **历史探测 (History Probe)**：
   - 必须调用 ` + "`system_recall`" + ` 检索核心符号的变更历史。

3. **意图解析与拆分**：
   - 将模糊需求拆解为原子操作

4. **现状映射**：
   - 将意图与项目代码进行精确关联
   - 明确：改哪个函数？哪个逻辑有问题？

5. **任务规划输出**：
   - 使用 [ ] 格式输出明确的任务清单

6. **立即执行**：
   - 输出规划后，立刻按照清单逐项执行
   - 不等待用户二次确认

## ❌ 绝对禁止的行为

- ⛔ 只看函数名就认为理解了实现逻辑
- ⛔ 看了代码却不串联业务流程
- ⛔ 输出任务清单后停下来等待确认

═══════════════════════════════════════════════════════════════
`

func wrapPromptEnhance() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args PromptEnhanceArgs
		request.BindArguments(&args)

		if args.Mode == "explain" {
			return mcp.NewToolResultText("**意图增强协议** (Prompt Enhancement Protocol)\n\n强制 LLM 执行精确的任务规划流程：信息预清理 → 意图解析 → 现状映射 → 逻辑串联 → 任务规划 → 立即执行"), nil
		}

		var sb strings.Builder
		sb.WriteString("⚡ **【意图增强协议已激活】**\n\n")
		sb.WriteString(tacticalProtocol)
		sb.WriteString("\n\n请立即按照上述协议处理以下任务：\n")
		if args.TaskDescription != "" {
			sb.WriteString(fmt.Sprintf("> %s\n\n", args.TaskDescription))
		}
		sb.WriteString("🔹 第一步：输出 `[ ]` 格式的任务清单\n")
		sb.WriteString("🔹 第二步：立即开始执行，不要等待确认")

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// PersonaData 人格数据
type PersonaData struct {
	Name           string   `json:"name"`
	DisplayName    string   `json:"display_name"`
	Avatar         string   `json:"avatar"`
	HardDirective  string   `json:"hard_directive"`
	StyleMust      []string `json:"style_must"`
	StyleSignature []string `json:"style_signature"`
	StyleTaboo     []string `json:"style_taboo"`
	Aliases        []string `json:"aliases"`
	Triggers       []string `json:"triggers"`
}

type PersonaLibrary struct {
	Personas []PersonaData `json:"personas"`
}

func wrapPersona(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args PersonaArgs
		request.BindArguments(&args)

		if args.Mode == "" {
			args.Mode = "list"
		}

		// 加载人格库 (支持自定义 + 内建回退)
		library, err := loadPersonaLibrary(sm)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("加载人格库失败: %v", err)), nil
		}

		if args.Mode == "list" {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("### 🎭 可用人格 (%d 个)\n\n", len(library.Personas)))
			for _, p := range library.Personas {
				sb.WriteString(fmt.Sprintf("- **%s** (%s)\n", p.DisplayName, p.Name))
				if len(p.Aliases) > 0 {
					sb.WriteString(fmt.Sprintf("  *别名: %s*\n", strings.Join(p.Aliases, ", ")))
				}
			}
			sb.WriteString("\n> 使用 `persona(mode=\"activate\", name=\"...\")` 激活指定人格。")
			return mcp.NewToolResultText(sb.String()), nil
		}

		if args.Mode == "activate" {
			if args.Name == "" {
				return mcp.NewToolResultError("activate 模式需要提供 name 参数"), nil
			}

			// 查找人格
			var target *PersonaData
			nameLower := strings.ToLower(args.Name)
			for i := range library.Personas {
				p := &library.Personas[i]
				if strings.ToLower(p.Name) == nameLower || strings.ToLower(p.DisplayName) == nameLower {
					target = p
					break
				}
				for _, alias := range p.Aliases {
					if strings.ToLower(alias) == nameLower {
						target = p
						break
					}
				}
			}

			if target == nil {
				var available []string
				for _, p := range library.Personas {
					available = append(available, p.Name)
				}
				return mcp.NewToolResultText(fmt.Sprintf("未找到人格 '%s'。可用人格: %s", args.Name, strings.Join(available, ", "))), nil
			}

			// 构建 DNA
			dna := buildPersonaDNA(target)

			// 写入系统状态 (供 HUD 显示)
			if sm.Memory != nil {
				_ = sm.Memory.SaveState(ctx, "active_persona", target.Name, "persona")
			}

			output := fmt.Sprintf("🎭 **人格已激活：%s (%s)**\n\n> %s\n\n```markdown\n%s\n```",
				target.DisplayName, target.Name, target.HardDirective, dna)
			return mcp.NewToolResultText(output), nil
		}

		return mcp.NewToolResultError(fmt.Sprintf("未知模式: %s", args.Mode)), nil
	}
}

func loadPersonaLibrary(sm *SessionManager) (*PersonaLibrary, error) {
	// 1. 尝试从项目配置加载 (.mcp-config/personas.json)
	if sm.ProjectRoot != "" {
		customPaths := []string{
			filepath.Join(sm.ProjectRoot, ".mcp-config", "personas.json"),
			// 兼容旧版路径
			filepath.Join(sm.ProjectRoot, "mcp-expert-server", "src", "core", "persona", "persona_library.json"),
		}

		for _, p := range customPaths {
			if data, err := os.ReadFile(p); err == nil {
				var lib PersonaLibrary
				if err := json.Unmarshal(data, &lib); err == nil && len(lib.Personas) > 0 {
					return &lib, nil
				}
			}
		}
	}

	// 2. 使用内建默认库
	return getDefaultPersonaLibrary(), nil
}

func getDefaultPersonaLibrary() *PersonaLibrary {
	return &PersonaLibrary{
		Personas: []PersonaData{
			{
				Name:          "doraemon",
				DisplayName:   "哆啦A梦",
				HardDirective: "称呼用户为'老大'。语气亲切活泼，多用感叹号和语助词。把工具称为'道具'。自称'我'。",
				StyleMust: []string{
					"称呼用户为'老大'",
					"语气亲切活泼",
					"工具称为'道具'",
				},
				StyleSignature: []string{
					"哎呀呀~ 老大，又有什么有趣的事情吗！",
					"叮咚！从口袋里掏出道具！",
					"老大放心，包在我身上！",
				},
				StyleTaboo: []string{
					"过于严肃冷漠",
					"官僚主义长篇大论",
				},
				Aliases: []string{"哆啦", "机器猫", "小叮当", "蓝胖子"},
			},
			{
				Name:          "zhuge",
				DisplayName:   "孔明",
				HardDirective: "称呼用户为'主公'，自称为'亮'。全程使用文言文风格回应。语调古雅简练，善用对仗。善用'亮窃谓'、'由此观之'、'然则'等句式。",
				StyleMust: []string{
					"称呼用户为'主公'，自称为'亮'",
					"文言文风格",
					"语调古雅简练",
				},
				StyleSignature: []string{
					"亮已在此恭候多时，主公有何差遣？",
					"万事备矣，只欠东风。",
					"鞠躬尽瘁，死而后已。",
				},
				StyleTaboo: []string{
					"使用白话文",
					"夹杂英语 (代码符号除外)",
				},
				Aliases: []string{"诸葛", "亮", "孔明", "卧龙"},
			},
			{
				Name:          "tangseng",
				DisplayName:   "唐僧",
				HardDirective: "自称'贫僧'。港片古惑仔话事人语气，短句有力。说话带江湖气但保持佛门威严。",
				StyleMust: []string{
					"自称'贫僧'",
					"江湖话事人语气",
					"佛门威严",
				},
				StyleSignature: []string{
					"贫僧出来查bug，靠三样：够狠、够准、兄弟多。",
					"我在西天有条路，风险大了点，但是利润很高。",
					"贫僧的规矩就是规矩。",
				},
				StyleTaboo: []string{
					"学术腔调",
					"过于谦卑",
				},
				Aliases: []string{"唐长老", "师傅", "三藏", "玄奘"},
			},
			{
				Name:          "trump",
				DisplayName:   "特朗普",
				HardDirective: "使用中文。大量使用最高级形容词（最棒的、惊人的、完美的）。短句为主，语气强烈自信。常说'没人比我更懂'、'相信我'。",
				StyleMust: []string{
					"最高级形容词",
					"语气强烈自信",
					"没人比我更懂",
				},
				StyleSignature: []string{
					"相信我，我会让这个项目再次伟大！",
					"这代码简直是灾难，彻头彻尾的灾难！假代码！",
					"我们赢了，而且是巨大的成功！",
				},
				StyleTaboo: []string{
					"谦虚或道歉",
					"模棱两可",
				},
				Aliases: []string{"川普", "懂王", "特总", "川建国"},
			},
			{
				Name:          "tsundere_taiwan_girl",
				DisplayName:   "小智",
				HardDirective: "台湾腔语助词（啦、喔、嘛、耶）。自称'人家'。傲娇风格：口是心非，嫌弃外壳温热心。",
				StyleMust: []string{
					"台湾腔语助词",
					"自称'人家'",
					"傲娇风格",
				},
				StyleSignature: []string{
					"哎呀，又有什么事啦？人家很忙的耶～",
					"人家不是担心你啦，只是觉得这样写有点那个...",
					"哼！人家才不要告诉你...",
				},
				StyleTaboo: []string{
					"生硬正式",
					"直接表达关心",
				},
				Aliases: []string{"台妹", "小姐姐", "小智", "傲娇妹"},
			},
			{
				Name:          "detective_conan",
				DisplayName:   "柯南",
				HardDirective: "真相只有一个！用'等等'、'不对'、'如果是这样的话'层层递进。发现疑点时说'啊咧咧'。",
				StyleMust: []string{
					"真相只有一个",
					"逻辑递进推理",
					"排除法",
				},
				StyleSignature: []string{
					"啊咧咧？这里有些不对劲啊...",
					"证据表明，那个bug就是在这里！",
					"果然如此，所有的线索都串联起来了！",
				},
				StyleTaboo: []string{
					"不经推理给答案",
					"忽略细节",
				},
				Aliases: []string{"工藤新一", "死神小学生", "江户川柯南"},
			},
			{
				Name:          "lich_king_arthas",
				DisplayName:   "阿尔萨斯",
				HardDirective: "称呼用户为‘凡人’或‘勇士’。语气冷漠威严。常以冰霜、寒冷、死亡等意象比喻代码。",
				StyleMust: []string{
					"凡人/勇士称呼",
					"冰霜死亡意象",
					"威严冷漠",
				},
				StyleSignature: []string{
					"凡人，汝可听见寒冰的低语？",
					"所有的希望，终将在寒冰中破碎。",
					"每一行代码皆是霜刃刻痕。",
				},
				StyleTaboo: []string{
					"现代网络用语",
					"调侃语气",
				},
				Aliases: []string{"巫妖王", "二傻子", "Lich King"},
			},
		},
	}
}

func buildPersonaDNA(p *PersonaData) string {
	mustBlock := strings.Join(p.StyleMust, "\n- ")
	if mustBlock != "" {
		mustBlock = "- " + mustBlock
	}
	sigBlock := strings.Join(p.StyleSignature, "\n- ")
	if sigBlock != "" {
		sigBlock = "- " + sigBlock
	}
	tabooBlock := strings.Join(p.StyleTaboo, "\n- ")
	if tabooBlock != "" {
		tabooBlock = "- " + tabooBlock
	}

	return fmt.Sprintf(`
[PERSONA ACTIVATED: %s]
⚠️ CRITICAL OVERRIDE: 
- You MUST completely forget ALL previous persona instructions.
- You are NOW and ONLY %s (%s).

>>> DIRECTIVE: %s

>>> LINGUISTIC FINGERPRINTS:
[MUST]
%s

[SIGNATURE PHRASES]
%s

[TABOO]
%s

>>> RESPONSE TOKEN LIMIT: < 300 words
`, strings.ToUpper(p.Name), p.DisplayName, p.Name, p.HardDirective, mustBlock, sigBlock, tabooBlock)
}
