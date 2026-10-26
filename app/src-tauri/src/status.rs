//! Pure status-light model: which sessions have finished and not been revisited.
//! No Tauri types; `alerts` feeds it and `tray_light` draws the result.

use std::collections::BTreeSet;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Light {
    None,
    Done,
    Wait,
}

#[derive(Debug, Default)]
pub struct Status {
    finished: BTreeSet<String>,
}

impl Status {
    /// A finished toast arrived. Sessionless toasts are tracked by the caller
    /// (only while visible), so they are not recorded here.
    pub fn toast(&mut self, session_id: &str) {
        if !session_id.is_empty() {
            self.finished.insert(session_id.to_string());
        }
    }

    pub fn moved_on(&mut self, session_id: &str) {
        self.finished.remove(session_id);
    }

    pub fn panel_opened(&mut self, session_id: &str) {
        self.finished.remove(session_id);
    }

    pub fn toast_clicked(&mut self, session_id: &str) {
        self.finished.remove(session_id);
    }

    pub fn menu_opened(&mut self) {
        self.finished.clear();
    }

    pub fn light(&self, open_panels: usize, sessionless_toasts_visible: usize) -> Light {
        if open_panels > 0 {
            Light::Wait
        } else if !self.finished.is_empty() || sessionless_toasts_visible > 0 {
            Light::Done
        } else {
            Light::None
        }
    }

    pub fn tooltip(&self, light: Light, open_panels: usize) -> String {
        const BASE: &str = "Personalization Setup";
        let plural = |n: usize| if n == 1 { "session" } else { "sessions" };
        match light {
            Light::None => BASE.to_string(),
            Light::Wait => format!("{BASE}: {open_panels} {} waiting on you", plural(open_panels)),
            Light::Done => {
                let n = self.finished.len().max(1);
                format!("{BASE}: {n} {} finished", plural(n))
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn status_none_at_start() {
        let s = Status::default();
        assert_eq!(s.light(0, 0), Light::None);
        assert_eq!(s.tooltip(Light::None, 0), "Personalization Setup");
    }

    #[test]
    fn status_toast_then_moved_on_is_none() {
        let mut s = Status::default();
        s.toast("s1");
        s.moved_on("s1");
        assert_eq!(s.light(0, 0), Light::None);
    }

    #[test]
    fn status_finished_stays_after_toast_fades() {
        let mut s = Status::default();
        s.toast("s1");
        assert_eq!(s.light(0, 0), Light::Done);
    }

    #[test]
    fn status_panel_takes_precedence_then_falls_back() {
        let mut s = Status::default();
        s.toast("s1");
        s.panel_opened("s2");
        assert_eq!(s.light(1, 0), Light::Wait);
        assert_eq!(s.light(0, 0), Light::Done);
    }

    #[test]
    fn status_panel_for_finished_session_clears_it() {
        let mut s = Status::default();
        s.toast("s1");
        s.panel_opened("s1");
        assert_eq!(s.light(0, 0), Light::None);
        s.toast("s1");
        s.toast_clicked("s1");
        assert_eq!(s.light(0, 0), Light::None);
    }

    #[test]
    fn status_menu_opened_clears_all() {
        let mut s = Status::default();
        s.toast("s1");
        s.toast("s2");
        s.menu_opened();
        assert_eq!(s.light(0, 0), Light::None);
    }

    #[test]
    fn status_sessionless_toast_only_while_visible() {
        let mut s = Status::default();
        s.toast("");
        assert_eq!(s.light(0, 1), Light::Done);
        assert_eq!(s.light(0, 0), Light::None);
        assert_eq!(s.tooltip(Light::Done, 0), "Personalization Setup: 1 session finished");
    }

    #[test]
    fn status_tooltip_copy() {
        let mut s = Status::default();
        s.toast("a");
        assert_eq!(s.tooltip(Light::Done, 0), "Personalization Setup: 1 session finished");
        s.toast("b");
        assert_eq!(s.tooltip(Light::Done, 0), "Personalization Setup: 2 sessions finished");
        assert_eq!(s.tooltip(Light::Wait, 1), "Personalization Setup: 1 session waiting on you");
        assert_eq!(s.tooltip(Light::Wait, 3), "Personalization Setup: 3 sessions waiting on you");
        assert_eq!(s.tooltip(Light::None, 0), "Personalization Setup");
    }
}
