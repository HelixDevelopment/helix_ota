use tauri::Manager;

/// Opens a file picker dialog and returns the selected path.
#[tauri::command]
fn pick_update_package(app: tauri::AppHandle) -> Result<String, String> {
    // Placeholder for native file dialog integration
    Err("Not implemented yet".to_string())
}

/// Reads a local update package metadata.
#[tauri::command]
fn read_package_metadata(path: String) -> Result<serde_json::Value, String> {
    let content = std::fs::read_to_string(&path)
        .map_err(|e| format!("Failed to read {}: {}", path, e))?;
    serde_json::from_str(&content)
        .map_err(|e| format!("Failed to parse metadata: {}", e))
}

/// Returns the system's temporary directory path.
#[tauri::command]
fn temp_dir() -> String {
    std::env::temp_dir().to_string_lossy().to_string()
}

pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_fs::init())
        .plugin(tauri_plugin_shell::init())
        .invoke_handler(tauri::generate_handler![
            pick_update_package,
            read_package_metadata,
            temp_dir,
        ])
        .run(tauri::generate_context!())
        .expect("error while running ota-manager");
}
