// Package recorder owns the recording lifecycle: the global click hook, click throttling and the
// capture → optimize → save → persist pipeline. It never talks to the model; analysis is manual.
package recorder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sysm0n/Screenshotinator/internal/capture"
	"github.com/sysm0n/Screenshotinator/internal/clickhook"
	"github.com/sysm0n/Screenshotinator/internal/events"
	"github.com/sysm0n/Screenshotinator/internal/imaging"
	"github.com/sysm0n/Screenshotinator/internal/models"
)

var (
	ErrActive    = errors.New("a session is already running")
	ErrNotActive = errors.New("no active session") // text is matched by the UI
)

// Store is the persistence the recorder needs (implemented by *storage.DB).
type Store interface {
	Insert(ctx context.Context, s *models.Screenshot) error
	ListBySession(ctx context.Context, sessionID string) ([]*models.Screenshot, error)
}

type Options struct {
	ShotsDir      string
	Cooldown      time.Duration
	CaptureDelay  time.Duration
	MaxImageWidth int
	JPEGQuality   int
}

type Recorder struct {
	opts   Options
	store  Store
	events events.Emitter
	log    *slog.Logger

	// Platform seams, replaced in tests.
	installHook func(onClick func()) (stop func(), err error)
	grabWindow  func() (*capture.Result, error)
	now         func() time.Time

	mu  sync.Mutex
	cur *session // nil while idle
}

type session struct {
	id       string
	dir      string
	stop     func()
	throttle *Throttle
	pending  sync.WaitGroup // capture steps in flight
}

func New(opts Options, store Store, ev events.Emitter, log *slog.Logger) *Recorder {
	return &Recorder{
		opts: opts, store: store, events: ev, log: log,
		installHook: clickhook.Start,
		grabWindow:  capture.ActiveWindow,
		now:         time.Now,
	}
}

// Active reports whether a session is recording.
func (r *Recorder) Active() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cur != nil
}

// Start begins a new session and installs the global click hook.
func (r *Recorder) Start() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cur != nil {
		return "", ErrActive
	}

	s := &session{id: uuid.NewString(), throttle: NewThrottle(r.opts.Cooldown)}
	s.dir = filepath.Join(r.opts.ShotsDir, s.id)
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return "", fmt.Errorf("create session dir: %w", err)
	}
	// Installing under the lock is safe: hooks deliver clicks only after installHook returns,
	// and onClick just blocks briefly on r.mu.
	stop, err := r.installHook(func() { r.onClick(s) })
	if err != nil {
		os.Remove(s.dir)
		return "", fmt.Errorf("install click hook: %w", err)
	}
	s.stop = stop
	r.cur = s
	r.log.Info("session started", "session", s.id)
	return s.id, nil
}

// Stop removes the click hook, waits for in-flight captures and returns the session.
func (r *Recorder) Stop(ctx context.Context) (*models.SessionResult, error) {
	r.mu.Lock()
	s := r.cur
	r.cur = nil // from here on onClick rejects every click of s
	r.mu.Unlock()
	if s == nil {
		return nil, ErrNotActive
	}

	s.stop()
	s.pending.Wait()

	shots, err := r.store.ListBySession(ctx, s.id)
	if err != nil {
		return nil, err
	}
	r.log.Info("session stopped", "session", s.id, "screenshots", len(shots))
	return &models.SessionResult{SessionID: s.id, Screenshots: shots}, nil
}

// onClick runs on the OS hook thread and must return quickly.
func (r *Recorder) onClick(s *session) {
	r.mu.Lock()
	if r.cur != s || !s.throttle.Allow(r.now()) {
		r.mu.Unlock()
		return
	}
	s.pending.Add(1) // under the lock, so Stop's Wait cannot miss it
	r.mu.Unlock()
	go r.captureStep(s)
}

func (r *Recorder) captureStep(s *session) {
	defer s.pending.Done()
	defer func() {
		if p := recover(); p != nil {
			r.log.Error("capture panic", "panic", p, "stack", string(debug.Stack()))
		}
	}()

	// Mouse-down arrives before the OS moves focus and repaints; wait so we grab the click's result.
	time.Sleep(r.opts.CaptureDelay)

	shot, err := r.captureAndStore(s)
	if err != nil {
		r.log.Warn("capture step skipped", "session", s.id, "err", err)
		return
	}
	r.events.Emit(events.ScreenshotAdded, shot)
}

func (r *Recorder) captureAndStore(s *session) (*models.Screenshot, error) {
	win, err := r.grabWindow()
	if err != nil {
		return nil, fmt.Errorf("capture window: %w", err)
	}
	jpeg, err := imaging.OptimizeJPEG(win.Image, r.opts.MaxImageWidth, r.opts.JPEGQuality)
	if err != nil {
		return nil, fmt.Errorf("encode image: %w", err)
	}

	ts := r.now()
	path := filepath.Join(s.dir, fmt.Sprintf("%d.jpg", ts.UnixNano()))
	if err := os.WriteFile(path, jpeg, 0o600); err != nil {
		return nil, fmt.Errorf("write image: %w", err)
	}
	shot := &models.Screenshot{
		SessionID: s.id,
		Timestamp: ts.Format(time.RFC3339),
		AppName:   win.Title,
		ImagePath: path,
	}
	if err := r.store.Insert(context.Background(), shot); err != nil {
		os.Remove(path)
		return nil, err
	}
	return shot, nil
}
