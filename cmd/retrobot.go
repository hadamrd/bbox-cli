package cmd

import (
	"fmt"
	"strconv"

	"github.com/hadamrd/bbox-cli/internal/client"
	"github.com/spf13/cobra"
)

var retrobotCmd = &cobra.Command{Use: "retrobot", Short: "retrobot proxy shortcut"}

var (
	rbTargetIP      string
	rbUser          string
	rbPassword      string
	rbAccountID     int
	rbSkipPortCheck bool
)
var retrobotSetupCmd = &cobra.Command{
	Use:   "setup NAME EXTERNAL_PORT",
	Args:  cobra.ExactArgs(2),
	Short: "add NAT rule + print SOCKS5 URL for accounts.socks5_proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		name := args[0]
		externalPort, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("external_port must be int: %w", err)
		}
		targetIP := rbTargetIP
		if targetIP == "" {
			me, err := c().HostMe()
			if err == nil {
				if s, ok := me["ipaddress"].(string); ok {
					targetIP = s
				}
			}
		}
		if targetIP == "" {
			return fmt.Errorf("could not determine LAN IP. Pass --target-ip explicitly.")
		}
		if !rbSkipPortCheck {
			if err := checkMAPTRange(externalPort); err != nil {
				return err
			}
		}
		newID, err := c().NATAdd(client.NATAddArgs{
			Description:  name,
			ExternalPort: externalPort,
			InternalIP:   targetIP,
			InternalPort: externalPort,
			Protocol:     "tcp",
		})
		if err != nil {
			return err
		}
		wan, _ := c().WAN()
		wanIP := "?"
		if s, ok := getMap(wan, "ip")["address"].(string); ok && s != "" {
			wanIP = s
		}
		socksURL := fmt.Sprintf("socks5://%s:%s@%s:%d", rbUser, rbPassword, wanIP, externalPort)
		fmt.Printf("OK: rule '%s' id=%d  wan:%d -> %s:%d/tcp\n", name, newID, externalPort, targetIP, externalPort)
		fmt.Println()
		fmt.Println("SOCKS5 URL (paste into retrobot accounts.socks5_proxy):")
		fmt.Printf("  %s\n", socksURL)
		fmt.Println()
		fmt.Printf("Now start a SOCKS5 server on THIS PC bound to 0.0.0.0:%d, e.g.:\n", externalPort)
		fmt.Printf("  gost -L \"socks5://%s:%s@:%d\"\n", rbUser, rbPassword, externalPort)
		fmt.Println()
		if rbAccountID != 0 {
			fmt.Println("To update the retrobot DB directly (via SSH to kamator-prod):")
			fmt.Printf("  ssh kamator-prod 'docker exec retrobot-postgres psql -U postgres -d retrobot -c \"UPDATE accounts SET socks5_proxy=\\'%s\\' WHERE id=%d;\"'\n", socksURL, rbAccountID)
		}
		return nil
	},
}

var retrobotTeardownCmd = &cobra.Command{
	Use: "teardown NAME", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		name := args[0]
		rules, err := c().NATRules()
		if err != nil {
			return err
		}
		matched := false
		for _, rAny := range rules {
			r, _ := rAny.(map[string]any)
			desc, _ := r["description"].(string)
			if desc == name {
				matched = true
				id := toIntAny(r["id"])
				if err := c().NATDel(id); err != nil {
					return err
				}
				fmt.Printf("OK: deleted id=%d (%s)\n", id, desc)
			}
		}
		if !matched {
			return fmt.Errorf("no rule '%s'", name)
		}
		return nil
	},
}

func init() {
	retrobotSetupCmd.Flags().StringVar(&rbTargetIP, "target-ip", "", "destination LAN IP (defaults to this host)")
	retrobotSetupCmd.Flags().StringVar(&rbUser, "user", "retrobot", "SOCKS5 auth user")
	retrobotSetupCmd.Flags().StringVar(&rbPassword, "password", "", "SOCKS5 auth password")
	retrobotSetupCmd.Flags().IntVar(&rbAccountID, "account-id", 0, "if given, prints SQL to update that account row")
	retrobotSetupCmd.Flags().BoolVar(&rbSkipPortCheck, "skip-port-check", false, "bypass MAP-T port-range check")
	_ = retrobotSetupCmd.MarkFlagRequired("password")

	retrobotCmd.AddCommand(retrobotSetupCmd, retrobotTeardownCmd)
	rootCmd.AddCommand(retrobotCmd)
}
