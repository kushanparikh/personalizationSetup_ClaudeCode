# Personalization Setup

A small macOS app that sets up Claude Code hooks for you. Add, edit, turn off or remove three hooks from one window: a git status summary when a session starts, a "finished working" banner, and an "input needed" alert that stays on screen until you act. Clicking an alert takes you to the Terminal tab of the session that sent it, which helps when several sessions run at once. Permissions are requested when a hook needs them, and anything missing is listed and can be fixed from the app.

## Requirements

- macOS 13 or later
- Claude Code (the installer can set it up for you)
- Terminal.app, for click-to-focus on the exact tab

## Install

```sh
git clone https://github.com/kushanparikh/personalizationSetup_ClaudeCode.git
cd personalizationSetup_ClaudeCode
./install.sh
```

The script installs the app to `~/Applications`, opens it, and checks that it started. Click **+ Add hook** to begin. Running it again upgrades the app in place.

Options:

| Flag | Meaning |
| --- | --- |
| `--zip <file>` | Install from a downloaded `Personalization-Setup-macos.zip` instead of downloading. The matching `.sha256` file must be next to it |
| `--version vX.Y.Z` | Install a specific release instead of the latest |
| `--install-claude` | Install Claude Code if it is missing, without asking |

Exit codes: `0` installed and launched; `1` a prerequisite, download or check failed (nothing installed); `2` installed, but the app did not start.

The first time it runs, the installer saves a copy of your Claude Code settings to `~/.claude/settings.json.pcs-original`. It skips this when the settings already contain Personalization Setup's hooks (or `~/.pcs` exists), because then the copy wouldn't be the original.

**Private repository.** If the download fails, install GitHub CLI (`brew install gh`), run `gh auth login`, and re-run. Or download `Personalization-Setup-macos.zip` and `Personalization-Setup-macos.zip.sha256` from the Releases page into the same folder and run `./install.sh --zip <file>`. After the checksum passes, the installer removes the browser's quarantine flag from the app.

**First launch.** The app is not signed with an Apple Developer ID, so macOS may block it. Open System Settings, go to Privacy & Security, click **Open Anyway** for Personalization Setup, then run the same `./install.sh` command again (with `--zip <file>` if you used it).

## Using the app

- There is no Dock icon. Use the menu-bar icon (Open / Quit) or Spotlight to open it.
- `⌘Q` quits the app. Any hook that fires relaunches it in the background.

## Hooks

| Hook | What it does |
| --- | --- |
| Git status on session start | Shows the repo, branch and whether there are uncommitted changes when a session starts. |
| Finished working | A banner that slides away when Claude finishes. Click it to jump to that session's terminal tab. |
| Input needed | A panel that stays on screen while Claude is waiting on you. Click it to jump to that session's terminal tab. |

The only permission is **Automation → Terminal** ("Control Terminal"), needed for click-to-focus. It is requested when you set up a hook that uses it, and the app lists it as pending if it is missing.

Only `~/.claude/settings.json` is edited, and only the hooks the app created.

### Spaces limit

Clicking an alert selects the session's Terminal tab and brings Terminal forward. If that Terminal window is on another desktop or a full-screen Space, macOS will not switch to it.

## Uninstall

```sh
./uninstall.sh        # asks first
./uninstall.sh --yes  # no prompt
```

Or use **Uninstall…** in the app's sidebar. This removes the app's hooks from `~/.claude/settings.json`, then the app, its saved answers and its logs. Your other hooks and settings are kept. `~/.claude/settings.json.pcs-original` is left in place.

## Developing

See [app/README.md](app/README.md) for building and running the app.

Run the tests with `scripts/test/run.sh` (shell scripts), plus the app, Rust and Go suites.

### Releasing

```sh
scripts/release.sh vX.Y.Z --dry-run   # build and zip only
scripts/release.sh vX.Y.Z             # also publish a GitHub release
```

Requirements: a clean, pushed `main` commit; the versions in `tauri.conf.json`, `Cargo.toml` and `package.json` matching the tag (see `scripts/check-version.sh`); and the Rust targets, via `rustup target add x86_64-apple-darwin aarch64-apple-darwin`.

## Troubleshooting

Logs are in `~/Library/Logs/Personalization Setup/`:

- `app.log` for the app
- `pcs.log` for the hook command
- `install.log` for `install.sh` and `uninstall.sh`

If the app does not start, open it from `~/Applications` to see the error.
