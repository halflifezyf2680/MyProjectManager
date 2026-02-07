use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf}; 

// Reuse the AppState from persona (or move it to lib.rs fully) for shared state.
// Actually lib.rs defines AppState as empty placeholder but persona.rs defines it with selected_persona.
// We should perhaps unify AppState in lib.rs or models.rs.
// For now, let's just make sure we can read the config.

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct ServerInfo {
    pub id: String,
    pub port: u16,
    pub project_path: String,
    // We only care about id and path for discovery
    #[serde(default)]
    pub project_name: String,
}

#[derive(Serialize, Deserialize)]
pub struct DiscoveryData {
    pub servers: Vec<ServerInfo>,
}

fn get_discovery_file() -> PathBuf {
    dirs::home_dir()
        .unwrap_or_default()
        .join(".mcp-cockpit")
        .join("servers.json")
}

#[tauri::command]
pub fn list_projects() -> Vec<ServerInfo> {
    let path = get_discovery_file();
    let mut servers = if path.exists() {
         if let Ok(content) = fs::read_to_string(&path) {
             if let Ok(data) = serde_json::from_str::<DiscoveryData>(&content) {
                 data.servers
             } else {
                 vec![]
             }
         } else {
             vec![]
         }
    } else {
        vec![]
    };

    // Auto-fill project names
    for server in &mut servers {
        if server.project_name.is_empty() {
            let path_obj = Path::new(&server.project_path);
            if let Some(name) = path_obj.file_name() {
                server.project_name = name.to_string_lossy().to_string();
            }
        }
    }
    
    // Add "Global" entry? No, separate concept. UI will handle "Global" vs "Project" switch.
    
    servers
}

/// 解析 SQLite 数据库路径 (核心逻辑)
/// project_path: 空字符串表示 Global, 否则为项目绝对路径
/// 返回: (db_path, is_valid)
#[tauri::command]
pub fn resolve_db_path(project_path: String) -> (String, bool) {
    if project_path == "global" || project_path.is_empty() {
        // Global Path Strategy
        return resolve_global_path();
    }
    
    // Project Path Strategy
    let path_obj = Path::new(&project_path);
    if !path_obj.exists() {
        return ("".to_string(), false);
    }
    
    let db_path = path_obj.join(".mcp-data").join("prompt_snippets.db");
    
    // Ensure dir exists
    if let Some(parent) = db_path.parent() {
        let _ = fs::create_dir_all(parent);
    }
    
    (db_path.to_string_lossy().to_string(), true)
}

fn resolve_global_path() -> (String, bool) {
    let exe_path = std::env::current_exe().unwrap_or_default();
    
    // 1. Try Production: ../.mcp-data/ (Assuming exe in bin/)
    if let Some(bin_dir) = exe_path.parent() {
         if let Some(root_dir) = bin_dir.parent() {
             let global_db = root_dir.join(".mcp-data").join("prompt_snippets.db");
             // Create if not exists logic will be in prompt.rs, here we just return path
             // But we should try to create dir at least to be safe
             if let Some(parent) = global_db.parent() {
                 let _ = fs::create_dir_all(parent);
             }
             return (global_db.to_string_lossy().to_string(), true);
         }
    }
    
    // 2. Try Dev: Use project root (.mcp-data inside mcp-server-go)
    // Find go.mod to locate root
    let root = exe_path.ancestors().find(|p| p.join("go.mod").exists());
    if let Some(project_root) = root {
        let global_db = project_root.join(".mcp-data").join("prompt_snippets.db");
        if let Some(parent) = global_db.parent() {
             let _ = fs::create_dir_all(parent);
         }
        return (global_db.to_string_lossy().to_string(), true);
    }

    // Fallback: Use home dir?
    let home = dirs::home_dir().unwrap_or(PathBuf::from("."));
    let global_db = home.join(".mcp-cockpit").join("global_prompts.db");
     if let Some(parent) = global_db.parent() {
         let _ = fs::create_dir_all(parent);
     }
    (global_db.to_string_lossy().to_string(), true)
}
