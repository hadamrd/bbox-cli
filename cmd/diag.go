package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hadamrd/bbox-cli/internal/client"
	"github.com/spf13/cobra"
)

// diagStatus is the four-value enum used in diag reports.
type diagStatus string

const (
	diagOK   diagStatus = "OK"
	diagWarn diagStatus = "WARN"
	diagFail diagStatus = "FAIL"
	diagSkip diagStatus = "SKIP"
)

// diagCheck is one line of the report.
type diagCheck struct {
	Check  string     `json:"check"`
	Status diagStatus `json:"status"`
	Detail string     `json:"detail"`
}

var diagJSON bool

var diagCmd = &cobra.Command{
	Use:   "diag",
	Short: "Self-diagnostic: run a battery of read-only checks + print report",
	Long: `Runs 10 read-only checks against the local config, the cached session, and (if
reachable) the router. Exits 0 if everything is OK or WARN, 1 if any check FAILs.

Use --json for machine-readable output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		results := runDiag()
		if diagJSON {
			b, _ := json.MarshalIndent(results, "", "  ")
			fmt.Println(string(b))
		} else {
			for _, r := range results {
				fmt.Printf("[%s] %-24s  %s\n", padStatus(r.Status), r.Check, r.Detail)
			}
		}
		for _, r := range results {
			if r.Status == diagFail {
				return fmt.Errorf("one or more checks failed")
			}
		}
		return nil
	},
}

func init() {
	diagCmd.Flags().BoolVar(&diagJSON, "json", false, "emit machine-readable JSON")
	rootCmd.AddCommand(diagCmd)
}

// padStatus centres the 4-char status glyph so the columns align.
func padStatus(s diagStatus) string {
	switch s {
	case diagOK:
		return " OK "
	case diagWarn:
		return "WARN"
	case diagFail:
		return "FAIL"
	case diagSkip:
		return "SKIP"
	}
	return string(s)
}

// runDiag runs all checks in order. Never modifies state.
func runDiag() []diagCheck {
	out := make([]diagCheck, 0, 10)

	// 1. config file present
	cfgPath := cfgFile
	if cfgPath == "" {
		cfgPath = defaultConfigPath()
	}
	if st, err := os.Stat(cfgPath); err == nil && !st.IsDir() {
		out = append(out, diagCheck{"config file present", diagOK, cfgPath})
	} else {
		out = append(out, diagCheck{"config file present", diagWarn, fmt.Sprintf("%s not found (using defaults)", cfgPath)})
	}

	// 2. session cached
	sessPath := client.SessionFile()
	sessionOK := diagSessionCached(sessPath)
	if sessionOK.Status != diagOK {
		out = append(out, sessionOK)
		// Cascade: skip the remaining router-dependent checks.
		for _, name := range []string{
			"router reachable", "TLS handshake", "session valid",
			"device token fetchable", "notification service",
			"WAN state", "MAP-T range", "WiFi bands",
		} {
			out = append(out, diagCheck{name, diagSkip, "no valid session"})
		}
		return out
	}
	out = append(out, sessionOK)

	// 3. router reachable (DNS + TCP)
	host, port := hostPort(client.BaseURL)
	reachable := diagCheck{Check: "router reachable"}
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(context.Background(), "tcp", net.JoinHostPort(host, port))
	if err != nil {
		reachable.Status = diagFail
		reachable.Detail = fmt.Sprintf("%s:%s — %v", host, port, err)
		out = append(out, reachable)
		for _, name := range []string{
			"TLS handshake", "session valid", "device token fetchable",
			"notification service", "WAN state", "MAP-T range", "WiFi bands",
		} {
			out = append(out, diagCheck{name, diagSkip, "router unreachable"})
		}
		return out
	}
	_ = conn.Close()
	reachable.Status = diagOK
	reachable.Detail = fmt.Sprintf("%s:%s", host, port)
	out = append(out, reachable)

	// 4. TLS handshake (ignore self-signed).
	tlsCheck := diagCheck{Check: "TLS handshake"}
	tlsConn, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp",
		net.JoinHostPort(host, port), &tls.Config{InsecureSkipVerify: true, ServerName: host})
	if err != nil {
		tlsCheck.Status = diagFail
		tlsCheck.Detail = err.Error()
	} else {
		cs := tlsConn.ConnectionState()
		tlsCheck.Status = diagOK
		tlsCheck.Detail = fmt.Sprintf("%s (self-signed OK)", tls.VersionName(cs.Version))
		_ = tlsConn.Close()
	}
	out = append(out, tlsCheck)

	// 5. session valid — GET /api/v1/device
	sessValid := diagCheck{Check: "session valid"}
	if _, err := c().Device(); err != nil {
		sessValid.Status = diagFail
		sessValid.Detail = shortErr(err)
		out = append(out, sessValid)
		for _, name := range []string{
			"device token fetchable", "notification service",
			"WAN state", "MAP-T range", "WiFi bands",
		} {
			out = append(out, diagCheck{name, diagSkip, "session invalid"})
		}
		return out
	}
	sessValid.Status = diagOK
	sessValid.Detail = "GET /api/v1/device → 200"
	out = append(out, sessValid)

	// 6. device token fetchable
	tokCheck := diagCheck{Check: "device token fetchable"}
	if tok, err := c().DeviceToken(); err != nil {
		tokCheck.Status = diagFail
		detail := shortErr(err)
		// "session valid" already passed above (we'd have returned), so if the
		// token endpoint 401s the culprit is almost always a session-import
		// that omitted the btoken cookie.
		if strings.Contains(detail, "401") {
			detail += " (session was imported without btoken cookie — writes will fail; see 'bbox session-import --help')"
		}
		tokCheck.Detail = detail
	} else if tok == "" {
		tokCheck.Status = diagFail
		tokCheck.Detail = "empty token"
	} else {
		tokCheck.Status = diagOK
		tokCheck.Detail = fmt.Sprintf("token=%s… (len=%d)", truncate(tok, 8), len(tok))
	}
	out = append(out, tokCheck)

	// 7. notification service (informational)
	notifCheck := diagCheck{Check: "notification service"}
	if raw, err := c().Notifications(); err != nil {
		notifCheck.Status = diagWarn
		notifCheck.Detail = shortErr(err)
	} else {
		cfg, ok := notificationConfigShape(raw)
		if !ok {
			notifCheck.Status = diagWarn
			notifCheck.Detail = "unrecognized shape"
		} else {
			enable := toStr(cfg["enable"])
			alerts, _ := cfg["alerts"].([]any)
			notifCheck.Status = diagOK
			notifCheck.Detail = fmt.Sprintf("enable=%s alerts=%d", fmtBool(enable), len(alerts))
		}
	}
	out = append(out, notifCheck)

	// 8. WAN state
	wanCheck := diagCheck{Check: "WAN state"}
	wan, err := c().WAN()
	if err != nil {
		wanCheck.Status = diagFail
		wanCheck.Detail = shortErr(err)
	} else {
		ip := getMap(wan, "ip")
		state := toStr(ip["state"])
		if strings.EqualFold(state, "Up") {
			wanCheck.Status = diagOK
			wanCheck.Detail = fmt.Sprintf("state=Up ip=%v", dash(ip["address"]))
		} else {
			wanCheck.Status = diagFail
			wanCheck.Detail = fmt.Sprintf("state=%q", state)
		}
	}
	out = append(out, wanCheck)

	// 9. MAP-T range
	rangeCheck := diagCheck{Check: "MAP-T range"}
	if err != nil {
		rangeCheck.Status = diagSkip
		rangeCheck.Detail = "WAN read failed"
	} else {
		ip := getMap(wan, "ip")
		r := toStr(ip["portrange"])
		if r == "" {
			rangeCheck.Status = diagWarn
			rangeCheck.Detail = "no MAP-T range published (are you on native IPv4?)"
		} else {
			rangeCheck.Status = diagOK
			rangeCheck.Detail = r
		}
	}
	out = append(out, rangeCheck)

	// 10. WiFi bands
	wifiCheck := diagCheck{Check: "WiFi bands"}
	if wifi, err := c().Wifi(); err != nil {
		wifiCheck.Status = diagWarn
		wifiCheck.Detail = shortErr(err)
	} else {
		enabled := []string{}
		for _, band := range []string{"radio24", "radio5", "radio6"} {
			if b, ok := wifi[band].(map[string]any); ok {
				if toStr(b["enable"]) == "1" || fmtBool(b["enable"]) == "on" {
					enabled = append(enabled, bandShort(band))
				}
			}
		}
		if guest, err := c().GuestEnable(); err == nil {
			if fmtBool(guest["enable"]) == "on" {
				enabled = append(enabled, "guest")
			}
		}
		if len(enabled) == 0 {
			wifiCheck.Status = diagWarn
			wifiCheck.Detail = "no bands enabled"
		} else {
			wifiCheck.Status = diagOK
			wifiCheck.Detail = strings.Join(enabled, ", ")
		}
	}
	out = append(out, wifiCheck)

	return out
}

// diagSessionCached checks the on-disk session file without touching the
// network. Matches LoadSession's shape.
func diagSessionCached(path string) diagCheck {
	c := diagCheck{Check: "session cached"}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			c.Status = diagFail
			c.Detail = fmt.Sprintf("%s missing (run `bbox login`)", path)
		} else {
			c.Status = diagFail
			c.Detail = err.Error()
		}
		return c
	}
	var raw map[string]struct {
		Value   string   `json:"value"`
		Expires *float64 `json:"expires"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		c.Status = diagFail
		c.Detail = fmt.Sprintf("parse: %v", err)
		return c
	}
	if len(raw) == 0 {
		c.Status = diagFail
		c.Detail = "empty session file"
		return c
	}
	now := float64(time.Now().Unix())
	for name, meta := range raw {
		if meta.Expires != nil && *meta.Expires < now {
			c.Status = diagFail
			c.Detail = fmt.Sprintf("cookie %q expired", name)
			return c
		}
	}
	c.Status = diagOK
	c.Detail = fmt.Sprintf("%d cookie(s) in %s", len(raw), path)
	return c
}

// hostPort splits a URL to (host, port). Defaults to 443 for https, 80 for http.
func hostPort(rawURL string) (string, string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "mabbox.bytel.fr", "443"
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if u.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	return host, port
}

func bandShort(band string) string {
	switch band {
	case "radio24":
		return "2.4"
	case "radio5":
		return "5"
	case "radio6":
		return "6"
	}
	return band
}

// shortErr trims very long transport errors so the report stays one-line-per-check.
func shortErr(err error) string {
	s := err.Error()
	if len(s) > 100 {
		s = s[:100] + "…"
	}
	return s
}
