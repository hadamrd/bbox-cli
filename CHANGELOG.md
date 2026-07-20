# Changelog

## [Unreleased]
### Features
- WiFi access control + WiFi-pause scheduler + parental control (write side).
  - SDK: `WifiACLToggle/WifiACLRules/WifiACLAddRule/WifiACLDelRule`,
    `WifiSchedulerOn/WifiSchedulerRules/WifiSchedulerAddRule/WifiSchedulerDelRule`,
    `ParentalToggle/ParentalSetPolicy/ParentalScheduler/ParentalRules/ParentalAddRule/ParentalDelRule/ParentalHostSet`,
    plus the shared `SchedulerRuleArgs`.
  - CLI: `bbox wifi acl {show,toggle,add,del}`, `bbox scheduler {on,add,del}`,
    `bbox parental {show,toggle,policy,add,del,device}`.
  - All endpoints reverse-engineered from the admin-UI bundle and round-trip
    verified against firmware 25.3.20 (PRV36AX349B).

## [v0.9.0] - 2026-07-18
### Features
- bbox lookup + bbox metrics (Prometheus) + integration test suite (87b04ac)

## [v0.8.0] - 2026-07-18
### Features
- session auto-refresh + richer help + auto-changelog (c6dbb43)

## [v0.7.0] - 2026-07-18
### Features
- notification config + events catalog + scheduler + completions + docs + snapshot (a0d63f9)

## [v0.6.0] - 2026-07-18
### Features
- host watch, summary, retries+timeout, export-config --diff, lint CI (9772229)

## [v0.5.0] - 2026-07-18

### Chore
- goreleaser + rewritten README (a76d441)

## [v0.4.0] - 2026-07-18
### Features
- actionable errors + config file + completion install docs (0c98555)

## [v0.3.0] - 2026-07-18
### Features
- --search, --type, --follow, decoded params (1fc684f)

## [v0.2.0] - 2026-07-18
### Features
- harden CLI — secrets redaction, guest WiFi mgmt, tests, CI (2f7a16b)

### Bug fixes
- decode session-file expires as float64 (e89aba1)

### Chore
- drop accidentally committed xargs stackdump (19bc7a6)
- fix module path to github.com/hadamrd/bbox-cli (1732b63)
