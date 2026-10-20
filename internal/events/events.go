// Package events names the backend → frontend events and the emitter abstraction.
// Keep the names in sync with frontend/src/lib/events.ts.
package events

const (
	ScreenshotAdded   = "screenshot:added"   // *models.Screenshot
	ScreenshotUpdated = "screenshot:updated" // *models.Screenshot
	SessionEnded      = "session:ended"      // *models.SessionResult
	WindowRestored    = "window:restored"    // no payload
	HotkeyError       = "hotkey:error"       // string
)

// Emitter delivers an event to the UI. Implementations must be safe for concurrent use.
type Emitter interface {
	Emit(name string, data any)
}

// EmitterFunc adapts a plain function to Emitter.
type EmitterFunc func(name string, data any)

func (f EmitterFunc) Emit(name string, data any) { f(name, data) }

// Discard drops every event.
var Discard Emitter = EmitterFunc(func(string, any) {})
