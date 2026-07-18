package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var hostStaleDays int

var hostStaleCmd = &cobra.Command{
	Use:   "stale",
	Short: "List LAN hosts with no DEVICE_UP in the last --days N days",
	Long: `Walk the current host list and cross-reference with recent DEVICE_UP
events from the device log. A host is stale when its most recent DEVICE_UP
is older than --days N days (or absent from the log window entirely).`,
	Example: `  # Hosts absent for more than 30 days (default)
  bbox host stale

  # Tighter window
  bbox host stale --days 7 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		hosts, err := c().Hosts()
		if err != nil {
			return err
		}
		logs, err := c().Logs()
		if err != nil {
			return err
		}
		stale, warn := computeStaleHosts(hosts, logs, hostStaleDays, time.Now())
		if emit(stale) {
			return nil
		}
		fmt.Printf("Stale hosts (last DEVICE_UP > %d days ago):\n", hostStaleDays)
		if warn != "" {
			fmt.Printf("  warning: %s\n", warn)
		}
		if len(stale) == 0 {
			fmt.Println("  (none)")
			return nil
		}
		for _, h := range stale {
			last := "never in log window"
			if h.LastSeen != nil {
				last = fmt.Sprintf("%s (%d days ago)", *h.LastSeen, *h.DaysAgo)
			}
			fmt.Printf("  id=%-3d %-22s %-15s %-19s last seen: %s\n",
				h.ID, truncate(h.Hostname, 22), h.IP, h.MAC, last)
		}
		return nil
	},
}

// staleHost is the per-host result row (also the JSON shape).
type staleHost struct {
	ID       int     `json:"id"`
	Hostname string  `json:"hostname"`
	IP       string  `json:"ip"`
	MAC      string  `json:"mac"`
	LastSeen *string `json:"last_seen"` // "YYYY-MM-DD" or nil
	DaysAgo  *int    `json:"days_ago"`  // nil if never seen
}

// computeStaleHosts returns hosts that haven't emitted DEVICE_UP within `days`.
// `now` is injected for deterministic tests. The second return is a human
// warning when the log window may be too short to answer the question.
func computeStaleHosts(hosts, logs []any, days int, now time.Time) ([]staleHost, string) {
	// Build MAC -> most-recent DEVICE_UP time from the log.
	lastUp := map[string]time.Time{}
	oldestLog := time.Time{}
	for _, e := range logs {
		m, _ := e.(map[string]any)
		event := strings.ToUpper(toStr(m["log"]))
		dateStr := toStr(m["date"])
		t, ok := parseBboxLogDate(dateStr)
		if ok {
			if oldestLog.IsZero() || t.Before(oldestLog) {
				oldestLog = t
			}
		}
		if event != "DEVICE_UP" {
			continue
		}
		// DEVICE_UP param: "mac;ip;host". MAC is the first segment.
		parts := strings.SplitN(toStr(m["param"]), ";", 3)
		if len(parts) == 0 {
			continue
		}
		mac := strings.ToLower(parts[0])
		if mac == "" || !ok {
			continue
		}
		if prev, exists := lastUp[mac]; !exists || t.After(prev) {
			lastUp[mac] = t
		}
	}

	cutoff := now.AddDate(0, 0, -days)
	var out []staleHost
	for _, hAny := range hosts {
		h, _ := hAny.(map[string]any)
		mac := strings.ToLower(toStr(h["macaddress"]))
		if mac == "" {
			continue
		}
		if t, ok := lastUp[mac]; ok {
			if t.After(cutoff) {
				continue // recent enough
			}
			ds := t.Format("2006-01-02")
			ago := int(now.Sub(t).Hours() / 24)
			out = append(out, staleHost{
				ID: toIntAny(h["id"]), Hostname: toStr(h["hostname"]),
				IP: toStr(h["ipaddress"]), MAC: toStr(h["macaddress"]),
				LastSeen: &ds, DaysAgo: &ago,
			})
			continue
		}
		out = append(out, staleHost{
			ID: toIntAny(h["id"]), Hostname: toStr(h["hostname"]),
			IP: toStr(h["ipaddress"]), MAC: toStr(h["macaddress"]),
		})
	}

	var warn string
	if !oldestLog.IsZero() && oldestLog.After(cutoff) {
		warn = fmt.Sprintf("log has only %d entries starting at %s; --days %d may miss earlier events",
			len(logs), oldestLog.Format("2006-01-02"), days)
	}
	return out, warn
}

// parseBboxLogDate accepts the handful of timestamp shapes the Bbox emits.
// Returns UTC time and ok=false when parsing fails.
func parseBboxLogDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func init() {
	hostStaleCmd.Flags().IntVar(&hostStaleDays, "days", 30, "staleness threshold (days without a DEVICE_UP)")
	hostCmd.AddCommand(hostStaleCmd)
}
