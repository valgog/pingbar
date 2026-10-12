package menubar

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// systray only sets 16x16 menu-item images, so the graph is set directly on
// the NSMenuItem, found through the systray app delegate's menu by tag (the
// tag is systray's item id).
static void pingbar_set_item_image(int tag, const void *bytes, int len, double w, double h) {
	NSData *data = [NSData dataWithBytes:bytes length:len];
	dispatch_async(dispatch_get_main_queue(), ^{
		@try {
			NSMenu *menu = [(NSObject *)[NSApp delegate] valueForKey:@"menu"];
			NSMenuItem *item = [menu itemWithTag:tag];
			if (item == nil) {
				return;
			}
			NSImage *img = [[NSImage alloc] initWithData:data];
			[img setSize:NSMakeSize(w, h)];
			item.image = img;
		} @catch (NSException *e) {
			NSLog(@"pingbar: cannot set menu image: %@", e);
		}
	});
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"fyne.io/systray"
)

const nativeImages = true

// setItemImage shows png (drawn at chart.Scale) on item at w×h points.
func setItemImage(item *systray.MenuItem, png []byte, w, h int) {
	var id int
	if _, err := fmt.Sscanf(item.String(), "MenuItem[%d", &id); err != nil || len(png) == 0 {
		return
	}
	C.pingbar_set_item_image(C.int(id), unsafe.Pointer(&png[0]), C.int(len(png)), C.double(w), C.double(h))
}
