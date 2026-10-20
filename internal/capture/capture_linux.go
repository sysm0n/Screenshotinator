//go:build linux

package capture

/*
#cgo LDFLAGS: -lX11
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <stdlib.h>
#include <string.h>

// X11 only. Returns 0 on success; data is malloc'd RGBA.
static int grabFocused(unsigned char **data, int *w, int *h, char *name, int nameLen) {
	Display *dpy = XOpenDisplay(NULL);
	if (!dpy) return -1;
	Window root = DefaultRootWindow(dpy), focus; int revert;
	XGetInputFocus(dpy, &focus, &revert);
	if (focus == None || focus == PointerRoot || focus == root) { XCloseDisplay(dpy); return -2; }

	// The focus window is often a child; climb to the top-level window (direct child of root).
	for (;;) {
		Window r, parent, *kids = NULL; unsigned int n;
		if (!XQueryTree(dpy, focus, &r, &parent, &kids, &n)) break;
		if (kids) XFree(kids);
		if (parent == root || parent == None) break;
		focus = parent;
	}

	XWindowAttributes at;
	if (!XGetWindowAttributes(dpy, focus, &at) || at.map_state != IsViewable) { XCloseDisplay(dpy); return -2; }
	int x, y; Window dummy;
	XTranslateCoordinates(dpy, focus, root, 0, 0, &x, &y, &dummy);

	// Clamp to the screen.
	XWindowAttributes rt; XGetWindowAttributes(dpy, root, &rt);
	int x0 = x < 0 ? 0 : x, y0 = y < 0 ? 0 : y;
	int x1 = x + at.width > rt.width ? rt.width : x + at.width;
	int y1 = y + at.height > rt.height ? rt.height : y + at.height;
	int cw = x1 - x0, ch = y1 - y0;
	if (cw <= 0 || ch <= 0) { XCloseDisplay(dpy); return -2; }

	char *fetched = NULL; XClassHint hint = {0};
	name[0] = 0;
	if (XGetClassHint(dpy, focus, &hint) && hint.res_class) { strncpy(name, hint.res_class, nameLen - 1); name[nameLen-1] = 0; }
	if (XFetchName(dpy, focus, &fetched) && fetched) {
		size_t l = strlen(name);
		if (l + 4 < (size_t)nameLen) { strcat(name, " - "); strncat(name, fetched, nameLen - l - 4); }
		XFree(fetched);
	}
	if (hint.res_name) XFree(hint.res_name);
	if (hint.res_class) XFree(hint.res_class);

	XImage *im = XGetImage(dpy, root, x0, y0, cw, ch, AllPlanes, ZPixmap);
	if (!im) { XCloseDisplay(dpy); return -3; }
	unsigned char *buf = malloc((size_t)cw * ch * 4);
	unsigned long rm = im->red_mask, gm = im->green_mask, bm = im->blue_mask;
	int rs = __builtin_ctzl(rm), gs = __builtin_ctzl(gm), bs = __builtin_ctzl(bm);
	for (int j = 0; j < ch; j++)
		for (int i = 0; i < cw; i++) {
			unsigned long p = XGetPixel(im, i, j);
			unsigned char *o = buf + ((size_t)j * cw + i) * 4;
			o[0] = (p & rm) >> rs; o[1] = (p & gm) >> gs; o[2] = (p & bm) >> bs; o[3] = 255;
		}
	XDestroyImage(im);
	XCloseDisplay(dpy);
	*data = buf; *w = cw; *h = ch;
	return 0;
}
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

	switch rc := C.grabFocused(&data, &w, &h, name, 512); rc {
	case 0:
	case -1:
		return nil, errors.New("cannot open X11 display (Wayland is not supported)")
	case -2:
		return nil, errors.New("no focused viewable window")
	default:
		return nil, errors.New("XGetImage failed")
	}
	defer C.free(unsafe.Pointer(data))

	width, height := int(w), int(h)
	pix := C.GoBytes(unsafe.Pointer(data), C.int(width*height*4))
	return &Result{
		Title: C.GoString(name),
		Image: &image.RGBA{Pix: pix, Stride: width * 4, Rect: image.Rect(0, 0, width, height)},
	}, nil
}
