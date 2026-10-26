//! Tauri commands exposing the `pcs` config operations to the frontend.
//! Names and argument names are the frontend contract.

use std::path::{Path, PathBuf};
use std::time::Duration;

use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use tauri::{AppHandle, State};
use tokio::sync::Mutex;

use crate::sidecar::{hook_args, is_id, run_pcs};

/// Serializes the commands that write config (settings.json, ~/.pcs).
pub struct WriteLock(pub Mutex<()>);

const DEFAULT_TIMEOUT: Duration = Duration::from_secs(15);
const PERM_TIMEOUT: Duration = Duration::from_secs(120);
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
    if !is_id(&id) {
        return Err(format!("invalid permission id: {id}"));
    }
    run_pcs(&app, strs(&["perm", "request", "--id", &id]), PERM_TIMEOUT).await
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
    open_target(&crate::log_dir().join("pcs.log").to_string_lossy())
}

/// What `pcs uninstall --yes` reports.
#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UninstallResult {
    pub removed_hooks: u32,
    pub backup: String,
}

const BUNDLE_NAME: &str = "Personalization Setup.app";

/// The bundle to delete after uninstalling: only the installed copy at
/// `<home>/Applications/Personalization Setup.app`. A dev build (`target/...`)
/// or any other location yields `None`, so it is never removed.
pub fn bundle_to_delete(exe: &Path, home: &Path) -> Option<PathBuf> {
    // <bundle>/Contents/MacOS/<exe>
    let bundle = exe.ancestors().nth(3)?;
    (bundle == home.join("Applications").join(BUNDLE_NAME)).then(|| bundle.to_path_buf())
}

fn home_dir() -> PathBuf {
    // PCS_HOME mirrors `pcs_command`: test runs never reach the real home.
    let var = match std::env::var_os("PCS_HOME") {
        Some(h) if !h.is_empty() => Some(h),
        _ => std::env::var_os("HOME"),
    };
    var.map(PathBuf::from).unwrap_or_default()
}

#[tauri::command]
pub async fn uninstall_app(
    app: AppHandle,
    lock: State<'_, WriteLock>,
) -> Result<UninstallResult, String> {
    let _g = lock.0.lock().await;
    let v = run_pcs(&app, strs(&["uninstall", "--yes"]), DEFAULT_TIMEOUT).await?;
    let result: UninstallResult = serde_json::from_value(v)
        .map_err(|e| format!("unexpected output from pcs uninstall: {e}"))?;

    match std::env::current_exe() {
        Ok(exe) => match bundle_to_delete(&exe, &home_dir()) {
            Some(bundle) => {
                log::info!("uninstall: deleting {} after exit", bundle.display());
                let spawned = std::process::Command::new("/bin/sh")
                    .args(["-c", "sleep 1; rm -rf \"$1\"", "sh"])
                    .arg(&bundle)
                    .stdin(std::process::Stdio::null())
                    .stdout(std::process::Stdio::null())
                    .stderr(std::process::Stdio::null())
                    .spawn();
                if let Err(e) = spawned {
                    log::error!("uninstall: could not start the delete of {}: {e}", bundle.display());
                }
            }
            None => log::info!("uninstall: not running from ~/Applications, leaving the bundle"),
        },
        Err(e) => log::error!("uninstall: cannot find the running executable: {e}"),
    }

    log::info!("uninstall: exiting");
    let handle = app.clone();
    tauri::async_runtime::spawn(async move {
        tokio::time::sleep(Duration::from_millis(300)).await;
        handle.exit(0);
    });
    Ok(result)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn uninstall_target_only_in_applications() {
        let home = Path::new("/Users/me");
        let exe = |bundle: &str| PathBuf::from(format!("{bundle}/Contents/MacOS/personalization-setup"));
        let installed = "/Users/me/Applications/Personalization Setup.app";
        assert_eq!(bundle_to_delete(&exe(installed), home), Some(PathBuf::from(installed)));
        assert_eq!(
            bundle_to_delete(
                &exe("/Users/me/code/app/src-tauri/target/release/bundle/macos/Personalization Setup.app"),
                home
            ),
            None
        );
        assert_eq!(bundle_to_delete(&exe("/Applications/Personalization Setup.app"), home), None);
        assert_eq!(bundle_to_delete(&exe("/Users/me/Applications/Other.app"), home), None);
        assert_eq!(bundle_to_delete(Path::new("/Users/me/Applications/Personalization Setup.app"), home), None);
        assert_eq!(bundle_to_delete(Path::new("x"), home), None);
    }

    #[test]
    fn uninstall_result_is_camel_case() {
        let r: UninstallResult = serde_json::from_str(r#"{"removedHooks":3,"backup":"/x"}"#).unwrap();
        assert_eq!(r.removed_hooks, 3);
        assert_eq!(serde_json::to_value(&r).unwrap(), json!({"removedHooks":3,"backup":"/x"}));
    }

    #[test]
    fn settings_url_allowlist() {
        assert!(is_settings_url(
            "x-apple.systempreferences:com.apple.preference.security?Privacy_Automation"
        ));
        assert!(!is_settings_url("https://evil"));
        assert!(!is_settings_url("file:///etc"));
    }
}
