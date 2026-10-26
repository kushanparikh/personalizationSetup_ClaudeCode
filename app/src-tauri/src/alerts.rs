//! Applies alert board changes to real windows: open, update, close, stack,
//! toast timers, and click-to-focus.

use std::sync::Mutex;
use std::time::Duration;

use serde::Serialize;
use tauri::{
    AppHandle, Emitter, EventTarget, LogicalPosition, LogicalSize, Manager, State, WebviewUrl,
    WebviewWindow, WebviewWindowBuilder,
};

use crate::board::{Board, Change, Kind, Shown};
use crate::events::ServeLine;
use crate::layout::{stack, Rect, WIDTH};
use crate::status::Status;
use crate::{macos, sidecar, sound, tray_light};

pub const UPDATED_EVENT: &str = "alert-updated";
const TOAST_LIFETIME: Duration = Duration::from_secs(6);
const FOCUS_TIMEOUT: Duration = Duration::from_secs(5);

/// The shown alerts, and the status light's memory of finished sessions.
/// Lock order when both are held: status, then board.
#[derive(Default)]
pub struct Alerts {
    board: Mutex<Board>,
    status: Mutex<Status>,
}

#[derive(Serialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "camelCase")]
pub struct AlertView {
    kind: Kind,
    /// Look of the alert (lamp colour). Defaulted by kind when the sidecar sends none.
    tone: String,
    title: String,
    message: String,
    focus: bool,
}

impl From<&Shown> for AlertView {
    fn from(s: &Shown) -> Self {
        let tone = match (s.alert.tone.as_str(), s.kind) {
            ("", Kind::Panel) => "wait",
            ("", Kind::Toast) => "done",
            (t, _) => t,
        };
        AlertView {
            kind: s.kind,
            tone: tone.to_string(),
            title: s.alert.title.clone(),
            message: s.alert.message.clone(),
            focus: s.alert.focus,
        }
    }
}

fn board(app: &AppHandle) -> std::sync::MutexGuard<'_, Board> {
    let state = app.state::<Alerts>();
    // A poisoned lock only means an earlier panic mid-update; keep going.
    state.inner().board.lock().unwrap_or_else(|e| e.into_inner())
}

fn status(app: &AppHandle) -> std::sync::MutexGuard<'_, Status> {
    let state = app.state::<Alerts>();
    state.inner().status.lock().unwrap_or_else(|e| e.into_inner())
}

/// Main thread only: recomputes the status light from the board and the
/// finished sessions, and draws it.
fn refresh_light(app: &AppHandle) {
    let (panels, sessionless) = {
        let b = board(app);
        (b.open_panels(), b.sessionless_toasts())
    };
    let (light, tooltip) = {
        let s = status(app);
        let light = s.light(panels, sessionless);
        (light, s.tooltip(light, panels))
    };
    tray_light::set(app, light, &tooltip);
}

/// The menu-bar menu opened: every finished session counts as seen.
/// Main thread only.
pub fn menu_opened(app: &AppHandle) {
    status(app).menu_opened();
    refresh_light(app);
}

/// Entry point for `pcs serve` lines.
pub fn handle(app: &AppHandle, line: ServeLine) {
    let a = app.clone();
    type Step = Box<dyn FnOnce(&mut Status, &mut Board) -> Vec<Change> + Send>;
    let run = move |f: Step| {
        let app = a.clone();
        if let Err(e) = a.run_on_main_thread(move || {
            let changes = f(&mut status(&app), &mut board(&app));
            apply(&app, changes);
        }) {
            log::error!("alerts: run_on_main_thread: {e}");
        }
    };
    match line {
        ServeLine::Panel(alert) => run(Box::new(move |s, b| {
            s.panel_opened(&alert.session_id);
            b.panel(alert)
        })),
        ServeLine::Toast(alert) => run(Box::new(move |s, b| {
            s.toast(&alert.session_id);
            b.toast(alert)
        })),
        ServeLine::MovedOn(sid) => run(Box::new(move |s, b| {
            s.moved_on(&sid);
            b.moved_on(&sid)
        })),
        ServeLine::Ready | ServeLine::Error(_) => {}
    }
}

/// Main thread only.
fn apply(app: &AppHandle, changes: Vec<Change>) {
    for c in changes {
        match c {
            Change::Open(s) => {
                if !open_window(app, &s.label, s.kind) {
                    board(app).remove(&s.label);
                    continue;
                }
                sound::play(&s.alert.sound);
                if s.kind == Kind::Toast {
                    schedule_toast_expiry(app, s.label.clone());
                }
            }
            Change::Update(s) => {
                let view = AlertView::from(&s);
                if let Err(e) = app.emit_to(EventTarget::webview_window(&s.label), UPDATED_EVENT, view)
                {
                    log::error!("alerts: emit {UPDATED_EVENT} to {}: {e}", s.label);
                }
                sound::play(&s.alert.sound);
            }
            Change::Close(label) => destroy(app, &label),
        }
    }
    relayout(app);
    refresh_light(app);
}

/// Main thread only. Labels are never reused, so a build only fails for
/// real reasons; the caller drops the board entry then.
fn open_window(app: &AppHandle, label: &str, kind: Kind) -> bool {
    let h = kind.height();
    let url = WebviewUrl::App(format!("alert.html?label={label}").into());
    let built = WebviewWindowBuilder::new(app, label, url)
        .title("")
        .inner_size(WIDTH, h)
        .decorations(false)
        .always_on_top(true)
        .visible_on_all_workspaces(true)
        .skip_taskbar(true)
        .resizable(false)
        .focused(false)
        .visible(false)
        .build();
    match built {
        Ok(w) => {
            log::info!("alerts: opened {label}");
            // Make sure the size is exactly what layout assumes.
            let _ = w.set_size(LogicalSize::new(WIDTH, h));
            true
        }
        Err(e) => {
            log::error!("alerts: open {label}: {e}");
            false
        }
    }
}

fn schedule_toast_expiry(app: &AppHandle, label: String) {
    let app = app.clone();
    tauri::async_runtime::spawn(async move {
        tokio::time::sleep(TOAST_LIFETIME).await;
        if board(&app).remove(&label).is_some() {
            close_and_relayout(&app, label);
        }
    });
}

fn destroy(app: &AppHandle, label: &str) {
    if let Some(w) = app.get_webview_window(label) {
        if let Err(e) = w.destroy() {
            log::error!("alerts: destroy {label}: {e}");
        } else {
            log::info!("alerts: closed {label}");
        }
    }
}

/// Destroys `label`'s window, restacks the rest and redraws the status
/// light, on the main thread.
fn close_and_relayout(app: &AppHandle, label: String) {
    let a = app.clone();
    if let Err(e) = app.run_on_main_thread(move || {
        destroy(&a, &label);
        relayout(&a);
        refresh_light(&a);
    }) {
        log::error!("alerts: run_on_main_thread: {e}");
    }
}

/// The primary monitor's work area in logical points (y from the top).
fn work_area(app: &AppHandle) -> Option<Rect> {
    let m = app.primary_monitor().ok().flatten()?;
    let s = m.scale_factor();
    let wa = m.work_area();
    Some(Rect {
        x: wa.position.x as f64 / s,
        y: wa.position.y as f64 / s,
        w: wa.size.width as f64 / s,
        h: wa.size.height as f64 / s,
    })
}

/// Main thread only: positions every shown alert, hides the ones that do
/// not fit (they appear as others close).
fn relayout(app: &AppHandle) {
    let Some(work) = work_area(app) else {
        log::error!("alerts: no primary monitor");
        return;
    };
    let items = board(app).heights();
    let heights: Vec<f64> = items.iter().map(|(_, h)| *h).collect();
    let slots = stack(work, &heights);
    log::debug!("alerts: relayout work={work:?} slots={slots:?}");
    for ((label, _), pos) in items.iter().zip(slots) {
        let Some(w) = app.get_webview_window(label) else { continue };
        place(&w, pos);
    }
}

fn place(w: &WebviewWindow, pos: Option<(f64, f64)>) {
    match pos {
        Some((x, y)) => {
            if let Err(e) = w.set_position(LogicalPosition::new(x, y)) {
                log::error!("alerts: position {}: {e}", w.label());
            }
            macos::float_everywhere(w);
        }
        None => {
            let _ = w.hide();
        }
    }
}

#[tauri::command]
pub fn alert_get(state: State<'_, Alerts>, label: String) -> Option<AlertView> {
    let b = state.board.lock().unwrap_or_else(|e| e.into_inner());
    b.get(&label).map(AlertView::from)
}

#[tauri::command]
pub async fn alert_click(app: AppHandle, label: String) {
    let Some(shown) = board(&app).remove(&label) else { return };
    if shown.kind == Kind::Toast {
        status(&app).toast_clicked(&shown.alert.session_id);
    }
    // Close first so a slow focus never leaves a dead panel up.
    close_and_relayout(&app, label.clone());
    if !shown.alert.focus {
        return;
    }
    let args = vec![
        "focus".to_string(),
        "--tty".to_string(),
        shown.alert.tty.clone(),
        "--bundle-id".to_string(),
        shown.alert.bundle_id.clone(),
    ];
    match sidecar::run_pcs(&app, args, FOCUS_TIMEOUT).await {
        Ok(v) => log::info!(
            "alerts: focus {label}: result={}",
            v.get("result").and_then(|r| r.as_str()).unwrap_or("?")
        ),
        Err(e) => log::error!("alerts: focus {label}: {e}"),
    }
}

#[tauri::command]
pub fn alert_dismiss(app: AppHandle, label: String) {
    if board(&app).remove(&label).is_some() {
        close_and_relayout(&app, label);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::events::Alert;

    #[test]
    fn alert_view_serialises_camel_case() {
        let s = Shown {
            label: "panel-a".into(),
            kind: Kind::Panel,
            alert: Alert { title: "Claude Code — x".into(), message: "m".into(), focus: true, ..Alert::default() },
        };
        let v = serde_json::to_value(AlertView::from(&s)).unwrap();
        assert_eq!(
            v,
            serde_json::json!({"kind":"panel","tone":"wait","title":"Claude Code — x","message":"m","focus":true})
        );
    }

    #[test]
    fn alert_view_defaults_tone_by_kind() {
        let view = |kind, tone: &str| {
            let alert = Alert { tone: tone.into(), ..Alert::default() };
            AlertView::from(&Shown { label: "l".into(), kind, alert }).tone
        };
        assert_eq!(view(Kind::Panel, ""), "wait");
        assert_eq!(view(Kind::Toast, ""), "done");
        assert_eq!(view(Kind::Toast, "danger"), "danger");
        assert_eq!(view(Kind::Panel, "done"), "done");
    }
}
