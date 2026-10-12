// Package stats turns raw ping samples into the numbers and traffic-light
// levels that matter for a video call.
package stats

import (
	"time"
)

// Targets probed by the monitor.
const (
	Internet = "internet"
	Router   = "router"
)

// Sample is the result of one ping. RTT is zero when OK is false.
type Sample struct {
	TS     time.Time
	Target string
	RTT    time.Duration
	OK     bool
}

// Level is a traffic-light rating of connection quality.
type Level int

const (
	Unknown Level = iota
	Good
	Degraded
	Bad
)

func (l Level) String() string {
	switch l {
	case Good:
		return "good"
	case Degraded:
		return "degraded"
	case Bad:
		return "bad"
	}
	return "unknown"
}

// Thresholds roughly follow what a Google Meet call tolerates: under 1% loss
// and 100 ms is fine, past 5% loss or 250 ms video freezes and audio drops.
var (
	GoodLoss     = 0.01
	GoodRTT      = 100 * time.Millisecond
	GoodJitter   = 30 * time.Millisecond
	DegradedLoss = 0.05
	DegradedRTT  = 250 * time.Millisecond
	DegradedJit  = 75 * time.Millisecond
)

// Classify rates a loss ratio, average RTT and jitter.
func Classify(sent int, loss float64, avg, jitter time.Duration) Level {
	switch {
	case sent == 0:
		return Unknown
	case loss < GoodLoss && avg < GoodRTT && jitter < GoodJitter:
		return Good
	case loss < DegradedLoss && avg < DegradedRTT && jitter < DegradedJit:
		return Degraded
	}
	return Bad
}

// Summary describes recent samples for one target.
type Summary struct {
	Sent, Recv int
	Loss       float64
	AvgRTT     time.Duration
	Jitter     time.Duration
	Level      Level
	Down       bool // the most recent pings all failed
	Last       *Sample
}

// DownAfter is how many consecutive lost pings count as the link being down.
const DownAfter = 3

// Summarize computes a Summary over chronologically ordered samples of one
// target. Jitter is the mean absolute difference between consecutive RTTs.
func Summarize(samples []Sample) Summary {
	var s Summary
	var sum, jitSum time.Duration
	var prev time.Duration
	var jitN int
	for i := range samples {
		x := samples[i]
		s.Sent++
		if !x.OK {
			continue
		}
		s.Recv++
		sum += x.RTT
		if s.Recv > 1 {
			d := x.RTT - prev
			if d < 0 {
				d = -d
			}
			jitSum += d
			jitN++
		}
		prev = x.RTT
	}
	if s.Sent == 0 {
		return s
	}
	s.Last = &samples[len(samples)-1]
	s.Loss = float64(s.Sent-s.Recv) / float64(s.Sent)
	if s.Recv > 0 {
		s.AvgRTT = sum / time.Duration(s.Recv)
	}
	if jitN > 0 {
		s.Jitter = jitSum / time.Duration(jitN)
	}
	if len(samples) >= DownAfter {
		s.Down = true
		for _, x := range samples[len(samples)-DownAfter:] {
			if x.OK {
				s.Down = false
				break
			}
		}
	} else {
		s.Down = s.Recv == 0
	}
	s.Level = Classify(s.Sent, s.Loss, s.AvgRTT, s.Jitter)
	if s.Down {
		s.Level = Bad
	}
	return s
}

// Bucket aggregates the samples of one target over a time slice.
type Bucket struct {
	Start  time.Time
	Sent   int
	Recv   int
	AvgRTT time.Duration
}

// Level rates a bucket on loss and latency only.
func (b Bucket) Level() Level {
	if b.Sent == 0 {
		return Unknown
	}
	return Classify(b.Sent, float64(b.Sent-b.Recv)/float64(b.Sent), b.AvgRTT, 0)
}

// Outage is a period during which a target did not answer.
type Outage struct {
	ID     int64
	Target string
	Start  time.Time
	End    time.Time // zero while ongoing
}

// Duration returns how long the outage lasted, measured up to now if ongoing.
func (o Outage) Duration(now time.Time) time.Duration {
	if o.End.IsZero() {
		return now.Sub(o.Start)
	}
	return o.End.Sub(o.Start)
}
