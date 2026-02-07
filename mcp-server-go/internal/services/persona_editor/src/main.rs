// 人格编辑器 - 独立 Tauri 应用程序
// 功能：人格管理、Prompt 生成、JSON 编辑
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    persona_editor::run()
}
