package stats

import (
	"testing"
	"time"
)

func mk(rtts ...int) []Sample {
	var out []Sample
	t0 := time.Unix(1000, 0)
	for i, r := range rtts {
		s := Sample{TS: t0.Add(time.Duration(i) * time.Second), Target: Internet}
		if r >= 0 {
			s.OK = true
			s.RTT = time.Duration(r) * time.Millisecond
		}
		out = append(out, s)
	}
	return out
}

func TestSummarizeGood(t *testing.T) {
	s := Summarize(mk(20, 22, 20, 22))
	if s.Sent != 4 || s.Recv != 4 || s.Loss != 0 {
		t.Fatalf("counts: %+v", s)
	}
	if s.AvgRTT != 21*time.Millisecond || s.Jitter != 2*time.Millisecond {
		t.Fatalf("avg %v jitter %v", s.AvgRTT, s.Jitter)
	}
	if s.Level != Good || s.Down {
		t.Fatalf("level %v down %v", s.Level, s.Down)
	}
}

func TestSummarizeDown(t *testing.T) {
	s := Summarize(mk(20, 20, 20, 20, 20, 20, 20, 20, 20, 20, -1, -1, -1))
	if !s.Down || s.Level != Bad {
		t.Fatalf("want down/bad, got %+v", s)
	}
}

func TestSummarizeSingleLossIsNotDown(t *testing.T) {
	s := Summarize(mk(20, 20, -1))
	if s.Down {
		t.Fatal("one lost ping should not count as down")
	}
	if s.Level != Bad {
		t.Fatalf("33%% loss should be bad, got %v", s.Level)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		loss   float64
		avg    time.Duration
		jitter time.Duration
		want   Level
	}{
		{0, 30 * time.Millisecond, 5 * time.Millisecond, Good},
		{0.02, 30 * time.Millisecond, 5 * time.Millisecond, Degraded},
		{0, 150 * time.Millisecond, 5 * time.Millisecond, Degraded},
		{0, 30 * time.Millisecond, 50 * time.Millisecond, Degraded},
		{0.10, 30 * time.Millisecond, 5 * time.Millisecond, Bad},
		{0, 400 * time.Millisecond, 5 * time.Millisecond, Bad},
	}
	for _, c := range cases {
		if got := Classify(10, c.loss, c.avg, c.jitter); got != c.want {
			t.Errorf("Classify(%v,%v,%v)=%v want %v", c.loss, c.avg, c.jitter, got, c.want)
		}
	}
	if Classify(0, 0, 0, 0) != Unknown {
		t.Error("no samples should be unknown")
	}
}
