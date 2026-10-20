package hotkey

import xhk "golang.design/x/hotkey"

// Default is the shortcut used until the user picks another one.
const Default = "ctrl+shift+e"

// Mod1 is Alt and Mod4 is Super on standard X11 keymaps.
func modifier(name string) (xhk.Modifier, bool) {
	switch name {
	case "ctrl":
		return xhk.ModCtrl, true
	case "shift":
		return xhk.ModShift, true
	case "alt", "option":
		return xhk.Mod1, true
	case "cmd", "win", "super":
		return xhk.Mod4, true
	}
	return 0, false
}
