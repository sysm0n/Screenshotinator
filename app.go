package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sysm0n/Screenshotinator/internal/analysis"
	"github.com/sysm0n/Screenshotinator/internal/events"
	"github.com/sysm0n/Screenshotinator/internal/export"
	"github.com/sysm0n/Screenshotinator/internal/hotkey"
	"github.com/sysm0n/Screenshotinator/internal/models"
	"github.com/sysm0n/Screenshotinator/internal/recorder"
	"github.com/sysm0n/Screenshotinator/internal/settings"
	"github.com/sysm0n/Screenshotinator/internal/storage"
)

// Deps are the services App orchestrates.
type Deps struct {
	Log      *slog.Logger
	Store    *storage.DB
	Settings *settings.Store
	Emitter  *uiEmitter
	Analysis *analysis.Service
	Recorder *recorder.Recorder
}

// App is the Wails binding layer: every exported method becomes a TS function in
// frontend/wailsjs/go/main/App. Methods stay thin and delegate to the services.
type App struct {
	Deps
	ctx    context.Context
	hotkey *hotkey.Binding
}

func NewApp(d Deps) *App {
	a := &App{Deps: d}
	a.hotkey = hotkey.NewBinding(a.onHotkey)
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.Emitter.bind(ctx)
	// Registered here, not in main: on macOS the hotkey needs the app's main event loop running.
	if err := a.hotkey.Bind(a.Settings.Get().Hotkey); err != nil {
		a.Log.Error("global shortcut unavailable", "err", err)
		a.Emitter.Emit(events.HotkeyError, err.Error())
	}
}

// shutdown stops every writer before main closes the database.
func (a *App) shutdown(ctx context.Context) {
	a.hotkey.Close()
	if _, err := a.Recorder.Stop(ctx); err != nil && !errors.Is(err, recorder.ErrNotActive) {
		a.Log.Warn("stop session on shutdown", "err", err)
	}
	a.Analysis.Close()
}

// onHotkey stops recording (if any) and brings the window back with the review screen.
func (a *App) onHotkey() {
	if res, err := a.Recorder.Stop(a.ctx); err == nil {
		a.Analysis.Decorate(res.Screenshots...)
		a.Emitter.Emit(events.SessionEnded, res)
	} else if !errors.Is(err, recorder.ErrNotActive) {
		a.Log.Error("stop session via shortcut", "err", err)
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true) // pop above the app that was being recorded
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	a.Emitter.Emit(events.WindowRestored, nil)
}

// ---- session -------------------------------------------------------------------------------

// HotkeyLabel returns the active stop shortcut, e.g. "ctrl+shift+e".
func (a *App) HotkeyLabel() string {
	if s := a.hotkey.Spec(); s != "" {
		return s
	}
	return a.Settings.Get().Hotkey
}

// StartSession begins recording and hides the window so it never captures itself.
func (a *App) StartSession() (_ string, err error) {
	defer recoverTo(&err)
	id, err := a.Recorder.Start()
	if err != nil {
		return "", err
	}
	runtime.WindowHide(a.ctx)
	return id, nil
}

// EndSession stops the click listener and returns every screenshot of the session.
func (a *App) EndSession() (_ *models.SessionResult, err error) {
	defer recoverTo(&err)
	res, err := a.Recorder.Stop(a.ctx)
	if err != nil {
		return nil, err
	}
	a.Analysis.Decorate(res.Screenshots...)
	return res, nil
}

// GetSession re-reads a session's screenshots.
func (a *App) GetSession(sessionID string) (_ []*models.Screenshot, err error) {
	defer recoverTo(&err)
	shots, err := a.Store.ListBySession(a.ctx, sessionID)
	if err != nil {
		return nil, err
	}
	a.Analysis.Decorate(shots...)
	return shots, nil
}

// ---- screenshots ---------------------------------------------------------------------------

// GetImage returns the screenshot as a data URL. The webview cannot read local paths, and in
// `wails dev` the Vite server would answer any custom asset route itself.
func (a *App) GetImage(id int64) (_ string, err error) {
	defer recoverTo(&err)
	shot, err := a.Store.Get(a.ctx, id)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(shot.ImagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(b), nil
}

// DeleteScreenshot drops a step from the session, including its image file.
func (a *App) DeleteScreenshot(id int64) (err error) {
	defer recoverTo(&err)
	a.Analysis.Forget(id)
	path, err := a.Store.Delete(a.ctx, id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		a.Log.Warn("remove image", "path", path, "err", err)
	}
	return nil
}

// UpdateNotes saves the user's notes for a step.
func (a *App) UpdateNotes(id int64, notes string) (err error) {
	defer recoverTo(&err)
	return a.Store.SetNotes(a.ctx, id, notes)
}

// AnalyzeScreenshot saves notes and queues ONE screenshot for the model. The batch-wide
// instructions come first, then the step's notes. Results arrive as screenshot:updated events.
func (a *App) AnalyzeScreenshot(id int64, notes, globalInstructions string) (err error) {
	defer recoverTo(&err)
	if err := a.Store.SetNotes(a.ctx, id, notes); err != nil {
		return err
	}
	return a.Analysis.Start(id, joinNonEmpty("\n\n", globalInstructions, notes))
}

// ---- export --------------------------------------------------------------------------------

// ChooseExportFolder opens a native directory picker ("" if cancelled).
func (a *App) ChooseExportFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "Choose export folder",
		CanCreateDirectories: true,
	})
}

// ExportMarkdownDoc writes the curated steps as a Markdown bundle inside exportFolder.
func (a *App) ExportMarkdownDoc(sessionID string, finalItems []models.FrontendScreenshotData, exportFolder string) (err error) {
	defer recoverTo(&err)
	if exportFolder == "" {
		return errors.New("no export folder selected")
	}
	doc, err := export.Write(a.ctx, a.Store, sessionID, finalItems, exportFolder, time.Now())
	if err != nil {
		return err
	}
	a.Log.Info("exported", "session", sessionID, "steps", len(finalItems), "doc", doc)
	return nil
}

// ---- settings ------------------------------------------------------------------------------

// GetSettings returns the current settings.
func (a *App) GetSettings() settings.Settings { return a.Settings.Get() }

// SaveSettings validates, applies the shortcut, then persists. If persisting fails the previous
// shortcut is restored, so the running state always matches settings.json.
func (a *App) SaveSettings(in settings.Settings) (err error) {
	defer recoverTo(&err)
	next := a.Settings.Resolve(in)
	if err := next.Validate(); err != nil {
		return err
	}
	prev := a.hotkey.Spec()
	if err := a.hotkey.Bind(next.Hotkey); err != nil {
		return err
	}
	if _, err := a.Settings.Save(next); err != nil {
		if prev != "" {
			_ = a.hotkey.Bind(prev)
		}
		return err
	}
	return nil
}

// ---- helpers -------------------------------------------------------------------------------

// recoverTo converts a panic in a bound method into an error for the UI (Wails would otherwise
// crash the process) and logs the stack.
func recoverTo(err *error) {
	if p := recover(); p != nil {
		slog.Error("panic in bound method", "panic", fmt.Sprint(p), "stack", string(debug.Stack()))
		*err = fmt.Errorf("internal error: %v", p)
	}
}

func joinNonEmpty(sep string, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}
