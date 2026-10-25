//! Supervises the long-running `pcs serve` child.

use std::sync::Mutex;
use std::time::Duration;

use serde::Serialize;
use tauri::{AppHandle, Emitter, Manager, State};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};

use crate::events::{parse_line, ServeLine};

pub const STATUS_EVENT: &str = "serve-status";
const MAX_LINE: usize = 64 * 1024;

#[derive(Serialize, Clone, Debug, PartialEq)]
#[serde(tag = "state", rename_all = "camelCase")]
pub enum ServeStatus {
    Starting,
    Ready,
    Down { reason: String },
}

/// Delay before restart attempt `attempt` (0-based): 1s doubling, capped at 30s.
pub fn backoff(attempt: u32) -> Duration {
    Duration::from_secs(1u64.checked_shl(attempt).unwrap_or(30).min(30))
}

pub fn status_from_exit(last_error: Option<&str>, code: Option<i32>) -> ServeStatus {
    let reason = if last_error == Some("already running") {
        "Another program is already listening for hook alerts. Quit it and Personalization Setup will retry."
            .to_string()
    } else {
        match code {
            Some(c) => format!("The hook listener stopped (exit {c})."),
            None => "The hook listener stopped.".to_string(),
        }
    };
    ServeStatus::Down { reason }
}

/// Managed state: current status plus the live child. The child must stay
/// stored for the app's whole life: dropping it closes stdin and `pcs serve`
/// exits on stdin EOF.
pub struct ServeState {
    status: Mutex<ServeStatus>,
    child: Mutex<Option<CommandChild>>,
}

fn set_status(app: &AppHandle, status: ServeStatus) {
    let state = app.state::<ServeState>();
    {
        let mut cur = state.status.lock().unwrap();
        if *cur == status {
            return;
        }
        *cur = status.clone();
    }
    log::info!("serve status: {status:?}");
    let _ = app.emit(STATUS_EVENT, status);
}

#[tauri::command]
pub fn serve_status(state: State<'_, ServeState>) -> ServeStatus {
    state.status.lock().unwrap().clone()
}

/// Splits a stdout event into lines. The shell plugin delivers one line per
/// event (newline possibly stripped), but may in principle send several
/// newline-joined lines, so split on `\n`. Events over 64 KiB are dropped.
fn lines_of(chunk: &[u8]) -> Vec<String> {
    if chunk.len() > MAX_LINE {
        return Vec::new();
    }
    String::from_utf8_lossy(chunk)
        .split('\n')
        .map(|l| l.trim_end_matches('\r'))
        .filter(|l| !l.trim().is_empty())
        .map(str::to_string)
        .collect()
}

/// Starts the supervisor: spawns `pcs serve`, forwards parsed lines to
/// `on_line`, and restarts with backoff when it exits.
pub fn start(app: AppHandle, on_line: impl Fn(&AppHandle, ServeLine) + Send + Sync + 'static) {
    app.manage(ServeState {
        status: Mutex::new(ServeStatus::Starting),
        child: Mutex::new(None),
    });
    tauri::async_runtime::spawn(async move {
        let mut attempt: u32 = 0;
        loop {
            set_status(&app, ServeStatus::Starting);
            let spawned = crate::pcs_command(&app)
                .and_then(|c| c.args(["serve"]).spawn().map_err(|e| e.to_string()));
            let mut last_error: Option<String> = None;
            let mut code: Option<i32> = None;
            match spawned {
                Ok((mut rx, child)) => {
                    *app.state::<ServeState>().child.lock().unwrap() = Some(child);
                    while let Some(ev) = rx.recv().await {
                        match ev {
                            CommandEvent::Stdout(b) => {
                                for line in lines_of(&b) {
                                    let Some(parsed) = parse_line(&line) else { continue };
                                    match &parsed {
                                        ServeLine::Ready => {
                                            attempt = 0;
                                            set_status(&app, ServeStatus::Ready);
                                        }
                                        ServeLine::Error(m) => last_error = Some(m.clone()),
                                        _ => {}
                                    }
                                    on_line(&app, parsed);
                                }
                            }
                            CommandEvent::Stderr(b) => {
                                log::warn!("pcs serve stderr: {}", String::from_utf8_lossy(&b).trim());
                            }
                            CommandEvent::Terminated(p) => code = p.code,
                            _ => {}
                        }
                    }
                    *app.state::<ServeState>().child.lock().unwrap() = None;
                }
                Err(e) => log::error!("pcs serve: failed to start: {e}"),
            }
            let status = status_from_exit(last_error.as_deref(), code);
            log::warn!("pcs serve exited: {status:?}");
            set_status(&app, status);
            tokio::time::sleep(backoff(attempt)).await;
            attempt = attempt.saturating_add(1);
        }
    });
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn lines_of_splits_and_drops() {
        assert_eq!(lines_of(b"a\nb\r\n\n"), vec!["a", "b"]);
        assert_eq!(lines_of(b"{\"x\":1}"), vec!["{\"x\":1}"]);
        assert!(lines_of(&vec![b'x'; MAX_LINE + 1]).is_empty());
    }

    #[test]
    fn backoff_doubles_to_cap() {
        let got: Vec<u64> = (0..=6).map(|a| backoff(a).as_secs()).collect();
        assert_eq!(got, vec![1, 2, 4, 8, 16, 30, 30]);
    }

    #[test]
    fn status_from_exit_already_running() {
        match status_from_exit(Some("already running"), Some(1)) {
            ServeStatus::Down { reason } => {
                assert!(reason.contains("Another program is already listening"), "{reason}")
            }
            s => panic!("{s:?}"),
        }
    }

    #[test]
    fn status_from_exit_plain() {
        assert_eq!(
            status_from_exit(None, Some(3)),
            ServeStatus::Down { reason: "The hook listener stopped (exit 3).".into() }
        );
    }
}
