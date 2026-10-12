package view

import (
	"testing"
	"time"

	"github.com/valgog/pingbar/internal/stats"
)

func TestHeadline(t *testing.T) {
	now := time.Unix(1000, 0)
	last := &stats.Sample{TS: now}
	cases := []struct {
		name string
		snap Snapshot
		want string
	}{
		{"stale", Snapshot{Now: now}, "no data"},
		{"good", Snapshot{Now: now, Internet: stats.Summary{Last: last, Level: stats.Good, AvgRTT: 23 * time.Millisecond}}, "23 ms"},
		{"lossy", Snapshot{Now: now, Internet: stats.Summary{Last: last, Level: stats.Bad, Loss: 0.1}}, "10% loss"},
		{"offline", Snapshot{Now: now, Internet: stats.Summary{Last: last, Down: true}}, "offline"},
		{"wifi", Snapshot{Now: now, Internet: stats.Summary{Last: last, Down: true}, Router: stats.Summary{Down: true}}, "Wi-Fi down"},
	}
	for _, c := range cases {
		if _, got := c.snap.Headline(); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		47 * time.Second:            "47s",
		192 * time.Second:           "3m12s",
		2*time.Hour + 5*time.Minute: "2h05m",
		1500 * time.Millisecond:     "2s",
	} {
		if got := FormatDuration(d); got != want {
			t.Errorf("%v: got %q want %q", d, got, want)
		}
	}
}
