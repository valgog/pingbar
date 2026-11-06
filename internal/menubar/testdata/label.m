#import "../label_darwin.m"
#include <assert.h>

static void check_label(NSMenuItem *item) {
	assert(!item.enabled);
	assert(item.view != nil);
	assert(item.view.subviews.count == 1);
	NSTextField *label = (NSTextField *)item.view.subviews.firstObject;
	assert([label.stringValue isEqualToString:item.title]);
	assert(label.enabled);
	assert(!label.editable);
	assert(!label.selectable);
	assert(!label.drawsBackground);
	assert([label.font isEqual:[NSFont menuFontOfSize:0]]);
	assert([label.textColor isEqual:[NSColor labelColor]]);
	assert(NSContainsRect(item.view.bounds, label.frame));
	assert(label.frame.size.width >= label.intrinsicContentSize.width);
	assert(label.frame.size.height >= label.intrinsicContentSize.height);
}

int main(void) {
	@autoreleasepool {
		NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@"Internet: no data"
			action:nil keyEquivalent:@""];
		item.enabled = NO;
		pingbar_update_label(item);
		check_label(item);
		NSView *view = item.view;
		CGFloat initialWidth = view.frame.size.width;

		item.title = @"Internet: 385 ms, 11% loss, jitter 394 ms";
		pingbar_update_label(item);
		check_label(item);
		assert(item.view == view);
		assert(view.frame.size.width > initialWidth);

		item.title = @"";
		pingbar_update_label(item);
		check_label(item);
		assert(view.frame.size.width < initialWidth);

		item.title = @"18:52:12  Internet  6s (ongoing)";
		pingbar_update_label(item);
		check_label(item);
		item.hidden = YES;
		assert(item.hidden);
		item.hidden = NO;
		assert(!item.hidden);

		for (NSAppearanceName name in @[NSAppearanceNameAqua, NSAppearanceNameDarkAqua]) {
			NSAppearance *appearance = [NSAppearance appearanceNamed:name];
			view.appearance = appearance;
			[appearance performAsCurrentDrawingAppearance:^{
				NSTextField *label = (NSTextField *)view.subviews.firstObject;
				NSColor *color = [label.textColor colorUsingColorSpace:[NSColorSpace genericRGBColorSpace]];
				NSColor *disabled = [[NSColor disabledControlTextColor]
					colorUsingColorSpace:[NSColorSpace genericRGBColorSpace]];
				assert(color != nil);
				assert(disabled != nil);
				CGFloat background = [name isEqualToString:NSAppearanceNameAqua] ? 1 : 0;
				CGFloat brightness = color.redComponent * color.alphaComponent
					+ background * (1 - color.alphaComponent);
				CGFloat dimmed = disabled.redComponent * disabled.alphaComponent
					+ background * (1 - disabled.alphaComponent);
				if ([name isEqualToString:NSAppearanceNameAqua]) {
					assert(brightness < dimmed);
				} else {
					assert(brightness > dimmed);
				}
			}];
		}
	}
	return 0;
}
