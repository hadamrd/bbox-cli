# bbox-cli

A Go CLI for the Bouygues **Bbox** admin API (reversed from `mabbox.bytel.fr` on 2026-07-17).
Cobra-based port of a Python original — same commands, same flags, same output text.

Manage NAT/PAT port-forwards, DMZ, UPnP, firewall, WiFi (SSID / key / channel / 2.4-5-6-guest),
DHCP reservations, DynDNS (DuckDNS / no-ip / OVH), router reboot / LED, WAN-IP tracking, and
full state export — all from the shell.

## Install

```bash
go install github.com/Hexalgo/bbox-cli@latest
# -> binary lands at $(go env GOPATH)/bin/bbox
```

## Authentication

The router password is sourced in this order:

1. `--password-file <path>`
2. `$BBOX_PASSWORD`
3. `~/.bbox-password`

The session (cookie jar) is cached at `~/.bbox-session.json` and auto-refreshed.

**Rate-limit warning.** Every failed `login` attempt while locked out *extends* the lockout.
If you hit HTTP 429, do NOT retry. Instead, bootstrap from Chrome DevTools:

```bash
# 1. F12 → Application → Cookies → https://mabbox.bytel.fr → copy BBOX_ID value.
bbox session-import --bbox-id <paste>
bbox status   # verify
```

## MAP-T port-range gotcha

With `cgnatenable=0 maptenable=1`, the Bbox only owns a WAN port *range*
(e.g. `40960:49151`). Forwards on ports outside that range **silently drop**.

```bash
bbox info                   # prints your range
bbox nat add ...            # refuses out-of-range ports
bbox nat add ... --skip-port-check   # override
```

## Examples

```bash
bbox status
bbox info
bbox wan-ip

# NAT: forward WAN 40960 -> LAN 192.168.1.42:22 (SSH)
bbox nat add ssh 40960 192.168.1.42 --internal-port 22
bbox nat list
bbox nat delete ssh

# WiFi
bbox wifi status
bbox wifi key 5 'NewSecret123!'
bbox wifi toggle guest on

# DynDNS with DuckDNS
bbox dyndns enable duckdns --hostname mypi.duckdns.org --password <token>

# Watch WAN IP for changes
bbox watch-ip --interval 30 --history ~/.bbox-wan-history.jsonl

# retrobot proxy shortcut (NAT rule + SOCKS5 URL for accounts.socks5_proxy)
bbox retrobot setup retrobot-socks5 40961 --password s3cret --account-id 42

# Raw debugging call
bbox raw GET /api/v1/summary
bbox raw PUT /api/v1/dyndns --body 'enable=1'
```

## Command reference

| Group       | Commands                                                                       |
|-------------|--------------------------------------------------------------------------------|
| auth        | `login`, `logout`, `session-import --bbox-id …`                                |
| introspect  | `status`, `info`, `wan-ip`, `log`, `log-clear`, `stats`, `export-config`       |
| hardware    | `reboot --confirm`, `led {off\|dim\|on\|max}`                                  |
| nat         | `nat {list\|add\|delete\|clear\|toggle}`                                       |
| dmz         | `dmz {show\|set IP\|off}`                                                      |
| upnp        | `upnp {show\|toggle\|rules}`                                                   |
| firewall    | `firewall {list\|add\|delete\|toggle\|toggle-rule}`                            |
| host        | `host {list\|me\|rename\|block\|unblock}`                                      |
| wifi        | `wifi {status\|guest\|toggle\|channel\|ssid\|key\|wps}`                        |
| dhcp        | `dhcp {show\|leases\|reserve MAC IP}`                                          |
| dyndns      | `dyndns {show\|enable PROVIDER --hostname H --password P\|disable}`            |
| hibernate   | `hibernate {show\|off}`                                                        |
| voip        | `voip show`                                                                    |
| tools       | `watch-ip`, `retrobot {setup\|teardown}`, `raw METHOD PATH`, `completion SHELL`|

### Persistent flags

- `--verbose, -v` — log HTTP calls to stderr.
- `--json` — emit JSON for read commands (scriptable).
- `--password-file <path>` — override password source.

## Endpoint pattern (reversed)

```
Reads   -> GET    /api/v1/<resource>
Writes  -> PUT    /api/v1/<resource>?btoken=<device_token>
Create  -> POST   /api/v1/<resource>?btoken=<device_token>
Delete  -> DELETE /api/v1/<resource>/<id>?btoken=<device_token>
```

Device token from `GET /api/v1/device/token`; short-lived, refreshed 30 s before expiry.
See `internal/client/` for the full reversed surface (28+ endpoints, 40+ commands).

## License

MIT. See [LICENSE](LICENSE).
