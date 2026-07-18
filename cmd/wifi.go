package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var wifiCmd = &cobra.Command{Use: "wifi", Short: "WiFi state (2.4/5/6 GHz + guest)"}

var wifiStatusCmd = &cobra.Command{
	Use: "status", Short: "WiFi status per band",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		bands := map[string]any{}
		for _, b := range []string{"24", "5", "6"} {
			info, _ := c().WifiBand(b)
			bands[b] = info
		}
		guest, err := c().WifiGuest()
		if err != nil {
			return err
		}
		if emit(map[string]any{"bands": bands, "guest": guest}) {
			return nil
		}
		fmt.Println("WiFi status")
		for _, b := range []string{"24", "5", "6"} {
			info, _ := bands[b].(map[string]any)
			radio := getMap(info, "radio")
			var ch any = radio["currentchannel"]
			if ch == nil {
				ch = radio["channel"]
			}
			label := b + " GHz"
			fmt.Printf("  %-7s  enabled=%s  standard=%v  channel=%v\n",
				label, fmtBool(radio["enable"]), firstOr(radio["standard"], "?"), ch)
		}
		for _, k := range []string{"guest24", "guest5"} {
			if gi, ok := guest[k].(map[string]any); ok {
				fmt.Printf("  %-7s  SSID=%v  passphrase=%v\n", k, gi["SSID"], gi["passphrase"])
			}
		}
		return nil
	},
}

var wifiGuestCmd = &cobra.Command{
	Use: "guest", Short: "Show guest WiFi",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		g, err := c().WifiGuest()
		if err != nil {
			return err
		}
		if emit(g) {
			return nil
		}
		b, _ := json.MarshalIndent(g, "", "  ")
		fmt.Println(string(b))
		return nil
	},
}

var wifiWpsCmd = &cobra.Command{
	Use: "wps", Short: "Trigger WPS push (2-min window)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().WPSTrigger(); err != nil {
			return err
		}
		fmt.Println("OK: WPS push triggered (2-min window)")
		return nil
	},
}

var wifiToggleCmd = &cobra.Command{
	Use:  "toggle BAND STATE",
	Args: cobra.ExactArgs(2),
	Short: "Enable/disable WiFi band (24|5|6|guest|all)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		band, state := args[0], args[1]
		on := state == "on"
		switch band {
		case "24", "5", "6":
			if err := c().WifiBandToggle(band, on); err != nil {
				return err
			}
		case "guest":
			if err := c().WifiGuestToggle(on); err != nil {
				return err
			}
		case "all":
			if err := c().WifiAllToggle(on); err != nil {
				return err
			}
		default:
			return fmt.Errorf("band must be one of: 24, 5, 6, guest, all")
		}
		fmt.Printf("OK: WiFi %s %s\n", band, strings.ToUpper(state))
		return nil
	},
}

var wifiChannelCmd = &cobra.Command{
	Use: "channel BAND CHANNEL", Args: cobra.ExactArgs(2), Short: "Set WiFi channel (number or 'auto')",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		band, channel := args[0], args[1]
		if err := c().WifiChannelSet(band, channel); err != nil {
			return err
		}
		fmt.Printf("OK: WiFi %s channel=%s\n", band, channel)
		return nil
	},
}

var wifiSSIDCmd = &cobra.Command{
	Use: "ssid BAND NEW_SSID", Args: cobra.ExactArgs(2), Short: "Rename WiFi SSID",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().WifiSSIDSet(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("OK: WiFi %s SSID=%s\n", args[0], args[1])
		return nil
	},
}

var wifiKeyCmd = &cobra.Command{
	Use: "key BAND NEW_KEY", Args: cobra.ExactArgs(2), Short: "Change WiFi WPA key",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().WifiKeySet(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("OK: WiFi %s key changed\n", args[0])
		return nil
	},
}

func init() {
	wifiCmd.AddCommand(wifiStatusCmd, wifiGuestCmd, wifiWpsCmd, wifiToggleCmd, wifiChannelCmd, wifiSSIDCmd, wifiKeyCmd)
	rootCmd.AddCommand(wifiCmd)
}
