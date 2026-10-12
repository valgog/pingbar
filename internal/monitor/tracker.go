package monitor

import (
	"time"

	"github.com/valgog/pingbar/internal/stats"
)

type EventKind int

const (
	NoEvent EventKind = iota
	OutageStarted
	OutageEnded
)

type Event struct {
	Kind EventKind
	At   time.Time
}

// Tracker turns a stream of samples for one target into outage start/end
// events. An outage starts once stats.DownAfter pings in a row are lost and is
// dated from the first of them; it ends at the first successful ping.
type Tracker struct {
	ID        int64 // database id of the open outage, managed by the caller
	fails     int
	firstFail time.Time
	open      bool
}

func (t *Tracker) Observe(s stats.Sample) Event {
	if s.OK {
		t.fails = 0
		if t.open {
			t.open = false
			return Event{OutageEnded, s.TS}
		}
		return Event{}
	}
	if t.fails == 0 {
		t.firstFail = s.TS
	}
	t.fails++
	if !t.open && t.fails >= stats.DownAfter {
		t.open = true
		return Event{OutageStarted, t.firstFail}
	}
	return Event{}
}
