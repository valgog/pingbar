#import <Cocoa/Cocoa.h>

// A custom label avoids AppKit's disabled-command tint without making an
// informational row clickable. System colors follow the menu's appearance.
static void pingbar_update_label(NSMenuItem *item) {
	NSView *view = item.view;
	NSTextField *label;
	if (view == nil) {
		view = [[NSView alloc] initWithFrame:NSZeroRect];
		label = [NSTextField labelWithString:item.title];
		label.font = [NSFont menuFontOfSize:0];
		label.textColor = [NSColor labelColor];
		label.selectable = NO;
		label.editable = NO;
		[view addSubview:label];
	} else {
		label = (NSTextField *)view.subviews.firstObject;
		label.stringValue = item.title;
	}

	[label sizeToFit];
	NSSize size = label.frame.size;
	label.frame = NSMakeRect(20, 4, size.width, size.height);
	view.frame = NSMakeRect(0, 0, size.width + 40, size.height + 8);
	item.view = view;
}

void pingbar_set_item_label(int tag) {
	dispatch_async(dispatch_get_main_queue(), ^{
		@try {
			NSMenu *menu = [(NSObject *)[NSApp delegate] valueForKey:@"menu"];
			NSMenuItem *item = [menu itemWithTag:tag];
			if (item == nil) {
				NSLog(@"pingbar: cannot set menu label: item %d not found", tag);
				return;
			}
			pingbar_update_label(item);
		} @catch (NSException *e) {
			NSLog(@"pingbar: cannot set menu label: %@", e);
		}
	});
}
