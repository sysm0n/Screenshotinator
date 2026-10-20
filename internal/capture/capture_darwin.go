//go:build darwin

package capture

/*
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics -framework CoreFoundation
#include <stdlib.h>
int captureFrontWindow(unsigned char **data, int *w, int *h, char *name, int nameLen);
*/
import "C"

import (
	"errors"
	"image"
	"unsafe"
)

func activeWindow() (*Result, error) {
	var (
		data *C.uchar
		w, h C.int
	)
	name := (*C.char)(C.malloc(512))
	defer C.free(unsafe.Pointer(name))

	switch C.captureFrontWindow(&data, &w, &h, name, 512) {
	case 0:
	case -2:
		return nil, errors.New("no on-screen window for the frontmost app (check Screen Recording permission)")
	default:
		return nil, errors.New("CGWindowListCreateImage failed (grant Screen Recording permission)")
	}
	defer C.free(unsafe.Pointer(data))

	width, height := int(w), int(h)
	pix := C.GoBytes(unsafe.Pointer(data), C.int(width*height*4))
	img := &image.RGBA{Pix: pix, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
	return &Result{Title: C.GoString(name), Image: img}, nil
}
