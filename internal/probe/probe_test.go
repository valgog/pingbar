package probe

import "testing"

func TestParseGateway(t *testing.T) {
	darwin := `   route to: default
destination: default
       mask: default
    gateway: 192.168.178.1
  interface: en0
`
	if got := ParseGateway(darwinGW, darwin); got != "192.168.178.1" {
		t.Errorf("darwin: %q", got)
	}
	if got := ParseGateway(linuxGW, "default via 10.0.0.1 dev eth0 proto dhcp\n"); got != "10.0.0.1" {
		t.Errorf("linux: %q", got)
	}
	if got := ParseGateway(darwinGW, "route: writing to routing socket: not in table\n"); got != "" {
		t.Errorf("no route: %q", got)
	}
}
