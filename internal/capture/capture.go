// Package capture grabs ONLY the currently focused foreground window.
// Each OS has its own implementation selected by Go build constraints (see capture_<os>.go).
package capture

import "image"

// Result is a single window grab.
type Result struct {
	Title string // "<process/app> – <window title>"
	Image image.Image
}

// ActiveWindow captures the focused window's bounding box (implemented per platform as activeWindow).
func ActiveWindow() (*Result, error) { return activeWindow() }
