package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	hostWatchInterval int
	hostWatchHistory  string
)

var hostWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Poll and alert on host connect/disconnect events",
	Long: `Poll GET /api/v1/hosts every --interval seconds and print UP / DOWN events
by diffing the set of currently-active hosts. The initial active set is seeded
silently; only subsequent changes are printed. Use --history to persist events
as JSONL for later analysis. Ctrl-C exits with status 130.`,
	Example: `  # Watch every 10s, print human-readable UP/DOWN events
  bbox host watch --interval 10

  # Persist events to JSONL for later grep / dashboarding
  bbox host watch --history ~/.bbox-host-events.jsonl

  # Machine-readable output (one JSON object per event, per line)
  bbox host watch --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		interval := hostWatchInterval
		if interval <= 0 {
			interval = 15
		}
		fmt.Fprintf(os.Stderr, "watching hosts every %ds. Ctrl-C to stop.\n", interval)

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)

		// active maps host key (mac or id fallback) → last snapshot for that host.
		active := map[string]map[string]any{}
		first := true
		for {
			hosts, err := c().Hosts()
			if err != nil {
				stamp := time.Now().Format("2006-01-02T15:04:05")
				fmt.Fprintf(os.Stderr, "[%s] error: %v\n", stamp, err)
			} else {
				current := map[string]map[string]any{}
				for _, hAny := range hosts {
					h, _ := hAny.(map[string]any)
					if !toBoolAny(h["active"]) {
						continue
					}
					current[hostKey(h)] = h
				}
				if first {
					active = current
					first = false
				} else {
					// UP: in current, not in active.
					for k, h := range current {
						if _, ok := active[k]; !ok {
							emitHostEvent("UP", h)
						}
					}
					// DOWN: in active, not in current.
					for k, h := range active {
						if _, ok := current[k]; !ok {
							emitHostEvent("DOWN", h)
						}
					}
					active = current
				}
			}

			select {
			case <-sigCh:
				fmt.Fprintln(os.Stderr, "\ninterrupted")
				os.Exit(130)
			case <-time.After(time.Duration(interval) * time.Second):
			}
		}
	},
}

// hostKey returns a stable dedupe key for a host: prefer mac, fall back to id.
func hostKey(h map[string]any) string {
	if s := toStr(h["macaddress"]); s != "" {
		return "mac:" + s
	}
	return fmt.Sprintf("id:%v", h["id"])
}

// emitHostEvent prints one UP/DOWN line (or a JSON object) and appends to the
// history file if --history is set.
func emitHostEvent(kind string, h map[string]any) {
	stamp := time.Now().Format("2006-01-02T15:04:05")
	entry := map[string]any{
		"at":       stamp,
		"event":    kind,
		"hostname": toStr(h["hostname"]),
		"ip":       toStr(h["ipaddress"]),
		"mac":      toStr(h["macaddress"]),
		"link":     toStr(h["link"]),
	}
	if jsonOut {
		b, _ := json.Marshal(entry)
		fmt.Println(string(b))
	} else {
		hn := toStr(h["hostname"])
		if hn == "" {
			hn = "?"
		}
		fmt.Printf("%s  %-5s  %-16s %-15s %-17s  %s\n",
			stamp, kind, truncate(hn, 16),
			toStr(h["ipaddress"]), toStr(h["macaddress"]), toStr(h["link"]))
	}
	if hostWatchHistory != "" {
		_ = os.MkdirAll(filepath.Dir(hostWatchHistory), 0755)
		f, err := os.OpenFile(hostWatchHistory, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			b, _ := json.Marshal(entry)
			_, _ = f.Write(append(b, '\n'))
			_ = f.Close()
		}
	}
}

func init() {
	hostWatchCmd.Flags().IntVar(&hostWatchInterval, "interval", 15, "seconds between polls")
	hostWatchCmd.Flags().StringVar(&hostWatchHistory, "history", "", "append every event to JSONL at this path")
	hostCmd.AddCommand(hostWatchCmd)
}
