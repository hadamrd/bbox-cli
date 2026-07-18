package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var dyndnsCmd = &cobra.Command{Use: "dyndns", Short: "DynDNS (DuckDNS, no-ip, OVH, ...)"}

var dyndnsShowCmd = &cobra.Command{
	Use: "show",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		d, err := c().DynDNS()
		if err != nil {
			return err
		}
		if emit(d) {
			return nil
		}
		hn := toStr(d["hostname"])
		if hn == "" {
			hn = "-"
		}
		pw, redacted := redact(d["password"])
		printKV([][2]any{
			{"enable", fmtBool(d["enable"])},
			{"state", dash(d["state"])},
			{"hostname", hn},
			{"username", dash(d["username"])},
			{"password", pw},
		})
		if redacted {
			fmt.Println("  (use --show-secrets to reveal)")
		}
		fmt.Println("\nSupported providers:")
		if arr, ok := d["servercapabilities"].([]any); ok {
			for _, x := range arr {
				m, _ := x.(map[string]any)
				fmt.Printf("  %-12v  %v  %v\n", m["name"], m["support"], toStr(m["site"]))
			}
		}
		return nil
	},
}

var (
	ddHostname string
	ddUsername string
	ddPassword string
)
var dyndnsEnableCmd = &cobra.Command{
	Use: "enable PROVIDER", Args: cobra.ExactArgs(1), Short: "Enable DynDNS (DuckDNS: empty username, token as password)",
	Long: `Enable the router's built-in DynDNS updater for one of the supported providers
(run bbox dyndns show to list them). DuckDNS is a special case: leave --username
empty and pass your DuckDNS token as --password. The Bbox will then push the
current WAN IP to the provider on every renewal.`,
	Example: `  # DuckDNS: no username, token-as-password
  bbox dyndns enable duckdns --hostname you.duckdns.org --password YOUR_TOKEN

  # no-ip / OVH with a user + password
  bbox dyndns enable no-ip --hostname you.ddns.net --username alice --password s3cret

  # Turn it off later
  bbox dyndns disable`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().DynDNSEnable(args[0], ddHostname, ddUsername, ddPassword); err != nil {
			return err
		}
		fmt.Printf("OK: dyndns enabled (%s) hostname=%s\n", args[0], ddHostname)
		return nil
	},
}

var dyndnsDisableCmd = &cobra.Command{
	Use: "disable",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().DynDNSDisable(); err != nil {
			return err
		}
		fmt.Println("OK: dyndns disabled")
		return nil
	},
}

func init() {
	dyndnsEnableCmd.Flags().StringVar(&ddHostname, "hostname", "", "e.g. yourname.duckdns.org")
	dyndnsEnableCmd.Flags().StringVar(&ddUsername, "username", "", "empty for DuckDNS")
	dyndnsEnableCmd.Flags().StringVar(&ddPassword, "password", "", "DuckDNS: your token from duckdns.org")
	_ = dyndnsEnableCmd.MarkFlagRequired("hostname")
	_ = dyndnsEnableCmd.MarkFlagRequired("password")
	dyndnsCmd.AddCommand(dyndnsShowCmd, dyndnsEnableCmd, dyndnsDisableCmd)
	rootCmd.AddCommand(dyndnsCmd)
}
