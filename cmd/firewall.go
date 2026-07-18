package cmd

import (
	"fmt"
	"strings"

	"github.com/Hexalgo/bbox-cli/internal/client"
	"github.com/spf13/cobra"
)

var firewallCmd = &cobra.Command{Use: "firewall", Short: "Firewall CRUD"}

var firewallListCmd = &cobra.Command{
	Use: "list", Short: "List firewall rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		fw, err := c().Firewall()
		if err != nil {
			return err
		}
		rules, err := c().FirewallRules()
		if err != nil {
			return err
		}
		if emit(map[string]any{"firewall": fw, "rules": rules}) {
			return nil
		}
		state := "OFF"
		if b, ok := fw["enable"]; ok && toBoolAny(b) {
			state = "ON"
		}
		var caps []string
		if arr, ok := fw["protoscapabilities"].([]any); ok {
			for _, x := range arr {
				caps = append(caps, fmt.Sprintf("%v", x))
			}
		}
		fmt.Printf("Firewall: %s   protocols: %s\n", state, strings.Join(caps, ", "))
		fmt.Printf("%-3s %-3s %-40s %-7s %-15s %-10s\n", "ID", "On", "Description", "Action", "Dst IP/port", "Proto")
		for _, rAny := range rules {
			r, _ := rAny.(map[string]any)
			desc := toStr(r["description"])
			desc = truncate(desc, 40)
			dstip := toStr(r["dstip"])
			dstport := toStr(r["dstport"])
			dst := truncate(dstip+":"+dstport, 15)
			fmt.Printf("%-3v %-3s %-40s %-7v %-15s %-10v\n",
				firstOr(r["id"], "?"), fmtBool(r["enable"]), desc, firstOr(r["action"], "?"), dst, firstOr(r["protocol"], "?"))
		}
		return nil
	},
}

var (
	fwAddAction  string
	fwAddProto   string
	fwAddDstIP   string
	fwAddDstPort string
	fwAddSrcIP   string
	fwAddSrcPort string
)
var firewallAddCmd = &cobra.Command{
	Use: "add NAME", Args: cobra.ExactArgs(1), Short: "Add a firewall rule",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		newID, err := c().FirewallRuleAdd(client.FirewallRuleArgs{
			Description: args[0], Action: fwAddAction, Protocol: fwAddProto,
			DstIP: fwAddDstIP, DstPort: fwAddDstPort,
			SrcIP: fwAddSrcIP, SrcPort: fwAddSrcPort,
			Enable: true,
		})
		if err != nil {
			return err
		}
		fmt.Printf("OK: firewall rule id=%d (%s %s to %s:%s)\n", newID, fwAddAction, fwAddProto, fwAddDstIP, fwAddDstPort)
		return nil
	},
}

var firewallDelCmd = &cobra.Command{
	Use: "delete RULE_ID", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		id := toIntAny(args[0])
		if err := c().FirewallRuleDel(id); err != nil {
			return err
		}
		fmt.Printf("OK: deleted firewall rule id=%d\n", id)
		return nil
	},
}

var firewallToggleCmd = &cobra.Command{
	Use: "toggle [on|off]", Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs), ValidArgs: []string{"on", "off"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().FirewallToggle(args[0] == "on"); err != nil {
			return err
		}
		fmt.Printf("OK: firewall %s\n", strings.ToUpper(args[0]))
		return nil
	},
}

var firewallToggleRuleCmd = &cobra.Command{
	Use: "toggle-rule RULE_ID [on|off]", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		id := toIntAny(args[0])
		state := args[1]
		if state != "on" && state != "off" {
			return fmt.Errorf("state must be on|off")
		}
		if err := c().FirewallRuleToggle(id, state == "on"); err != nil {
			return err
		}
		fmt.Printf("OK: firewall rule %d %s\n", id, state)
		return nil
	},
}

func init() {
	firewallAddCmd.Flags().StringVar(&fwAddAction, "action", "Drop", "Drop | Accept")
	firewallAddCmd.Flags().StringVar(&fwAddProto, "protocol", "tcp", "tcp|udp|icmp|esp|ah|icmpv6|igmp|gre")
	firewallAddCmd.Flags().StringVar(&fwAddDstIP, "dst-ip", "", "destination IP")
	firewallAddCmd.Flags().StringVar(&fwAddDstPort, "dst-port", "", "destination port")
	firewallAddCmd.Flags().StringVar(&fwAddSrcIP, "src-ip", "", "source IP")
	firewallAddCmd.Flags().StringVar(&fwAddSrcPort, "src-port", "", "source port")

	firewallCmd.AddCommand(firewallListCmd, firewallAddCmd, firewallDelCmd, firewallToggleCmd, firewallToggleRuleCmd)
	rootCmd.AddCommand(firewallCmd)
}

func toBoolAny(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	case string:
		return x == "1" || strings.EqualFold(x, "on")
	}
	return false
}

func firstOr(v any, def string) any {
	if v == nil {
		return def
	}
	if s, ok := v.(string); ok && s == "" {
		return def
	}
	return v
}
