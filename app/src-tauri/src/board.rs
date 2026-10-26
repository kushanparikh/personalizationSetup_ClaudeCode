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

impl Kind {
    pub fn height(self) -> f64 {
        match self {
            Kind::Panel => PANEL_H,
            Kind::Toast => TOAST_H,
        }
    }
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

#[derive(Default)]
pub struct Board {
    items: Vec<Shown>,
    next_toast: u64,
    next_panel: u64,
}

impl Board {
    /// Update the session's open panel in place, or open a new one under a
    /// fresh label (a destroyed label stays registered for a moment).
    pub fn panel(&mut self, a: Alert) -> Vec<Change> {
        if let Some(slot) = self
            .items
            .iter_mut()
            .find(|s| s.kind == Kind::Panel && s.alert.session_id == a.session_id)
        {
            slot.alert = a;
            return vec![Change::Update(slot.clone())];
        }
        self.next_panel += 1;
        let shown = Shown {
            label: format!("panel-{}", self.next_panel),
            kind: Kind::Panel,
            alert: a,
        };
        self.items.push(shown.clone());
        vec![Change::Open(shown)]
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
        let Some(i) = self
            .items
            .iter()
            .position(|s| s.kind == Kind::Panel && s.alert.session_id == session_id)
        else {
            return Vec::new();
        };
        vec![Change::Close(self.items.remove(i).label)]
    }
    pub fn remove(&mut self, label: &str) -> Option<Shown> {
        let i = self.items.iter().position(|s| s.label == label)?;
        Some(self.items.remove(i))
    }
    pub fn get(&self, label: &str) -> Option<&Shown> {
        self.items.iter().find(|s| s.label == label)
    }
    /// Panels on the board (the amber count).
    pub fn open_panels(&self) -> usize {
        self.items.iter().filter(|s| s.kind == Kind::Panel).count()
    }
    /// Toasts on the board with no session id ("Send test"): they light
    /// green only while shown.
    pub fn sessionless_toasts(&self) -> usize {
        self.items
            .iter()
            .filter(|s| s.kind == Kind::Toast && s.alert.session_id.is_empty())
            .count()
    }
    pub fn heights(&self) -> Vec<(String, f64)> {
        self.items
            .iter()
            .map(|s| {
                (s.label.clone(), s.kind.height())
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
    fn light_counts_from_board() {
        let mut b = Board::default();
        assert_eq!((b.open_panels(), b.sessionless_toasts()), (0, 0));
        b.panel(alert("a", "x"));
        b.panel(alert("a", "again"));
        b.panel(alert("b", "y"));
        b.toast(alert("c", "done"));
        b.toast(alert("", "test"));
        assert_eq!((b.open_panels(), b.sessionless_toasts()), (2, 1));
        b.moved_on("a");
        b.remove("toast-2");
        assert_eq!((b.open_panels(), b.sessionless_toasts()), (1, 0));
    }

    #[test]
    fn panel_new_session_opens() {
        let mut b = Board::default();
        let c = b.panel(alert("a", "hi"));
        assert_eq!(c.len(), 1);
        match &c[0] {
            Change::Open(s) => {
                assert_eq!(s.label, "panel-1");
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
        assert_eq!(b.get("panel-1").unwrap().alert.message, "two");
    }

    #[test]
    fn two_sessions_two_panels() {
        let mut b = Board::default();
        b.panel(alert("a", "x"));
        b.panel(alert("b", "y"));
        let labels: Vec<_> = b.heights().into_iter().map(|(l, _)| l).collect();
        assert_eq!(labels, vec!["panel-1", "panel-2"]);
    }

    #[test]
    fn moved_on_closes_only_that_session() {
        let mut b = Board::default();
        b.panel(alert("a", "x"));
        b.panel(alert("b", "y"));
        assert_eq!(b.moved_on("a"), vec![Change::Close("panel-1".into())]);
        assert!(b.get("panel-1").is_none());
        assert!(b.get("panel-2").is_some());
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
    fn board_relayout_after_remove() {
        let mut b = Board::default();
        b.panel(alert("a", "x"));
        b.panel(alert("b", "y"));
        assert!(b.remove("panel-1").is_some());
        assert!(b.heights().iter().all(|(l, _)| l != "panel-1"));
        assert_eq!(b.heights(), vec![("panel-2".to_string(), PANEL_H)]);
        assert!(b.remove("panel-1").is_none());
    }

    fn open_label(c: &[Change]) -> String {
        match c {
            [Change::Open(s)] => s.label.clone(),
            other => panic!("expected Open, got {other:?}"),
        }
    }

    #[test]
    fn panel_reopen_after_moved_on_gets_new_label() {
        let mut b = Board::default();
        let first = open_label(&b.panel(alert("a", "x")));
        assert_eq!(b.moved_on("a"), vec![Change::Close(first.clone())]);
        let second = open_label(&b.panel(alert("a", "y")));
        assert_ne!(first, second);
        assert!(second.starts_with("panel-"));
        assert!(b.get(&first).is_none());
        assert_eq!(b.get(&second).unwrap().alert.message, "y");
    }

    #[test]
    fn update_keeps_position_among_several() {
        let mut b = Board::default();
        b.panel(alert("a", "1"));
        b.panel(alert("b", "2"));
        b.panel(alert("c", "3"));
        let c = b.panel(alert("b", "2b"));
        assert!(matches!(c.as_slice(), [Change::Update(s)] if s.label == "panel-2"));
        let labels: Vec<_> = b.heights().into_iter().map(|(l, _)| l).collect();
        assert_eq!(labels, vec!["panel-1", "panel-2", "panel-3"]);
        assert_eq!(b.get("panel-2").unwrap().alert.message, "2b");
    }

    #[test]
    fn long_session_ids_differing_after_64_chars_stay_separate() {
        let mut b = Board::default();
        let base = "a".repeat(64);
        b.panel(alert(&format!("{base}1"), "x"));
        let c = b.panel(alert(&format!("{base}2"), "y"));
        assert!(matches!(c.as_slice(), [Change::Open(_)]));
        assert_eq!(b.heights().len(), 2);
        assert_eq!(b.moved_on(&format!("{base}1")).len(), 1);
        assert_eq!(b.heights().len(), 1);
    }
}
