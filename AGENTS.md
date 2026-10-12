# AGENTS.md

Context for coding agents (and humans) working on pingbar. Read this first; `README.md` covers end-user usage.

## What it is

A single Go binary that pings the internet (default `8.8.8.8`) and the default router once a second, stores every result in SQLite, and shows status in a terminal dashboard (`pingbar`) or a macOS menu-bar item (`pingbar menubar`, installed as a LaunchAgent by `pingbar install`). The goal is knowing whether the network was fine during a Google Meet call, so thresholds and wording are tuned for video calls.

macOS is the target platform. Linux builds and runs (CI tests it) so most code can be developed and tested there, but the menu bar and `install` only matter on macOS.

## Commands

```sh
go build ./...                 # build everything
go test ./...                  # unit tests; no network or root needed
go vet ./...
gofmt -l .                     # must print nothing (CI fails otherwise)
make build                     # ./pingbar
make install                   # copies to ~/.local/bin
PINGBAR_DIR=$(mktemp -d) ./pingbar   # run against a throwaway database
```

CI (`.github/workflows/ci.yml`) runs gofmt, vet, test, build and a `status`/`outages` smoke test on `macos-latest` and `ubuntu-latest`. The macOS job is the only place the cgo/Objective-C code is compiled, so a change to `internal/menubar/*_darwin.go` is only verified once CI runs.

## Layout

| Package | Role |
|---|---|
| `main.go` | Subcommand dispatch (`tui` default, `menubar`, `install`, `uninstall`, `status`, `outages`) and shared flags. |
| `internal/probe` | One ICMP echo per call via pro-bing (unprivileged datagram sockets by default) and default-gateway lookup (`route -n get default` on macOS, `ip route` on Linux). |
| `internal/monitor` | Probe loops, one goroutine per target, a single writer goroutine, and outage detection (`Tracker`). Leader election through `flock` on `probe.lock`. |
| `internal/store` | SQLite (pure-Go `modernc.org/sqlite`, WAL). Tables `samples(ts ms, target, rtt_us NULL=lost)` and `outages(id, target, start_ts, end_ts NULL=ongoing)`. Bucketed aggregation is done in SQL. |
| `internal/stats` | Pure types and math: `Sample`, `Summary`, `Bucket`, `Outage`, `Level` and the quality thresholds. |
| `internal/view` | What both UIs show: `Snapshot` (last 30 s summary plus recent outages), `Headline`, and formatting helpers. Put shared display logic here, not in a UI package. |
| `internal/tui` | Bubble Tea dashboard. |
| `internal/menubar` | fyne.io/systray menu. `image_darwin.go` uses cgo to put a full-size image on an `NSMenuItem`, because systray forces 16×16 icons. `image_other.go` is the no-op stub. |
| `internal/chart` | Draws the dropdown latency graph as a PNG (pure Go, Go font). |
| `internal/launchd` | Writes `~/Library/LaunchAgents/com.github.valgog.pingbar.plist` and calls `launchctl bootstrap/bootout`. |
| `internal/paths` | Data directory: `~/Library/Application Support/pingbar` (or `$PINGBAR_DIR`). |

## Design rules worth keeping

- **One prober, many viewers.** Only the process holding the `flock` writes samples; every UI reads from SQLite, including the prober's own UI. Don't add a second data path that bypasses the database. A viewer takes over within about 2 s when the prober exits.
- **Per-target ordering.** Each target has its own sequential loop and the ping timeout equals the interval, so samples for one target arrive in order. `Tracker` relies on that.
- **The router only counts once it has answered.** Some routers drop ICMP and VPNs can hide the gateway, so router samples are recorded only after the current gateway replied once. Keep this, or the router will look permanently down for those users.
- **Outages** start after `stats.DownAfter` (3) consecutive losses, dated from the first loss, and end at the next reply. A process that dies leaves `end_ts` NULL; `CloseDangling` fixes those on the next start.
- **Thresholds** live in `internal/stats` (good: < 1% loss, < 100 ms, < 30 ms jitter; degraded: < 5%, < 250 ms, < 75 ms). Change them there and keep the README table in sync.
- **No sudo.** Unprivileged ICMP must keep working on macOS; `-privileged` is opt-in.
- **Main thread.** `systray.Run` must be called from `main`'s goroutine. UI updates from other goroutines go through systray (or `dispatch_async` to the main queue in the cgo code).
- Logs go to `pingbar.log` in the data directory; the TUI redirects `log` there so it never draws over the screen.

## Testing approach

- Logic is kept out of the UI packages so it can be unit-tested on Linux without a display or network: stats, store (temp-dir SQLite), monitor (fake `Pinger` and `Gateway`), tracker, chart (decodes the PNG; set `CHART_OUT=dir` to write it out for a look), launchd plist rendering.
- The sandbox and CI usually cannot send real ICMP, so don't write tests that need the network.
- To see the TUI without a real terminal, run it under a pty. Bubble Tea waits up to 5 s for a terminal background-color reply before the first frame, so give it more than that.
- Menu-bar changes need a manual check on a Mac: `make install && pingbar install`, then click the icon.

## Conventions

- Standard Go style, `gofmt`, short doc comments on packages and exported identifiers; comments explain *why*.
- Keep dependencies few and pure Go where possible; cgo is only used for macOS UI.
- Work on a branch and open a PR against `main`; CI must be green before merging. The PR description says what changed for the user (before/after) and what was verified where (sandbox vs. a real Mac).
- License: MIT (`LICENSE`).
