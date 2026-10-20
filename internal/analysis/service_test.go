package analysis

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sysm0n/Screenshotinator/internal/events"
	"github.com/sysm0n/Screenshotinator/internal/llm"
	"github.com/sysm0n/Screenshotinator/internal/models"
)

type memStore struct {
	mu    sync.Mutex
	shots map[int64]*models.Screenshot
}

func (m *memStore) Get(_ context.Context, id int64) (*models.Screenshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.shots[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *s
	return &cp, nil
}

func (m *memStore) SetSummary(_ context.Context, id int64, summary string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shots[id].AISummary = summary
	return nil
}

func (m *memStore) summary(id int64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shots[id].AISummary
}

type analyzerFunc func(ctx context.Context, req llm.Request) (string, error)

func (f analyzerFunc) Analyze(ctx context.Context, req llm.Request) (string, error) {
	return f(ctx, req)
}

// recorder collects ScreenshotUpdated events.
type recorder struct{ ch chan *models.Screenshot }

func (r recorder) Emit(name string, data any) {
	if name == events.ScreenshotUpdated {
		r.ch <- data.(*models.Screenshot)
	}
}

// waitIdle returns the first update event for id that reports the job as finished.
func (r recorder) waitIdle(t *testing.T, id int64) *models.Screenshot {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case s := <-r.ch:
			if s.ID == id && !s.Analyzing {
				return s
			}
		case <-timeout:
			t.Fatal("timed out waiting for analysis to finish")
		}
	}
}

func setup(t *testing.T, ai Analyzer) (*Service, *memStore, recorder) {
	t.Helper()
	img := filepath.Join(t.TempDir(), "1.jpg")
	if err := os.WriteFile(img, []byte("jpeg-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &memStore{shots: map[int64]*models.Screenshot{1: {ID: 1, AppName: "Code", ImagePath: img}}}
	ev := recorder{ch: make(chan *models.Screenshot, 32)}
	s := New(ai, store, ev, slog.New(slog.NewTextHandler(io.Discard, nil)), 2)
	t.Cleanup(s.Close)
	return s, store, ev
}

func TestAnalyzeSuccess(t *testing.T) {
	var got llm.Request
	s, store, ev := setup(t, analyzerFunc(func(_ context.Context, req llm.Request) (string, error) {
		got = req
		return "A code editor.", nil
	}))

	if err := s.Start(1, "focus on errors"); err != nil {
		t.Fatal(err)
	}
	final := ev.waitIdle(t, 1)

	if final.AISummary != "A code editor." || final.AIError != "" {
		t.Fatalf("final = %+v", final)
	}
	if store.summary(1) != "A code editor." {
		t.Error("summary not persisted")
	}
	if string(got.JPEG) != "jpeg-bytes" || got.WindowTitle != "Code" || got.Instructions != "focus on errors" {
		t.Errorf("request = %+v", got)
	}
}

func TestAnalyzeFailureKeepsSummaryAndReportsError(t *testing.T) {
	s, store, ev := setup(t, analyzerFunc(func(context.Context, llm.Request) (string, error) {
		return "", errors.New("connection refused")
	}))
	store.shots[1].AISummary = "old summary"

	if err := s.Start(1, ""); err != nil {
		t.Fatal(err)
	}
	final := ev.waitIdle(t, 1)

	if final.AIError != "connection refused" {
		t.Errorf("AIError = %q", final.AIError)
	}
	if store.summary(1) != "old summary" {
		t.Errorf("summary overwritten on failure: %q", store.summary(1))
	}
}

func TestNewerJobSupersedesOlder(t *testing.T) {
	firstStarted := make(chan struct{})
	var calls int
	var mu sync.Mutex
	s, store, ev := setup(t, analyzerFunc(func(ctx context.Context, req llm.Request) (string, error) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			close(firstStarted)
			<-ctx.Done() // the first request hangs until it is cancelled
			return "", ctx.Err()
		}
		return "second: " + req.Instructions, nil
	}))

	if err := s.Start(1, "one"); err != nil {
		t.Fatal(err)
	}
	<-firstStarted
	if err := s.Start(1, "two"); err != nil {
		t.Fatal(err)
	}
	final := ev.waitIdle(t, 1)

	if final.AISummary != "second: two" || final.AIError != "" {
		t.Fatalf("final = %+v", final)
	}
	if store.summary(1) != "second: two" {
		t.Errorf("stored summary = %q", store.summary(1))
	}
}

func TestCloseCancelsJobsAndRejectsNewOnes(t *testing.T) {
	started := make(chan struct{})
	s, _, _ := setup(t, analyzerFunc(func(ctx context.Context, _ llm.Request) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}))
	if err := s.Start(1, ""); err != nil {
		t.Fatal(err)
	}
	<-started

	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return: in-flight job was not cancelled")
	}
	if err := s.Start(1, ""); !errors.Is(err, ErrClosed) {
		t.Fatalf("Start after Close: err = %v, want ErrClosed", err)
	}
}
