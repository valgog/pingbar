// Package monitor runs the probe loops and records results.
//
// Only one process probes at a time: whoever holds the lock file writes
// samples, and every other pingbar process (the TUI while the menu bar is
// running, or vice versa) just reads the database. If the prober exits, a
// waiting process takes over within a couple of seconds.
package monitor

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/valgog/pingbar/internal/probe"
	"github.com/valgog/pingbar/internal/stats"
	"github.com/valgog/pingbar/internal/store"
)

type Monitor struct {
	Store     *store.Store
	Host      string        // internet target, e.g. 8.8.8.8
	Interval  time.Duration // time between pings per target
	Retention time.Duration // how long raw samples are kept
	LockPath  string
	Ping      probe.Pinger
	Gateway   func(context.Context) string

	probing atomic.Bool
}

// Probing reports whether this process is currently the one sending pings.
func (m *Monitor) Probing() bool { return m.probing.Load() }

// Run blocks until ctx is cancelled, probing whenever this process holds the lock.
func (m *Monitor) Run(ctx context.Context) {
	for {
		unlock, ok := tryLock(m.LockPath)
		if ok {
			m.probing.Store(true)
			m.probe(ctx)
			m.probing.Store(false)
			unlock()
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func tryLock(path string) (func(), bool) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		log.Printf("lock: %v", err)
		return nil, false
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, false
	}
	return func() { f.Close() }, true
}

func (m *Monitor) probe(ctx context.Context) {
	if err := m.Store.CloseDangling(); err != nil {
		log.Printf("close dangling outages: %v", err)
	}
	samples := make(chan stats.Sample, 64)
	go m.loop(ctx, stats.Internet, func(context.Context) string { return m.Host }, false, samples)
	go m.loop(ctx, stats.Router, m.Gateway, true, samples)

	trackers := map[string]*Tracker{stats.Internet: {}, stats.Router: {}}
	prune := time.NewTicker(time.Hour)
	defer prune.Stop()
	m.prune()
	for {
		select {
		case <-ctx.Done():
			m.closeOpen(trackers, time.Now())
			return
		case <-prune.C:
			m.prune()
		case s := <-samples:
			if err := m.Store.Insert(s); err != nil {
				log.Printf("insert: %v", err)
			}
			m.track(trackers[s.Target], s)
		}
	}
}

func (m *Monitor) prune() {
	if m.Retention > 0 {
		if err := m.Store.Prune(time.Now().Add(-m.Retention)); err != nil {
			log.Printf("prune: %v", err)
		}
	}
}

func (m *Monitor) track(t *Tracker, s stats.Sample) {
	switch ev := t.Observe(s); ev.Kind {
	case OutageStarted:
		id, err := m.Store.StartOutage(s.Target, ev.At)
		if err != nil {
			log.Printf("start outage: %v", err)
		}
		t.ID = id
	case OutageEnded:
		if t.ID != 0 {
			if err := m.Store.EndOutage(t.ID, ev.At); err != nil {
				log.Printf("end outage: %v", err)
			}
		}
		t.ID = 0
	}
}

func (m *Monitor) closeOpen(trackers map[string]*Tracker, now time.Time) {
	for _, t := range trackers {
		if t.ID != 0 {
			m.Store.EndOutage(t.ID, now)
		}
	}
}

// loop pings one target every Interval. Ticks are dropped rather than queued
// if a ping runs long, so samples for a target stay in order.
//
// For the router, samples are only recorded once the current gateway has
// answered at least once: some routers ignore ping, and a VPN can leave no
// pingable gateway at all, which would otherwise look like a permanent outage.
func (m *Monitor) loop(ctx context.Context, target string, addrFn func(context.Context) string, needReply bool, out chan<- stats.Sample) {
	tick := time.NewTicker(m.Interval)
	defer tick.Stop()
	var addr string
	var lastLookup time.Time
	answered := false
	for {
		now := time.Now()
		if addr == "" || now.Sub(lastLookup) > 15*time.Second {
			if a := addrFn(ctx); a != addr {
				addr, answered = a, false
			}
			lastLookup = now
		}
		s := stats.Sample{TS: now, Target: target}
		if addr != "" {
			rtt, err := m.Ping(ctx, addr, m.Interval)
			if ctx.Err() != nil {
				return
			}
			s.OK, s.RTT = err == nil, rtt
		}
		if s.OK {
			answered = true
		} else if target == stats.Router {
			lastLookup = time.Time{} // re-detect the gateway next time
		}
		if !needReply || answered {
			select {
			case out <- s:
			case <-ctx.Done():
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
