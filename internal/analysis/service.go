// Package analysis runs vision-model jobs for screenshots and tracks their state.
//
// Every job sends exactly one screenshot. Jobs run concurrently up to a worker limit; starting a
// job for a screenshot that is already being analyzed cancels the earlier one, so the latest
// instructions always win.
package analysis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"

	"github.com/sysm0n/Screenshotinator/internal/events"
	"github.com/sysm0n/Screenshotinator/internal/llm"
	"github.com/sysm0n/Screenshotinator/internal/models"
)

// ErrClosed is returned by Start after Close.
var ErrClosed = errors.New("analysis service is shut down")

// Analyzer is the model backend (implemented by *llm.Client).
type Analyzer interface {
	Analyze(ctx context.Context, req llm.Request) (string, error)
}

// Store is the persistence the service needs (implemented by *storage.DB).
type Store interface {
	Get(ctx context.Context, id int64) (*models.Screenshot, error)
	SetSummary(ctx context.Context, id int64, summary string) error
}

type Service struct {
	ai     Analyzer
	store  Store
	events events.Emitter
	log    *slog.Logger
	sem    chan struct{}

	ctx    context.Context // cancelled by Close
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu   sync.Mutex
	jobs map[int64]*job   // in-flight, by screenshot ID
	errs map[int64]string // last failure, by screenshot ID
}

type job struct{ cancel context.CancelFunc }

func New(ai Analyzer, store Store, ev events.Emitter, log *slog.Logger, workers int) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		ai: ai, store: store, events: ev, log: log,
		sem:    make(chan struct{}, max(workers, 1)),
		ctx:    ctx,
		jobs:   make(map[int64]*job),
		errs:   make(map[int64]string),
		cancel: cancel,
	}
}

// Start queues an analysis of screenshot id. It returns once the job is queued; progress and the
// result are reported through events.ScreenshotUpdated.
func (s *Service) Start(id int64, instructions string) error {
	shot, err := s.store.Get(s.ctx, id)
	if err != nil {
		return err
	}
	jpeg, err := os.ReadFile(shot.ImagePath)
	if err != nil {
		return fmt.Errorf("read screenshot image: %w", err)
	}

	s.mu.Lock()
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		return ErrClosed
	}
	if prev := s.jobs[id]; prev != nil {
		prev.cancel()
	}
	ctx, cancel := context.WithCancel(s.ctx)
	j := &job{cancel: cancel}
	s.jobs[id] = j
	delete(s.errs, id)
	s.wg.Add(1)
	s.mu.Unlock()

	s.emitUpdate(id)
	go s.run(ctx, id, j, llm.Request{JPEG: jpeg, WindowTitle: shot.AppName, Instructions: instructions})
	return nil
}

// Forget cancels any job for id and drops its state (call when the screenshot is deleted).
func (s *Service) Forget(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j := s.jobs[id]; j != nil {
		j.cancel()
		delete(s.jobs, id)
	}
	delete(s.errs, id)
}

// Decorate fills the runtime fields (Analyzing, AIError) of the given screenshots.
func (s *Service) Decorate(shots ...*models.Screenshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sh := range shots {
		sh.Analyzing = s.jobs[sh.ID] != nil
		sh.AIError = s.errs[sh.ID]
	}
}

// Close cancels all jobs and waits for them to exit. Start fails afterwards.
func (s *Service) Close() {
	s.mu.Lock()
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *Service) run(ctx context.Context, id int64, j *job, req llm.Request) {
	defer s.wg.Done()
	defer j.cancel()

	summary, err := s.analyze(ctx, req)
	if err == nil && s.isCurrent(id, j) {
		err = s.store.SetSummary(s.ctx, id, summary)
	}

	s.mu.Lock()
	if s.jobs[id] != j { // superseded by a newer job or forgotten: it owns the state now
		s.mu.Unlock()
		return
	}
	delete(s.jobs, id)
	if err != nil && !errors.Is(err, context.Canceled) {
		s.errs[id] = err.Error()
	}
	s.mu.Unlock()

	if err != nil {
		s.log.Warn("analysis failed", "screenshot", id, "err", err)
	}
	if s.ctx.Err() == nil {
		s.emitUpdate(id)
	}
}

// analyze waits for a worker slot and calls the model, converting a panic into an error.
func (s *Service) analyze(ctx context.Context, req llm.Request) (summary string, err error) {
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() {
		if p := recover(); p != nil {
			s.log.Error("analysis panic", "panic", p, "stack", string(debug.Stack()))
			err = fmt.Errorf("internal error: %v", p)
		}
	}()
	return s.ai.Analyze(ctx, req)
}

func (s *Service) isCurrent(id int64, j *job) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobs[id] == j
}

func (s *Service) emitUpdate(id int64) {
	shot, err := s.store.Get(s.ctx, id)
	if err != nil {
		return // deleted meanwhile
	}
	s.Decorate(shot)
	s.events.Emit(events.ScreenshotUpdated, shot)
}
