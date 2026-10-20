package main

import (
	"context"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// uiEmitter forwards events to the Wails frontend once the runtime context is bound at startup.
// Events emitted before that are dropped (no window exists yet).
type uiEmitter struct {
	ctx atomic.Pointer[context.Context]
}

func (e *uiEmitter) bind(ctx context.Context) { e.ctx.Store(&ctx) }

func (e *uiEmitter) Emit(name string, data any) {
	if ctx := e.ctx.Load(); ctx != nil {
		runtime.EventsEmit(*ctx, name, data)
	}
}
