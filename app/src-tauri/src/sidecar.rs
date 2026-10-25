//! Running the `pcs` sidecar and interpreting its one-JSON-object output.

use std::time::Duration;

use serde_json::Value;
use tauri::AppHandle;
use tauri_plugin_shell::process::CommandEvent;

pub const HOOK_TYPES: [&str; 3] = ["git-status", "finished", "input-needed"];

/// Interprets `pcs` output: exactly one JSON object on stdout; on failure it
/// is `{"error": "..."}` with a non-zero exit.
pub fn parse_output(code: Option<i32>, stdout: &[u8]) -> Result<Value, String> {
    let text = String::from_utf8_lossy(stdout);
    let parsed: Option<Value> = serde_json::from_str(text.trim()).ok();
    if let Some(Value::Object(obj)) = &parsed {
        if let Some(msg) = obj.get("error").and_then(Value::as_str) {
            return Err(msg.to_string());
        }
    }
    match (code, parsed) {
        (Some(0), Some(v @ Value::Object(_))) => Ok(v),
        (Some(0), _) => Err(format!("unexpected output from pcs: {}", text.trim())),
        (Some(c), _) => Err(format!("pcs failed (exit {c})")),
        (None, _) => Err("pcs failed (terminated by signal)".to_string()),
    }
}

/// Builds argv for a hook command (`apply`, `enable`, `disable`, `remove`,
/// `send-test`), rejecting unknown hook types.
pub fn hook_args(
    cmd: &str,
    hook_type: &str,
    answers: Option<&Value>,
) -> Result<Vec<String>, String> {
    if !HOOK_TYPES.contains(&hook_type) {
        return Err(format!("unknown hook type: {hook_type}"));
    }
    let mut v = vec![cmd.to_string(), "--type".to_string(), hook_type.to_string()];
    if let Some(a) = answers {
        v.push("--answers".to_string());
        v.push(serde_json::to_string(a).map_err(|e| e.to_string())?);
    }
    Ok(v)
}

/// Runs `pcs <args>` and parses its output. On timeout the child is killed
/// explicitly: dropping the plugin's `CommandChild` does not kill it.
pub async fn run_pcs(
    app: &AppHandle,
    args: Vec<String>,
    timeout: Duration,
) -> Result<Value, String> {
    let label = args.join(" ");
    let result = run_inner(app, args, timeout).await;
    match &result {
        Ok(_) => log::info!("pcs {label}: ok"),
        Err(e) => log::error!("pcs {label}: {e}"),
    }
    result
}

async fn run_inner(app: &AppHandle, args: Vec<String>, timeout: Duration) -> Result<Value, String> {
    let first = args.first().cloned().unwrap_or_default();
    let (mut rx, child) = crate::pcs_command(app)?
        .args(args)
        .spawn()
        .map_err(|e| format!("failed to start pcs: {e}"))?;

    let collect = async {
        let mut code = None;
        let mut stdout: Vec<u8> = Vec::new();
        while let Some(ev) = rx.recv().await {
            match ev {
                CommandEvent::Stdout(b) => {
                    stdout.extend(b);
                    stdout.push(b'\n');
                }
                CommandEvent::Terminated(p) => code = p.code,
                _ => {}
            }
        }
        (code, stdout)
    };

    match tokio::time::timeout(timeout, collect).await {
        Ok((code, stdout)) => parse_output(code, &stdout),
        Err(_) => {
            let _ = child.kill();
            Err(format!("pcs {first} timed out after {}s", timeout.as_secs()))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn parse_ok_object() {
        let v = parse_output(Some(0), b"{\"hooks\":[]}\n").unwrap();
        assert!(v["hooks"].is_array());
    }

    #[test]
    fn parse_error_object() {
        let e = parse_output(Some(1), b"{\"error\":\"settings.json is not valid JSON\"}")
            .unwrap_err();
        assert_eq!(e, "settings.json is not valid JSON");
    }

    #[test]
    fn parse_garbage() {
        assert!(parse_output(Some(0), b"oops").unwrap_err().contains("unexpected output"));
    }

    #[test]
    fn parse_nonzero_without_error() {
        assert!(parse_output(Some(2), b"{}").unwrap_err().contains("exit 2"));
    }

    #[test]
    fn hook_args_apply() {
        let a = json!({"sound":"Glass","clickToFocus":true,"untrackedDirty":false});
        let got = hook_args("apply", "finished", Some(&a)).unwrap();
        assert_eq!(
            got,
            vec!["apply", "--type", "finished", "--answers", &serde_json::to_string(&a).unwrap()]
        );
        assert!(!got[4].contains(' '));
    }

    #[test]
    fn hook_args_rejects_unknown_type() {
        assert!(hook_args("enable", "rm -rf", None).is_err());
    }
}
