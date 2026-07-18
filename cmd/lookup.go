package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var lookupIgnoreCase bool

var lookupCmd = &cobra.Command{
	Use:   "lookup QUERY",
	Short: "Cross-section search: hostname / IP / MAC / description / port / ID",
	Long: `Search hosts, NAT rules, firewall rules and DHCP reservations for a single
substring / IP / MAC / numeric ID. Individual endpoint failures are non-fatal
(printed to stderr, treated as empty).`,
	Example: `  bbox lookup DESKTOP
  bbox lookup 192.168.1.125
  bbox lookup 45080
  bbox lookup 28:ee:52:00:0a:44`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		q := args[0]
		res := runLookup(q, lookupIgnoreCase)
		if emit(res) {
			return nil
		}
		printLookup(res)
		return nil
	},
}

// lookupResult is the JSON-shape and the human-render input.
type lookupResult struct {
	Hosts        []map[string]any `json:"hosts"`
	NATRules     []map[string]any `json:"nat_rules"`
	FirewallRule []map[string]any `json:"firewall_rules"`
	DHCPReserved []map[string]any `json:"dhcp_reservations"`
}

// runLookup fans out to the four read endpoints in parallel. An error in one
// endpoint prints a warning to stderr and returns that section as empty.
func runLookup(query string, ignoreCase bool) lookupResult {
	var wg sync.WaitGroup
	var out lookupResult

	needle := query
	if ignoreCase {
		needle = strings.ToLower(query)
	}
	match := func(s string) bool {
		if s == "" {
			return false
		}
		if ignoreCase {
			return strings.Contains(strings.ToLower(s), needle)
		}
		return strings.Contains(s, needle)
	}

	// Prime the shared client before fanning out — c() lazy-inits bx and
	// several goroutines racing on that init would trip the race detector.
	cli := c()

	wg.Add(4)
	go func() {
		defer wg.Done()
		hosts, err := cli.Hosts()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: hosts: %v\n", err)
			return
		}
		out.Hosts = filterHosts(hosts, match)
	}()
	go func() {
		defer wg.Done()
		rules, err := cli.NATRules()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: nat rules: %v\n", err)
			return
		}
		out.NATRules = filterNAT(rules, match)
	}()
	go func() {
		defer wg.Done()
		rules, err := cli.FirewallRules()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: firewall rules: %v\n", err)
			return
		}
		out.FirewallRule = filterFirewall(rules, match)
	}()
	go func() {
		defer wg.Done()
		dhcp, err := cli.DHCPClients()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: dhcp clients: %v\n", err)
			return
		}
		out.DHCPReserved = filterDHCP(dhcp, match)
	}()
	wg.Wait()
	return out
}

func filterHosts(hosts []any, match func(string) bool) []map[string]any {
	var out []map[string]any
	for _, hAny := range hosts {
		h, _ := hAny.(map[string]any)
		if h == nil {
			continue
		}
		fields := []string{
			toStr(h["hostname"]),
			toStr(h["ipaddress"]),
			toStr(h["macaddress"]),
			toStr(h["id"]),
		}
		if anyMatch(fields, match) {
			out = append(out, h)
		}
	}
	return out
}

func filterNAT(rules []any, match func(string) bool) []map[string]any {
	var out []map[string]any
	for _, rAny := range rules {
		r, _ := rAny.(map[string]any)
		if r == nil {
			continue
		}
		fields := []string{
			toStr(r["description"]),
			toStr(r["externalport"]),
			toStr(r["internalport"]),
			toStr(r["internalip"]),
			toStr(r["id"]),
		}
		if anyMatch(fields, match) {
			out = append(out, r)
		}
	}
	return out
}

func filterFirewall(rules []any, match func(string) bool) []map[string]any {
	var out []map[string]any
	for _, rAny := range rules {
		r, _ := rAny.(map[string]any)
		if r == nil {
			continue
		}
		fields := []string{
			toStr(r["description"]),
			toStr(r["dstip"]),
			toStr(r["dstport"]),
			toStr(r["srcip"]),
			toStr(r["srcport"]),
			toStr(r["id"]),
		}
		if anyMatch(fields, match) {
			out = append(out, r)
		}
	}
	return out
}

// filterDHCP walks the loose /dhcp/clients shape, which nests reservations
// under various keys depending on firmware ("clients", "reserved", or a flat
// list). We walk any list of maps we find at depth 1.
func filterDHCP(clients map[string]any, match func(string) bool) []map[string]any {
	var out []map[string]any
	visit := func(arr []any) {
		for _, e := range arr {
			m, _ := e.(map[string]any)
			if m == nil {
				continue
			}
			fields := []string{
				toStr(m["hostname"]),
				toStr(m["ipaddress"]),
				toStr(m["macaddress"]),
				toStr(m["id"]),
			}
			if anyMatch(fields, match) {
				out = append(out, m)
			}
		}
	}
	for _, v := range clients {
		if arr, ok := v.([]any); ok {
			visit(arr)
			continue
		}
		// firmware sometimes wraps under {"clients": {"list": [...]}}
		if inner, ok := v.(map[string]any); ok {
			for _, iv := range inner {
				if arr, ok := iv.([]any); ok {
					visit(arr)
				}
			}
		}
	}
	return out
}

func anyMatch(fields []string, match func(string) bool) bool {
	for _, f := range fields {
		if match(f) {
			return true
		}
	}
	return false
}

func printLookup(r lookupResult) {
	// hosts
	if len(r.Hosts) == 0 {
		fmt.Println("LAN hosts: (no match)")
	} else {
		fmt.Printf("LAN hosts (%d match)\n", len(r.Hosts))
		for _, h := range r.Hosts {
			act := "inactive"
			if toBoolAny(h["active"]) {
				act = "active"
			}
			fmt.Printf("  id=%v  %v  %v  %v  %v  %s\n",
				firstOr(h["id"], "?"),
				firstOr(h["hostname"], "?"),
				firstOr(h["ipaddress"], "?"),
				firstOr(h["macaddress"], "?"),
				firstOr(h["link"], "?"),
				act)
		}
	}
	// nat
	fmt.Println()
	if len(r.NATRules) == 0 {
		fmt.Println("NAT rules: (no match)")
	} else {
		fmt.Printf("NAT rules (%d match)\n", len(r.NATRules))
		for _, rr := range r.NATRules {
			fmt.Printf("  id=%v  %v  %v  wan:%v -> %v:%v\n",
				firstOr(rr["id"], "?"),
				firstOr(rr["description"], "?"),
				firstOr(rr["protocol"], "?"),
				firstOr(rr["externalport"], "?"),
				firstOr(rr["internalip"], "?"),
				firstOr(rr["internalport"], "?"))
		}
	}
	// firewall
	fmt.Println()
	if len(r.FirewallRule) == 0 {
		fmt.Println("Firewall rules: (no match)")
	} else {
		fmt.Printf("Firewall rules (%d match)\n", len(r.FirewallRule))
		for _, rr := range r.FirewallRule {
			fmt.Printf("  id=%v  %v  %v  %v -> %v:%v\n",
				firstOr(rr["id"], "?"),
				firstOr(rr["description"], "?"),
				firstOr(rr["action"], "?"),
				firstOr(rr["protocol"], "?"),
				firstOr(rr["dstip"], "?"),
				firstOr(rr["dstport"], "?"))
		}
	}
	// dhcp
	fmt.Println()
	if len(r.DHCPReserved) == 0 {
		fmt.Println("DHCP reservations: (no match)")
	} else {
		fmt.Printf("DHCP reservations (%d match)\n", len(r.DHCPReserved))
		for _, rr := range r.DHCPReserved {
			fmt.Printf("  %v  %v  %v\n",
				firstOr(rr["hostname"], "?"),
				firstOr(rr["ipaddress"], "?"),
				firstOr(rr["macaddress"], "?"))
		}
	}
}

func init() {
	lookupCmd.Flags().BoolVarP(&lookupIgnoreCase, "ignore-case", "i", true, "case-insensitive match")
	rootCmd.AddCommand(lookupCmd)
}
