package launchd

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestPlistIsValidXML(t *testing.T) {
	p := Plist([]string{"/Users/me/bin/pingbar", "menubar", "-host", "1.1.1.1&x"}, "/tmp/pingbar.log")
	d := xml.NewDecoder(strings.NewReader(p))
	d.Strict = false
	for {
		if _, err := d.Token(); err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("invalid xml: %v\n%s", err, p)
		}
	}
	for _, want := range []string{"<string>menubar</string>", "1.1.1.1&amp;x", Label} {
		if !strings.Contains(p, want) {
			t.Errorf("plist missing %q", want)
		}
	}
}
