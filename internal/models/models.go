// Package models defines the data exchanged between the backend services and the UI.
// JSON tags are the wire format consumed by the React frontend.
package models

// Screenshot is one captured step of a recording session.
type Screenshot struct {
	ID        int64  `json:"id"`
	SessionID string `json:"sessionId"`
	Timestamp string `json:"timestamp"` // RFC 3339
	AppName   string `json:"appName"`
	ImagePath string `json:"imagePath"`
	UserNotes string `json:"userNotes"`
	AISummary string `json:"aiSummary"`

	// Runtime analysis state, filled in by the analysis service; never persisted.
	Analyzing bool   `json:"analyzing"`
	AIError   string `json:"aiError,omitempty"`
}

// SessionResult is returned when a recording session ends.
type SessionResult struct {
	SessionID   string        `json:"sessionId"`
	Screenshots []*Screenshot `json:"screenshots"`
}

// FrontendScreenshotData is one curated step the UI sends for export, in final order.
// Deleted steps are simply absent. Image paths are deliberately not accepted from the UI.
type FrontendScreenshotData struct {
	ID        int64  `json:"id"`
	UserNotes string `json:"userNotes"`
	AISummary string `json:"aiSummary"`
}
