//! Pure alert bookkeeping: which windows exist and in what order.

use crate::events::Alert;
use crate::layout::{PANEL_H, TOAST_H};
use serde::Serialize;

#[derive(Clone, Copy, Debug, PartialEq, Serialize)]
#[serde(rename_all = "camelCase")]
pub enum Kind {
    Panel,
    Toast,
}

#[derive(Clone, Debug, PartialEq)]
pub struct Shown {
    pub label: String,
    pub kind: Kind,
    pub alert: Alert,
}

#[derive(Debug, PartialEq)]
pub enum Change {
    Open(Shown),
    Update(Shown),
    Close(String),
}

/// Window label for a session's panel: `[A-Za-z0-9-]` only, max 64 chars.
pub fn panel_label(session_id: &str) -> String {
    let clean: String = session_id
        .chars()
        .filter(|c| c.is_ascii_alphanumeric() || *c == '-')
        .take(64)
        .collect();
    if clean.is_empty() {
        "panel-unknown".to_string()
    } else {
        format!("panel-{clean}")
    }
}

#[derive(Default)]
pub struct Board {
    items: Vec<Shown>,
    next_toast: u64,
}

impl Board {
    pub fn panel(&mut self, a: Alert) -> Vec<Change> {
        let label = panel_label(&a.session_id);
        let shown = Shown { label: label.clone(), kind: Kind::Panel, alert: a };
        match self.items.iter_mut().find(|s| s.label == label) {
            Some(slot) => {
                *slot = shown.clone();
                vec![Change::Update(shown)]
            }
            None => {
                self.items.push(shown.clone());
                vec![Change::Open(shown)]
            }
        }
    }
    pub fn toast(&mut self, a: Alert) -> Vec<Change> {
        self.next_toast += 1;
        let shown = Shown {
            label: format!("toast-{}", self.next_toast),
            kind: Kind::Toast,
            alert: a,
        };
        self.items.push(shown.clone());
        vec![Change::Open(shown)]
    }
    pub fn moved_on(&mut self, session_id: &str) -> Vec<Change> {
        let label = panel_label(session_id);
        match self.get(&label) {
            Some(s) if s.kind == Kind::Panel => {
                self.remove(&label);
                vec![Change::Close(label)]
            }
            _ => Vec::new(),
        }
    }
    pub fn remove(&mut self, label: &str) -> Option<Shown> {
        let i = self.items.iter().position(|s| s.label == label)?;
        Some(self.items.remove(i))
    }
    pub fn get(&self, label: &str) -> Option<&Shown> {
        self.items.iter().find(|s| s.label == label)
    }
    pub fn heights(&self) -> Vec<(String, f64)> {
        self.items
            .iter()
            .map(|s| {
                let h = match s.kind {
                    Kind::Panel => PANEL_H,
                    Kind::Toast => TOAST_H,
                };
                (s.label.clone(), h)
            })
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn alert(sid: &str, msg: &str) -> Alert {
        Alert { session_id: sid.into(), message: msg.into(), ..Alert::default() }
    }

    #[test]
    fn panel_new_session_opens() {
        let mut b = Board::default();
        let c = b.panel(alert("a", "hi"));
        assert_eq!(c.len(), 1);
        match &c[0] {
            Change::Open(s) => {
                assert_eq!(s.label, "panel-a");
                assert_eq!(s.kind, Kind::Panel);
            }
            other => panic!("expected Open, got {other:?}"),
        }
    }

    #[test]
    fn panel_same_session_updates() {
        let mut b = Board::default();
        b.panel(alert("a", "one"));
        let c = b.panel(alert("a", "two"));
        assert!(matches!(c.as_slice(), [Change::Update(s)] if s.alert.message == "two"));
        assert_eq!(b.heights().len(), 1);
        assert_eq!(b.get("panel-a").unwrap().alert.message, "two");
    }

    #[test]
    fn two_sessions_two_panels() {
        let mut b = Board::default();
        b.panel(alert("a", "x"));
        b.panel(alert("b", "y"));
        let labels: Vec<_> = b.heights().into_iter().map(|(l, _)| l).collect();
        assert_eq!(labels, vec!["panel-a", "panel-b"]);
    }

    #[test]
    fn moved_on_closes_only_that_session() {
        let mut b = Board::default();
        b.panel(alert("a", "x"));
        b.panel(alert("b", "y"));
        assert_eq!(b.moved_on("a"), vec![Change::Close("panel-a".into())]);
        assert!(b.get("panel-a").is_none());
        assert!(b.get("panel-b").is_some());
    }

    #[test]
    fn moved_on_unknown_is_noop() {
        let mut b = Board::default();
        b.panel(alert("a", "x"));
        assert!(b.moved_on("zzz").is_empty());
        assert_eq!(b.heights().len(), 1);
    }

    #[test]
    fn moved_on_ignores_toasts() {
        let mut b = Board::default();
        b.toast(alert("a", "done"));
        assert!(b.moved_on("a").is_empty());
        assert_eq!(b.heights().len(), 1);
    }

    #[test]
    fn toast_labels_unique() {
        let mut b = Board::default();
        let l = |c: Vec<Change>| match &c[0] {
            Change::Open(s) => s.label.clone(),
            other => panic!("expected Open, got {other:?}"),
        };
        let a = l(b.toast(alert("a", "1")));
        let c = l(b.toast(alert("a", "2")));
        assert_ne!(a, c);
        assert!(a.starts_with("toast-"));
    }

    #[test]
    fn panel_label_sanitises() {
        assert_eq!(panel_label("../x y"), "panel-xy");
        assert_eq!(panel_label(""), "panel-unknown");
        assert_eq!(panel_label(&"a".repeat(100)).len(), "panel-".len() + 64);
    }

    #[test]
    fn board_relayout_after_remove() {
        let mut b = Board::default();
        b.panel(alert("a", "x"));
        b.panel(alert("b", "y"));
        assert!(b.remove("panel-a").is_some());
        assert!(b.heights().iter().all(|(l, _)| l != "panel-a"));
        assert_eq!(b.heights(), vec![("panel-b".to_string(), PANEL_H)]);
        assert!(b.remove("panel-a").is_none());
    }
}
