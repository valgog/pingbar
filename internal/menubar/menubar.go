// Package menubar shows the connection status as a menu-bar item.
package menubar

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"fyne.io/systray"

	"github.com/valgog/pingbar/internal/stats"
	"github.com/valgog/pingbar/internal/store"
	"github.com/valgog/pingbar/internal/view"
)

const (
	maxOutages   = 8
	historyCells = 30
	historyCell  = 30 * time.Second // 30 cells × 30s = last 15 minutes
)

func dot(l stats.Level) string {
	switch l {
	case stats.Good:
		return "🟢"
	case stats.Degraded:
		return "🟡"
	case stats.Bad:
		return "🔴"
	}
	return "⚪️"
}

func square(b stats.Bucket) string {
	switch b.Level() {
	case stats.Good:
		return "🟩"
	case stats.Degraded:
		return "🟨"
	case stats.Bad:
		return "🟥"
	}
	return "⬜️"
}

// Run shows the menu-bar item; it must be called from main and blocks until Quit.
func Run(st *store.Store, onExit func()) {
	systray.Run(func() { onReady(st) }, onExit)
}

func onReady(st *store.Store) {
	systray.SetTitle("⚪️")
	systray.SetTooltip("pingbar")

	internet := systray.AddMenuItem("Internet: …", "")
	internet.Disable()
	router := systray.AddMenuItem("Router: …", "")
	router.Disable()
	systray.AddSeparator()
	histTitle := systray.AddMenuItem("Last 15 minutes", "")
	histTitle.Disable()
	hist := systray.AddMenuItem("", "Each square is 30 seconds")
	hist.Disable()
	systray.AddSeparator()
	outTitle := systray.AddMenuItem("Outages today", "")
	outTitle.Disable()
	outs := make([]*systray.MenuItem, maxOutages)
	for i := range outs {
		outs[i] = systray.AddMenuItem("", "")
		outs[i].Disable()
		outs[i].Hide()
	}
	systray.AddSeparator()
	openTUI := systray.AddMenuItem("Open dashboard in Terminal", "Run pingbar in a Terminal window")
	quit := systray.AddMenuItem("Quit pingbar", "")

	go func() {
		for {
			select {
			case <-openTUI.ClickedCh:
				openTerminal()
			case <-quit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	update := func() {
		now := time.Now()
		snap, err := view.Load(st, now, maxOutages)
		if err != nil {
			systray.SetTitle("⚠️")
			systray.SetTooltip(err.Error())
			return
		}
		level, text := snap.Headline()
		systray.SetTitle(dot(level) + " " + text)
		systray.SetTooltip("pingbar: " + text)
		internet.SetTitle(describe("Internet", snap.Internet))
		router.SetTitle(describe("Router", snap.Router))

		start := now.Truncate(historyCell).Add(-(historyCells - 1) * historyCell)
		if bs, err := st.Buckets(stats.Internet, start, historyCell, historyCells); err == nil {
			var sb strings.Builder
			for _, b := range bs {
				sb.WriteString(square(b))
			}
			hist.SetTitle(sb.String())
		}

		midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		n := 0
		for _, o := range snap.Outages {
			if o.Start.Before(midnight) || n >= len(outs) {
				break
			}
			label := fmt.Sprintf("%s  %s  %s", o.Start.Format("15:04:05"), view.TargetName(o.Target), view.FormatDuration(o.Duration(now)))
			if o.End.IsZero() {
				label += " (ongoing)"
			}
			outs[n].SetTitle(label)
			outs[n].Show()
			n++
		}
		for _, m := range outs[n:] {
			m.Hide()
		}
		if n == 0 {
			outTitle.SetTitle("No outages today")
		} else {
			outTitle.SetTitle("Outages today")
		}
	}
	update()
	go func() {
		for range time.Tick(time.Second) {
			update()
		}
	}()
}

func describe(name string, s stats.Summary) string {
	if s.Sent == 0 {
		return name + ": no data"
	}
	if s.Down {
		return fmt.Sprintf("%s %s: down", dot(s.Level), name)
	}
	return fmt.Sprintf("%s %s: %s, %.0f%% loss, jitter %s", dot(s.Level), name,
		view.FormatRTT(s.AvgRTT), s.Loss*100, view.FormatRTT(s.Jitter))
}

func openTerminal() {
	if runtime.GOOS != "darwin" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		log.Printf("open terminal: %v", err)
		return
	}
	if err := exec.Command("open", "-a", "Terminal", exe).Run(); err != nil {
		log.Printf("open terminal: %v", err)
	}
}

// Quit closes the menu-bar item, making Run return.
func Quit() { systray.Quit() }
