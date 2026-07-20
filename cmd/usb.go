package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var usbCmd = &cobra.Command{Use: "usb", Short: "USB ports (USB 3.0 mode)"}

var usbShowCmd = &cobra.Command{
	Use: "show", Short: "Show USB state",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		u, err := c().USB()
		if err != nil {
			return err
		}
		if emit(u) {
			return nil
		}
		printKV([][2]any{{"usb3_enabled", fmtBool(getMap(u, "usb3")["enable"])}})
		return nil
	},
}

var usb3Cmd = &cobra.Command{
	Use: "usb3 STATE", Args: cobra.ExactArgs(1), Short: "Enable/disable USB 3.0 mode (on|off)",
	Long: `Toggle USB 3.0 mode. USB 3.0 can interfere with the 2.4 GHz WiFi band, so
some setups keep it off; disabling it also reduces attack surface if no USB
device is used.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().USB3Toggle(args[0] == "on"); err != nil {
			return err
		}
		fmt.Printf("OK: USB 3.0 mode %s\n", strings.ToUpper(args[0]))
		return nil
	},
}

func init() {
	usbCmd.AddCommand(usbShowCmd, usb3Cmd)
	rootCmd.AddCommand(usbCmd)
}
