// Package settings holds the user-editable configuration and persists it as JSON.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrInvalid wraps every validation failure.
var ErrInvalid = errors.New("invalid settings")

// Settings is the user-editable configuration. JSON tags are shared with the UI and settings.json.
type Settings struct {
	URL    string `json:"url"`    // OpenAI-compatible chat completions endpoint
	APIKey string `json:"apiKey"` // optional; no Authorization header is sent when empty
	Model  string `json:"model"`
	Hotkey string `json:"hotkey"` // e.g. "ctrl+shift+e"
}

// FromEnv returns the built-in defaults, overridable through RECALL_LLM_URL, RECALL_LLM_MODEL
// and RECALL_LLM_API_KEY.
func FromEnv(defaultHotkey string) Settings {
	return Settings{
		URL:    envOr("RECALL_LLM_URL", "http://localhost:11434/v1/chat/completions"),
		Model:  envOr("RECALL_LLM_MODEL", "llama3.2-vision"),
		APIKey: os.Getenv("RECALL_LLM_API_KEY"),
		Hotkey: defaultHotkey,
	}
}

// Validate reports whether s is usable. It expects normalized input (see Store.Resolve).
func (s Settings) Validate() error {
	u, err := url.Parse(s.URL)
	switch {
	case s.URL == "":
		return fmt.Errorf("%w: endpoint URL is required", ErrInvalid)
	case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "":
		return fmt.Errorf("%w: endpoint URL must be an absolute http(s) URL", ErrInvalid)
	case s.Model == "":
		return fmt.Errorf("%w: model ID is required", ErrInvalid)
	case s.Hotkey == "":
		return fmt.Errorf("%w: restore shortcut is required", ErrInvalid)
	}
	return nil
}

func (s Settings) trimmed() Settings {
	return Settings{
		URL:    strings.TrimSpace(s.URL),
		APIKey: strings.TrimSpace(s.APIKey),
		Model:  strings.TrimSpace(s.Model),
		Hotkey: strings.ToLower(strings.Join(strings.Fields(s.Hotkey), "")),
	}
}

// Store is the concurrency-safe, file-backed settings holder.
type Store struct {
	path     string
	defaults Settings

	mu  sync.RWMutex
	cur Settings
}

// NewStore returns a store holding defaults; call Load to read the file.
func NewStore(path string, defaults Settings) *Store {
	d := defaults.trimmed()
	return &Store{path: path, defaults: d, cur: d}
}

// Load reads the settings file. A missing file keeps the defaults; on error the store is unchanged.
func (s *Store) Load() error {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	v := s.defaults // fields absent from the file keep their defaults
	if err := json.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("parse %s: %w", s.path, err)
	}
	v = s.Resolve(v)
	if err := v.Validate(); err != nil {
		return fmt.Errorf("%s: %w", s.path, err)
	}
	s.mu.Lock()
	s.cur = v
	s.mu.Unlock()
	return nil
}

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cur
}

// Resolve normalizes v and fills optional fields from the defaults. It does not validate.
func (s *Store) Resolve(v Settings) Settings {
	v = v.trimmed()
	if v.Hotkey == "" {
		v.Hotkey = s.defaults.Hotkey
	}
	return v
}

// Save resolves, validates and atomically persists v, then makes it current.
func (s *Store) Save(v Settings) (Settings, error) {
	v = s.Resolve(v)
	if err := v.Validate(); err != nil {
		return Settings{}, err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return Settings{}, err
	}
	if err := writeFileAtomic(s.path, b, 0o600); err != nil {
		return Settings{}, fmt.Errorf("write settings: %w", err)
	}
	s.mu.Lock()
	s.cur = v
	s.mu.Unlock()
	return v, nil
}

// writeFileAtomic writes to a temp file in the same directory and renames it into place,
// so a crash never leaves a truncated settings file (the file contains an API key).
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
