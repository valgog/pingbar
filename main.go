// pingbar continuously pings the internet and your router and shows the
// result in the terminal or as a macOS menu-bar item, with history.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/valgog/pingbar/internal/launchd"
	"github.com/valgog/pingbar/internal/menubar"
	"github.com/valgog/pingbar/internal/monitor"
	"github.com/valgog/pingbar/internal/paths"
	"github.com/valgog/pingbar/internal/probe"
	"github.com/valgog/pingbar/internal/store"
	"github.com/valgog/pingbar/internal/tui"
	"github.com/valgog/pingbar/internal/view"
)

const usage = `pingbar: watch your internet connection, with history.

Usage:
  pingbar [flags]             live dashboard in the terminal
  pingbar menubar [flags]     show status in the macOS menu bar
  pingbar install [flags]     start the menu-bar item now and at every login
  pingbar uninstall           remove the menu-bar login item
  pingbar status              print the current status once
  pingbar outages             list outages of the last 7 days

Flags:
`

type options struct {
	host       string
	interval   time.Duration
	retention  int
	privileged bool
}

func (o *options) register(fs *flag.FlagSet) {
	fs.StringVar(&o.host, "host", "8.8.8.8", "internet host to ping")
	fs.DurationVar(&o.interval, "interval", time.Second, "time between pings")
	fs.IntVar(&o.retention, "retention-days", 30, "days of per-ping history to keep (outages are kept forever)")
	fs.BoolVar(&o.privileged, "privileged", false, "use raw ICMP sockets (needs root)")
}

// args renders the flags back for the LaunchAgent.
func (o *options) args() []string {
	return []string{"-host", o.host, "-interval", o.interval.String(),
		"-retention-days", fmt.Sprint(o.retention), fmt.Sprintf("-privileged=%v", o.privileged)}
}

func main() {
	cmd := "tui"
	args := os.Args[1:]
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' {
		cmd, args = args[0], args[1:]
	}
	var opts options
	fs := flag.NewFlagSet("pingbar", flag.ExitOnError)
	opts.register(fs)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), usage)
		fs.PrintDefaults()
	}
	fs.Parse(args)

	if err := run(cmd, opts, fs); err != nil {
		fmt.Fprintln(os.Stderr, "pingbar:", err)
		os.Exit(1)
	}
}

func run(cmd string, opts options, fs *flag.FlagSet) error {
	dir, err := paths.DataDir()
	if err != nil {
		return err
	}
	switch cmd {
	case "install":
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if exe, err = filepath.EvalSymlinks(exe); err != nil {
			return err
		}
		plist, err := launchd.Install(append([]string{exe, "menubar"}, opts.args()...), filepath.Join(dir, "pingbar.log"))
		if err != nil {
			return err
		}
		fmt.Printf("Installed %s\nThe menu-bar item is starting now and will start at every login.\n", plist)
		return nil
	case "uninstall":
		plist, err := launchd.Uninstall()
		if err != nil {
			return err
		}
		fmt.Printf("Removed %s\n", plist)
		return nil
	case "tui", "menubar", "status", "outages":
	case "help":
		fs.Usage()
		return nil
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %q", cmd)
	}

	st, err := store.Open(filepath.Join(dir, "history.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	switch cmd {
	case "status":
		return printStatus(st)
	case "outages":
		return printOutages(st)
	}

	mon := &monitor.Monitor{
		Store:     st,
		Host:      opts.host,
		Interval:  opts.interval,
		Retention: time.Duration(opts.retention) * 24 * time.Hour,
		LockPath:  filepath.Join(dir, "probe.lock"),
		Ping:      probe.ICMP(opts.privileged),
		Gateway:   probe.Gateway,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	done := make(chan struct{})
	go func() { mon.Run(ctx); close(done) }()
	stop := func() { cancel(); <-done }

	if cmd == "menubar" {
		go func() { <-ctx.Done(); menubar.Quit() }()
		menubar.Run(st, stop)
		return nil
	}
	// The TUI owns the terminal, so keep library logging out of it.
	log.SetOutput(openLog(dir))
	defer stop()
	return tui.Run(st, mon)
}

func openLog(dir string) *os.File {
	f, err := os.OpenFile(filepath.Join(dir, "pingbar.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return os.Stderr
	}
	return f
}

func printStatus(st *store.Store) error {
	snap, err := view.Load(st, time.Now(), 1)
	if err != nil {
		return err
	}
	level, text := snap.Headline()
	fmt.Printf("%s: %s\n", level, text)
	if snap.Stale() {
		fmt.Println("(no pingbar process is probing; start `pingbar` or `pingbar install`)")
	}
	return nil
}

func printOutages(st *store.Store) error {
	now := time.Now()
	os_, err := st.Outages(now.Add(-7*24*time.Hour), 1000)
	if err != nil {
		return err
	}
	if len(os_) == 0 {
		fmt.Println("No outages in the last 7 days.")
	}
	for _, o := range os_ {
		end := "ongoing"
		if !o.End.IsZero() {
			end = o.End.Format("15:04:05")
		}
		fmt.Printf("%s – %-8s  %-8s  %s\n", o.Start.Format("2006-01-02 15:04:05"), end, view.TargetName(o.Target), view.FormatDuration(o.Duration(now)))
	}
	return nil
}
