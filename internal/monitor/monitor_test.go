package monitor

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valgog/pingbar/internal/stats"
	"github.com/valgog/pingbar/internal/store"
)

func TestTracker(t *testing.T) {
	var tr Tracker
	t0 := time.Unix(0, 0)
	at := func(i int) time.Time { return t0.Add(time.Duration(i) * time.Second) }
	seq := []bool{true, false, false, true, false, false, false, false, true}
	var events []Event
	for i, ok := range seq {
		if ev := tr.Observe(stats.Sample{TS: at(i), OK: ok}); ev.Kind != NoEvent {
			events = append(events, ev)
		}
	}
	want := []Event{{OutageStarted, at(4)}, {OutageEnded, at(8)}}
	if len(events) != len(want) || events[0] != want[0] || events[1] != want[1] {
		t.Fatalf("got %+v want %+v", events, want)
	}
}

func TestMonitorRecordsOutage(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var n atomic.Int32
	m := &Monitor{
		Store:    st,
		Host:     "192.0.2.1",
		Interval: 10 * time.Millisecond,
		LockPath: filepath.Join(dir, "lock"),
		// Internet: 3 OK, 5 lost, then OK forever. Router never answers.
		Ping: func(ctx context.Context, addr string, _ time.Duration) (time.Duration, error) {
			if addr == "10.0.0.1" {
				return 0, errors.New("lost")
			}
			i := n.Add(1)
			if i > 3 && i <= 8 {
				return 0, errors.New("lost")
			}
			return 5 * time.Millisecond, nil
		},
		Gateway: func(context.Context) string { return "10.0.0.1" },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	m.Run(ctx)

	inet, _ := st.Since(stats.Internet, time.Time{})
	if len(inet) < 10 {
		t.Fatalf("expected internet samples, got %d", len(inet))
	}
	if r, _ := st.Since(stats.Router, time.Time{}); len(r) != 0 {
		t.Fatalf("router never answered, so nothing should be recorded; got %d", len(r))
	}
	os, _ := st.Outages(time.Time{}, 10)
	if len(os) != 1 || os[0].Target != stats.Internet || os[0].End.IsZero() {
		t.Fatalf("outages: %+v", os)
	}
}

func TestOnlyOneProber(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "lock")
	unlock, ok := tryLock(lock)
	if !ok {
		t.Fatal("first lock failed")
	}
	if _, ok := tryLock(lock); ok {
		t.Fatal("second lock should fail while the first is held")
	}
	unlock()
	if u, ok := tryLock(lock); !ok {
		t.Fatal("lock should be free after release")
	} else {
		u()
	}
}
