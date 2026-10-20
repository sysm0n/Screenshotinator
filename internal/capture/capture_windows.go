//go:build windows

package capture

import (
	"errors"
	"image"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	pGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	pGetWindowRect       = user32.NewProc("GetWindowRect")
	pGetWindowTextW      = user32.NewProc("GetWindowTextW")
	pGetWindowThreadPID  = user32.NewProc("GetWindowThreadProcessId")
	pGetDC               = user32.NewProc("GetDC")
	pReleaseDC           = user32.NewProc("ReleaseDC")
	pIsIconic            = user32.NewProc("IsIconic")

	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pGetDIBits              = gdi32.NewProc("GetDIBits")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pDeleteDC               = gdi32.NewProc("DeleteDC")

	pOpenProcess      = kernel32.NewProc("OpenProcess")
	pCloseHandle      = kernel32.NewProc("CloseHandle")
	pQueryFullImgName = kernel32.NewProc("QueryFullProcessImageNameW")
)

const (
	srccopy                 = 0x00CC0020
	captureblt              = 0x40000000
	biRGB                   = 0
	dibRGBColors            = 0
	processQueryLimitedInfo = 0x1000
)

type rect struct{ Left, Top, Right, Bottom int32 }

type bitmapInfoHeader struct {
	Size          uint32
	Width, Height int32
	Planes, Bits  uint16
	Compression   uint32
	SizeImage     uint32
	XPels, YPels  int32
	ClrUsed       uint32
	ClrImportant  uint32
}

func activeWindow() (*Result, error) {
	hwnd, _, _ := pGetForegroundWindow.Call()
	if hwnd == 0 {
		return nil, errors.New("no foreground window")
	}
	if r, _, _ := pIsIconic.Call(hwnd); r != 0 {
		return nil, errors.New("foreground window is minimized")
	}
	var rc rect
	if r, _, _ := pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc))); r == 0 {
		return nil, errors.New("GetWindowRect failed")
	}
	w, h := int(rc.Right-rc.Left), int(rc.Bottom-rc.Top)
	if w <= 0 || h <= 0 {
		return nil, errors.New("empty window rect")
	}

	// Copy the window's rectangle from the screen DC (BitBlt + CAPTUREBLT keeps layered windows).
	screenDC, _, _ := pGetDC.Call(0)
	if screenDC == 0 {
		return nil, errors.New("GetDC failed")
	}
	defer pReleaseDC.Call(0, screenDC)
	memDC, _, _ := pCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, errors.New("CreateCompatibleDC failed")
	}
	defer pDeleteDC.Call(memDC)
	bmp, _, _ := pCreateCompatibleBitmap.Call(screenDC, uintptr(w), uintptr(h))
	if bmp == 0 {
		return nil, errors.New("CreateCompatibleBitmap failed")
	}
	defer pDeleteObject.Call(bmp)
	old, _, _ := pSelectObject.Call(memDC, bmp)
	defer pSelectObject.Call(memDC, old)

	if r, _, _ := pBitBlt.Call(memDC, 0, 0, uintptr(w), uintptr(h), screenDC,
		uintptr(rc.Left), uintptr(rc.Top), srccopy|captureblt); r == 0 {
		return nil, errors.New("BitBlt failed")
	}

	bi := bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width: int32(w), Height: -int32(h), Planes: 1, Bits: 32, Compression: biRGB} // negative height = top-down
	pix := make([]byte, w*h*4)
	if r, _, _ := pGetDIBits.Call(memDC, bmp, 0, uintptr(h),
		uintptr(unsafe.Pointer(&pix[0])), uintptr(unsafe.Pointer(&bi)), dibRGBColors); r == 0 {
		return nil, errors.New("GetDIBits failed")
	}
	for i := 0; i < len(pix); i += 4 { // BGRA -> RGBA, force opaque
		pix[i], pix[i+2], pix[i+3] = pix[i+2], pix[i], 255
	}
	img := &image.RGBA{Pix: pix, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	return &Result{Title: windowLabel(hwnd), Image: img}, nil
}

func windowLabel(hwnd uintptr) string {
	buf := make([]uint16, 512)
	n, _, _ := pGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	title := syscall.UTF16ToString(buf[:n])
	proc := processName(hwnd)
	switch {
	case proc != "" && title != "":
		return proc + " – " + title
	case proc != "":
		return proc
	}
	return title
}

func processName(hwnd uintptr) string {
	var pid uint32
	pGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	h, _, _ := pOpenProcess.Call(processQueryLimitedInfo, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer pCloseHandle.Call(h)
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	if r, _, _ := pQueryFullImgName.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r == 0 {
		return ""
	}
	return filepath.Base(syscall.UTF16ToString(buf[:size]))
}
