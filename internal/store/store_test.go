package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/valgog/pingbar/internal/stats"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSamplesRoundTrip(t *testing.T) {
	s := open(t)
	t0 := time.UnixMilli(1_700_000_000_000)
	in := []stats.Sample{
		{TS: t0, Target: stats.Internet, RTT: 20 * time.Millisecond, OK: true},
		{TS: t0.Add(time.Second), Target: stats.Internet},
		{TS: t0.Add(time.Second), Target: stats.Router, RTT: 2 * time.Millisecond, OK: true},
	}
	if err := s.Insert(in...); err != nil {
		t.Fatal(err)
	}
	got, err := s.Since(stats.Internet, t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].OK || got[0].RTT != 20*time.Millisecond || got[1].OK {
		t.Fatalf("got %+v", got)
	}
}

func TestBuckets(t *testing.T) {
	s := open(t)
	t0 := time.UnixMilli(1_700_000_000_000)
	var in []stats.Sample
	for i := 0; i < 20; i++ {
		in = append(in, stats.Sample{TS: t0.Add(time.Duration(i) * time.Second), Target: stats.Internet, RTT: 10 * time.Millisecond, OK: i != 12})
	}
	if err := s.Insert(in...); err != nil {
		t.Fatal(err)
	}
	b, err := s.Buckets(stats.Internet, t0, 10*time.Second, 3)
	if err != nil {
		t.Fatal(err)
	}
	if b[0].Sent != 10 || b[0].Recv != 10 || b[0].AvgRTT != 10*time.Millisecond {
		t.Errorf("bucket 0: %+v", b[0])
	}
	if b[1].Sent != 10 || b[1].Recv != 9 {
		t.Errorf("bucket 1: %+v", b[1])
	}
	if b[2].Sent != 0 || b[2].Level() != stats.Unknown {
		t.Errorf("bucket 2: %+v", b[2])
	}
}

func TestOutages(t *testing.T) {
	s := open(t)
	t0 := time.UnixMilli(1_700_000_000_000)
	id, err := s.StartOutage(stats.Internet, t0)
	if err != nil {
		t.Fatal(err)
	}
	s.Insert(stats.Sample{TS: t0.Add(5 * time.Second), Target: stats.Internet})
	if err := s.CloseDangling(); err != nil {
		t.Fatal(err)
	}
	os, err := s.Outages(t0.Add(-time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(os) != 1 || os[0].ID != id || os[0].Duration(time.Time{}) != 5*time.Second {
		t.Fatalf("got %+v", os)
	}
}
