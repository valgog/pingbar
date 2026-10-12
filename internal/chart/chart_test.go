package chart

import (
	"bytes"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/valgog/pingbar/internal/stats"
)

func sampleBuckets(n int) []stats.Bucket {
	out := make([]stats.Bucket, n)
	for i := range out {
		out[i] = stats.Bucket{Sent: 10, Recv: 10, AvgRTT: time.Duration(20+i%7*5) * time.Millisecond}
	}
	out[n/3] = stats.Bucket{Sent: 10, Recv: 0}                                  // outage
	out[n/2] = stats.Bucket{Sent: 10, Recv: 10, AvgRTT: 180 * time.Millisecond} // slow
	out[n-2] = stats.Bucket{}                                                   // no data
	return out
}

func TestCeiling(t *testing.T) {
	if c := Ceiling(nil); c != 100*time.Millisecond {
		t.Errorf("empty: %v", c)
	}
	if c := Ceiling(sampleBuckets(30)); c != 200*time.Millisecond {
		t.Errorf("with a 180ms bucket: %v", c)
	}
	if c := Ceiling([]stats.Bucket{{Sent: 1, Recv: 1, AvgRTT: 9 * time.Second}}); c != 5*time.Second {
		t.Errorf("huge: %v", c)
	}
}

func TestBarHeight(t *testing.T) {
	ceil := 100 * time.Millisecond
	if h := barHeight(stats.Bucket{Sent: 5}, ceil, 40); h != 40 {
		t.Errorf("all lost should fill the plot, got %d", h)
	}
	if h := barHeight(stats.Bucket{}, ceil, 40); h != 0 {
		t.Errorf("no data should draw nothing, got %d", h)
	}
	if h := barHeight(stats.Bucket{Sent: 1, Recv: 1, AvgRTT: 50 * time.Millisecond}, ceil, 40); h != 20 {
		t.Errorf("half the ceiling should be half height, got %d", h)
	}
	if h := barHeight(stats.Bucket{Sent: 1, Recv: 1, AvgRTT: time.Second}, ceil, 40); h != 40 {
		t.Errorf("above the ceiling should clip, got %d", h)
	}
}

func TestPNGs(t *testing.T) {
	for name, b := range map[string][]byte{
		"full": Full(sampleBuckets(90), 15*time.Minute, 300, 110),
	} {
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if img.Bounds().Dx() == 0 {
			t.Fatalf("%s: empty image", name)
		}
		if dir := os.Getenv("CHART_OUT"); dir != "" {
			os.WriteFile(dir+"/"+name+".png", b, 0o644)
		}
	}
}
