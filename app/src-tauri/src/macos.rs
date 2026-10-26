//! Native NSWindow tweaks for the floating alert windows.

/// Floats `w` above full-screen apps on every Space and shows it without
/// activating the app. Must run on the main thread.
#[cfg(target_os = "macos")]
pub fn float_everywhere(w: &tauri::WebviewWindow) {
    use objc2_app_kit::{NSWindow, NSWindowCollectionBehavior};

    /// NSStatusWindowLevel.
    const STATUS_WINDOW_LEVEL: isize = 25;

    let ptr = match w.ns_window() {
        Ok(p) if !p.is_null() => p,
        Ok(_) => {
            log::error!("float_everywhere {}: null NSWindow", w.label());
            return;
        }
        Err(e) => {
            log::error!("float_everywhere {}: {e}", w.label());
            return;
        }
    };
    // SAFETY: tauri hands back the live NSWindow* for this window; we only
    // borrow it for the duration of this call, on the main thread.
    let ns: &NSWindow = unsafe { &*ptr.cast::<NSWindow>() };
    ns.setCollectionBehavior(
        NSWindowCollectionBehavior::CanJoinAllSpaces
            | NSWindowCollectionBehavior::Stationary
            | NSWindowCollectionBehavior::FullScreenAuxiliary,
    );
    ns.setLevel(STATUS_WINDOW_LEVEL);
    // Without this the borderless, unfocused window is never drawn.
    ns.orderFrontRegardless();
}

#[cfg(not(target_os = "macos"))]
pub fn float_everywhere(w: &tauri::WebviewWindow) {
    let _ = w.show();
}
