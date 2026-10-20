// Package hotkey binds the global "stop recording / show window" shortcut.
//
// Shortcuts are written as "mod+mod+key", e.g. "ctrl+shift+e". Modifiers: ctrl, shift,
// alt (option), cmd (win/super). Keys: a-z, 0-9, f1-f12, space.
package hotkey

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	xhk "golang.design/x/hotkey"
)

// ErrSyntax wraps every parse failure.
var ErrSyntax = errors.New("invalid shortcut")

// Shortcut is a parsed, platform-resolved key combination.
type Shortcut struct {
	Spec string // canonical form: lower case, no spaces
	mods []xhk.Modifier
	key  xhk.Key
}

// Parse validates spec and resolves it for the current platform.
func Parse(spec string) (Shortcut, error) {
	canon := strings.ToLower(strings.Join(strings.Fields(spec), ""))
	sc := Shortcut{Spec: canon}
	var haveKey bool
	for _, part := range strings.Split(canon, "+") {
		if m, ok := modifier(part); ok {
			sc.mods = append(sc.mods, m)
			continue
		}
		k, ok := keyTable[part]
		if !ok || haveKey {
			return Shortcut{}, fmt.Errorf("%w %q: unknown or second key %q", ErrSyntax, spec, part)
		}
		sc.key, haveKey = k, true
	}
	if !haveKey || len(sc.mods) == 0 {
		return Shortcut{}, fmt.Errorf("%w %q: needs at least one modifier and exactly one key", ErrSyntax, spec)
	}
	return sc, nil
}

// Binding keeps one global shortcut registered and calls onPress for every key-down.
type Binding struct {
	onPress func()

	mu   sync.Mutex
	sc   Shortcut
	hk   *xhk.Hotkey
	quit chan struct{}
}

func NewBinding(onPress func()) *Binding { return &Binding{onPress: onPress} }

// Spec returns the currently bound shortcut ("" if none).
func (b *Binding) Spec() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sc.Spec
}

// Bind registers spec, replacing the current shortcut. Rebinding the same shortcut is a no-op.
// If registration fails, the previous shortcut is restored.
func (b *Binding) Bind(spec string) error {
	sc, err := Parse(spec)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.hk != nil && sc.Spec == b.sc.Spec {
		return nil
	}

	// Release first: Windows refuses to register a combination that is already registered,
	// even by this process.
	prev := b.sc
	b.unbindLocked()
	if err := b.bindLocked(sc); err != nil {
		if prev.Spec != "" {
			_ = b.bindLocked(prev)
		}
		return fmt.Errorf("register %q (already used by another program?): %w", sc.Spec, err)
	}
	return nil
}

// Close unregisters the shortcut.
func (b *Binding) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.unbindLocked()
}

func (b *Binding) bindLocked(sc Shortcut) error {
	hk := xhk.New(sc.mods, sc.key)
	if err := hk.Register(); err != nil {
		return err
	}
	quit := make(chan struct{})
	go b.listen(hk.Keydown(), quit)
	b.sc, b.hk, b.quit = sc, hk, quit
	return nil
}

func (b *Binding) unbindLocked() {
	if b.hk == nil {
		return
	}
	close(b.quit)
	_ = b.hk.Unregister()
	b.sc, b.hk, b.quit = Shortcut{}, nil, nil
}

func (b *Binding) listen(keydown <-chan xhk.Event, quit <-chan struct{}) {
	for {
		select {
		case <-quit:
			return
		case _, ok := <-keydown:
			if !ok {
				return
			}
			b.onPress()
		}
	}
}
