package hotkey

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	valid := map[string]string{
		"ctrl+shift+e":       "ctrl+shift+e",
		" Ctrl + Shift + E ": "ctrl+shift+e",
		"alt+f9":             "alt+f9",
		"cmd+alt+space":      "cmd+alt+space",
	}
	for in, want := range valid {
		sc, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q): %v", in, err)
			continue
		}
		if sc.Spec != want {
			t.Errorf("Parse(%q).Spec = %q, want %q", in, sc.Spec, want)
		}
	}

	for _, in := range []string{"", "e", "ctrl", "ctrl+shift", "ctrl+e+f", "ctrl+banana", "ctrl++e"} {
		if _, err := Parse(in); !errors.Is(err, ErrSyntax) {
			t.Errorf("Parse(%q): err = %v, want ErrSyntax", in, err)
		}
	}
}
