# Personalization Setup app

Tauri 2 desktop app (Svelte 5 + Vite + TypeScript frontend, Rust in `src-tauri`) that bundles the Go `pcs` sidecar from `../sidecar`.

## Prerequisites

- macOS with Xcode Command Line Tools
- Rust via rustup (`source ~/.cargo/env`)
- Go
- Node.js and npm

## Setup

```sh
npm install
```

## Develop

```sh
npm run tauri dev
```

Tip: set `PCS_HOME` to run against a scratch home instead of your real one. Every `pcs` the app spawns then gets `HOME=$PCS_HOME`:

```sh
PCS_HOME=/tmp/pcs-dev npm run tauri dev
```

Keep the path short; the socket path must stay under 104 bytes.

## Test

```sh
npm test                          # vitest
npm run check                     # svelte-check
(cd src-tauri && cargo test)      # Rust
(cd ../sidecar && go test ./...)  # Go
```

## Install a dev build

```sh
scripts/dev-install.sh
```

This builds the release bundle, quits any running copy, replaces `~/Applications/Personalization Setup.app` and opens it.

**Warning:** this installs the real app. On start it rewrites `~/.pcs/bin/pcs-hook` and then manages your real `~/.claude/settings.json` hooks. Back that file up first.

## Logs

- `~/Library/Logs/Personalization Setup/app.log`
- `~/Library/Logs/Personalization Setup/pcs.log`
