# bbox-cli

Go CLI for the Bouygues Bbox admin API — port-forwards, WiFi, DHCP, firewall, DynDNS, and 30+ more, without opening the web UI.

[![Go Report Card](https://goreportcard.com/badge/github.com/hadamrd/bbox-cli)](https://goreportcard.com/report/github.com/hadamrd/bbox-cli)
[![CI](https://github.com/hadamrd/bbox-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/hadamrd/bbox-cli/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/hadamrd/bbox-cli/blob/main/LICENSE)
[![Latest release](https://img.shields.io/github/v/release/hadamrd/bbox-cli)](https://github.com/hadamrd/bbox-cli/releases/latest)

Reversed from `mabbox.bytel.fr` on 2026-07-17. Cobra-based, single static binary.

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
bbox status
```

## Auth

The router password is resolved in this order:

1. `--password-file <path>`
2. `$BBOX_PASSWORD`
3. `~/.bbox-password`

The session cookie jar is cached at `~/.bbox-session.json` and auto-refreshed.

**If you hit HTTP 429**, import an existing browser session instead of retrying `login` (each failed retry extends the lockout):

```bash
# Chrome DevTools -> Application -> Cookies -> https://mabbox.bytel.fr
# Copy the BBOX_ID cookie value, then:
bbox session-import --bbox-id <paste>
bbox status
```

## Configuration

Persistent settings live in `~/.bbox.yaml` (override with `--config PATH`). Precedence, highest wins:

```text
CLI flag  >  BBOX_* env var  >  config file  >  built-in default
```

| Key             | Type   | Env                  | Default | Meaning                                        |
| --------------- | ------ | -------------------- | ------- | ---------------------------------------------- |
| `verbose`       | bool   | `BBOX_VERBOSE`       | `false` | Print HTTP calls to stderr.                    |
| `show_secrets`  | bool   | `BBOX_SHOW_SECRETS`  | `false` | Reveal WiFi / DynDNS secrets in human output.  |
| `password_file` | string | `BBOX_PASSWORD_FILE` | `""`    | Password file path (see Auth).                 |
| `json`          | bool   | `BBOX_JSON`          | `false` | Emit JSON for read commands (scriptable).      |

```bash
bbox config init          # write a commented example to ~/.bbox.yaml
bbox config show          # print each key + where its value came from
bbox --config ./ci.yaml status
```

A missing config file is silent; a broken file prints a warning to stderr and the CLI keeps running with defaults (so you can still run `bbox logout` or `bbox session-import`).

## MAP-T port-range gotcha

> ⚠️ With `cgnatenable=0 maptenable=1`, the Bbox only owns a WAN port *range* (e.g. `40960:49151`). Forwards on ports outside that range **silently drop**.

```bash
bbox info                            # prints your range
bbox nat add ...                     # refuses out-of-range ports
bbox nat add ... --skip-port-check   # override
```

## Command reference

Every command supports `--verbose, -v`, `--json`, and `--password-file <path>`.

<details>
<summary><b>auth</b> — login, logout, session-import</summary>

| Command                              | Description                                        | Key flags       |
| ------------------------------------ | -------------------------------------------------- | --------------- |
| `bbox login`                         | Authenticate and cache the session.                | —               |
| `bbox logout`                        | Clear the cached session.                          | —               |
| `bbox session-import --bbox-id <id>` | Bootstrap from a browser `BBOX_ID` cookie (bypass rate-limit). | `--bbox-id` |

</details>

<details>
<summary><b>introspection</b> — status, info, WAN IP, logs, stats</summary>

| Command              | Description                                    | Key flags |
| -------------------- | ---------------------------------------------- | --------- |
| `bbox status`        | One-line health summary.                       | —         |
| `bbox info`          | Full device info (model, uptime, port range).  | —         |
| `bbox wan-ip`        | Current WAN IPv4.                              | —         |
| `bbox log`           | Print the router event log.                    | —         |
| `bbox log-clear`     | Clear the router event log.                    | —         |
| `bbox stats`         | Traffic counters.                              | —         |
| `bbox export-config` | Dump full router state as JSON.                | `--out`   |

</details>

<details>
<summary><b>hardware</b> — reboot, LED</summary>

| Command                         | Description                          | Key flags   |
| ------------------------------- | ------------------------------------ | ----------- |
| `bbox reboot --confirm`         | Reboot the router.                   | `--confirm` |
| `bbox led {off\|dim\|on\|max}`  | Set front-panel LED brightness.      | —           |

</details>

<details>
<summary><b>NAT</b> — port forwards</summary>

| Command                                                    | Description                        | Key flags                                      |
| ---------------------------------------------------------- | ---------------------------------- | ---------------------------------------------- |
| `bbox nat list`                                            | List rules.                        | —                                              |
| `bbox nat add NAME WAN_PORT LAN_IP`                        | Add a rule.                        | `--internal-port`, `--proto`, `--skip-port-check` |
| `bbox nat delete NAME`                                     | Delete a rule.                     | —                                              |
| `bbox nat clear`                                           | Delete all rules.                  | `--confirm`                                    |
| `bbox nat toggle NAME`                                     | Enable/disable a rule.             | —                                              |

Example: forward WAN 40960 to LAN 192.168.1.42:22 (SSH):

```bash
bbox nat add ssh 40960 192.168.1.42 --internal-port 22
```

</details>

<details>
<summary><b>DMZ</b></summary>

| Command             | Description                     | Key flags |
| ------------------- | ------------------------------- | --------- |
| `bbox dmz show`     | Show the DMZ target.            | —         |
| `bbox dmz set IP`   | Route unmatched WAN traffic to IP. | —      |
| `bbox dmz off`      | Disable DMZ.                    | —         |

</details>

<details>
<summary><b>UPnP</b></summary>

| Command             | Description                     | Key flags |
| ------------------- | ------------------------------- | --------- |
| `bbox upnp show`    | Show UPnP state.                | —         |
| `bbox upnp toggle`  | Toggle UPnP on/off.             | —         |
| `bbox upnp rules`   | List UPnP-created rules.        | —         |

</details>

<details>
<summary><b>firewall</b></summary>

| Command                          | Description               | Key flags |
| -------------------------------- | ------------------------- | --------- |
| `bbox firewall list`             | List firewall rules.      | —         |
| `bbox firewall add ...`          | Add a rule.               | `--proto`, `--src`, `--dst`, `--port` |
| `bbox firewall delete ID`        | Delete a rule.            | —         |
| `bbox firewall toggle`           | Toggle firewall on/off.   | —         |
| `bbox firewall toggle-rule ID`   | Enable/disable one rule.  | —         |

</details>

<details>
<summary><b>hosts</b></summary>

| Command                     | Description                          | Key flags |
| --------------------------- | ------------------------------------ | --------- |
| `bbox host list`            | LAN hosts (leases + MAC + names).    | —         |
| `bbox host me`              | Info about the current host.         | —         |
| `bbox host rename MAC NAME` | Rename a host.                       | —         |
| `bbox host block MAC`       | Block a host from the LAN.           | —         |
| `bbox host unblock MAC`     | Unblock.                             | —         |

</details>

<details>
<summary><b>WiFi</b></summary>

| Command                          | Description                                    | Key flags |
| -------------------------------- | ---------------------------------------------- | --------- |
| `bbox wifi status`               | SSIDs, channels, band state.                   | —         |
| `bbox wifi guest {on\|off}`      | Toggle guest network.                          | —         |
| `bbox wifi toggle BAND`          | Toggle a band (`2`, `5`, `6`, `guest`).        | —         |
| `bbox wifi channel BAND N`       | Set channel.                                   | —         |
| `bbox wifi ssid BAND NAME`       | Rename SSID.                                   | —         |
| `bbox wifi key BAND KEY`         | Change WPA key.                                | —         |
| `bbox wifi wps`                  | Trigger WPS.                                   | —         |

</details>

<details>
<summary><b>DHCP</b></summary>

| Command                       | Description                          | Key flags |
| ----------------------------- | ------------------------------------ | --------- |
| `bbox dhcp show`              | DHCP server config.                  | —         |
| `bbox dhcp leases`            | Active leases.                       | —         |
| `bbox dhcp reserve MAC IP`    | Static reservation for a MAC.        | —         |

</details>

<details>
<summary><b>DynDNS</b></summary>

| Command                                                                | Description                | Key flags |
| ---------------------------------------------------------------------- | -------------------------- | --------- |
| `bbox dyndns show`                                                     | Current DynDNS config.     | —         |
| `bbox dyndns enable PROVIDER --hostname H --password P`                | Enable (DuckDNS/no-ip/OVH).| `--hostname`, `--password` |
| `bbox dyndns disable`                                                  | Disable DynDNS.            | —         |

</details>

<details>
<summary><b>hibernate</b></summary>

| Command                | Description               | Key flags |
| ---------------------- | ------------------------- | --------- |
| `bbox hibernate show`  | Show hibernate schedule.  | —         |
| `bbox hibernate off`   | Disable hibernation.      | —         |

</details>

<details>
<summary><b>VoIP</b></summary>

| Command          | Description             | Key flags |
| ---------------- | ----------------------- | --------- |
| `bbox voip show` | Show VoIP state.        | —         |

</details>

<details>
<summary><b>watch-ip</b></summary>

| Command          | Description                                   | Key flags                |
| ---------------- | --------------------------------------------- | ------------------------ |
| `bbox watch-ip`  | Poll WAN IP and log changes to JSONL.         | `--interval`, `--history`|

</details>

<details>
<summary><b>retrobot</b> — proxy shortcut</summary>

| Command                                                    | Description                          | Key flags |
| ---------------------------------------------------------- | ------------------------------------ | --------- |
| `bbox retrobot setup NAME WAN_PORT --password P --account-id ID` | NAT rule + SOCKS5 URL for `accounts.socks5_proxy`. | `--password`, `--account-id` |
| `bbox retrobot teardown NAME`                              | Remove the rule.                     | —         |

</details>

<details>
<summary><b>raw</b> — direct API calls</summary>

| Command                              | Description                             | Key flags |
| ------------------------------------ | --------------------------------------- | --------- |
| `bbox raw METHOD PATH`               | Raw HTTP call against `/api/v1/...`.    | `--body`  |
| `bbox completion {bash\|zsh\|fish\|powershell}` | Shell completion.            | —         |

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
