# bbox-cli

Go CLI for the Bouygues Bbox admin API — port-forwards, WiFi, DHCP, firewall, DynDNS, plus a one-shot dashboard, Prometheus metrics exporter, self-diagnostic, live event/host monitoring, and snapshot-based drift detection.

[![Go Report Card](https://goreportcard.com/badge/github.com/hadamrd/bbox-cli)](https://goreportcard.com/report/github.com/hadamrd/bbox-cli)
[![CI](https://github.com/hadamrd/bbox-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/hadamrd/bbox-cli/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/hadamrd/bbox-cli/blob/main/LICENSE)
[![Latest release](https://img.shields.io/github/v/release/hadamrd/bbox-cli)](https://github.com/hadamrd/bbox-cli/releases/latest)

Reversed from `mabbox.bytel.fr` on 2026-07-17. Cobra-based, single static binary.

## Highlights

- **One-shot dashboard** (`bbox summary`) — link, WAN, hosts, services, WiFi and VoIP in one call.
- **Live monitoring** — `bbox log --follow` tails router events, `bbox host watch` prints connect/disconnect, `bbox watch-ip` alerts on WAN IP change.
- **Prometheus exporter** (`bbox metrics --listen …`) — scrapes the router and exposes `/metrics` + `/health`.
- **Self-diagnostic** (`bbox diag`) — 10 read-only checks against config, session, and router; JSON mode for CI.
- **Universal search** (`bbox lookup <query>`) — resolves a hostname / IP / MAC / port / ID across hosts, NAT, firewall, DHCP.
- **Snapshot + diff** (`bbox export-config --snapshot` / `--diff PATH`) — timestamped exports plus semantic drift diff.
- **Session auto-refresh + transient-error retry** — long-running commands survive short outages.

## Install

**Prebuilt binary.** Grab an archive for your OS/arch from the [Releases page](https://github.com/hadamrd/bbox-cli/releases/latest), extract, and drop `bbox` on your `PATH`.

**Go install** (requires Go 1.22+):

```bash
go install github.com/hadamrd/bbox-cli@latest
# -> binary at $(go env GOPATH)/bin/bbox
```

**From source:**

```bash
git clone https://github.com/hadamrd/bbox-cli
cd bbox-cli
go build -o bbox .
```

## Quick start

```bash
# 1. Save the router admin password once.
echo 'my-router-password' > ~/.bbox-password
chmod 600 ~/.bbox-password

# 2. Log in (creates ~/.bbox-session.json) and check status.
bbox login
bbox summary
```

## Monitoring the router

```bash
# Live router event stream (DEVICE_UP/DOWN, LOGIN_LOCAL_LOCKED, service transitions).
bbox log --follow --interval 5
```

```bash
# Device connect / disconnect. Persist to JSONL for later grep or dashboards.
bbox host watch --interval 10 --history ~/.bbox-host-events.jsonl
```

```bash
# Prometheus exporter — bind to loopback, scrape every 30s.
bbox metrics --listen 127.0.0.1:9100 --interval 30s
```

Drop the following into `prometheus.yml` to scrape it:

```yaml
scrape_configs:
  - job_name: bbox
    static_configs: [{ targets: ['127.0.0.1:9100'] }]
```

## Auth

The router password is resolved in this order:

1. `--password-file <path>`
2. `$BBOX_PASSWORD`
3. `~/.bbox-password`

The session cookie jar is cached at `~/.bbox-session.json` and auto-refreshed before expiry.

**If you hit HTTP 429**, import an existing browser session instead of retrying `login` (each failed retry extends the lockout):

```bash
# Chrome DevTools -> Application -> Cookies -> https://mabbox.bytel.fr
# Copy the BBOX_ID cookie value, then:
bbox session-import --bbox-id <paste>
bbox summary
```

### Read-only vs read-write sessions

A browser-imported `BBOX_ID` cookie authenticates GET endpoints fine, but the device-token endpoint used for writes (NAT/firewall/DHCP mutations) may return 401 — the router treats imported sessions as read-only. If writes 401, run `bbox login` for a full-privilege session; use `session-import` only when locked out.

## Configuration

Persistent settings live in `~/.bbox.yaml` (override with `--config PATH`). Precedence, highest wins:

```text
CLI flag  >  BBOX_* env var  >  config file  >  built-in default
```

| Key             | Type   | Env                  | Default | Meaning                                             |
| --------------- | ------ | -------------------- | ------- | --------------------------------------------------- |
| `verbose`       | bool   | `BBOX_VERBOSE`       | `false` | Print HTTP calls to stderr.                         |
| `show_secrets`  | bool   | `BBOX_SHOW_SECRETS`  | `false` | Reveal WiFi / DynDNS secrets in human output.       |
| `password_file` | string | `BBOX_PASSWORD_FILE` | `""`    | Password file path (see Auth).                      |
| `json`          | bool   | `BBOX_JSON`          | `false` | Emit JSON for read commands (scriptable).           |
| `retries`       | int    | `BBOX_RETRIES`       | `2`     | Retry transient network errors N times (exp back-off). |
| `timeout`       | string | `BBOX_TIMEOUT`       | `15s`   | Per-request HTTP timeout (Go duration).             |

```bash
bbox config init          # write a commented example to ~/.bbox.yaml
bbox config show          # print each key + where its value came from
bbox --config ./ci.yaml summary
```

`bbox config show` output (example):

```text
verbose       = false        (default)
show_secrets  = false        (default)
password_file = ""           (default)
json          = false        (default)
retries       = 5            (env BBOX_RETRIES)
timeout       = 30s          (file ~/.bbox.yaml)
```

A missing config file is silent; a broken file prints a warning to stderr and the CLI keeps running with defaults (so you can still run `bbox logout` or `bbox session-import`).

## Health check

```bash
$ bbox diag
[ OK ] config file           ~/.bbox.yaml parses cleanly
[ OK ] password source       ~/.bbox-password (mode 0600)
[ OK ] session cache         ~/.bbox-session.json valid, expires in 42m
[ OK ] router reachable      https://mabbox.bytel.fr responds in 38ms
[ OK ] authentication        session accepted, device-token endpoint 200
[ OK ] WAN link              connected, 300 Mb/s down / 300 Mb/s up
[ OK ] WAN IPv4              203.0.113.42
[ OK ] MAP-T port range      40960:49151 (8192 ports)
[ OK ] DHCP server           enabled, 12 active leases
[ OK ] NAT rules             7 rules, all inside MAP-T range

10 checks, 10 OK, 0 WARN, 0 FAIL
```

Exit code: `0` if all OK/WARN, `1` if any FAIL. `--json` for CI.

## MAP-T port-range gotcha

> ⚠️ With `cgnatenable=0 maptenable=1`, the Bbox only owns a WAN port *range* (e.g. `40960:49151`). Forwards on ports outside that range **silently drop**.

Both `bbox info` and `bbox diag` surface the current range:

```bash
bbox info                            # prints your range
bbox diag                            # includes it in the "MAP-T port range" check
bbox nat add ...                     # refuses out-of-range ports
bbox nat add ... --skip-port-check   # override
```

## Command reference

Every command supports `--verbose, -v`, `--json`, `--password-file <path>`, `--retries <n>`, `--timeout <duration>`, and `--config <path>`.

<details>
<summary><b>auth</b> — login, logout, session-import</summary>

| Command                              | Description                                                     |
| ------------------------------------ | --------------------------------------------------------------- |
| `bbox login`                         | Authenticate and cache the session.                             |
| `bbox logout`                        | Clear the cached session.                                       |
| `bbox session-import --bbox-id <id>` | Bootstrap from a browser `BBOX_ID` cookie (bypass rate-limit).  |

</details>

<details>
<summary><b>introspection</b> — status, info, WAN IP, stats</summary>

| Command       | Description                                    |
| ------------- | ---------------------------------------------- |
| `bbox status` | One-line health summary.                       |
| `bbox info`   | Full device info (model, uptime, port range).  |
| `bbox wan-ip` | Current WAN IPv4 (scriptable).                 |
| `bbox stats`  | Traffic counters.                              |

</details>

<details>
<summary><b>dashboard</b> — summary</summary>

| Command        | Description                                                                                     |
| -------------- | ----------------------------------------------------------------------------------------------- |
| `bbox summary` | One-shot aggregate: auth, link, WAN v4/v6, active hosts, service flags, wireless, VoIP. `--json` supported. |

</details>

<details>
<summary><b>hardware</b> — reboot, LED</summary>

| Command                        | Description                          |
| ------------------------------ | ------------------------------------ |
| `bbox reboot --confirm`        | Reboot the router.                   |
| `bbox led {off\|dim\|on\|max}` | Set front-panel LED brightness.      |

</details>

<details>
<summary><b>NAT</b> — port forwards</summary>

| Command                              | Description                | Key flags                                            |
| ------------------------------------ | -------------------------- | ---------------------------------------------------- |
| `bbox nat list`                      | List rules.                | —                                                    |
| `bbox nat add NAME WAN_PORT LAN_IP`  | Add a rule.                | `--internal-port`, `--proto`, `--skip-port-check`    |
| `bbox nat delete NAME`               | Delete a rule (id or name).| —                                                    |
| `bbox nat clear --confirm`           | Delete all rules.          | `--confirm`                                          |
| `bbox nat toggle`                    | Enable/disable NAT/PAT.    | —                                                    |

Example — forward WAN 40960 to LAN 192.168.1.42:22 (SSH):

```bash
bbox nat add ssh 40960 192.168.1.42 --internal-port 22
```

</details>

<details>
<summary><b>DMZ</b></summary>

| Command           | Description                        |
| ----------------- | ---------------------------------- |
| `bbox dmz show`   | Show the DMZ target.               |
| `bbox dmz set IP` | Route unmatched WAN traffic to IP. |
| `bbox dmz off`    | Disable DMZ.                       |

</details>

<details>
<summary><b>UPnP</b></summary>

| Command            | Description                     |
| ------------------ | ------------------------------- |
| `bbox upnp show`   | Show UPnP status + rules.       |
| `bbox upnp toggle` | Toggle UPnP on/off.             |
| `bbox upnp rules`  | List UPnP-created rules.        |

</details>

<details>
<summary><b>firewall</b></summary>

| Command                          | Description               | Key flags                             |
| -------------------------------- | ------------------------- | ------------------------------------- |
| `bbox firewall list`             | List firewall rules.      | —                                     |
| `bbox firewall add ...`          | Add a rule.               | `--proto`, `--src`, `--dst`, `--port` |
| `bbox firewall delete ID`        | Delete a rule.            | —                                     |
| `bbox firewall toggle`           | Toggle firewall on/off.   | —                                     |
| `bbox firewall toggle-rule ID`   | Enable/disable one rule.  | —                                     |

</details>

<details>
<summary><b>hosts</b></summary>

| Command                     | Description                                                       | Key flags               |
| --------------------------- | ----------------------------------------------------------------- | ----------------------- |
| `bbox host list`            | LAN hosts (leases + MAC + names).                                 | —                       |
| `bbox host me`              | Info about the current host.                                      | —                       |
| `bbox host rename MAC NAME` | Rename a host.                                                    | —                       |
| `bbox host block MAC`       | Block a host from the LAN.                                        | —                       |
| `bbox host unblock MAC`     | Unblock.                                                          | —                       |
| `bbox host watch`           | Poll and alert on connect/disconnect (long-running).              | `--interval`, `--history` |

</details>

<details>
<summary><b>WiFi</b></summary>

| Command                                            | Description                                       |
| -------------------------------------------------- | ------------------------------------------------- |
| `bbox wifi status`                                 | SSIDs, channels, band state.                      |
| `bbox wifi toggle {24\|5\|6\|guest\|all}`          | Enable/disable a band.                            |
| `bbox wifi channel BAND {N\|auto}`                 | Set channel (or auto).                            |
| `bbox wifi ssid BAND NAME`                         | Rename SSID.                                      |
| `bbox wifi key BAND KEY`                           | Change WPA key.                                   |
| `bbox wifi wps`                                    | Trigger WPS (2-min window).                       |
| `bbox wifi guest`                                  | Show guest state + SSID/passphrase.               |
| `bbox wifi guest toggle {on\|off}`                 | Enable/disable guest WiFi.                        |
| `bbox wifi guest ssid NAME`                        | Rename guest SSID.                                |
| `bbox wifi guest key KEY`                          | Change guest passphrase.                          |

</details>

<details>
<summary><b>DHCP</b></summary>

| Command                    | Description                          |
| -------------------------- | ------------------------------------ |
| `bbox dhcp show`           | DHCP server config.                  |
| `bbox dhcp leases`         | Active leases.                       |
| `bbox dhcp reserve MAC IP` | Static reservation for a MAC.        |

</details>

<details>
<summary><b>DynDNS</b></summary>

| Command                                                    | Description                                | Key flags                  |
| ---------------------------------------------------------- | ------------------------------------------ | -------------------------- |
| `bbox dyndns show`                                         | Current DynDNS config.                     | —                          |
| `bbox dyndns enable PROVIDER --hostname H --password P`    | Enable (DuckDNS: empty user, token as pw). | `--hostname`, `--password` |
| `bbox dyndns disable`                                      | Disable DynDNS.                            | —                          |

</details>

<details>
<summary><b>hibernate</b> — router power scheduler</summary>

| Command               | Description               |
| --------------------- | ------------------------- |
| `bbox hibernate show` | Show hibernate schedule.  |
| `bbox hibernate off`  | Disable hibernation.      |

</details>

<details>
<summary><b>VoIP</b></summary>

| Command          | Description             |
| ---------------- | ----------------------- |
| `bbox voip show` | Show VoIP line status.  |

</details>

<details>
<summary><b>notification</b> — router alerts + event catalog</summary>

| Command                     | Description                                                       | Key flags     |
| --------------------------- | ----------------------------------------------------------------- | ------------- |
| `bbox notification list`    | Show notification config (rules + contacts + enable) or pending.  | —             |
| `bbox notification events`  | List the router's notification event catalog.                     | `--category`  |
| `bbox notification clear`   | Delete all pending notifications.                                 | —             |

</details>

<details>
<summary><b>scheduler</b> — wireless power windows</summary>

| Command               | Description                                             |
| --------------------- | ------------------------------------------------------- |
| `bbox scheduler show` | Show scheduler state and entries.                       |
| `bbox scheduler off`  | Disable the scheduler (leaves entries in place).        |

</details>

<details>
<summary><b>watch-ip</b> — WAN IP monitor</summary>

| Command         | Description                                    | Key flags                 |
| --------------- | ---------------------------------------------- | ------------------------- |
| `bbox watch-ip` | Poll WAN IP and log changes to JSONL.          | `--interval`, `--history` |

</details>

<details>
<summary><b>log</b> — device event log</summary>

| Command          | Description                                                      | Key flags                                              |
| ---------------- | ---------------------------------------------------------------- | ------------------------------------------------------ |
| `bbox log`       | Print recent entries; filter and tail.                           | `--last`, `--search`, `--type`, `--follow`, `--interval` |
| `bbox log-clear` | Clear the router event log.                                      | —                                                      |

</details>

<details>
<summary><b>lookup</b> — universal cross-section search</summary>

| Command              | Description                                                                                              | Key flags        |
| -------------------- | -------------------------------------------------------------------------------------------------------- | ---------------- |
| `bbox lookup QUERY`  | Search hosts, NAT, firewall, DHCP reservations for a hostname / IP / MAC / port / description / ID.      | `--ignore-case`  |

</details>

<details>
<summary><b>metrics</b> — Prometheus exporter</summary>

| Command        | Description                                                                                | Key flags               |
| -------------- | ------------------------------------------------------------------------------------------ | ----------------------- |
| `bbox metrics` | Serve `/metrics` + `/health` (long-running). Caches the last successful scrape.            | `--listen`, `--interval` |

</details>

<details>
<summary><b>diag</b> — self-diagnostic</summary>

| Command     | Description                                                             | Key flags |
| ----------- | ----------------------------------------------------------------------- | --------- |
| `bbox diag` | 10 read-only checks (config / session / router). Exit 1 if any FAIL.    | `--json`  |

</details>

<details>
<summary><b>export-config</b> — full JSON dump, snapshot, diff</summary>

| Command                          | Description                                                                       | Key flags     |
| -------------------------------- | --------------------------------------------------------------------------------- | ------------- |
| `bbox export-config`             | Dump full router state (device, WAN, LAN, NAT, firewall, WiFi, VoIP, ...) as JSON.| `--file, -o`  |
| `bbox export-config --snapshot`  | Timestamped dump into `~/.bbox-snapshots/YYYYMMDD-HHMMSS.json`.                   | `--snapshot`  |
| `bbox export-config --diff PATH` | Semantic diff of live state against a saved snapshot.                             | `--diff`      |

</details>

<details>
<summary><b>retrobot</b> — proxy shortcut</summary>

| Command                                                          | Description                                       | Key flags                    |
| ---------------------------------------------------------------- | ------------------------------------------------- | ---------------------------- |
| `bbox retrobot setup NAME WAN_PORT --password P --account-id ID` | NAT rule + SOCKS5 URL for `accounts.socks5_proxy`.| `--password`, `--account-id` |
| `bbox retrobot teardown NAME`                                    | Remove the rule.                                  | —                            |

</details>

<details>
<summary><b>raw</b> — direct API calls</summary>

| Command                | Description                             | Key flags                              |
| ---------------------- | --------------------------------------- | -------------------------------------- |
| `bbox raw METHOD PATH` | Raw HTTP call against `/api/v1/...`.    | `--body`, `--include-headers`, `--skip-btoken` |

</details>

<details>
<summary><b>docs</b> — maintainer tool</summary>

| Command         | Description                                     |
| --------------- | ----------------------------------------------- |
| `bbox docs man` | Generate groff man pages, one per command node. |
| `bbox docs md`  | Generate Markdown docs, one per command node.   |

</details>

<details>
<summary><b>config</b></summary>

| Command            | Description                                                       |
| ------------------ | ----------------------------------------------------------------- |
| `bbox config init` | Write a commented example config to `~/.bbox.yaml`.               |
| `bbox config show` | Print effective config with source per key (default/env/file/flag). |

</details>

<details>
<summary><b>completion</b> / <b>version</b></summary>

| Command                                          | Description                              |
| ------------------------------------------------ | ---------------------------------------- |
| `bbox completion {bash\|zsh\|fish\|powershell}`  | Shell completion.                        |
| `bbox version` / `bbox --version`                | Print version, commit, build date, Go.   |

</details>

<details>
<summary><b>How the API was reversed</b></summary>

```text
Reads   -> GET    /api/v1/<resource>
Writes  -> PUT    /api/v1/<resource>?btoken=<device_token>
Create  -> POST   /api/v1/<resource>?btoken=<device_token>
Delete  -> DELETE /api/v1/<resource>/<id>?btoken=<device_token>
```

Device token comes from `GET /api/v1/device/token`; it is short-lived and refreshed 30 s before expiry. See `internal/client/` for the full reversed surface (28+ endpoints, 40+ commands).

</details>

## Contributing

- Run `go test ./... -race` before pushing.
- Run `golangci-lint run` (config at repo root) and keep it clean.
- PRs welcome — small, focused changes preferred.

## License

MIT. See [LICENSE](https://github.com/hadamrd/bbox-cli/blob/main/LICENSE).
