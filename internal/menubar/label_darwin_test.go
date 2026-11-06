package menubar

import (
	_ "embed"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed testdata/label.m
var menuLabelTestSource string

func TestNativeMenuLabel(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "menu-label-test")
	cmd := exec.Command("xcrun", "clang", "-fobjc-arc", "-framework", "Cocoa",
		"-Itestdata", "-x", "objective-c", "-", "-o", bin)
	cmd.Stdin = strings.NewReader(menuLabelTestSource)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile native menu label test: %v\n%s", err, out)
	}
	if out, err := exec.Command(bin).CombinedOutput(); err != nil {
		t.Fatalf("native menu label test: %v\n%s", err, out)
	}
}
