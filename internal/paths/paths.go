// Package paths locates pingbar's data directory.
package paths

import (
	"os"
	"path/filepath"
)

// DataDir returns ~/Library/Application Support/pingbar on macOS
// (~/.config/pingbar elsewhere), creating it if needed. PINGBAR_DIR overrides it.
func DataDir() (string, error) {
	dir := os.Getenv("PINGBAR_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "pingbar")
	}
	return dir, os.MkdirAll(dir, 0o755)
}
