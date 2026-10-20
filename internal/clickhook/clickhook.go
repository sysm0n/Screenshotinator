// Package clickhook delivers global mouse-down events.
//
// Start(onClick) installs the hook and returns a stop func that removes it completely; clicks are
// only delivered after Start returns. onClick runs on the hook's own thread and MUST return quickly.
//
// Windows: pure Go (SetWindowsHookEx WH_MOUSE_LL via syscall, no CGO) so it cross-compiles from
// any OS. macOS/Linux: gohook (CGO).
package clickhook
