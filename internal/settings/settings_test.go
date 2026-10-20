package settings

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

var defaults = Settings{URL: "http://localhost:11434/v1/chat/completions", Model: "llama3.2-vision", Hotkey: "ctrl+shift+e"}

func TestLoadMissingFileKeepsDefaults(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "settings.json"), defaults)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	if s.Get() != defaults {
		t.Errorf("Get() = %+v", s.Get())
	}
}

func TestSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := NewStore(path, defaults)

	saved, err := s.Save(Settings{URL: " https://dgx.lab/v1/chat/completions ", Model: " qwen2.5-vl ", APIKey: "", Hotkey: "Ctrl + Shift + F9"})
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{URL: "https://dgx.lab/v1/chat/completions", Model: "qwen2.5-vl", Hotkey: "ctrl+shift+f9"}
	if saved != want {
		t.Errorf("saved = %+v, want %+v", saved, want)
	}

	reloaded := NewStore(path, defaults)
	if err := reloaded.Load(); err != nil {
		t.Fatal(err)
	}
	if reloaded.Get() != want {
		t.Errorf("reloaded = %+v, want %+v", reloaded.Get(), want)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
			t.Errorf("settings file mode = %v, want 0600 (contains API key)", fi.Mode().Perm())
		}
	}
}

func TestSaveFillsDefaultHotkey(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "settings.json"), defaults)
	got, err := s.Save(Settings{URL: defaults.URL, Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Hotkey != defaults.Hotkey {
		t.Errorf("Hotkey = %q, want default", got.Hotkey)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := NewStore(path, defaults)
	for _, in := range []Settings{
		{URL: "", Model: "m"},
		{URL: "localhost:11434", Model: "m"},
		{URL: "ftp://host/x", Model: "m"},
		{URL: defaults.URL, Model: "  "},
	} {
		if _, err := s.Save(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("Save(%+v): err = %v, want ErrInvalid", in, err)
		}
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("invalid settings were written to disk")
	}
	if s.Get() != defaults {
		t.Error("invalid settings became current")
	}
}

func TestLoadCorruptFileKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte("{not json"), 0o600)
	s := NewStore(path, defaults)
	if err := s.Load(); err == nil {
		t.Fatal("expected error for corrupt file")
	}
	if s.Get() != defaults {
		t.Errorf("Get() = %+v", s.Get())
	}
}
