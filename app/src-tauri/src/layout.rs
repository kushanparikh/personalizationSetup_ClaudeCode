//! Pure stacking math for alert windows (logical points, y from the top).

pub const WIDTH: f64 = 360.0;
pub const MARGIN: f64 = 16.0;
pub const GAP: f64 = 8.0;
pub const PANEL_H: f64 = 96.0;
pub const TOAST_H: f64 = 64.0;

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Rect {
    pub x: f64,
    pub y: f64,
    pub w: f64,
    pub h: f64,
}

/// Positions `heights` (arrival order, oldest on top) in the top-right of
/// `work`. Once one item does not fit, it and every later item are `None`
/// (no backfill, so arrival order stays stable).
pub fn stack(work: Rect, heights: &[f64]) -> Vec<Option<(f64, f64)>> {
    let x = work.x + work.w - WIDTH - MARGIN;
    let limit = work.y + work.h - MARGIN;
    let mut y = work.y + MARGIN;
    let mut full = false;
    heights
        .iter()
        .map(|&h| {
            if full || y + h > limit {
                full = true;
                return None;
            }
            let pos = (x, y);
            y += h + GAP;
            Some(pos)
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn work() -> Rect {
        Rect { x: 0.0, y: 25.0, w: 1920.0, h: 1055.0 }
    }

    #[test]
    fn stack_first_top_right() {
        assert_eq!(stack(work(), &[96.0]), vec![Some((1544.0, 41.0))]);
    }

    #[test]
    fn stack_second_below() {
        let r = stack(work(), &[96.0, 64.0]);
        assert_eq!(r[1], Some((1544.0, 41.0 + 96.0 + 8.0)));
    }

    #[test]
    fn stack_respects_origin() {
        let w = Rect { x: -1920.0, ..work() };
        assert_eq!(stack(w, &[96.0])[0], Some((-376.0, 41.0)));
    }

    #[test]
    fn stack_overflow_none() {
        let w = Rect { h: 200.0, ..work() };
        let r = stack(w, &[96.0, 96.0, 96.0]);
        assert!(r[0].is_some());
        assert_eq!(r[1], None);
        assert_eq!(r[2], None);
    }

    #[test]
    fn stack_no_backfill() {
        // fits exactly one 96pt item: bottom limit = 25+200-16 = 209; 41+96=137
        // second would end at 241 (no fit); a 40pt third would end at 145+... but must stay None.
        let w = Rect { h: 200.0, ..work() };
        let r = stack(w, &[96.0, 96.0, 40.0]);
        assert!(r[0].is_some());
        assert_eq!(r[1], None);
        assert_eq!(r[2], None);
    }
}
