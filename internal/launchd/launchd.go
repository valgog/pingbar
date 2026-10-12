// Package launchd installs pingbar as a per-user LaunchAgent so the menu-bar
// item starts at login.
package launchd

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

const Label = "com.github.valgog.pingbar"

// PlistPath is ~/Library/LaunchAgents/<Label>.plist.
func PlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

func esc(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// Plist renders the LaunchAgent. KeepAlive restarts pingbar if it crashes but
// not after "Quit" in the menu, which exits cleanly.
func Plist(args []string, logPath string) string {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + Label + `</string>
	<key>ProgramArguments</key>
	<array>
`)
	for _, a := range args {
		fmt.Fprintf(&b, "\t\t<string>%s</string>\n", esc(a))
	}
	fmt.Fprintf(&b, `	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>LimitLoadToSessionType</key>
	<string>Aqua</string>
	<key>ProcessType</key>
	<string>Interactive</string>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, esc(logPath), esc(logPath))
	return b.String()
}

func domain() string { return "gui/" + strconv.Itoa(os.Getuid()) }

// Install writes the LaunchAgent for args (args[0] is the executable) and starts it.
func Install(args []string, logPath string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("install is only supported on macOS")
	}
	path, err := PlistPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	// Replace any running copy so a reinstall picks up a new binary or flags.
	exec.Command("launchctl", "bootout", domain()+"/"+Label).Run()
	if err := os.WriteFile(path, []byte(Plist(args, logPath)), 0o644); err != nil {
		return "", err
	}
	if out, err := exec.Command("launchctl", "bootstrap", domain(), path).CombinedOutput(); err != nil {
		return path, fmt.Errorf("launchctl bootstrap: %v: %s", err, bytes.TrimSpace(out))
	}
	return path, nil
}

// Uninstall stops the LaunchAgent and removes its plist.
func Uninstall() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("uninstall is only supported on macOS")
	}
	path, err := PlistPath()
	if err != nil {
		return "", err
	}
	exec.Command("launchctl", "bootout", domain()+"/"+Label).Run()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return path, err
	}
	return path, nil
}
