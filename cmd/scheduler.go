package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hadamrd/bbox-cli/pkg/client"
	"github.com/spf13/cobra"
)

// dayIndex maps day names to the Bbox occurency index (Mon=1..Sat=6, Sun=0).
var dayIndex = map[string]string{
	"mon": "1", "tue": "2", "wed": "3", "thu": "4", "fri": "5", "sat": "6", "sun": "0",
}

// parseDays converts a friendly --days value into a Bbox "occurency" string
// (comma-separated indices). Accepts day names (mon,tue,…), raw indices (1,2,…),
// or the shortcuts weekdays / weekends / everyday|all.
func parseDays(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "weekdays":
		return "1,2,3,4,5", nil
	case "weekends", "weekend":
		return "6,0", nil
	case "everyday", "all", "":
		return "1,2,3,4,5,6,0", nil
	}
	var out []string
	for _, tok := range strings.Split(s, ",") {
		tok = strings.TrimSpace(tok)
		if idx, ok := dayIndex[tok]; ok {
			out = append(out, idx)
			continue
		}
		if n, err := strconv.Atoi(tok); err == nil && n >= 0 && n <= 6 {
			out = append(out, tok)
			continue
		}
		return "", fmt.Errorf("invalid day %q (use mon..sun, 0..6, or weekdays/weekends/everyday)", tok)
	}
	return strings.Join(out, ","), nil
}

// validHHMM checks a "HH:MM" 24h time.
func validHHMM(s string) bool {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	return err1 == nil && err2 == nil && h >= 0 && h <= 24 && m >= 0 && m < 60
}

// schedulerRuleFromFlags builds a SchedulerRuleArgs shared by `scheduler add`
// and `parental add`.
func schedulerRuleFromFlags(name, days, start, end string, enable bool) (client.SchedulerRuleArgs, error) {
	if name == "" {
		return client.SchedulerRuleArgs{}, fmt.Errorf("--name is required")
	}
	if !validHHMM(start) || !validHHMM(end) {
		return client.SchedulerRuleArgs{}, fmt.Errorf("--start/--end must be HH:MM (24h)")
	}
	occ, err := parseDays(days)
	if err != nil {
		return client.SchedulerRuleArgs{}, err
	}
	return client.SchedulerRuleArgs{
		Name:      name,
		Occurency: occ,
		Intervals: start + "," + end,
		Enable:    enable,
	}, nil
}

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Wireless power scheduler (auto on/off windows)",
}

var schedulerShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show scheduler state and entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		s, err := c().WifiScheduler()
		if err != nil {
			return err
		}
		if emit(s) {
			return nil
		}
		if len(s) == 0 {
			fmt.Println("(scheduler not supported by this Bbox model)")
			return nil
		}
		printKV([][2]any{
			{"enabled", fmtBool(s["enable"])},
			{"now", dash(s["now"])},
		})
		// savedRules is the editable window set (real ids, intervals, occurency).
		// When the scheduler is enabled, `rules` holds a runtime-expanded view
		// (id -1, name null, start/end objects) — not manageable — so prefer
		// savedRules and only fall back to rules if savedRules is absent.
		entries, _ := s["savedRules"].([]any)
		if len(entries) == 0 {
			entries, _ = s["rules"].([]any)
		}
		if len(entries) == 0 {
			fmt.Println("  windows      -")
			return nil
		}
		fmt.Printf("\n%-4s %-4s %-16s %-14s %s\n", "ID", "En", "Window", "Days", "Name")
		for _, eAny := range entries {
			e, _ := eAny.(map[string]any)
			fmt.Printf("%-4v %-4s %-16v %-14v %v\n",
				firstOr(e["id"], "?"), fmtBool(e["enable"]),
				dash(e["intervals"]), dash(e["occurency"]), dash(e["name"]))
		}
		return nil
	},
}

var schedulerOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable the scheduler (leaves entries in place)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().WifiSchedulerOff(); err != nil {
			return err
		}
		fmt.Println("OK: scheduler disabled")
		return nil
	},
}

var schedulerOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable the scheduler / WiFi-pause windows (leaves entries in place)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().WifiSchedulerOn(); err != nil {
			return err
		}
		fmt.Println("OK: scheduler enabled")
		return nil
	},
}

var (
	schedName, schedDays, schedStart, schedEnd string
	schedEnable                                bool
)

var schedulerAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a WiFi-pause window (WiFi off during the window)",
	Long: `Add a recurring WiFi-pause window. The WiFi radios switch off during the
window on the selected days. Windows are stored even while the scheduler is
disabled; run 'bbox scheduler on' to activate them.`,
	Example: `  # WiFi off 22:30→07:00 on weeknights, active immediately
  bbox scheduler add --name "School nights" --days weekdays --start 22:30 --end 07:00
  bbox scheduler on`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		rule, err := schedulerRuleFromFlags(schedName, schedDays, schedStart, schedEnd, schedEnable)
		if err != nil {
			return err
		}
		id, err := c().WifiSchedulerAddRule(rule)
		if err != nil {
			return err
		}
		fmt.Printf("OK: added WiFi-pause window id=%d (%s %s→%s days=%s)\n", id, rule.Name, schedStart, schedEnd, rule.Occurency)
		return nil
	},
}

var schedulerDelCmd = &cobra.Command{
	Use: "del ID", Args: cobra.ExactArgs(1), Short: "Remove a WiFi-pause window by id",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("ID must be a number")
		}
		if err := c().WifiSchedulerDelRule(id); err != nil {
			return err
		}
		fmt.Printf("OK: removed WiFi-pause window id=%d\n", id)
		return nil
	},
}

func init() {
	schedulerAddCmd.Flags().StringVar(&schedName, "name", "", "window label (required)")
	schedulerAddCmd.Flags().StringVar(&schedDays, "days", "everyday", "days: mon..sun / 0..6 / weekdays / weekends / everyday")
	schedulerAddCmd.Flags().StringVar(&schedStart, "start", "", "start time HH:MM (required)")
	schedulerAddCmd.Flags().StringVar(&schedEnd, "end", "", "end time HH:MM (required)")
	schedulerAddCmd.Flags().BoolVar(&schedEnable, "enable", true, "create the window enabled")
	schedulerCmd.AddCommand(schedulerShowCmd, schedulerOnCmd, schedulerOffCmd, schedulerAddCmd, schedulerDelCmd)
	rootCmd.AddCommand(schedulerCmd)
}
