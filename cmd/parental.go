package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var parentalCmd = &cobra.Command{Use: "parental", Short: "Parental control (internet access windows + per-device)"}

var parentalShowCmd = &cobra.Command{
	Use: "show", Short: "Show parental-control state, policy, and access windows",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		s, err := c().ParentalScheduler()
		if err != nil {
			return err
		}
		// defaultpolicy lives on the base /parentalcontrol read, not on /scheduler.
		policy := s["defaultpolicy"]
		if policy == nil {
			if base, err := c().Parental(); err == nil {
				policy = getMap(base, "scheduler")["defaultpolicy"]
			}
		}
		if emit(map[string]any{"scheduler": s, "defaultpolicy": policy}) {
			return nil
		}
		printKV([][2]any{
			{"enabled", fmtBool(s["enable"])},
			{"defaultpolicy", dash(policy)},
			{"now", dash(s["now"])},
		})
		// Prefer savedRules (editable set with real ids); `rules` is the
		// runtime-expanded view when parental control is enabled.
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

var parentalToggleCmd = &cobra.Command{
	Use: "toggle STATE", Args: cobra.ExactArgs(1), Short: "Enable/disable parental control (on|off)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().ParentalToggle(args[0] == "on"); err != nil {
			return err
		}
		fmt.Printf("OK: parental control %s\n", strings.ToUpper(args[0]))
		return nil
	},
}

var parentalPolicyCmd = &cobra.Command{
	Use: "policy POLICY", Args: cobra.ExactArgs(1), Short: "Set default policy (forbid|allow)",
	Long: `Set the default access policy applied outside the configured windows:
  forbid  -> access blocked by default (Forbidden), windows grant access
  allow   -> access allowed by default (Accept), windows restrict access`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		var policy string
		switch strings.ToLower(args[0]) {
		case "forbid", "forbidden":
			policy = "Forbidden"
		case "allow", "accept":
			policy = "Accept"
		default:
			return fmt.Errorf("policy must be 'forbid' or 'allow'")
		}
		if err := c().ParentalSetPolicy(policy); err != nil {
			return err
		}
		fmt.Printf("OK: parental default policy = %s\n", policy)
		return nil
	},
}

var (
	pcName, pcDays, pcStart, pcEnd string
	pcEnable                       bool
)

var parentalAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a parental-control access window",
	Example: `  # Allow internet 16:00→20:00 on weekdays (with default policy 'forbid')
  bbox parental policy forbid
  bbox parental add --name "After school" --days weekdays --start 16:00 --end 20:00
  bbox parental toggle on`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		rule, err := schedulerRuleFromFlags(pcName, pcDays, pcStart, pcEnd, pcEnable)
		if err != nil {
			return err
		}
		id, err := c().ParentalAddRule(rule)
		if err != nil {
			return err
		}
		fmt.Printf("OK: added parental window id=%d (%s %s→%s days=%s)\n", id, rule.Name, pcStart, pcEnd, rule.Occurency)
		return nil
	},
}

var parentalDelCmd = &cobra.Command{
	Use: "del ID", Args: cobra.ExactArgs(1), Short: "Remove a parental-control window by id",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("ID must be a number")
		}
		if err := c().ParentalDelRule(id); err != nil {
			return err
		}
		fmt.Printf("OK: removed parental window id=%d\n", id)
		return nil
	},
}

var parentalDeviceCmd = &cobra.Command{
	Use: "device MAC STATE", Args: cobra.ExactArgs(2), Short: "Enrol/release a device from parental control (on|off)",
	Long: `Place a device under parental control (on) or release it (off), keyed by MAC.
While enrolled, the device's internet access follows the parental-control policy
and windows.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().ParentalHostSet(args[0], args[1] == "on"); err != nil {
			return err
		}
		fmt.Printf("OK: device %s parental control %s\n", args[0], strings.ToUpper(args[1]))
		return nil
	},
}

func init() {
	parentalAddCmd.Flags().StringVar(&pcName, "name", "", "window label (required)")
	parentalAddCmd.Flags().StringVar(&pcDays, "days", "everyday", "days: mon..sun / 0..6 / weekdays / weekends / everyday")
	parentalAddCmd.Flags().StringVar(&pcStart, "start", "", "start time HH:MM (required)")
	parentalAddCmd.Flags().StringVar(&pcEnd, "end", "", "end time HH:MM (required)")
	parentalAddCmd.Flags().BoolVar(&pcEnable, "enable", true, "create the window enabled")
	parentalCmd.AddCommand(parentalShowCmd, parentalToggleCmd, parentalPolicyCmd, parentalAddCmd, parentalDelCmd, parentalDeviceCmd)
	rootCmd.AddCommand(parentalCmd)
}
