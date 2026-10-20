package recorder

import (
	"context"
	"errors"
	"image"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/sysm0n/Screenshotinator/internal/capture"
	"github.com/sysm0n/Screenshotinator/internal/events"
	"github.com/sysm0n/Screenshotinator/internal/storage"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeClock) Add(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

type fakeHook struct {
	mu      sync.Mutex
	onClick func()
	stopped bool
}

func (h *fakeHook) install(onClick func()) (func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onClick, h.stopped = onClick, false
	return func() { h.mu.Lock(); h.stopped = true; h.mu.Unlock() }, nil
}

func (h *fakeHook) click() {
	h.mu.Lock()
	fn := h.onClick
	h.mu.Unlock()
	fn()
}

func newTestRecorder(t *testing.T) (*Recorder, *fakeHook, *fakeClock, *storage.DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(context.Background(), dir+"/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	r := New(Options{ShotsDir: dir, Cooldown: 1500 * time.Millisecond, MaxImageWidth: 1280, JPEGQuality: 80},
		db, events.Discard, slog.New(slog.NewTextHandler(io.Discard, nil)))
	hook := &fakeHook{}
	clock := &fakeClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	r.installHook = hook.install
	r.now = clock.Now
	r.grabWindow = func() (*capture.Result, error) {
		return &capture.Result{Title: "Terminal – zsh", Image: image.NewRGBA(image.Rect(0, 0, 64, 32))}, nil
	}
	return r, hook, clock, db
}

func TestRecorderThrottlesAndCollectsSession(t *testing.T) {
	r, hook, clock, _ := newTestRecorder(t)

	id, err := r.Start()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Start(); !errors.Is(err, ErrActive) {
		t.Fatalf("second Start: err = %v, want ErrActive", err)
	}

	hook.click()                      // accepted
	hook.click()                      // same instant: throttled
	clock.Add(time.Second)            //
	hook.click()                      // 1s later: throttled
	clock.Add(600 * time.Millisecond) //
	hook.click()                      // 1.6s after the first: accepted

	res, err := r.Stop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != id || len(res.Screenshots) != 2 {
		t.Fatalf("got session %s with %d shots, want %s with 2", res.SessionID, len(res.Screenshots), id)
	}
	if got := res.Screenshots[0].AppName; got != "Terminal – zsh" {
		t.Errorf("AppName = %q", got)
	}
	if !hook.stopped {
		t.Error("click hook was not removed")
	}
	if r.Active() {
		t.Error("recorder still active after Stop")
	}
}

func TestRecorderIgnoresClicksAfterStop(t *testing.T) {
	r, hook, clock, db := newTestRecorder(t)
	id, _ := r.Start()
	if _, err := r.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Add(time.Minute)
	hook.click() // late event from the old hook

	shots, err := db.ListBySession(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(shots) != 0 {
		t.Fatalf("captured %d screenshots after Stop", len(shots))
	}
	if _, err := r.Stop(context.Background()); !errors.Is(err, ErrNotActive) {
		t.Fatalf("second Stop: err = %v, want ErrNotActive", err)
	}
}

func TestRecorderSkipsFailedCaptures(t *testing.T) {
	r, hook, _, _ := newTestRecorder(t)
	r.grabWindow = func() (*capture.Result, error) { return nil, errors.New("no foreground window") }

	if _, err := r.Start(); err != nil {
		t.Fatal(err)
	}
	hook.click()
	res, err := r.Stop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Screenshots) != 0 {
		t.Fatalf("got %d screenshots, want 0", len(res.Screenshots))
	}
}
