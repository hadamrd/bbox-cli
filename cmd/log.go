package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	logLastN    int
	logSearch   string
	logTypes    []string
	logFollow   bool
	logInterval int
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show recent device logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if logFollow {
			return followLogs()
		}
		logs, err := c().Logs()
		if err != nil {
			return err
		}
		logs = filterLogs(logs, logSearch, logTypes)
		if logLastN > 0 && logLastN < len(logs) {
			logs = logs[:logLastN]
		}
		if emit(logs) {
			return nil
		}
		for _, e := range logs {
			printLogEntry(e)
		}
		return nil
	},
}

// followLogs polls at logInterval and prints only entries newer than the
// newest date seen. Ctrl-C exits with code 130 (SIGINT convention).
func followLogs() error {
	interval := logInterval
	if interval <= 0 {
		interval = 5
	}
	fmt.Printf("watching log every %ds. Ctrl-C to stop.\n", interval)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	var lastSeen string // the "date" of the newest already-printed entry
	first := true
	for {
		logs, err := c().Logs()
		if err != nil {
			stamp := time.Now().Format("2006-01-02T15:04:05")
			fmt.Fprintf(os.Stderr, "[%s] error: %v\n", stamp, err)
		} else {
			logs = filterLogs(logs, logSearch, logTypes)
			// API returns newest first; walk in reverse so oldest-new prints first.
			var fresh []any
			for _, e := range logs {
				m, _ := e.(map[string]any)
				d := toStr(m["date"])
				if first {
					// On first poll, don't backfill — just record the newest date.
					break
				}
				if d == lastSeen {
					break
				}
				fresh = append(fresh, e)
			}
			// reverse fresh
			for i, j := 0, len(fresh)-1; i < j; i, j = i+1, j-1 {
				fresh[i], fresh[j] = fresh[j], fresh[i]
			}
			for _, e := range fresh {
				if jsonOut {
					b, _ := json.Marshal(e)
					fmt.Println(string(b))
				} else {
					printLogEntry(e)
				}
			}
			if len(logs) > 0 {
				m, _ := logs[0].(map[string]any)
				lastSeen = toStr(m["date"])
			}
			first = false
		}

		select {
		case <-sigCh:
			fmt.Fprintln(os.Stderr, "\ninterrupted")
			os.Exit(130)
		case <-time.After(time.Duration(interval) * time.Second):
		}
	}
}

func printLogEntry(e any) {
	m, _ := e.(map[string]any)
	date := toStr(m["date"])
	if date == "" {
		date = "?"
	}
	event := toStr(m["log"])
	if event == "" {
		event = "?"
	}
	fmt.Printf("%-26s  %-25s  %s\n", date, event, formatLogParam(event, toStr(m["param"])))
}

// formatLogParam decodes the `param` string for known event families.
// Unknown events fall back to the raw string.
func formatLogParam(event, param string) string {
	switch strings.ToUpper(event) {
	case "DEVICE_UP", "DEVICE_DOWN":
		parts := strings.SplitN(param, ";", 3)
		if len(parts) == 3 {
			return fmt.Sprintf("mac=%s ip=%s host=%s", parts[0], parts[1], parts[2])
		}
	case "LOGIN_LOCAL_LOCKED":
		parts := strings.SplitN(param, ";", 4)
		if len(parts) == 4 {
			return fmt.Sprintf("ip=%s attempts=%s lockout=%ss host=%s", parts[0], parts[1], parts[2], parts[3])
		}
	}
	return param
}

// filterLogs applies --search (case-insensitive substring on "event param")
// and --type (OR across types, AND with search).
func filterLogs(logs []any, search string, types []string) []any {
	if search == "" && len(types) == 0 {
		return logs
	}
	searchLC := strings.ToLower(search)
	typesLC := make([]string, len(types))
	for i, t := range types {
		typesLC[i] = strings.ToLower(t)
	}
	out := make([]any, 0, len(logs))
	for _, e := range logs {
		m, _ := e.(map[string]any)
		event := toStr(m["log"])
		param := toStr(m["param"])
		eventLC := strings.ToLower(event)
		if len(typesLC) > 0 {
			match := false
			for _, t := range typesLC {
				if eventLC == t {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		if searchLC != "" {
			hay := strings.ToLower(event + " " + param)
			if !strings.Contains(hay, searchLC) {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

func init() {
	logCmd.Flags().IntVar(&logLastN, "last", 20, "how many log entries to show (ignored with --follow)")
	logCmd.Flags().StringVar(&logSearch, "search", "", "case-insensitive substring filter across event+param")
	logCmd.Flags().StringSliceVar(&logTypes, "type", nil, "filter by event code (repeatable, OR)")
	logCmd.Flags().BoolVarP(&logFollow, "follow", "f", false, "poll and print new entries as they arrive")
	logCmd.Flags().IntVar(&logInterval, "interval", 5, "seconds between polls in --follow mode")
	rootCmd.AddCommand(logCmd)
}
