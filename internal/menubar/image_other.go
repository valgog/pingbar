//go:build !darwin

package menubar

import "fyne.io/systray"

// Large menu images are only implemented on macOS; elsewhere the menu falls
// back to text.
const nativeImages = false

func setItemImage(*systray.MenuItem, []byte, int, int) {}
