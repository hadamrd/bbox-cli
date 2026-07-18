package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hadamrd/bbox-cli/pkg/client"
	"github.com/spf13/cobra"
)

var natCmd = &cobra.Command{Use: "nat", Short: "NAT / PAT port-forwards"}

var natListCmd = &cobra.Command{
	Use:   "list",
	Short: "List NAT rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		rules, err := c().NATRules()
		if err != nil {
			return err
		}
		on, err := c().NATEnabled()
		if err != nil {
			return err
		}
		if emit(map[string]any{"enable": on, "rules": rules}) {
			return nil
		}
		state := "OFF"
		if on {
			state = "ON"
		}
		fmt.Printf("NAT service: %s   (%d rule(s))\n", state, len(rules))
		if len(rules) == 0 {
			return nil
		}
		fmt.Printf("%-4s %-25s %-5s %-10s %-16s %-10s\n", "ID", "Description", "Proto", "WAN port", "-> LAN IP", "LAN port")
		for _, rAny := range rules {
			r, _ := rAny.(map[string]any)
			fmt.Printf("%-4v %-25v %-5v %-10v %-16v %-10v\n",
				r["id"], r["description"], r["protocol"], r["externalport"], r["internalip"], r["internalport"])
		}
		return nil
	},
}

var (
	natAddInternalPort  int
	natAddProtocol      string
	natAddRemoteIP      string
	natAddSkipPortCheck bool
)
var natAddCmd = &cobra.Command{
	Use:   "add NAME EXTERNAL_PORT TARGET_IP",
	Short: "Add a NAT rule (MAP-T port-range validated)",
	Long: `Add a port-forward rule. The Bbox is behind Bouygues MAP-T, which reserves
a specific WAN port range for your subscriber. bbox nat add refuses to create
rules with EXTERNAL_PORT outside that range unless --skip-port-check is passed.
Run bbox info to see your allowed range.`,
	Example: `  # Expose a local SSH server on WAN port 40080 (inside MAP-T range)
  bbox nat add ssh 40080 192.168.1.42 --internal-port 22

  # Expose a UDP game server
  bbox nat add game 40100 192.168.1.30 --protocol udp

  # Bypass the port-range check (advanced)
  bbox nat add unrestricted 22 192.168.1.42 --skip-port-check`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		externalPort, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("external_port must be int: %w", err)
		}
		targetIP := args[2]
		if err := ensureAuth(); err != nil {
			return err
		}
		if !natAddSkipPortCheck {
			if err := checkMAPTRange(externalPort); err != nil {
				return err
			}
		}
		internalPort := natAddInternalPort
		if internalPort == 0 {
			internalPort = externalPort
		}
		newID, err := c().NATAdd(client.NATAddArgs{
			Description:  name,
			ExternalPort: externalPort,
			InternalIP:   targetIP,
			InternalPort: internalPort,
			Protocol:     natAddProtocol,
			RemoteIP:     natAddRemoteIP,
		})
		if err != nil {
			return err
		}
		fmt.Printf("OK: rule '%s' id=%d  wan:%d -> %s:%d/%s\n",
			name, newID, externalPort, targetIP, internalPort, natAddProtocol)
		return nil
	},
}

var natDelCmd = &cobra.Command{
	Use:   "delete SELECTOR",
	Short: "Delete a NAT rule by id or description",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		rules, err := c().NATRules()
		if err != nil {
			return err
		}
		sel := args[0]
		var rid int
		if n, err := strconv.Atoi(sel); err == nil {
			rid = n
			found := false
			for _, rAny := range rules {
				r, _ := rAny.(map[string]any)
				if toIntAny(r["id"]) == rid {
					found = true
					break
				}
			}
			if !found {
				var ids []int
				for _, rAny := range rules {
					r, _ := rAny.(map[string]any)
					ids = append(ids, toIntAny(r["id"]))
				}
				return fmt.Errorf("no rule id=%d. IDs: %v", rid, ids)
			}
		} else {
			var matches []map[string]any
			var names []string
			for _, rAny := range rules {
				r, _ := rAny.(map[string]any)
				desc, _ := r["description"].(string)
				names = append(names, desc)
				if desc == sel {
					matches = append(matches, r)
				}
			}
			if len(matches) == 0 {
				return fmt.Errorf("no rule '%s'. Names: %v", sel, names)
			}
			if len(matches) > 1 {
				return fmt.Errorf("ambiguous — pass ID instead")
			}
			rid = toIntAny(matches[0]["id"])
		}
		if err := c().NATDel(rid); err != nil {
			return err
		}
		fmt.Printf("OK: deleted id=%d\n", rid)
		return nil
	},
}

var natClearYes bool
var natClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete every NAT rule",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		rules, err := c().NATRules()
		if err != nil {
			return err
		}
		if len(rules) == 0 {
			fmt.Println("(no rules)")
			return nil
		}
		if !natClearYes {
			fmt.Println("Will delete:")
			for _, rAny := range rules {
				r, _ := rAny.(map[string]any)
				fmt.Printf("  id=%v  %v  %v->%v:%v\n", r["id"], r["description"], r["externalport"], r["internalip"], r["internalport"])
			}
			fmt.Print("Confirm? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(strings.ToLower(line))
			if line != "y" && line != "yes" {
				fmt.Println("aborted")
				return nil
			}
		}
		for _, rAny := range rules {
			r, _ := rAny.(map[string]any)
			id := toIntAny(r["id"])
			if err := c().NATDel(id); err != nil {
				return err
			}
			fmt.Printf("  deleted id=%d (%v)\n", id, r["description"])
		}
		return nil
	},
}

var natToggleCmd = &cobra.Command{
	Use:       "toggle [on|off]",
	Short:     "Enable or disable NAT/PAT",
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	ValidArgs: []string{"on", "off"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().NATToggle(args[0] == "on"); err != nil {
			return err
		}
		fmt.Printf("OK: NAT/PAT %s\n", strings.ToUpper(args[0]))
		return nil
	},
}

func init() {
	natAddCmd.Flags().IntVar(&natAddInternalPort, "internal-port", 0, "internal (LAN) port; defaults to external")
	natAddCmd.Flags().StringVar(&natAddProtocol, "protocol", "tcp", "tcp | udp")
	natAddCmd.Flags().StringVar(&natAddRemoteIP, "remote-ip", "", "restrict source to this IP")
	natAddCmd.Flags().BoolVar(&natAddSkipPortCheck, "skip-port-check", false, "bypass MAP-T port-range check")

	natClearCmd.Flags().BoolVarP(&natClearYes, "yes", "y", false, "no confirm prompt")

	natCmd.AddCommand(natListCmd, natAddCmd, natDelCmd, natClearCmd, natToggleCmd)
	rootCmd.AddCommand(natCmd)
}

// checkMAPTRange reads WAN port-range and returns an error if externalPort is outside it.
func checkMAPTRange(externalPort int) error {
	wan, err := c().WAN()
	if err != nil {
		return err
	}
	rng, _ := getMap(wan, "ip")["portrange"].(string)
	if rng == "" || !strings.Contains(rng, ":") {
		return nil
	}
	parts := strings.SplitN(rng, ":", 2)
	lo, _ := strconv.Atoi(parts[0])
	hi, _ := strconv.Atoi(parts[1])
	if externalPort < lo || externalPort > hi {
		return fmt.Errorf("port %d outside MAP-T range %d-%d. Use --skip-port-check to override.", externalPort, lo, hi)
	}
	return nil
}

func toIntAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}
