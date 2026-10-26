//! Draws the status light: a coloured dot over the menu-bar glyph (macOS)
//! and the tray tooltip.

use tauri::{tray::TrayIcon, AppHandle, Manager};

use crate::status::Light;

/// Shows `light` and `tooltip` on the menu-bar item. Main thread only.
pub fn set(app: &AppHandle, light: Light, tooltip: &str) {
    let Some(tray) = app.try_state::<TrayIcon>() else {
        // Before setup has built the tray there is nothing to draw on.
        return;
    };
    if let Err(e) = tray.set_tooltip(Some(tooltip)) {
        log::error!("tray light: tooltip: {e}");
    }
    #[cfg(target_os = "macos")]
    if let Err(e) = tray.with_inner_tray_icon(move |t| match t.ns_status_item() {
        Some(item) => mac::show(&item, light),
        None => log::error!("tray light: no NSStatusItem"),
    }) {
        log::error!("tray light: {e}");
    }
    #[cfg(not(target_os = "macos"))]
    let _ = light;
}

#[cfg(target_os = "macos")]
mod mac {
    use std::cell::{Cell, RefCell};

    use objc2::rc::Retained;
    use objc2::{define_class, msg_send, DeclaredClass, MainThreadOnly};
    use objc2_app_kit::{NSColor, NSStatusItem, NSView};
    use objc2_core_foundation::{CGFloat, CGPoint, CGRect, CGSize};
    use objc2_foundation::MainThreadMarker;

    use crate::status::Light;

    /// Diameter of the coloured disc, in points.
    const DOT: CGFloat = 7.0;
    /// Width of the ring around it, in the menu bar's background colour.
    const RING: CGFloat = 1.0;
    /// The glyph's size if the button reports no image (viewBox 20×18).
    const GLYPH: CGSize = CGSize { width: 20.0, height: 18.0 };

    const AMBER: (CGFloat, CGFloat, CGFloat) = (242.0 / 255.0, 181.0 / 255.0, 58.0 / 255.0);
    const GREEN: (CGFloat, CGFloat, CGFloat) = (79.0 / 255.0, 181.0 / 255.0, 159.0 / 255.0);

    pub struct DotIvars {
        fill: Cell<(CGFloat, CGFloat, CGFloat)>,
    }

    define_class!(
        /// A layer-backed round view. Colours are set in `updateLayer`, which
        /// AppKit calls with the view's appearance current, so the ring
        /// follows light and dark menu bars without our help.
        #[unsafe(super(NSView))]
        #[thread_kind = MainThreadOnly]
        #[name = "PSStatusDot"]
        #[ivars = DotIvars]
        pub struct DotView;

        impl DotView {
            #[unsafe(method(wantsUpdateLayer))]
            fn wants_update_layer(&self) -> bool {
                true
            }

            #[unsafe(method(updateLayer))]
            fn update_layer(&self) {
                let Some(layer) = self.layer() else { return };
                let (r, g, b) = self.ivars().fill.get();
                let fill = NSColor::colorWithSRGBRed_green_blue_alpha(r, g, b, 1.0);
                layer.setBackgroundColor(Some(&fill.CGColor()));
                layer.setBorderColor(Some(&NSColor::windowBackgroundColor().CGColor()));
                layer.setBorderWidth(RING);
                layer.setCornerRadius(self.bounds().size.width / 2.0);
            }

            #[unsafe(method(viewDidChangeEffectiveAppearance))]
            fn appearance_changed(&self) {
                // SAFETY: NSView implements this method; calling super is
                // what an override is expected to do.
                let _: () = unsafe { msg_send![super(self), viewDidChangeEffectiveAppearance] };
                self.setNeedsDisplay(true);
            }

            /// Never take clicks: the tray's own click target sits under us
            /// and must get every mouse event.
            #[unsafe(method(hitTest:))]
            fn hit_test(&self, _point: CGPoint) -> *mut NSView {
                std::ptr::null_mut()
            }
        }
    );

    thread_local! {
        /// The one dot, created on first use. Main thread only, like AppKit.
        static DOT_VIEW: RefCell<Option<Retained<DotView>>> = const { RefCell::new(None) };
    }

    fn new_dot(mtm: MainThreadMarker) -> Retained<DotView> {
        let side = DOT + 2.0 * RING;
        let frame = CGRect::new(CGPoint::new(0.0, 0.0), CGSize::new(side, side));
        let this = mtm.alloc::<DotView>().set_ivars(DotIvars { fill: Cell::new(GREEN) });
        // SAFETY: `initWithFrame:` is NSView's designated initialiser, called
        // once on a freshly allocated instance whose ivars are set.
        let view: Retained<DotView> = unsafe { msg_send![super(this), initWithFrame: frame] };
        view.setWantsLayer(true);
        view.setHidden(true);
        view
    }

    /// Where the dot goes in `bounds`: centred on the glyph's top-right
    /// corner (the glyph is centred in the button), then pulled inside the
    /// button so the menu bar's edge never cuts it.
    pub(super) fn dot_frame(bounds: CGSize, glyph: CGSize, flipped: bool) -> CGRect {
        let side = DOT + 2.0 * RING;
        let corner_x = (bounds.width + glyph.width) / 2.0;
        let top_gap = (bounds.height - glyph.height) / 2.0;
        let corner_y = if flipped { top_gap } else { bounds.height - top_gap };
        let clamp = |v: CGFloat, max: CGFloat| v.min(max).max(0.0);
        CGRect::new(
            CGPoint::new(
                clamp(corner_x - side / 2.0, bounds.width - side),
                clamp(corner_y - side / 2.0, bounds.height - side),
            ),
            CGSize::new(side, side),
        )
    }

    /// Main thread only (tauri runs `with_inner_tray_icon` there).
    pub fn show(item: &NSStatusItem, light: Light) {
        let Some(mtm) = MainThreadMarker::new() else {
            log::error!("tray light: not on the main thread");
            return;
        };
        let Some(button) = item.button(mtm) else {
            log::error!("tray light: status item has no button");
            return;
        };
        let fill = match light {
            Light::None => None,
            Light::Done => Some(GREEN),
            Light::Wait => Some(AMBER),
        };
        DOT_VIEW.with(|cell| {
            let mut slot = cell.borrow_mut();
            if fill.is_none() && slot.is_none() {
                return; // never lit yet: nothing to hide
            }
            let dot = slot.get_or_insert_with(|| {
                let dot = new_dot(mtm);
                button.addSubview(&dot);
                dot
            });
            let glyph = button.image().map(|i| i.size()).unwrap_or(GLYPH);
            dot.setFrame(dot_frame(button.bounds().size, glyph, button.isFlipped()));
            match fill {
                Some(rgb) => {
                    dot.ivars().fill.set(rgb);
                    dot.setNeedsDisplay(true);
                    dot.setHidden(false);
                }
                None => dot.setHidden(true),
            }
        });
    }
}

#[cfg(all(test, target_os = "macos"))]
mod tests {
    use super::mac::dot_frame;
    use objc2_core_foundation::CGSize;

    #[test]
    fn dot_sits_on_glyph_top_right_inside_button() {
        let glyph = CGSize::new(20.0, 18.0);
        // Roomy button: centred on the corner, (28, 7) flipped, (28, 25) not.
        let f = dot_frame(CGSize::new(36.0, 32.0), glyph, true);
        assert_eq!((f.origin.x, f.origin.y, f.size.width, f.size.height), (23.5, 2.5, 9.0, 9.0));
        let f = dot_frame(CGSize::new(36.0, 32.0), glyph, false);
        assert_eq!((f.origin.x, f.origin.y), (23.5, 20.5));
        // Standard 22 pt menu bar, tight button: pulled inside the bounds.
        let f = dot_frame(CGSize::new(28.0, 22.0), glyph, true);
        assert_eq!((f.origin.x, f.origin.y), (19.0, 0.0));
        let f = dot_frame(CGSize::new(28.0, 22.0), glyph, false);
        assert_eq!((f.origin.x, f.origin.y), (19.0, 13.0));
    }
}
