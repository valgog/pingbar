// Package probe sends ICMP echo requests and finds the default gateway.
package probe

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"runtime"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// ErrLost means no reply arrived before the timeout.
var ErrLost = errors.New("no reply")

// Pinger sends one echo request to addr and returns the round-trip time.
type Pinger func(ctx context.Context, addr string, timeout time.Duration) (time.Duration, error)

// ICMP returns a Pinger using ICMP datagram sockets. Unprivileged sockets work
// without sudo on macOS; on Linux they need net.ipv4.ping_group_range.
func ICMP(privileged bool) Pinger {
	return func(ctx context.Context, addr string, timeout time.Duration) (time.Duration, error) {
		p, err := probing.NewPinger(addr)
		if err != nil {
			return 0, err
		}
		p.SetPrivileged(privileged)
		p.Count = 1
		p.Timeout = timeout
		if err := p.RunWithContext(ctx); err != nil {
			return 0, err
		}
		st := p.Statistics()
		if st.PacketsRecv == 0 || len(st.Rtts) == 0 {
			return 0, ErrLost
		}
		return st.Rtts[0], nil
	}
}

var (
	darwinGW = regexp.MustCompile(`(?m)^\s*gateway:\s*(\S+)`)
	linuxGW  = regexp.MustCompile(`default via (\S+)`)
)

// Gateway returns the IPv4 address of the default router, or "" if there is none.
func Gateway(ctx context.Context) string {
	var out []byte
	var re *regexp.Regexp
	switch runtime.GOOS {
	case "darwin":
		out, _ = exec.CommandContext(ctx, "/sbin/route", "-n", "get", "default").Output()
		re = darwinGW
	case "linux":
		out, _ = exec.CommandContext(ctx, "ip", "-4", "route", "show", "default").Output()
		re = linuxGW
	default:
		return ""
	}
	return ParseGateway(re, string(out))
}

// ParseGateway extracts the gateway address from route output.
func ParseGateway(re *regexp.Regexp, out string) string {
	m := re.FindStringSubmatch(out)
	if m == nil {
		return ""
	}
	return m[1]
}
