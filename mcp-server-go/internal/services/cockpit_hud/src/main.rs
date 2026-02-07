// 隐藏 Windows 控制台窗口，仅在非调试构建时生效
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    mcp_cockpit_hud::run();
}
