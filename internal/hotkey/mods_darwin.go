package hotkey

import xhk "golang.design/x/hotkey"

// Default is the shortcut used until the user picks another one.
const Default = "cmd+alt+e"

func modifier(name string) (xhk.Modifier, bool) {
	switch name {
	case "ctrl":
		return xhk.ModCtrl, true
	case "shift":
		return xhk.ModShift, true
	case "alt", "option":
		return xhk.ModOption, true
	case "cmd", "win", "super":
		return xhk.ModCmd, true
	}
	return 0, false
}
