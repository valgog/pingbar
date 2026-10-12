// Package view loads what the TUI and the menu bar show, so both agree.
package view

import (
	"fmt"
	"time"

	"github.com/valgog/pingbar/internal/stats"
	"github.com/valgog/pingbar/internal/store"
)

// Window is the span the current status is computed over.
const Window = 30 * time.Second

type Snapshot struct {
	Now      time.Time
	Internet stats.Summary
	Router   stats.Summary
	Outages  []stats.Outage // newest first
}

// Load reads the current status and the outages of the last day.
func Load(st *store.Store, now time.Time, outageLimit int) (Snapshot, error) {
	snap := Snapshot{Now: now}
	for _, t := range []struct {
		name string
		dst  *stats.Summary
	}{{stats.Internet, &snap.Internet}, {stats.Router, &snap.Router}} {
		samples, err := st.Since(t.name, now.Add(-Window))
		if err != nil {
			return snap, err
		}
		*t.dst = stats.Summarize(samples)
	}
	var err error
	snap.Outages, err = st.Outages(now.Add(-24*time.Hour), outageLimit)
	return snap, err
}

// Stale reports whether nothing has been probed recently, i.e. no pingbar
// process is currently sending pings.
func (s Snapshot) Stale() bool {
	return s.Internet.Last == nil || s.Now.Sub(s.Internet.Last.TS) > 5*time.Second
}

// Headline is the one-line verdict shown in the menu bar.
func (s Snapshot) Headline() (stats.Level, string) {
	in := s.Internet
	switch {
	case s.Stale():
		return stats.Unknown, "no data"
	case in.Down && s.Router.Down:
		return stats.Bad, "Wi-Fi down"
	case in.Down:
		return stats.Bad, "offline"
	case in.Loss > 0:
		return in.Level, fmt.Sprintf("%.0f%% loss", in.Loss*100)
	}
	return in.Level, FormatRTT(in.AvgRTT)
}

// FormatRTT renders a round-trip time compactly, e.g. "23 ms".
func FormatRTT(d time.Duration) string {
	if d < 10*time.Millisecond {
		return fmt.Sprintf("%.1f ms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%d ms", d.Milliseconds())
}

// FormatDuration renders an outage length, e.g. "47s" or "3m12s".
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// TargetName is the human label for a target.
func TargetName(t string) string {
	if t == stats.Router {
		return "Router"
	}
	return "Internet"
}
