package cmd

import (
	"fmt"
	"strconv"
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
		guestEnable, _ := c().GuestEnable()
		if emit(map[string]any{"bands": bands, "guest": guest, "guest_enable": guestEnable}) {
			return nil
		}
		fmt.Println("WiFi status")
		redactHintShown := false
		for _, b := range []string{"24", "5", "6"} {
			info, _ := bands[b].(map[string]any)
			radio := getMap(info, "radio")
			label := b + " GHz"
			// A missing band endpoint returns an empty map: treat as
			// "not supported by this Bbox model" so we don't show <nil>.
			if len(radio) == 0 {
				fmt.Printf("  %-7s  enabled=off  (not supported by this Bbox model)\n", label)
				continue
			}
			var ch any = radio["currentchannel"]
			if ch == nil {
				ch = radio["channel"]
			}
			fmt.Printf("  %-7s  enabled=%s  standard=%v  channel=%v\n",
				label, fmtBool(radio["enable"]), dash(radio["standard"]), dash(ch))
		}
		for _, k := range []string{"guest24", "guest5"} {
			gi, ok := guest[k].(map[string]any)
			if !ok {
				continue
			}
			pass, redacted := redact(gi["passphrase"])
			fmt.Printf("  %-7s  SSID=%v  passphrase=%v\n", k, dash(gi["SSID"]), pass)
			if redacted && !redactHintShown {
				fmt.Println("           (use --show-secrets to reveal)")
				redactHintShown = true
			}
		}
		return nil
	},
}

var wifiGuestCmd = &cobra.Command{
	Use: "guest", Short: "Show guest WiFi (state + SSID/passphrase)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		g, err := c().WifiGuest()
		if err != nil {
			return err
		}
		ge, _ := c().GuestEnable()
		if emit(map[string]any{"guest": g, "guest_enable": ge}) {
			return nil
		}
		fmt.Println("Guest WiFi")
		// Prefer showing the guest24 SSID/passphrase (the guest network is
		// mirrored on both bands with the same credentials on Bbox firmware).
		var ssid, pass any
		for _, k := range []string{"guest24", "guest5"} {
			if gi, ok := g[k].(map[string]any); ok {
				if ssid == nil {
					ssid = gi["SSID"]
				}
				if pass == nil {
					pass = gi["passphrase"]
				}
			}
		}
		passOut, redacted := redact(pass)
		printKV([][2]any{
			{"enabled", fmtBool(ge["enable"])},
			{"radiostatus", fmtBool(ge["radiostatus"])},
			{"SSID", dash(ssid)},
			{"passphrase", passOut},
		})
		if redacted {
			fmt.Println("  (use --show-secrets to reveal)")
		}
		return nil
	},
}

var wifiGuestKeyCmd = &cobra.Command{
	Use: "key NEW_PASSPHRASE", Args: cobra.ExactArgs(1), Short: "Change guest WiFi passphrase",
	Long: `Rotate the guest-WiFi WPA passphrase. The guest network is mirrored across
2.4 and 5 GHz on Bbox firmware, so this one call updates both. Existing guest
clients are disconnected; they must reconnect with the new passphrase.`,
	Example: `  # Rotate to a fresh passphrase (quote if it contains shell metachars)
  bbox wifi guest key 'Correct-Horse-Battery-Staple-42'

  # Verify the change (append --show-secrets to see the passphrase in cleartext)
  bbox wifi guest --show-secrets`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().GuestKeySet(args[0]); err != nil {
			return err
		}
		fmt.Println("OK: guest WiFi passphrase changed")
		return nil
	},
}

var wifiGuestSSIDCmd = &cobra.Command{
	Use: "ssid NEW_SSID", Args: cobra.ExactArgs(1), Short: "Rename guest WiFi SSID",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().GuestSSIDSet(args[0]); err != nil {
			return err
		}
		fmt.Printf("OK: guest WiFi SSID=%s\n", args[0])
		return nil
	},
}

var wifiGuestToggleCmd = &cobra.Command{
	Use: "toggle STATE", Args: cobra.ExactArgs(1), Short: "Enable/disable guest WiFi (on|off)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		on := args[0] == "on"
		if err := c().WifiGuestToggle(on); err != nil {
			return err
		}
		fmt.Printf("OK: guest WiFi %s\n", strings.ToUpper(args[0]))
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
	Use:   "toggle BAND STATE",
	Args:  cobra.ExactArgs(2),
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

// ─── WiFi access control (MAC ACL) ──────────────────────────────────────────

var wifiACLCmd = &cobra.Command{Use: "acl", Short: "WiFi access control (MAC allow/deny filtering)"}

var wifiACLShowCmd = &cobra.Command{
	Use: "show", Short: "Show MAC access-control state + entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		acl, err := c().WifiACL()
		if err != nil {
			return err
		}
		if emit(acl) {
			return nil
		}
		printKV([][2]any{{"enabled", fmtBool(acl["enable"])}})
		rules, _ := acl["rules"].([]any)
		if len(rules) == 0 {
			fmt.Println("  entries      -")
			return nil
		}
		fmt.Printf("\n%-4s %-4s %-18s\n", "ID", "En", "MAC")
		for _, rAny := range rules {
			r, _ := rAny.(map[string]any)
			fmt.Printf("%-4v %-4s %-18v\n", firstOr(r["id"], "?"), fmtBool(r["enable"]), dash(r["macaddress"]))
		}
		return nil
	},
}

var wifiACLToggleCmd = &cobra.Command{
	Use: "toggle STATE", Args: cobra.ExactArgs(1), Short: "Enable/disable MAC filtering (on|off)",
	Long: `Turn WiFi MAC access-control filtering on or off.

WARNING: enabling filtering makes the router enforce the entry list. Confirm the
device you are managing from is covered before enabling, or you may lock yourself
out of WiFi. Wired access is unaffected.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		on := args[0] == "on"
		if err := c().WifiACLToggle(on); err != nil {
			return err
		}
		fmt.Printf("OK: WiFi MAC filtering %s\n", strings.ToUpper(args[0]))
		return nil
	},
}

var wifiACLEnable bool
var wifiACLAddCmd = &cobra.Command{
	Use: "add MAC", Args: cobra.ExactArgs(1), Short: "Add a MAC to the access-control list",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		id, err := c().WifiACLAddRule(args[0], wifiACLEnable)
		if err != nil {
			return err
		}
		fmt.Printf("OK: added ACL entry id=%d mac=%s enabled=%v\n", id, args[0], wifiACLEnable)
		return nil
	},
}

var wifiACLDelCmd = &cobra.Command{
	Use: "del ID", Args: cobra.ExactArgs(1), Short: "Remove an access-control entry by id",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("ID must be a number")
		}
		if err := c().WifiACLDelRule(id); err != nil {
			return err
		}
		fmt.Printf("OK: removed ACL entry id=%d\n", id)
		return nil
	},
}

func init() {
	wifiACLAddCmd.Flags().BoolVar(&wifiACLEnable, "enable", true, "create the entry enabled")
	wifiACLCmd.AddCommand(wifiACLShowCmd, wifiACLToggleCmd, wifiACLAddCmd, wifiACLDelCmd)
	wifiGuestCmd.AddCommand(wifiGuestKeyCmd, wifiGuestSSIDCmd, wifiGuestToggleCmd)
	wifiCmd.AddCommand(wifiStatusCmd, wifiGuestCmd, wifiWpsCmd, wifiToggleCmd, wifiChannelCmd, wifiSSIDCmd, wifiKeyCmd, wifiACLCmd)
	rootCmd.AddCommand(wifiCmd)
}
