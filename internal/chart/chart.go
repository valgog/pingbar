// Package chart draws the latency graph shown in the menu-bar dropdown.
package chart

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/valgog/pingbar/internal/stats"
)

// Scale is the pixel density the PNGs are drawn at (2 = Retina). Sizes in
// this package are in points; the caller sets the NSImage size in points.
const Scale = 2

var (
	green  = color.RGBA{0x34, 0xC7, 0x59, 0xFF}
	yellow = color.RGBA{0xFF, 0xB3, 0x00, 0xFF}
	red    = color.RGBA{0xFF, 0x3B, 0x30, 0xFF}
	gray   = color.RGBA{0x8E, 0x8E, 0x93, 0xFF}
	grid   = color.RGBA{0x8E, 0x8E, 0x93, 0x60}
)

func levelColor(l stats.Level) color.Color {
	switch l {
	case stats.Good:
		return green
	case stats.Degraded:
		return yellow
	case stats.Bad:
		return red
	}
	return gray
}

var ceilings = []time.Duration{100, 200, 300, 500, 1000, 2000, 5000}

// Ceiling picks the top of the y-axis: the smallest round value at or above
// the slowest bucket, never below 100 ms so normal jitter looks flat.
func Ceiling(buckets []stats.Bucket) time.Duration {
	var max time.Duration
	for _, b := range buckets {
		if b.Recv > 0 && b.AvgRTT > max {
			max = b.AvgRTT
		}
	}
	for _, c := range ceilings {
		if max <= c*time.Millisecond {
			return c * time.Millisecond
		}
	}
	return ceilings[len(ceilings)-1] * time.Millisecond
}

// barHeight is how tall a bucket's bar is in a plot h pixels high. Buckets
// where every ping was lost fill the plot so outages stand out.
func barHeight(b stats.Bucket, ceil time.Duration, h int) int {
	if b.Sent == 0 {
		return 0
	}
	if b.Recv == 0 {
		return h
	}
	y := int(float64(h) * float64(b.AvgRTT) / float64(ceil))
	if y < Scale {
		y = Scale // keep fast pings visible
	}
	if y > h {
		y = h
	}
	return y
}

func fill(img draw.Image, r image.Rectangle, c color.Color) {
	draw.Draw(img, r, &image.Uniform{c}, image.Point{}, draw.Over)
}

// bars draws one bar per bucket into plot, bottom-aligned.
func bars(img draw.Image, plot image.Rectangle, buckets []stats.Bucket, ceil time.Duration) {
	n := len(buckets)
	if n == 0 {
		return
	}
	step := float64(plot.Dx()) / float64(n)
	for i, b := range buckets {
		x0 := plot.Min.X + int(float64(i)*step)
		x1 := plot.Min.X + int(float64(i+1)*step)
		if x1-x0 > 2 {
			x1-- // gap between bars
		}
		h := barHeight(b, ceil, plot.Dy())
		if h == 0 {
			continue
		}
		fill(img, image.Rect(x0, plot.Max.Y-h, x1, plot.Max.Y), levelColor(b.Level()))
	}
}

func encode(img image.Image) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

var face = func() font.Face {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		panic(err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: 9 * Scale, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	return face
}()

func text(img draw.Image, x, y int, s string, alignRight bool) {
	d := &font.Drawer{Dst: img, Src: &image.Uniform{gray}, Face: face}
	if alignRight {
		x -= d.MeasureString(s).Ceil()
	}
	d.Dot = fixed.P(x, y)
	d.DrawString(s)
}

// Full draws a w×h point graph with a latency y-axis in milliseconds and a
// time x-axis covering span, for the dropdown menu.
func Full(buckets []stats.Bucket, span time.Duration, w, h int) []byte {
	const left, bottom, top, right = 40, 14, 6, 4
	img := image.NewRGBA(image.Rect(0, 0, w*Scale, h*Scale))
	plot := image.Rect(left*Scale, top*Scale, (w-right)*Scale, (h-bottom)*Scale)
	ceil := Ceiling(buckets)
	asc := face.Metrics().Ascent.Ceil() / 2

	for _, frac := range []float64{0, 0.5, 1} {
		y := plot.Max.Y - int(frac*float64(plot.Dy()))
		fill(img, image.Rect(plot.Min.X, y, plot.Max.X, y+1), grid)
		v := time.Duration(frac * float64(ceil))
		text(img, plot.Min.X-4*Scale, y+asc, fmt.Sprintf("%d ms", v.Milliseconds()), true)
	}
	bars(img, plot, buckets, ceil)

	lbl := plot.Max.Y + 11*Scale
	text(img, plot.Min.X, lbl, "-"+short(span), false)
	for _, frac := range []float64{1.0 / 3, 2.0 / 3} {
		x := plot.Min.X + int(frac*float64(plot.Dx()))
		s := "-" + short(time.Duration((1-frac)*float64(span)))
		text(img, x-(&font.Drawer{Face: face}).MeasureString(s).Ceil()/2, lbl, s, false)
	}
	text(img, plot.Max.X, lbl, "now", true)
	return encode(img)
}

func short(d time.Duration) string {
	d = d.Round(time.Second)
	if d >= time.Hour && d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	if d >= time.Minute {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
