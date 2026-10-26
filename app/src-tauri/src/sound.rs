//! Alert sounds: a fixed allowlist of macOS system sounds, played with afplay.

use std::path::PathBuf;

pub const SOUNDS: [&str; 14] = [
    "Basso", "Blow", "Bottle", "Frog", "Funk", "Glass", "Hero", "Morse", "Ping", "Pop", "Purr",
    "Sosumi", "Submarine", "Tink",
];

/// The system sound file for an allowlisted name; `None` for anything else
/// (including empty and "none", which mean silent).
pub fn sound_file(name: &str) -> Option<PathBuf> {
    SOUNDS
        .contains(&name)
        .then(|| PathBuf::from(format!("/System/Library/Sounds/{name}.aiff")))
}

/// Plays an allowlisted sound without blocking; silent for anything else.
pub fn play(name: &str) {
    let Some(file) = sound_file(name) else { return };
    std::thread::spawn(move || {
        match std::process::Command::new("/usr/bin/afplay").arg(&file).status() {
            Ok(_) => {}
            Err(e) => log::warn!("afplay {}: {e}", file.display()),
        }
    });
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sound_allowlist() {
        assert_eq!(sound_file("Glass"), Some(PathBuf::from("/System/Library/Sounds/Glass.aiff")));
        for bad in ["", "none", "../../x", "glass"] {
            assert_eq!(sound_file(bad), None, "{bad:?}");
        }
    }
}
