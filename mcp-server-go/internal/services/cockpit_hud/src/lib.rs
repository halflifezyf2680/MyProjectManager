use std::fs;
use std::path::PathBuf;
use std::process::Command;
use serde::{Deserialize, Serialize};
use tauri::{State, Manager};
use std::sync::Mutex;

// =========== 数据结构 ===========

#[derive(Default)]
pub struct AppState {
    current_server_id: Mutex<Option<String>>,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct ServerInfo {
    pub id: String,
    pub port: u16,
    pub project_path: String,
    #[serde(default)]
    pub project_name: String,
    #[serde(default)]
    pub persona: String,
    pub heartbeat: f64,
    pub started: String,
    // 🆕 Hook 相关字段
    #[serde(default)]
    pub pending_hooks: u32,
    #[serde(default)]
    pub hook_summary: String,
    // 🆕 父进程 PID（IDE 进程）
    #[serde(default)]
    pub parent_pid: Option<u32>,
}

#[derive(Serialize, Deserialize)]
pub struct DiscoveryData {
    pub servers: Vec<ServerInfo>,
}

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct PersonaConfig {
    pub name: String,
    pub display_name: String,
    #[serde(default)]
    pub avatar: String,  // emoji图标
}

#[derive(Serialize, Deserialize)]
pub struct PersonaLibrary {
    pub personas: Vec<PersonaConfig>,
}

#[derive(Serialize, Deserialize, Default)]
pub struct AppConfig {
    pub auto_open: bool,
}

// =========== 路径工具 ===========

fn get_discovery_file() -> PathBuf {
    dirs::home_dir()
        .unwrap_or_default()
        .join(".mcp-cockpit")
        .join("servers.json")
}

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

fn get_config_file_path() -> PathBuf {
    dirs::home_dir()
        .unwrap_or_default()
        .join(".mcp-cockpit")
        .join("config.json")
}

// =========== Tauri 命令 ===========

use rusqlite::{Connection, OpenFlags};

// ... (existing imports)

// Helper: Fetch state directly from project DB
fn fetch_project_state(project_path: &str) -> (u32, String, String) {
    let db_path = PathBuf::from(project_path).join(".mcp-data").join("mcp_memory.db");
    
    // Default values
    let default_persona = "jarvis".to_string();
    
    if !db_path.exists() {
        return (0, "".to_string(), default_persona);
    }

    let conn = match Connection::open_with_flags(&db_path, OpenFlags::SQLITE_OPEN_READ_ONLY) {
        Ok(c) => c,
        Err(_) => return (0, "".to_string(), default_persona),
    };

    // 1. Get Pending Hooks Count
    let count: u32 = conn.query_row(
        "SELECT COUNT(*) FROM pending_hooks WHERE status = 'open'",
        [], |row| row.get(0)
    ).unwrap_or(0);

    // 2. Get Hook Summary (Top 10)
    let mut summary = String::new();
    if let Ok(mut stmt) = conn.prepare("SELECT summary, tag, description FROM pending_hooks WHERE status = 'open' ORDER BY priority DESC, created_at DESC LIMIT 10") {
        let rows = stmt.query_map([], |row| {
             let number: Option<String> = row.get(0).ok();  // summary 存储编号
             let tag: Option<String> = row.get(1).ok();     // tag 是用户标签
             let desc: String = row.get(2).unwrap_or_default();
             Ok((number, tag, desc))
        });
        
        if let Ok(iter) = rows {
            let mut lines = Vec::new();
            for item in iter {
                if let Ok((number, tag, desc)) = item {
                   let num = number.unwrap_or_else(|| "???".to_string());
                   let label = if let Some(t) = tag {
                       if !t.is_empty() {
                           format!("{} ({})", num, t)
                       } else {
                           num
                       }
                   } else {
                       num
                   };
                   lines.push(format!("• [{}] {}", label, desc));
                }
            }
            if count > 10 {
                lines.push(format!("... +{} more", count - 10));
            }
            summary = lines.join("\n");
        }
    }

    // 3. Get Active Persona (🆕 V6.0: 切换到 system_state)
    let mut persona = "None".to_string();
    match conn.query_row::<String, _, _>(
        "SELECT value FROM system_state WHERE key = 'active_persona' LIMIT 1",
        [], |row| row.get(0)
    ) {
        Ok(val) => {
             // Clean up quotes if it was stored as JSON string, and force lowercase
             persona = val.trim_matches('"').to_lowercase();
             // DEBUG: Indicate source
             summary.push_str("\n[DEBUG: Persona from system_state]");
        },
        Err(e) => {
             // DEBUG: Indicate failure - No fallback to default_persona
             summary.push_str(&format!("\n[DEBUG: Persona Read Failed: {}]", e));
        }
    }
    
    (count, summary, persona)
}

#[tauri::command]
fn list_servers() -> Vec<ServerInfo> {
    let path = get_discovery_file();
    if !path.exists() { return vec![]; }
    
    if let Ok(content) = fs::read_to_string(&path) {
        if let Ok(mut data) = serde_json::from_str::<DiscoveryData>(&content) {
            // Enrich with real-time DB data and filter invalid paths
            let mut valid_servers = Vec::new();
            for server in &mut data.servers {
                // 跳过无效路径
                if server.project_path == "Unknown" || server.project_path.is_empty() {
                    continue;
                }
                
                // 验证路径是否存在
                let path_obj = std::path::Path::new(&server.project_path);
                if !path_obj.exists() {
                    continue; // 跳过不存在的路径
                }
                
                // If path is valid, fetch from DB
                let (hooks, summary, p) = fetch_project_state(&server.project_path);
                server.pending_hooks = hooks;
                server.hook_summary = summary;
                server.persona = p;
                
                // Auto-fill project name if missing
                if server.project_name.is_empty() {
                    if let Some(name) = path_obj.file_name() {
                        server.project_name = name.to_string_lossy().to_string();
                    }
                }
                
                valid_servers.push(server.clone());
            }
            return valid_servers;
        }
    }
    vec![]
}

#[tauri::command]
fn get_personas() -> Vec<PersonaConfig> {
    let path = get_persona_lib_file();
    if !path.exists() { return vec![]; }
    if let Ok(content) = fs::read_to_string(path) {
        if let Ok(lib) = serde_json::from_str::<PersonaLibrary>(&content) {
            return lib.personas;
        }
    }
    vec![]
}

#[tauri::command]
fn select_server(id: String, state: State<AppState>) {
    let mut current = state.current_server_id.lock().unwrap();
    *current = Some(id);
}

/// 🔧 简化版：直接写入 servers.json，不发 HTTP
#[tauri::command]
fn update_persona(persona: String, state: State<'_, AppState>) -> Result<String, String> {
    let path = get_discovery_file();
    
    // 1. 读取 servers.json 仅为了查找 Project Path
    let content = fs::read_to_string(&path)
        .map_err(|e| format!("Read failed: {}", e))?;
    
    let data: DiscoveryData = serde_json::from_str(&content)
        .map_err(|e| format!("Parse failed: {}", e))?;
    
    // 2. 确定目标 Project Path
    let current_id_opt = state.current_server_id.lock().unwrap().clone();
    let target_server = if let Some(id) = current_id_opt {
        data.servers.into_iter().find(|s| s.id == id)
    } else {
        // Fallback: Default to first server
        data.servers.into_iter().next()
    };
    
    let project_path = match target_server {
        Some(s) => s.project_path,
        None => return Err("No active server found".to_string()),
    };

    if project_path == "Unknown" {
         return Err("Invalid project path".to_string());
    }

    // 3. Update DB
    let db_path = PathBuf::from(&project_path).join(".mcp-data").join("mcp_memory.db");
    
    let conn = Connection::open(&db_path)
        .map_err(|e| format!("DB Open failed: {}", e))?;
        
    // 🔧 CRITICAL: Normalize to lowercase to match Python expectations
    let p_name = persona.to_lowercase();
    
    // Insert or Update the fact (🆕 V6.0: 切换到 system_state)
    conn.execute(
        "INSERT INTO system_state (key, value, category)
         VALUES ('active_persona', ?1, 'state')
         ON CONFLICT(key) DO UPDATE SET
            value = excluded.value,
            updated_at = CURRENT_TIMESTAMP",
        [&p_name],
    ).map_err(|e| format!("DB Write failed: {}", e))?;
    
    Ok(p_name)
}

#[tauri::command]
fn set_window_size(width: f64, height: f64, app_handle: tauri::AppHandle) {
    // 🔧 使用 LogicalSize 而非 PhysicalSize，确保在高 DPI 缩放下尺寸正确
    // LogicalSize 单位与 CSS 像素一致，Tauri 会自动处理 devicePixelRatio 转换
    if let Some(window) = app_handle.get_webview_window("main") {
        let _ = window.set_size(tauri::Size::Logical(tauri::LogicalSize { 
            width, 
            height,
        }));
    }
}

#[tauri::command]
fn exit_app(app_handle: tauri::AppHandle) {
    app_handle.exit(0);
}

// 🆕 获取当前活跃窗口的进程树（用于自动切换 IDE）
// 返回从活跃窗口进程到系统根进程的所有 PID
#[tauri::command]
fn get_foreground_pid() -> Vec<u32> {
    #[cfg(target_os = "windows")]
    {
        use windows::Win32::UI::WindowsAndMessaging::{GetForegroundWindow, GetWindowThreadProcessId};
        
        unsafe {
            let hwnd = GetForegroundWindow();
            if hwnd.0.is_null() { return vec![]; }
            
            let mut pid: u32 = 0;
            GetWindowThreadProcessId(hwnd, Some(&mut pid));
            if pid == 0 { return vec![]; }

            // 🆕 关键修复：如果当前聚焦的是 HUD 进程本身，则返回空，防止点击 HUD 导致误切换
            let self_pid = std::process::id();
            if pid == self_pid { return vec![]; }
            
            // 遍历进程树，收集所有父进程的 PID
            let mut pids = vec![pid];
            let mut current_pid = pid;
            
            for _ in 0..20 { // 最多遍历 20 层
                match get_parent_pid(current_pid) {
                    Some(parent) if parent > 0 && parent != current_pid => {
                        // 如果父进程链中包含 HUD 自身（理论上不应该，但作为二次保险），也直接中断
                        if parent == self_pid { break; }
                        pids.push(parent);
                        current_pid = parent;
                    }
                    _ => break,
                }
            }
            
            pids
        }
    }
    #[cfg(not(target_os = "windows"))]
    {
        vec![]
    }
}

// 辅助函数：获取进程的父进程 PID
#[cfg(target_os = "windows")]
fn get_parent_pid(pid: u32) -> Option<u32> {
    use windows::Win32::System::Diagnostics::ToolHelp::{
        CreateToolhelp32Snapshot, Process32First, Process32Next,
        PROCESSENTRY32, TH32CS_SNAPPROCESS
    };
    
    unsafe {
        let snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0).ok()?;
        
        let mut entry = PROCESSENTRY32 {
            dwSize: std::mem::size_of::<PROCESSENTRY32>() as u32,
            ..Default::default()
        };
        
        if Process32First(snapshot, &mut entry).is_ok() {
            loop {
                if entry.th32ProcessID == pid {
                    let parent = entry.th32ParentProcessID;
                    let _ = windows::Win32::Foundation::CloseHandle(snapshot);
                    return Some(parent);
                }
                if Process32Next(snapshot, &mut entry).is_err() {
                    break;
                }
            }
        }
        
        let _ = windows::Win32::Foundation::CloseHandle(snapshot);
        None
    }
}



#[tauri::command]
fn get_config() -> AppConfig {
    let path = get_config_file_path();
    if path.exists() {
        if let Ok(content) = fs::read_to_string(path) {
            if let Ok(config) = serde_json::from_str::<AppConfig>(&content) {
                return config;
            }
        }
    }
    // 默认为 true
    AppConfig { auto_open: true }
}

#[tauri::command]
fn save_config(config: AppConfig) {
    let path = get_config_file_path();
    if let Some(parent) = path.parent() {
        let _ = fs::create_dir_all(parent);
    }
    if let Ok(json) = serde_json::to_string_pretty(&config) {
        let _ = fs::write(path, json);
    }
}

/// 打开人格编辑器
#[tauri::command]
fn open_persona_editor() -> Result<(), String> {
    // 查找 persona-editor.exe 路径
    let exe_path = std::env::current_exe()
        .map_err(|e| format!("获取当前路径失败: {}", e))?;

    // 从当前 exe 位置推导 bin 目录
    let editor_path = if let Some(parent) = exe_path.parent() {
        // 尝试 ../bin/persona-editor.exe（当前在 bin/目录下）
        let path = parent.join("persona-editor.exe");
        if path.exists() {
            path
        } else {
            // 尝试当前目录的 persona-editor.exe
            exe_path.parent().unwrap().join("persona-editor.exe")
        }
    } else {
        return Err("无法确定路径".to_string());
    };

    // 启动编辑器进程（分离模式，不阻塞 HUD）
    let mut cmd = Command::new(&editor_path);
    #[cfg(target_os = "windows")]
    {
        use std::os::windows::process::CommandExt;
        cmd.creation_flags(0x08000000); // CREATE_NO_WINDOW
    }

    cmd.spawn()
        .map_err(|e| format!("启动编辑器失败: {} (路径: {:?})", e, editor_path))?;

    Ok(())
}

pub fn run() {
    tauri::Builder::default()
        .setup(|app| {
            app.manage(AppState::default());
            
            let window = app.get_webview_window("main").unwrap();
            let _ = window.set_shadow(false);
            let _ = window.set_decorations(false);
            let _ = window.set_always_on_top(true);
            
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            list_servers,
            get_personas,
            select_server,
            update_persona,
            set_window_size,
            exit_app,
            get_foreground_pid,
            get_config,
            save_config,
            open_persona_editor
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
