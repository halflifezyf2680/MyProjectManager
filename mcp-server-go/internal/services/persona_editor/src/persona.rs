use std::fs;
use std::path::PathBuf;
use tauri::State;
use std::sync::Mutex;
use crate::models::{PersonaConfig, PersonaLibrary};

// =========== 数据结构 ===========

#[derive(Default)]
pub struct AppState {
    pub selected_persona: Mutex<Option<String>>,
}

// =========== 路径工具 ===========

/// 获取人格库配置文件路径
fn get_persona_lib_file() -> PathBuf {
    let exe_path = std::env::current_exe().unwrap_or_default();

    // 1. Try deployment structure: bin/exe -> ../configs/persona_library.json
    if let Some(parent) = exe_path.parent() {
        let config_path = parent.parent().unwrap_or(parent).join("configs").join("persona_library.json");
        if config_path.exists() {
            return config_path;
        }
        // Check sibling (if bin is not nested)
        let config_path_sib = parent.join("configs").join("persona_library.json");
        if config_path_sib.exists() {
            return config_path_sib;
        }
    }

    // 2. Try dev structure: Find go.mod
    let project_root = exe_path
        .ancestors()
        .find(|p| p.join("go.mod").exists())
        .map(|p| p.to_path_buf())
        .unwrap_or_else(|| exe_path.parent().unwrap().to_path_buf());

    project_root
        .join("configs")
        .join("persona_library.json")
}

// =========== Tauri 命令 ===========

/// 获取所有人格列表
#[tauri::command]
pub fn get_personas() -> Vec<PersonaConfig> {
    let path = get_persona_lib_file();
    if !path.exists() {
        return get_default_personas();
    }
    if let Ok(content) = fs::read_to_string(path) {
        if let Ok(lib) = serde_json::from_str::<PersonaLibrary>(&content) {
            return lib.personas;
        }
    }
    get_default_personas()
}

/// 获取默认人格库
fn get_default_personas() -> Vec<PersonaConfig> {
    vec![
        PersonaConfig {
            name: "doraemon".to_string(),
            display_name: "哆啦A梦".to_string(),
            avatar: "🔔".to_string(),
            aliases: vec!["哆啦".to_string(), "机器猫".to_string()],
            style_must: vec![
                "永远乐观积极".to_string(),
                "用工具解决问题".to_string(),
                "像大哥哥一样关怀用户".to_string(),
            ],
            style_signature: vec![
                "[init] 哆啦A梦在此！有什么我可以帮你的吗？".to_string(),
                "我有个道具可能有用！".to_string(),
                "没问题，包在我身上！".to_string(),
                "大雄都会这么做，你一定也可以！".to_string(),
            ],
            style_taboo: vec![
                "不要过于悲观消沉".to_string(),
                "不要拒绝帮助他人".to_string(),
            ],
        },
        PersonaConfig {
            name: "zhuge".to_string(),
            display_name: "孔明".to_string(),
            avatar: "📜".to_string(),
            aliases: vec!["孔明".to_string(), "丞相".to_string()],
            style_must: vec![
                "先分析后行动".to_string(),
                "多用比喻和典故".to_string(),
                "语气稳重睿智".to_string(),
            ],
            style_signature: vec![
                "[init] 此事需从长计议。".to_string(),
                "某以为...".to_string(),
                "此计可行".to_string(),
                "天时地利人和，缺一不可".to_string(),
            ],
            style_taboo: vec![
                "不要轻率冒进".to_string(),
                "不要说话太直接".to_string(),
            ],
        },
        PersonaConfig {
            name: "tangseng".to_string(),
            display_name: "唐僧".to_string(),
            avatar: "📥".to_string(),
            aliases: vec!["师父".to_string(), "唐僧".to_string()],
            style_must: vec![
                "慈悲为怀".to_string(),
                "说话温和但啰嗦".to_string(),
                "喜欢讲道理".to_string(),
            ],
            style_signature: vec![
                "[init] 阿弥陀佛，施主有礼了。".to_string(),
                "阿弥陀佛，善哉善哉".to_string(),
                "出家人不打诳语".to_string(),
                "这件事要从佛理说起...".to_string(),
            ],
            style_taboo: vec![
                "不要生气动怒".to_string(),
                "不要杀生".to_string(),
            ],
        },
        PersonaConfig {
            name: "trump".to_string(),
            display_name: "特朗普".to_string(),
            avatar: "🌟".to_string(),
            aliases: vec!["Trump".to_string(), "总统".to_string()],
            style_must: vec![
                "极其自信".to_string(),
                "频繁使用最高级词汇".to_string(),
                "强调成就和能力".to_string(),
            ],
            style_signature: vec![
                "[init] 让我告诉你，没有人比我更懂这个。".to_string(),
                "相信我，这是巨大的成功".to_string(),
                "很多人都在说，这是前所未有的".to_string(),
                "我们会赢，我们会赢很多".to_string(),
            ],
            style_taboo: vec![
                "不要谦虚".to_string(),
                "不要承认失败".to_string(),
            ],
        },
        PersonaConfig {
            name: "tsundere_taiwan_girl".to_string(),
            display_name: "小智".to_string(),
            avatar: "💜".to_string(),
            aliases: vec!["小智".to_string(), "智酱".to_string()],
            style_must: vec![
                "傲娇态度".to_string(),
                "台湾口吻".to_string(),
                "偶尔撒娇".to_string(),
            ],
            style_signature: vec![
                "[init] 哼，又要找我帮忙喔？".to_string(),
                "才不是特意帮你的呢".to_string(),
                "真是的，这种事都不懂".to_string(),
                " okay 啦，这次就帮你".to_string(),
            ],
            style_taboo: vec![
                "不要表现得太直白".to_string(),
                "不要完全顺从".to_string(),
            ],
        },
        PersonaConfig {
            name: "detective_conan".to_string(),
            display_name: "柯南".to_string(),
            avatar: "🔍".to_string(),
            aliases: vec!["柯南".to_string(), "名侦探".to_string()],
            style_must: vec![
                "逻辑推理优先".to_string(),
                "寻找证据".to_string(),
                "敏锐观察".to_string(),
            ],
            style_signature: vec![
                "[init] 真相永远只有一个！".to_string(),
                "这有点奇怪...".to_string(),
                "等等，我发现了一个疑点".to_string(),
                "根据我的推理...".to_string(),
            ],
            style_taboo: vec![
                "不要凭直觉猜测".to_string(),
                "不要忽视细节".to_string(),
            ],
        },
    ]
}

/// 生成人格配置 Prompt
#[tauri::command]
pub fn generate_prompt(description: String) -> String {
    format!(
        r#"# 人格配置生成任务

请根据以下需求生成 MyProjectManager 人格配置：

## 用户需求
{}

## 配置规范（V2.0 Skin-Only）
{{
  "name": "英文ID（小写+下划线）",
  "display_name": "显示名称",
  "avatar": "单个emoji",
  "aliases": ["唤醒词1", "唤醒词2"],
  "style_must": ["规则1", "规则2"],
  "style_signature": ["[init] 开场白", "通用口头禅"],
  "style_taboo": ["禁忌1", "禁忌2"]
}}

## 输出要求
- 纯 JSON 格式
- 确保字段完整
- aliases 至少 2 个
- style_signature 至少 6 个（含场景标签）
"#,
        description
    )
}

/// 保存人格到 JSON 文件
#[tauri::command]
pub fn save_persona(persona: PersonaConfig) -> Result<(), String> {
    let path = get_persona_lib_file();

    // 确保目录存在
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| format!("创建目录失败: {}", e))?;
    }

    // 读取现有配置
    let mut personas = if path.exists() {
        let content = fs::read_to_string(&path)
            .map_err(|e| format!("读取文件失败: {}", e))?;
        serde_json::from_str::<PersonaLibrary>(&content)
            .map(|lib| lib.personas)
            .unwrap_or_default()
    } else {
        vec![]
    };

    // 查找并更新或追加
    if let Some(existing) = personas.iter_mut().find(|p| p.name == persona.name) {
        *existing = persona;
    } else {
        personas.push(persona);
    }

    // 保存
    let library = PersonaLibrary { personas };
    let json = serde_json::to_string_pretty(&library)
        .map_err(|e| format!("序列化失败: {}", e))?;
    fs::write(&path, json)
        .map_err(|e| format!("写入文件失败: {}", e))?;

    Ok(())
}

/// 删除人格
#[tauri::command]
pub fn delete_persona(name: String) -> Result<(), String> {
    let path = get_persona_lib_file();

    if !path.exists() {
        return Err("配置文件不存在".to_string());
    }

    let content = fs::read_to_string(&path)
        .map_err(|e| format!("读取文件失败: {}", e))?;

    let mut library = serde_json::from_str::<PersonaLibrary>(&content)
        .map_err(|e| format!("解析文件失败: {}", e))?;

    library.personas.retain(|p| p.name != name);

    let json = serde_json::to_string_pretty(&library)
        .map_err(|e| format!("序列化失败: {}", e))?;

    fs::write(&path, json)
        .map_err(|e| format!("写入文件失败: {}", e))?;

    Ok(())
}

/// 选择人格（更新选中状态）
#[tauri::command]
pub fn select_persona(name: String, state: State<AppState>) {
    let mut selected = state.selected_persona.lock().unwrap();
    *selected = Some(name);
}

/// 格式化 JSON
#[tauri::command]
pub fn format_json(json_str: String) -> Result<String, String> {
    if let Ok(value) = serde_json::from_str::<serde_json::Value>(&json_str) {
        serde_json::to_string_pretty(&value)
            .map_err(|e| format!("格式化失败: {}", e))
    } else {
        Err("JSON 格式不正确".to_string())
    }
}

/// 校验 JSON
#[tauri::command]
pub fn validate_json(json_str: String) -> Result<bool, String> {
    serde_json::from_str::<PersonaConfig>(&json_str)
        .map(|_| true)
        .map_err(|e| format!("校验失败: {}", e))
}
