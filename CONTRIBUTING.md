# Contributing

Small, focused PRs preferred. Assume Go 1.22+ and a POSIX shell (or Git-Bash on Windows).

## Local dev

```bash
make build     # build ./bbox with version ldflags
make test      # go test ./... -race -count=1
make lint      # golangci-lint run (config: .golangci.yml)
make vet
make fmt
make docs      # regenerate docs/man/* and docs/md/* from cobra
make release-dry-run   # goreleaser --snapshot --skip=publish,sign
```

Run `bbox diag` after any auth-touching change — it's a read-only 10-check self-test.

## Commit style

Conventional commits: `feat: …`, `fix: …`, `perf: …`, `docs: …`, `chore: …`.
Scope in parens is optional (`feat(nat): …`). The changelog script + goreleaser
group commits by these prefixes.

## Adding a new command

1. Create `cmd/foo.go`. Copy the shape from any existing small command (e.g. `cmd/version.go` or `cmd/dmz.go`).
2. Wire it in `init()`:
   ```go
   func init() { rootCmd.AddCommand(fooCmd) }
   ```
3. If it hits the router, call `ensureAuth()` at the top of `RunE`, then use `c().Foo()` from `pkg/client/endpoints.go`.
4. Read commands honour `--json` via `if emit(v) { return nil }`.
5. Write commands need a device token — that's already automatic in `Client.Write`.
6. Add a test in `cmd/foo_test.go`. Use the `mockBboxServer` pattern from `cmd/integration_test.go` for round-trip coverage.

## Adding a new endpoint

- Add a wrapper in `pkg/client/endpoints.go` next to the closest neighbour (WiFi with WiFi, NAT with NAT).
- Reads go through `GetFirst` + `unwrap`. Writes go through `Write` — btoken is auto-appended.
- Error messages start with the command name (`fmt.Errorf("dhcp_reserve: HTTP %d — %s", …)`).

## Testing against a real Bbox

Hitting `mabbox.bytel.fr` rate-limits **hard** on repeated login failures — each 401 EXTENDS the lockout. Don't loop `bbox login` in a script. Instead:

1. Open Chrome DevTools → Application → Cookies → https://mabbox.bytel.fr
2. Copy the `BBOX_ID` value.
3. `bbox session-import --bbox-id <paste>` — bypasses login entirely.

The session cache lives at `~/.bbox-session.json` and is honoured by every command.

## Code of conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Report issues via GitHub Discussions.
