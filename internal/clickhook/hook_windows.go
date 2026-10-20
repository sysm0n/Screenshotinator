//go:build windows

package clickhook

import (
	"errors"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	pSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	pUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	pCallNextHookEx      = user32.NewProc("CallNextHookEx")
	pGetMessageW         = user32.NewProc("GetMessageW")
	pPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	pGetCurrentThreadId  = kernel32.NewProc("GetCurrentThreadId")
)

const (
	whMouseLL     = 14
	wmQuit        = 0x0012
	wmLButtonDown = 0x0201
	wmRButtonDown = 0x0204
	wmMButtonDown = 0x0207
)

// syscall.NewCallback slots are never freed, so create exactly one and route through a variable.
var (
	cbOnce  sync.Once
	cb      uintptr
	handler struct {
		sync.RWMutex
		fn func()
	}
)

// All parameters are uintptr: syscall.NewCallback only supports pointer-sized arguments.
func callback(nCode, wParam, lParam uintptr) uintptr {
	defer func() { _ = recover() }()
	if int32(nCode) >= 0 {
		switch wParam {
		case wmLButtonDown, wmRButtonDown, wmMButtonDown:
			handler.RLock()
			fn := handler.fn
			handler.RUnlock()
			if fn != nil {
				fn()
			}
		}
	}
	r, _, _ := pCallNextHookEx.Call(0, nCode, wParam, lParam)
	return r
}

func Start(onClick func()) (func(), error) {
	cbOnce.Do(func() { cb = syscall.NewCallback(callback) })
	handler.Lock()
	handler.fn = onClick
	handler.Unlock()

	type ready struct {
		tid uint32
		err error
	}
	rdy := make(chan ready, 1)
	finished := make(chan struct{})

	go func() {
		defer close(finished)
		// Low-level hooks are delivered to the installing thread's message loop.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		tid, _, _ := pGetCurrentThreadId.Call()
		h, _, err := pSetWindowsHookExW.Call(whMouseLL, cb, 0, 0)
		if h == 0 {
			rdy <- ready{err: errors.New("SetWindowsHookEx failed: " + err.Error())}
			return
		}
		rdy <- ready{tid: uint32(tid)}

		var msg [48]byte // MSG is 48 bytes on amd64, 28 on 386; 48 is large enough for both
		for {
			r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg[0])), 0, 0, 0)
			if int32(r) <= 0 { // WM_QUIT or error
				break
			}
		}
		pUnhookWindowsHookEx.Call(h)
	}()

	r := <-rdy
	if r.err != nil {
		return nil, r.err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			pPostThreadMessageW.Call(uintptr(r.tid), wmQuit, 0, 0)
			<-finished
			handler.Lock()
			handler.fn = nil
			handler.Unlock()
		})
	}, nil
}
