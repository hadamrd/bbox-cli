package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "One-shot pretty-printed router summary (link / WAN / hosts / services / WiFi / VoIP)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		s, err := c().Summary()
		if err != nil {
			return err
		}
		if emit(s) {
			return nil
		}
		printSummary(s)
		return nil
	},
}

// printSummary renders the observed /api/v1/summary shape. This endpoint is
// a dashboard aggregate — its per-domain shape differs from the dedicated
// endpoints (wan.ip.state is {ip:int,ipv6:string}, not the WAN detail block).
func printSummary(s map[string]any) {
	anyPrinted := false

	// --- Header (auth + last-seen timestamp + link/internet health) ---
	link := getMap(s, "link")
	inet := getMap(s, "internet")
	fmt.Println("Status")
	printKV([][2]any{
		{"now", dash(s["now"])},
		{"authenticated", fmtBool(s["authenticated"])},
		{"link", dash(link["state"])},
		{"link type", dash(link["type"])},
		{"internet state", dash(inet["state"])},
		{"alerts", dash(getMap(s, "alerts")["count"])},
	})
	anyPrinted = true

	// --- WAN state (summary shape: wan.ip.state.{ip,ipv6}) ---
	wanIP := getMap(getMap(s, "wan"), "ip")
	wanState := getMap(wanIP, "state")
	if len(wanState) > 0 {
		fmt.Println("\nWAN")
		printKV([][2]any{
			{"ipv4 state", dash(wanState["ip"])},
			{"ipv6 state", dash(wanState["ipv6"])},
		})
	}

	// --- Hosts (summary lists {hostname, ipaddress} without MAC / active flag) ---
	if hosts, ok := s["hosts"].([]any); ok && len(hosts) > 0 {
		fmt.Printf("\nHosts (%d shown)\n", len(hosts))
		for _, h := range hosts {
			hm, _ := h.(map[string]any)
			fmt.Printf("  %-25s %s\n", dash(hm["hostname"]), dash(hm["ipaddress"]))
		}
	}

	// --- Services (enable flags, sorted for stable output) ---
	svc := getMap(s, "services")
	if len(svc) > 0 {
		fmt.Println("\nServices")
		names := make([]string, 0, len(svc))
		for k := range svc {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, name := range names {
			sm := getMap(svc, name)
			if len(sm) == 0 {
				continue
			}
			// upnp is nested (services.upnp.igd.enable); flatten it.
			if name == "upnp" {
				sm = getMap(sm, "igd")
			}
			line := fmt.Sprintf("  %-22s enable=%s", name, fmtBool(sm["enable"]))
			if v, ok := sm["status"]; ok && v != nil && v != "" {
				line += fmt.Sprintf("  status=%v", v)
			}
			fmt.Println(line)
		}
	}

	// --- Wireless flags ---
	wl := getMap(s, "wireless")
	if len(wl) > 0 {
		fmt.Println("\nWireless")
		printKV([][2]any{
			{"status", dash(wl["status"])},
			{"radio", fmtBool(wl["radio"])},
			{"guest 2.4", fmtBool(wl["guest24enable"])},
			{"guest 5", fmtBool(wl["guest5enable"])},
			{"guest master", fmtBool(wl["guestenable"])},
			{"config changed", dash(wl["changedate"])},
		})
	}

	// --- VoIP lines ---
	if v, ok := s["voip"].([]any); ok && len(v) > 0 {
		fmt.Println("\nVoIP")
		for i, line := range v {
			lm, _ := line.(map[string]any)
			fmt.Printf("  line %d  status=%v  callstate=%v  msgs=%v  missed=%v\n",
				i+1, dash(lm["status"]), dash(lm["callstate"]),
				dash(lm["message"]), dash(lm["notanswered"]))
		}
	}

	if !anyPrinted {
		keys := make([]string, 0, len(s))
		for k := range s {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(os.Stderr, "warning: unrecognised summary shape; top-level keys: %v — dumping raw JSON.\n", keys)
		b, _ := json.MarshalIndent(s, "", "  ")
		fmt.Println(string(b))
	}
}

func init() {
	rootCmd.AddCommand(summaryCmd)
}
