# pingbar

Keeps an eye on your internet connection so you know whether it's your network when a Google Meet call stutters.

pingbar pings `8.8.8.8` and your router once a second, stores every result in a local SQLite file, and shows the status:

- **in the macOS menu bar**: 🟢 `23 ms`, 🟡 `180 ms`, 🔴 `offline` or 🔴 `Wi-Fi down`. Clicking it shows details, a latency graph of the last 15 minutes (milliseconds on the y-axis, lost pings as red bars), and today's outages.
- **in the terminal**: a live dashboard with a zoomable history (1s, 10s, 1m or 5m per cell) and an outage log.

Because the router is pinged as well, it can tell a Wi-Fi or router problem (both down) apart from an internet/ISP problem (router fine, internet down).

## Install

Requires Go 1.26+ and Xcode command-line tools (the menu-bar item uses Cocoa through cgo).

```sh
git clone git@github.com:valgog/pingbar.git
cd pingbar
make install            # copies the binary to ~/.local/bin
pingbar install         # starts the menu-bar item now and at every login
```

`pingbar uninstall` removes the login item. The binary path is recorded in the login item, so run `pingbar install` again after moving or rebuilding it somewhere else.

## Usage

```
pingbar                 live dashboard in the terminal
pingbar menubar         show status in the menu bar (what the login item runs)
pingbar install         start the menu-bar item now and at every login
pingbar uninstall       remove the login item
pingbar status          print the current status once
pingbar outages         list outages of the last 7 days

Flags: -host 8.8.8.8  -interval 1s  -retention-days 30  -privileged
```

You can run the menu-bar item and any number of terminal dashboards at the same time. Only one process sends pings (whoever holds the lock file); the others read the shared history. If the probing process quits, another one takes over within two seconds.

## How it rates the connection

Over the last 30 seconds, roughly what a video call tolerates:

| Level | Loss | Avg latency | Jitter |
|---|---|---|---|
| 🟢 good | < 1% | < 100 ms | < 30 ms |
| 🟡 degraded | < 5% | < 250 ms | < 75 ms |
| 🔴 bad | otherwise | | |

Three lost pings in a row count as an outage and are logged with start time and duration. A reply slower than the ping interval (1 s) counts as lost.

The router is only tracked once it has answered at least one ping, because some routers ignore ping and a VPN can hide the gateway; otherwise that would look like a permanent outage.

## Data

Everything lives in `~/Library/Application Support/pingbar/`: `history.db` (SQLite), `pingbar.log` and `probe.lock`. Per-ping samples are kept for 30 days (about 170k rows a day); outages are kept forever. Set `PINGBAR_DIR` to use another directory.

## Ideas for later

- Detect when a Google Meet call is running and mark outages that happened during a call.
- Notifications when the connection drops during a call.

## Contributing

See [AGENTS.md](AGENTS.md) for the architecture, commands and conventions. It's written for coding agents but works as a contributor guide too.

## License

MIT, see [LICENSE](LICENSE).
