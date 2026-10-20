// Command screenshotinator is a manual-session "Recall": it captures the focused window on every
// click, lets the user curate the steps and analyze them with a vision model, and exports Markdown.
package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/sysm0n/Screenshotinator/internal/analysis"
	"github.com/sysm0n/Screenshotinator/internal/config"
	"github.com/sysm0n/Screenshotinator/internal/hotkey"
	"github.com/sysm0n/Screenshotinator/internal/llm"
	"github.com/sysm0n/Screenshotinator/internal/recorder"
	"github.com/sysm0n/Screenshotinator/internal/settings"
	"github.com/sysm0n/Screenshotinator/internal/storage"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	closeLog := setupLogging(cfg.LogPath)
	defer closeLog()
	defer logPanic()
	slog.Info("starting", "os", runtime.GOOS, "arch", runtime.GOARCH, "dataDir", cfg.DataDir)

	db, err := storage.Open(context.Background(), cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close() // after OnShutdown has stopped every writer

	prefs := settings.NewStore(cfg.SettingsPath, settings.FromEnv(hotkey.Default))
	if err := prefs.Load(); err != nil {
		slog.Warn("using default settings", "err", err)
	}

	ui := &uiEmitter{}
	client := llm.New(cfg.LLMTimeout, func() llm.Endpoint {
		s := prefs.Get()
		return llm.Endpoint{URL: s.URL, Model: s.Model, APIKey: s.APIKey}
	})
	app := NewApp(Deps{
		Log:      slog.Default(),
		Store:    db,
		Settings: prefs,
		Emitter:  ui,
		Analysis: analysis.New(client, db, ui, slog.Default(), cfg.LLMWorkers),
		Recorder: recorder.New(recorder.Options{
			ShotsDir:      cfg.ShotsDir,
			Cooldown:      cfg.ClickCooldown,
			CaptureDelay:  cfg.CaptureDelay,
			MaxImageWidth: cfg.MaxImageWidth,
			JPEGQuality:   cfg.JPEGQuality,
		}, db, ui, slog.Default()),
	})

	return wails.Run(&options.App{
		Title:            "Screenshotinator",
		Width:            1200,
		Height:           820,
		MinWidth:         720,
		MinHeight:        520,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
	})
}

// setupLogging logs to stderr and <DataDir>/app.log (Windows GUI builds have no console).
func setupLogging(path string) (closeFn func()) {
	var w io.Writer = os.Stderr
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err == nil {
		w = io.MultiWriter(os.Stderr, f)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err != nil {
		slog.Warn("log file unavailable", "path", path, "err", err)
		return func() {}
	}
	return func() { f.Close() }
}

// logPanic records a crash in app.log before letting it propagate.
func logPanic() {
	if p := recover(); p != nil {
		slog.Error("panic", "panic", fmt.Sprint(p), "stack", string(debug.Stack()))
		panic(p)
	}
}
