//go:build !windows

package clickhook

import hook "github.com/robotn/gohook"

func Start(onClick func()) (func(), error) {
	events := hook.Start()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				if ev.Kind == hook.MouseDown {
					onClick()
				}
			}
		}
	}()
	return func() {
		close(done)
		hook.End()
	}, nil
}
