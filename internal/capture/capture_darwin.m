//go:build darwin

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>

// CGWindowListCreateImage is flagged deprecated/obsolete in recent SDKs; resolve it at runtime
// so the build never depends on the SDK's availability annotations.
typedef CGImageRef (*CGWLCreateImageFn)(CGRect, CGWindowListOption, CGWindowID, CGWindowImageOption);

// Returns 0 on success, -1 if the image could not be created, -2 if no window was found.
// *data is malloc'd RGBA (non-premultiplied alpha is irrelevant: we force opaque) and must be freed by the caller.
int captureFrontWindow(unsigned char **data, int *w, int *h, char *name, int nameLen) {
	@autoreleasepool {
		NSRunningApplication *front = [[NSWorkspace sharedWorkspace] frontmostApplication];
		if (!front) return -2;
		pid_t pid = front.processIdentifier;

		CFArrayRef list = CGWindowListCopyWindowInfo(
			kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
		if (!list) return -2;

		CGWindowID winID = 0;
		NSString *title = @"";
		// The list is ordered front-to-back; the first layer-0 window of the frontmost app is the focused one.
		for (NSDictionary *info in (__bridge NSArray *)list) {
			if ([info[(id)kCGWindowOwnerPID] intValue] != pid) continue;
			if ([info[(id)kCGWindowLayer] intValue] != 0) continue;
			CGRect b;
			CGRectMakeWithDictionaryRepresentation((__bridge CFDictionaryRef)info[(id)kCGWindowBounds], &b);
			if (b.size.width < 50 || b.size.height < 50) continue;
			winID = [info[(id)kCGWindowNumber] unsignedIntValue];
			NSString *owner = info[(id)kCGWindowOwnerName] ?: @"";
			NSString *wname = info[(id)kCGWindowName] ?: @"";
			title = wname.length ? [NSString stringWithFormat:@"%@ – %@", owner, wname] : owner;
			break;
		}
		CFRelease(list);
		if (!winID) return -2;

		CGWLCreateImageFn create = (CGWLCreateImageFn)dlsym(RTLD_DEFAULT, "CGWindowListCreateImage");
		if (!create) return -1;
		CGImageRef img = create(CGRectNull, kCGWindowListOptionIncludingWindow, winID,
			kCGWindowImageBoundsIgnoreFraming | kCGWindowImageBestResolution);
		if (!img) return -1;

		size_t iw = CGImageGetWidth(img), ih = CGImageGetHeight(img);
		if (iw == 0 || ih == 0) { CGImageRelease(img); return -1; }
		unsigned char *buf = calloc(iw * ih * 4, 1);
		CGColorSpaceRef cs = CGColorSpaceCreateDeviceRGB();
		CGContextRef ctx = CGBitmapContextCreate(buf, iw, ih, 8, iw * 4, cs,
			kCGImageAlphaPremultipliedLast | kCGBitmapByteOrder32Big);
		CGContextDrawImage(ctx, CGRectMake(0, 0, iw, ih), img);
		CGContextRelease(ctx);
		CGColorSpaceRelease(cs);
		CGImageRelease(img);

		*data = buf; *w = (int)iw; *h = (int)ih;
		strncpy(name, title.UTF8String, nameLen - 1);
		name[nameLen - 1] = 0;
		return 0;
	}
}
