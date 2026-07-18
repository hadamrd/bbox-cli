package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	watchInterval int
	watchHistory  string
	watchVerbose  bool
)

var watchIPCmd = &cobra.Command{
	Use:   "watch-ip",
	Short: "poll and alert on WAN IP change",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		interval := watchInterval
		if interval <= 0 {
			interval = 60
		}
		fmt.Printf("watching WAN IP every %ds. Ctrl-C to stop.\n", interval)
		var prev *string
		for {
			var currentPtr *string
			wan, err := c().WAN()
			if err != nil {
				stamp := time.Now().Format("2006-01-02T15:04:05")
				fmt.Fprintf(os.Stderr, "[%s] error: %v\n", stamp, err)
				time.Sleep(time.Duration(interval) * time.Second)
				continue
			}
			if s, ok := getMap(wan, "ip")["address"].(string); ok {
				currentPtr = &s
			}
			stamp := time.Now().Format("2006-01-02T15:04:05")
			changed := false
			if prev == nil && currentPtr != nil {
				changed = true
			} else if prev != nil && currentPtr == nil {
				changed = true
			} else if prev != nil && currentPtr != nil && *prev != *currentPtr {
				changed = true
			}
			if changed {
				ct := "initial"
				if prev != nil {
					ct = "CHANGED"
				}
				fmt.Printf("[%s] %s: %s -> %s\n", stamp, ct, ptrOrNone(prev), ptrOrNone(currentPtr))
				if watchHistory != "" {
					_ = os.MkdirAll(filepath.Dir(watchHistory), 0755)
					f, err := os.OpenFile(watchHistory, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if err == nil {
						entry := map[string]any{"at": stamp, "prev": prev, "ip": currentPtr}
						b, _ := json.Marshal(entry)
						_, _ = f.Write(append(b, '\n'))
						_ = f.Close()
					}
				}
				prev = currentPtr
			} else if watchVerbose {
				fmt.Printf("[%s] stable: %s\n", stamp, ptrOrNone(currentPtr))
			}
			time.Sleep(time.Duration(interval) * time.Second)
		}
	},
}

func ptrOrNone(p *string) string {
	if p == nil {
		return "None"
	}
	return *p
}

func init() {
	watchIPCmd.Flags().IntVar(&watchInterval, "interval", 60, "seconds between polls (default 60)")
	watchIPCmd.Flags().StringVar(&watchHistory, "history", "", "append every observed change to JSONL at this path")
	watchIPCmd.Flags().BoolVar(&watchVerbose, "verbose-watch", false, "print every poll, not just changes")
	rootCmd.AddCommand(watchIPCmd)
}
