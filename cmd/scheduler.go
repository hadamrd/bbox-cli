package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

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
		entries, _ := s["scheduler"].([]any)
		if len(entries) == 0 {
			// Some firmwares expose entries under other keys.
			entries, _ = s["entries"].([]any)
		}
		if len(entries) == 0 {
			fmt.Println("  entries      -")
			return nil
		}
		fmt.Printf("\n%-4s %-4s %-10s %-10s\n", "ID", "En", "Start", "End")
		for _, eAny := range entries {
			e, _ := eAny.(map[string]any)
			fmt.Printf("%-4v %-4s %-10v %-10v\n",
				firstOr(e["id"], "?"), fmtBool(e["enable"]),
				firstOr(e["starttime"], "?"), firstOr(e["endtime"], "?"))
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

func init() {
	schedulerCmd.AddCommand(schedulerShowCmd, schedulerOffCmd)
	rootCmd.AddCommand(schedulerCmd)
}
