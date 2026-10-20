package hotkey

import xhk "golang.design/x/hotkey"

// Default is the shortcut used until the user picks another one. It is deliberately not
// Ctrl+Alt: on German/Austrian layouts that is AltGr (Ctrl+Alt+E types "€").
const Default = "ctrl+shift+e"

func modifier(name string) (xhk.Modifier, bool) {
	switch name {
	case "ctrl":
		return xhk.ModCtrl, true
	case "shift":
		return xhk.ModShift, true
	case "alt", "option":
		return xhk.ModAlt, true
	case "cmd", "win", "super":
		return xhk.ModWin, true
	}
	return 0, false
}
