package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var rebootConfirm bool
var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot the router (requires --confirm)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if !rebootConfirm {
			return fmt.Errorf("refusing to reboot without --confirm. Router will be offline ~90-120s.")
		}
		if err := c().Reboot(); err != nil {
			return err
		}
		fmt.Println("OK: reboot triggered. Router will be back in ~90-120s.")
		return nil
	},
}

var ledCmd = &cobra.Command{
	Use:       "led [off|dim|on|max]",
	Short:     "Set front-panel LED luminosity",
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	ValidArgs: []string{"off", "dim", "on", "max"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		var v int
		switch args[0] {
		case "off":
			v = 0
		case "dim":
			v = 1
		case "on":
			v = 2
		case "max":
			v = 3
		}
		if err := c().LEDSet(v); err != nil {
			return err
		}
		fmt.Printf("OK: LED %s\n", args[0])
		return nil
	},
}

func init() {
	rebootCmd.Flags().BoolVar(&rebootConfirm, "confirm", false, "confirm reboot")
	rootCmd.AddCommand(rebootCmd, ledCmd)
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
