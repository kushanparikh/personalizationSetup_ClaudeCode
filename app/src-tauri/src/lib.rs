//! Personalization Setup: app setup, window lifecycle, and launcher install.

use std::path::{Path, PathBuf};

pub mod alerts;
mod board;
pub mod commands;
pub mod events;
mod layout;
mod macos;
pub mod serve;
pub mod sidecar;
mod sound;

use tauri::{AppHandle, Manager, RunEvent, WindowEvent};
use tauri_plugin_log::{Target, TargetKind};
use tauri_plugin_shell::{process::Command, ShellExt};

const MAIN_WINDOW: &str = "main";

/// True when the app was started with `--background` (as `pcs` launches it):
/// stay running without showing the config window.
pub fn is_background(args: &[String]) -> bool {
    args.iter().skip(1).any(|a| a == "--background")
}

/// The bundled `pcs` sidecar lives next to the app executable
/// (`Contents/MacOS/pcs`).
pub fn launcher_bin(exe: &Path) -> PathBuf {
    exe.parent().unwrap_or_else(|| Path::new("")).join("pcs")
}

/// Builds a `pcs` sidecar command. Every `pcs` child the app spawns must go
/// through here: when `PCS_HOME` is set, the child gets `HOME=$PCS_HOME`, so
/// test runs never touch the user's real `~/.pcs` or `~/.claude`.
pub(crate) fn pcs_command(app: &AppHandle) -> Result<Command, String> {
    let cmd = app.shell().sidecar("pcs").map_err(|e| e.to_string())?;
    Ok(match std::env::var_os("PCS_HOME") {
        Some(home) if !home.is_empty() => cmd.env("HOME", home),
        _ => cmd,
    })
}

fn log_dir() -> PathBuf {
    let home = std::env::var_os("HOME").map(PathBuf::from).unwrap_or_default();
    home.join("Library/Logs/Personalization Setup")
}

fn show_main(app: &AppHandle) {
    if let Some(w) = app.get_webview_window(MAIN_WINDOW) {
        let _ = w.show();
        let _ = w.unminimize();
        let _ = w.set_focus();
    }
}

/// Points `~/.pcs/bin/pcs-hook` at this bundle's `pcs`. Logged, never fatal.
async fn install_launcher(app: AppHandle) {
    let bin = match std::env::current_exe() {
        Ok(exe) => launcher_bin(&exe),
        Err(e) => {
            log::error!("install-launcher: cannot resolve current exe: {e}");
            return;
        }
    };
    let cmd = match pcs_command(&app) {
        Ok(c) => c.args(["install-launcher", "--bin", &bin.to_string_lossy()]),
        Err(e) => {
            log::error!("install-launcher: {e}");
            return;
        }
    };
    match cmd.output().await {
        Ok(out) => {
            let stdout = String::from_utf8_lossy(&out.stdout);
            let stderr = String::from_utf8_lossy(&out.stderr);
            if out.status.success() {
                log::info!("install-launcher --bin {}: {}", bin.display(), stdout.trim());
            } else {
                log::error!(
                    "install-launcher --bin {} failed (exit {:?}): {} {}",
                    bin.display(),
                    out.status.code(),
                    stdout.trim(),
                    stderr.trim()
                );
            }
        }
        Err(e) => log::error!("install-launcher: spawn failed: {e}"),
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let background = is_background(&std::env::args().collect::<Vec<_>>());

    tauri::Builder::default()
        .plugin(
            tauri_plugin_log::Builder::new()
                .clear_targets()
                .target(Target::new(TargetKind::Stdout))
                .target(Target::new(TargetKind::Folder {
                    path: log_dir(),
                    file_name: Some("app".into()),
                }))
                .level(log::LevelFilter::Info)
                .build(),
        )
        .plugin(tauri_plugin_shell::init())
        .manage(commands::WriteLock(tokio::sync::Mutex::new(())))
        .manage(alerts::Alerts::default())
        .invoke_handler(tauri::generate_handler![
            commands::get_state,
            commands::apply_hook,
            commands::enable_hook,
            commands::disable_hook,
            commands::remove_hook,
            commands::request_permission,
            commands::send_test,
            commands::open_settings,
            commands::open_log,
            serve::serve_status,
            alerts::alert_get,
            alerts::alert_click,
            alerts::alert_dismiss,
        ])
        .setup(move |app| {
            log::info!("starting (background={background})");
            tauri::async_runtime::spawn(install_launcher(app.handle().clone()));
            serve::start(app.handle().clone(), |app, line| {
                log::info!("serve line: {line:?}");
                alerts::handle(app, line);
            });
            if !background {
                show_main(app.handle());
            }
            Ok(())
        })
        .on_window_event(|window, event| {
            // Closing the config window only hides it; the app keeps running.
            if let WindowEvent::CloseRequested { api, .. } = event {
                if window.label() == MAIN_WINDOW {
                    api.prevent_close();
                    let _ = window.hide();
                }
            }
        })
        .build(tauri::generate_context!())
        .expect("error while building tauri application")
        .run(|app, event| {
            // Dock click brings the config window back.
            if let RunEvent::Reopen { .. } = event {
                show_main(app);
            }
        });
}

#[cfg(test)]
mod tests {
    use super::*;

    fn args(v: &[&str]) -> Vec<String> {
        v.iter().map(|s| s.to_string()).collect()
    }

    #[test]
    fn is_background_detects_flag() {
        assert!(is_background(&args(&["app", "--background"])));
        assert!(!is_background(&args(&["app"])));
        assert!(!is_background(&args(&["app", "-psn_0_1"])));
    }

    #[test]
    fn launcher_bin_is_sibling() {
        assert_eq!(
            launcher_bin(Path::new("/A.app/Contents/MacOS/personalization-setup")),
            PathBuf::from("/A.app/Contents/MacOS/pcs")
        );
    }
}
