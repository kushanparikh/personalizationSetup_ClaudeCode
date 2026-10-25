//! Tauri commands exposing the `pcs` config operations to the frontend.
//! Names and argument names are the frontend contract.

use std::time::Duration;

use serde_json::{json, Value};
use tauri::{AppHandle, State};
use tokio::sync::Mutex;

use crate::sidecar::{hook_args, run_pcs};

/// Serializes the commands that write config (settings.json, ~/.pcs).
pub struct WriteLock(pub Mutex<()>);

const DEFAULT_TIMEOUT: Duration = Duration::from_secs(30);
const PERM_TIMEOUT: Duration = Duration::from_secs(120);
const PERM_ID: &str = "automation-terminal";
const SETTINGS_PREFIX: &str = "x-apple.systempreferences:";

pub fn is_settings_url(url: &str) -> bool {
    url.starts_with(SETTINGS_PREFIX)
}

fn strs(v: &[&str]) -> Vec<String> {
    v.iter().map(|s| s.to_string()).collect()
}

fn open_target(target: &str) -> Result<Value, String> {
    let status = std::process::Command::new("open")
        .arg(target)
        .status()
        .map_err(|e| format!("failed to run open: {e}"))?;
    if status.success() {
        Ok(json!({}))
    } else {
        Err(format!("open failed ({status})"))
    }
}

#[tauri::command]
pub async fn get_state(app: AppHandle) -> Result<Value, String> {
    run_pcs(&app, strs(&["state"]), DEFAULT_TIMEOUT).await
}

#[tauri::command]
pub async fn apply_hook(
    app: AppHandle,
    lock: State<'_, WriteLock>,
    hook_type: String,
    answers: Value,
) -> Result<Value, String> {
    let args = hook_args("apply", &hook_type, Some(&answers))?;
    let _g = lock.0.lock().await;
    run_pcs(&app, args, DEFAULT_TIMEOUT).await
}

#[tauri::command]
pub async fn enable_hook(
    app: AppHandle,
    lock: State<'_, WriteLock>,
    hook_type: String,
) -> Result<Value, String> {
    let args = hook_args("enable", &hook_type, None)?;
    let _g = lock.0.lock().await;
    run_pcs(&app, args, DEFAULT_TIMEOUT).await
}

#[tauri::command]
pub async fn disable_hook(
    app: AppHandle,
    lock: State<'_, WriteLock>,
    hook_type: String,
) -> Result<Value, String> {
    let args = hook_args("disable", &hook_type, None)?;
    let _g = lock.0.lock().await;
    run_pcs(&app, args, DEFAULT_TIMEOUT).await
}

#[tauri::command]
pub async fn remove_hook(
    app: AppHandle,
    lock: State<'_, WriteLock>,
    hook_type: String,
) -> Result<Value, String> {
    let args = hook_args("remove", &hook_type, None)?;
    let _g = lock.0.lock().await;
    run_pcs(&app, args, DEFAULT_TIMEOUT).await
}

#[tauri::command]
pub async fn request_permission(app: AppHandle, id: String) -> Result<Value, String> {
    if id != PERM_ID {
        return Err(format!("unknown permission id: {id}"));
    }
    run_pcs(&app, strs(&["perm", "request", "--id", PERM_ID]), PERM_TIMEOUT).await
}

#[tauri::command]
pub async fn send_test(app: AppHandle, hook_type: String) -> Result<Value, String> {
    let args = hook_args("send-test", &hook_type, None)?;
    run_pcs(&app, args, DEFAULT_TIMEOUT).await
}

#[tauri::command]
pub fn open_settings(url: String) -> Result<Value, String> {
    if !is_settings_url(&url) {
        return Err("URL not allowed".to_string());
    }
    open_target(&url)
}

#[tauri::command]
pub fn open_log() -> Result<Value, String> {
    let home = std::env::var("HOME").map_err(|_| "HOME is not set".to_string())?;
    open_target(&format!("{home}/Library/Logs/Personalization Setup/pcs.log"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn settings_url_allowlist() {
        assert!(is_settings_url(
            "x-apple.systempreferences:com.apple.preference.security?Privacy_Automation"
        ));
        assert!(!is_settings_url("https://evil"));
        assert!(!is_settings_url("file:///etc"));
    }
}
