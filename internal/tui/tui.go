// Package tui is the terminal dashboard.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/valgog/pingbar/internal/monitor"
	"github.com/valgog/pingbar/internal/probe"
	"github.com/valgog/pingbar/internal/stats"
	"github.com/valgog/pingbar/internal/store"
	"github.com/valgog/pingbar/internal/view"
)

var zooms = []time.Duration{time.Second, 10 * time.Second, time.Minute, 5 * time.Minute}

var (
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	gray   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	bold   = lipgloss.NewStyle().Bold(true)
)

func levelStyle(l stats.Level) lipgloss.Style {
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

type tickMsg time.Time

type model struct {
	st      *store.Store
	mon     *monitor.Monitor
	host    string
	gateway string
	zoom    int
	width   int
	snap    view.Snapshot
	lines   map[string][]stats.Bucket
	err     error
	ticks   int
}

// Run shows the dashboard until the user quits.
func Run(st *store.Store, mon *monitor.Monitor) error {
	m := &model{st: st, mon: mon, host: mon.Host, zoom: 1, width: 80}
	m.refresh()
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *model) Init() tea.Cmd { return tick() }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "+", "=", "right", "l":
			if m.zoom < len(zooms)-1 {
				m.zoom++
			}
			m.refresh()
		case "-", "left", "h":
			if m.zoom > 0 {
				m.zoom--
			}
			m.refresh()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.refresh()
	case tickMsg:
		m.ticks++
		m.refresh()
		return m, tick()
	}
	return m, nil
}

const labelWidth = 10

func (m *model) cells() int {
	n := m.width - labelWidth - 2
	if n < 10 {
		n = 10
	}
	return n
}

func (m *model) refresh() {
	now := time.Now()
	if m.ticks%15 == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		m.gateway = probe.Gateway(ctx)
		cancel()
	}
	m.snap, m.err = view.Load(m.st, now, 10)
	if m.err != nil {
		return
	}
	bw := zooms[m.zoom]
	n := m.cells()
	start := now.Truncate(bw).Add(-time.Duration(n-1) * bw)
	m.lines = map[string][]stats.Bucket{}
	for _, t := range []string{stats.Internet, stats.Router} {
		m.lines[t], m.err = m.st.Buckets(t, start, bw, n)
		if m.err != nil {
			return
		}
	}
}

func (m *model) View() string {
	var b strings.Builder
	mode := "probing"
	if !m.mon.Probing() {
		mode = "showing pings from the running menu-bar instance"
	}
	fmt.Fprintf(&b, "%s  %s\n\n", bold.Render("pingbar"), gray.Render(mode))
	if m.err != nil {
		fmt.Fprintf(&b, "%s\n", red.Render("error: "+m.err.Error()))
	}

	gw := m.gateway
	if gw == "" {
		gw = "no gateway"
	}
	m.statusLine(&b, "Internet", m.host, m.snap.Internet)
	m.statusLine(&b, "Router", gw, m.snap.Router)
	if m.snap.Stale() {
		fmt.Fprintf(&b, "%s\n", yellow.Render("No fresh pings yet. Waiting for data…"))
	}

	bw := zooms[m.zoom]
	n := m.cells()
	fmt.Fprintf(&b, "\n%s %s\n", bold.Render("History"), gray.Render(fmt.Sprintf("(%s per cell, last %s)", bw, view.FormatDuration(bw*time.Duration(n)))))
	for _, t := range []string{stats.Internet, stats.Router} {
		fmt.Fprintf(&b, "%-*s", labelWidth, view.TargetName(t))
		for _, c := range m.lines[t] {
			if c.Sent == 0 {
				b.WriteString(gray.Render("·"))
			} else {
				b.WriteString(levelStyle(c.Level()).Render("█"))
			}
		}
		b.WriteString("\n")
	}
	left := "-" + view.FormatDuration(bw*time.Duration(n))
	pad := n - len(left) - 3
	if pad < 1 {
		pad = 1
	}
	fmt.Fprintf(&b, "%s%s%s%s\n", strings.Repeat(" ", labelWidth), gray.Render(left), strings.Repeat(" ", pad), gray.Render("now"))

	fmt.Fprintf(&b, "\n%s %s\n", bold.Render("Outages"), gray.Render("(last 24h)"))
	if len(m.snap.Outages) == 0 {
		b.WriteString(green.Render("none") + "\n")
	}
	for _, o := range m.snap.Outages {
		dur := view.FormatDuration(o.Duration(m.snap.Now))
		if o.End.IsZero() {
			dur = red.Render(dur + " ongoing")
		}
		fmt.Fprintf(&b, "%s  %-9s %s\n", o.Start.Format("Mon 15:04:05"), view.TargetName(o.Target), dur)
	}

	fmt.Fprintf(&b, "\n%s\n", gray.Render("+/- zoom · q quit"))
	return b.String()
}

func (m *model) statusLine(b *strings.Builder, name, addr string, s stats.Summary) {
	dot := levelStyle(s.Level).Render("●")
	if s.Sent == 0 {
		fmt.Fprintf(b, "%s %-9s %-16s %s\n", dot, name, addr, gray.Render("no data"))
		return
	}
	verdict := s.Level.String()
	if s.Down {
		verdict = "DOWN"
	}
	rtt, jitter := "–", "–"
	if s.Recv > 0 {
		rtt, jitter = view.FormatRTT(s.AvgRTT), view.FormatRTT(s.Jitter)
	}
	fmt.Fprintf(b, "%s %-9s %-16s %8s  loss %5.1f%%  jitter %7s  %s\n",
		dot, name, addr, rtt, s.Loss*100, jitter, levelStyle(s.Level).Render(verdict))
}
