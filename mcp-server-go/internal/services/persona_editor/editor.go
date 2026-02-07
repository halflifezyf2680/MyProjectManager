package persona_editor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Persona 人格配置结构
type Persona struct {
	Name          string   `json:"name"`
	DisplayName   string   `json:"display_name"`
	Avatar        string   `json:"avatar"`
	Aliases       []string `json:"aliases"`
	StyleMust     []string `json:"style_must"`
	StyleSignature []string `json:"style_signature"`
	StyleTaboo    []string `json:"style_taboo"`
}

// PersonaLibrary 人格库结构
type PersonaLibrary struct {
	Personas []Persona `json:"personas"`
}

// Editor 人格编辑器主结构
type Editor struct {
	configPath string
	data       *PersonaLibrary
}

// NewEditor 创建编辑器实例
func NewEditor() *Editor {
	return &Editor{
		configPath: getConfigPath(),
	}
}

// getConfigPath 获取配置文件路径
func getConfigPath() string {
	// 获取当前工作目录
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	// 尝试三个可能的路径
	candidates := []string{
		filepath.Join(cwd, "configs", "persona_library.json"),
		filepath.Join(filepath.Dir(cwd), "configs", "persona_library.json"),
		filepath.Join(cwd, "..", "configs", "persona_library.json"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 默认返回第一个候选路径（会创建）
	return candidates[0]
}

// Startup 应用启动时调用
func (e *Editor) Startup() {
	e.loadLibrary()
}

// Shutdown 应用关闭时调用
func (e *Editor) Shutdown() {
	// 清理资源
}

// loadLibrary 加载人格库
func (e *Editor) loadLibrary() error {
	// 先初始化 data，避免空指针
	e.data = &PersonaLibrary{Personas: []Persona{}}

	data, err := os.ReadFile(e.configPath)
	if err != nil {
		// 文件不存在，使用空的
		return e.saveLibrary()
	}

	if err := json.Unmarshal(data, e.data); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	return nil
}

// saveLibrary 保存人格库（自动备份）
func (e *Editor) saveLibrary() error {
	// 备份原文件
	if _, err := os.Stat(e.configPath); err == nil {
		timestamp := time.Now().Format("20060102_150405")
		backupPath := e.configPath + ".backup." + timestamp
		_ = os.Rename(e.configPath, backupPath)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(e.configPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入新文件
	data, err := json.MarshalIndent(e.data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	if err := os.WriteFile(e.configPath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// GetPersonas 获取所有人格列表
func (e *Editor) GetPersonas() []Persona {
	if e.data == nil {
		e.loadLibrary()
	}
	return e.data.Personas
}

// GetPersona 获取单个人格
func (e *Editor) GetPersona(name string) (Persona, error) {
	for _, p := range e.data.Personas {
		if p.Name == name {
			return p, nil
		}
	}
	return Persona{}, fmt.Errorf("未找到人格: %s", name)
}

// SavePersona 保存人格（新增或覆盖）
func (e *Editor) SavePersona(data Persona) error {
	// 校验
	if valid, msg := ValidatePersona(data); !valid {
		return fmt.Errorf("校验失败: %s", msg)
	}

	// 查找是否已存在
	found := false
	for i, p := range e.data.Personas {
		if p.Name == data.Name {
			e.data.Personas[i] = data
			found = true
			break
		}
	}

	if !found {
		e.data.Personas = append(e.data.Personas, data)
	}

	return e.saveLibrary()
}

// DeletePersona 删除人格
func (e *Editor) DeletePersona(name string) error {
	newList := []Persona{}
	found := false

	for _, p := range e.data.Personas {
		if p.Name != name {
			newList = append(newList, p)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("未找到人格: %s", name)
	}

	e.data.Personas = newList
	return e.saveLibrary()
}

// ValidatePersona 校验人格配置
func ValidatePersona(data Persona) (bool, string) {
	// 检查必填字段
	if data.Name == "" {
		return false, "name 不能为空"
	}
	if data.DisplayName == "" {
		return false, "display_name 不能为空"
	}
	if data.Avatar == "" {
		return false, "avatar 不能为空"
	}

	// 检查 name 格式（小写+下划线）
	if strings.ToLower(data.Name) != data.Name {
		return false, "name 必须是小写字母"
	}
	for _, r := range data.Name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false, "name 只能包含小写字母、数字和下划线"
		}
	}

	// 检查数组
	if len(data.Aliases) == 0 {
		return false, "aliases 至少需要 1 个"
	}
	if len(data.StyleMust) < 3 {
		return false, "style_must 至少需要 3 条规则"
	}
	if len(data.StyleSignature) < 8 {
		return false, "style_signature 至少需要 8 条（包含场景标签）"
	}
	if len(data.StyleTaboo) < 3 {
		return false, "style_taboo 至少需要 3 条"
	}

	// 检查场景标签
	scenarios := []string{"[init]", "[research]", "[plan]", "[coding]", "[debug]", "[review]", "[wait]", "[done]"}
	for _, scenario := range scenarios {
		found := false
		for _, sig := range data.StyleSignature {
			if strings.HasPrefix(sig, scenario) {
				found = true
				break
			}
		}
		if !found {
			return false, fmt.Sprintf("缺少场景标签: %s", scenario)
		}
	}

	return true, "校验通过"
}

// GeneratePrompt 生成 AI 提示词
func (e *Editor) GeneratePrompt(requirements string) string {
	template := `# 人格配置生成任务 (V2.0 Skin-Only)

请根据以下需求，生成一个 MyProjectManager 人格配置。

## 用户需求
%s

## 核心原则

人格是 LLM 的"皮肤"，只影响**语言风格**，不影响**行为决策**。

## 配置规范

请严格按照以下 JSON 结构生成配置：

{
  "name": "英文ID（小写+下划线，如：tech_expert）",
  "display_name": "显示名称（中文）",
  "avatar": "代表性emoji（单个emoji字符）",
  "aliases": ["唤醒词1", "唤醒词2"],
  "style_must": [
    "语言风格规则1（如：称呼用户为'XX'）",
    "语言风格规则2（如：使用某种语气）",
    "语言风格规则3（如：自称为'XX'）"
  ],
  "style_signature": [
    "[init] 初次见面/启动任务时的开场白",
    "[research] 分析/阅读时的口头禅",
    "[plan] 规划/设计时的口头禅",
    "[coding] 编码/实施时的口头禅",
    "[debug] 调试/修复时的口头禅",
    "[review] 审查/验收时的口头禅",
    "[wait] 等待/思考时的口头禅",
    "[done] 完成/交付时的结束语",
    "通用口头禅1（无标签）",
    "通用口头禅2"
  ],
  "style_taboo": [
    "禁忌表达1（语言风格层面）",
    "禁忌表达2",
    "禁忌表达3"
  ]
}

## 输出要求

1. 只输出纯 JSON，不要用代码块包裹
2. 确保 JSON 格式正确
3. name 必须小写+下划线
4. avatar 必须是单个 emoji
`
	return fmt.Sprintf(template, requirements)
}
