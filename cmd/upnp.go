package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var upnpCmd = &cobra.Command{Use: "upnp", Short: "UPnP IGD"}

var upnpShowCmd = &cobra.Command{
	Use: "show", Short: "Show UPnP status + rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		u, err := c().UPnP()
		if err != nil {
			return err
		}
		rules, err := c().UPnPRules()
		if err != nil {
			return err
		}
		if emit(map[string]any{"upnp": u, "rules": rules}) {
			return nil
		}
		printKV([][2]any{
			{"enable", fmtBool(u["enable"])},
			{"state", u["state"]},
			{"name", u["friendlyname"]},
		})
		fmt.Printf("\n%d UPnP rule(s)\n", len(rules))
		for _, r := range rules {
			fmt.Printf("  %v\n", r)
		}
		return nil
	},
}

var upnpRulesCmd = &cobra.Command{
	Use: "rules", Short: "List UPnP rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		rules, err := c().UPnPRules()
		if err != nil {
			return err
		}
		if emit(rules) {
			return nil
		}
		fmt.Printf("%d UPnP rule(s)\n", len(rules))
		for _, r := range rules {
			fmt.Printf("  %v\n", r)
		}
		return nil
	},
}

var upnpToggleCmd = &cobra.Command{
	Use: "toggle [on|off]", Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs), ValidArgs: []string{"on", "off"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().UPnPToggle(args[0] == "on"); err != nil {
			return err
		}
		fmt.Printf("OK: UPnP %s\n", strings.ToUpper(args[0]))
		return nil
	},
}

func init() {
	upnpCmd.AddCommand(upnpShowCmd, upnpRulesCmd, upnpToggleCmd)
	rootCmd.AddCommand(upnpCmd)
}
