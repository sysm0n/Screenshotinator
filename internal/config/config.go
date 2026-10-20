// Package config resolves the application's on-disk locations and static tuning values.
// User-editable settings live in package settings.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// AppDirName is the per-user data directory name.
const AppDirName = "my-recall-app"

type Config struct {
	DataDir      string
	ShotsDir     string
	DBPath       string
	SettingsPath string
	LogPath      string

	ClickCooldown time.Duration // minimum gap between two accepted clicks
	CaptureDelay  time.Duration // wait after mouse-down so focus/repaint have happened
	MaxImageWidth int
	JPEGQuality   int

	LLMTimeout time.Duration
	LLMWorkers int // concurrent requests to the vision endpoint
}

// Load resolves the platform data directory and creates it.
func Load() (Config, error) {
	dir, err := dataDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve data dir: %w", err)
	}
	c := Config{
		DataDir:       dir,
		ShotsDir:      filepath.Join(dir, "screenshots"),
		DBPath:        filepath.Join(dir, "recall.db"),
		SettingsPath:  filepath.Join(dir, "settings.json"),
		LogPath:       filepath.Join(dir, "app.log"),
		ClickCooldown: 1500 * time.Millisecond,
		CaptureDelay:  150 * time.Millisecond,
		MaxImageWidth: 1280,
		JPEGQuality:   80,
		LLMTimeout:    3 * time.Minute,
		LLMWorkers:    2,
	}
	if err := os.MkdirAll(c.ShotsDir, 0o700); err != nil {
		return Config{}, fmt.Errorf("create data dir: %w", err)
	}
	return c, nil
}

// dataDir returns ~/.local/share/my-recall-app (Linux, honouring XDG_DATA_HOME),
// ~/Library/Application Support/my-recall-app (macOS) or %LOCALAPPDATA%\my-recall-app (Windows).
func dataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, AppDirName), nil
		}
		return filepath.Join(home, "AppData", "Local", AppDirName), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", AppDirName), nil
	default:
		if v := os.Getenv("XDG_DATA_HOME"); v != "" {
			return filepath.Join(v, AppDirName), nil
		}
		return filepath.Join(home, ".local", "share", AppDirName), nil
	}
}
