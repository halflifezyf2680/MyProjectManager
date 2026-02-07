use tauri::Manager;

#[derive(Default)]
pub struct AppState {
    // 占位，可能被 persona 模块覆盖
}

pub mod persona;
pub mod models;
pub mod prompt;    
pub mod discovery; 

// Re-export commands
pub use persona::*;
pub use prompt::*;
pub use discovery::*;

pub fn run() {
    tauri::Builder::default()
        .setup(|app| {
            // 使用 persona 模块的 AppState
            app.manage(persona::AppState::default());

            let window = app.get_webview_window("main").unwrap();
            let _ = window.set_shadow(true);

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            // Persona
            get_personas,
            generate_prompt,
            save_persona,
            delete_persona,
            select_persona,
            format_json,
            validate_json,
            // Discovery
            list_projects,
            resolve_db_path,
            // Prompt
            list_prompts,
            save_prompt_snippet,
            delete_prompt_snippet
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
