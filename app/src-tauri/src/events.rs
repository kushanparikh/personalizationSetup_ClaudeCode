//! Lines emitted by `pcs serve` on stdout.

use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Deserialize, Serialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "camelCase", default)]
pub struct Alert {
    pub session_id: String,
    pub title: String,
    pub message: String,
    pub sound: String,
    pub tty: String,
    pub bundle_id: String,
    pub focus: bool,
}

impl Default for Alert {
    fn default() -> Self {
        Alert {
            session_id: String::new(),
            title: String::new(),
            message: String::new(),
            sound: String::new(),
            tty: String::new(),
            bundle_id: String::new(),
            focus: false,
        }
    }
}

#[derive(Clone, Debug, PartialEq)]
pub enum ServeLine {
    Ready,
    Banner(Alert),
    Panel(Alert),
    MovedOn(String),
    Error(String),
}

/// Parses one stdout line; `None` for anything that is not a v1 event.
pub fn parse_line(line: &str) -> Option<ServeLine> {
    let v: Value = serde_json::from_str(line.trim()).ok()?;
    if let Some(msg) = v.get("error").and_then(Value::as_str) {
        return Some(ServeLine::Error(msg.to_string()));
    }
    if v.get("v").and_then(Value::as_u64) != Some(1) {
        return None;
    }
    let kind = v.get("kind").and_then(Value::as_str)?.to_string();
    if kind == "ready" {
        return Some(ServeLine::Ready);
    }
    let alert: Alert = serde_json::from_value(v).ok()?;
    match kind.as_str() {
        "banner" => Some(ServeLine::Banner(alert)),
        "panel" if !alert.session_id.is_empty() => Some(ServeLine::Panel(alert)),
        "moved_on" if !alert.session_id.is_empty() => Some(ServeLine::MovedOn(alert.session_id)),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_ready() {
        assert_eq!(parse_line(r#"{"v":1,"kind":"ready","focus":false}"#), Some(ServeLine::Ready));
    }

    #[test]
    fn parse_panel() {
        let l = r#"{"v":1,"kind":"panel","sessionId":"spike1","title":"Claude Code — spike","message":"Needs input","sound":"Glass","tty":"/dev/ttys003","bundleId":"com.apple.Terminal","focus":true}"#;
        match parse_line(l) {
            Some(ServeLine::Panel(a)) => {
                assert_eq!(a.session_id, "spike1");
                assert_eq!(a.tty, "/dev/ttys003");
                assert_eq!(a.bundle_id, "com.apple.Terminal");
                assert!(a.focus);
            }
            other => panic!("got {other:?}"),
        }
    }

    #[test]
    fn parse_banner_without_session() {
        let l = r#"{"v":1,"kind":"banner","title":"Claude Code — x","message":"Finished working","focus":false}"#;
        match parse_line(l) {
            Some(ServeLine::Banner(a)) => {
                assert_eq!(a.session_id, "");
                assert_eq!(a.message, "Finished working");
            }
            other => panic!("got {other:?}"),
        }
    }

    #[test]
    fn parse_moved_on() {
        assert_eq!(
            parse_line(r#"{"v":1,"kind":"moved_on","sessionId":"s1","focus":false}"#),
            Some(ServeLine::MovedOn("s1".into()))
        );
    }

    #[test]
    fn parse_error_line() {
        assert_eq!(
            parse_line(r#"{"error":"already running"}"#),
            Some(ServeLine::Error("already running".into()))
        );
    }

    #[test]
    fn parse_ignores() {
        for l in [
            "{}",
            r#"{"v":2,"kind":"panel","sessionId":"a"}"#,
            r#"{"v":1,"kind":"weird"}"#,
            "not json",
            r#"{"v":1,"kind":"panel","title":"t"}"#,
        ] {
            assert_eq!(parse_line(l), None, "{l}");
        }
    }
}
