# Screenshotinator

[![CI](https://github.com/sysm0n/Screenshotinator/actions/workflows/ci.yml/badge.svg)](https://github.com/sysm0n/Screenshotinator/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A manual-session "Recall" for developers. Start a session, work normally, and every click captures the
window you're working in. When you're done, review the steps, let a local vision model describe them,
add your own notes, and export everything as a Markdown document for Obsidian or GitHub.

Built with [Wails](https://wails.io) (Go + React + TypeScript + Tailwind). Runs on macOS and Windows; Linux (X11) builds from source.

## Features

- **Manual sessions.** Start recording from the app; the window hides itself so it never captures its own UI.
- **Click-triggered capture.** Each click grabs only the focused window, throttled to one capture per 1.5 s.
- **Global stop shortcut.** Stops recording and opens the review screen (`ctrl+shift+e` on Windows/Linux, `⌘⌥E` on macOS; configurable).
- **Review before AI.** Nothing is sent to a model until you start the analysis. Remove steps, add notes, set instructions for the whole batch, then analyze all or individual steps.
- **Local LLM.** Works with any OpenAI-compatible vision endpoint (Ollama, LM Studio, vLLM, a DGX server, …). Each request carries exactly one screenshot.
- **Markdown export.** One folder per session with the `.md` document and the screenshots, using relative links.
- **Private by default.** Screenshots, the database and settings stay on your machine.

## Install

Download the latest build from [Releases](https://github.com/sysm0n/Screenshotinator/releases), or build it yourself (below).

### Permissions

- **macOS:** grant **Accessibility** (global click listener) and **Screen Recording** (window capture) in System Settings → Privacy & Security, then restart the app.
- **Windows:** no extra setup. Some antivirus tools block global mouse hooks; allow the app if clicks aren't captured.
- **Linux:** X11 only (Wayland is not supported).

## Usage

1. Open **⚙ Settings** and set your endpoint URL, model ID and (optional) API key.
2. Click **Start Session**. The window hides.
3. Work as usual. Every click captures the focused window.
4. Press the stop shortcut. Recording stops and the review screen opens.
5. Remove steps you don't need, add notes, optionally write instructions for all steps, then click **Analyze all unanalyzed**. A step's own notes are sent along as extra instructions for that step.
6. Click **Export Markdown** and choose a folder.

Settings are stored in `settings.json` in the data folder:

| OS | Data folder |
| --- | --- |
| macOS | `~/Library/Application Support/my-recall-app/` |
| Windows | `%LOCALAPPDATA%\my-recall-app\` |
| Linux | `~/.local/share/my-recall-app/` (or `$XDG_DATA_HOME`) |

The same folder holds the screenshots, the SQLite database and `app.log`. `RECALL_LLM_URL`, `RECALL_LLM_MODEL` and
`RECALL_LLM_API_KEY` set the defaults used before you save settings for the first time.

## Development

Requirements: Go (see `go.mod`), Node.js 22+, and the [Wails CLI](https://wails.io/docs/gettingstarted/installation).

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor          # checks platform dependencies
make dev              # hot-reload dev mode (also generates frontend/wailsjs)
make check            # go vet (native + Windows) and go test -race
make build            # native build into build/bin/
make build-windows    # Windows .exe from any OS, no C compiler needed
```

macOS and Linux builds need CGO and must be built on that OS. Linux additionally needs
`libx11-dev libxtst-dev libxkbcommon-dev` plus the GTK/WebKit packages listed by `wails doctor`.

Pushing a tag like `v1.0.0` runs the release workflow, which builds macOS (universal) and Windows binaries and attaches them to a GitHub release.

### Architecture

```
main.go        wiring: config, logging, storage, services → Wails
app.go         Wails binding layer (thin; every exported method is callable from TS)
emitter.go     forwards backend events to the frontend

internal/
  recorder/    session lifecycle: click hook, 1.5 s throttle, capture → JPEG → disk → DB
  analysis/    vision-model jobs: worker limit, cancel-on-reanalyze, per-step status/errors
  llm/         OpenAI-compatible client, exactly one image per request
  storage/     SQLite repository (pure-Go driver)
  settings/    user settings (endpoint, model, API key, shortcut), atomic JSON persistence
  export/      Markdown bundle writer (doc + copied images)
  hotkey/      global stop shortcut, configurable, safe rebinding
  capture/     focused-window grab: Windows (user32/gdi32), macOS (CoreGraphics), Linux (X11)
  clickhook/   global mouse-down hook: Windows pure Go, macOS/Linux gohook
  imaging/     downscale to ≤1280 px + JPEG
  config/      data dir + static tuning values
  events/      backend → frontend event names (mirrored in frontend/src/lib/events.ts)
  models/      wire types shared with the UI
frontend/      React + TS + Tailwind dashboard
```

Services depend on small consumer-side interfaces (`recorder.Store`, `analysis.Analyzer`, …), so each is unit-tested with fakes.

## License

[MIT](LICENSE)
