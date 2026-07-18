package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var dmzCmd = &cobra.Command{Use: "dmz", Short: "Single-host DMZ"}

var dmzShowCmd = &cobra.Command{
	Use: "show", Short: "Show DMZ status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		d, err := c().DMZ()
		if err != nil {
			return err
		}
		if emit(d) {
			return nil
		}
		target := toStr(d["ipaddress"])
		if target == "" {
			target = "-"
		}
		printKV([][2]any{
			{"enable", fmtBool(d["enable"])},
			{"target", target},
			{"dnsprotect", d["dnsprotect"]},
		})
		return nil
	},
}

var dmzSetCmd = &cobra.Command{
	Use: "set IP", Short: "Enable DMZ to given LAN IP", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().DMZSet(args[0]); err != nil {
			return err
		}
		fmt.Printf("OK: DMZ -> %s\n", args[0])
		return nil
	},
}

var dmzOffCmd = &cobra.Command{
	Use: "off", Short: "Disable DMZ",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().DMZOff(); err != nil {
			return err
		}
		fmt.Println("OK: DMZ off")
		return nil
	},
}

func init() {
	dmzCmd.AddCommand(dmzShowCmd, dmzSetCmd, dmzOffCmd)
	rootCmd.AddCommand(dmzCmd)
}
