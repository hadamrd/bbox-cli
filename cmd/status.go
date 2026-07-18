package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Router / WAN / services summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		dev, err := c().Device()
		if err != nil {
			return err
		}
		wan, err := c().WAN()
		if err != nil {
			return err
		}
		svc, err := c().Services()
		if err != nil {
			return err
		}
		payload := map[string]any{"device": dev, "wan": wan, "services": svc}
		if emit(payload) {
			return nil
		}
		ip := getMap(wan, "ip")
		fmt.Println("Router")
		fw := getMap(dev, "main")["version"]
		if fw == nil {
			fw = dev["firmware"]
		}
		printKV([][2]any{
			{"model", dash(dev["modelname"])},
			{"serial", dash(dev["serialnumber"])},
			{"firmware", dash(fw)},
			{"uptime (boots)", dash(dev["numberofboots"])},
			{"now", dash(dev["now"])},
		})
		fmt.Println("\nWAN")
		portRange := toStr(ip["portrange"])
		if portRange == "" {
			portRange = "(full)"
		}
		printKV([][2]any{
			{"ipv4", firstNonEmpty(ip["address"], "?")},
			{"state", firstNonEmpty(ip["state"], "?")},
			{"mac", firstNonEmpty(ip["mac"], "?")},
			{"port range", portRange},
			{"MAP-T", fmtBool(ip["maptenable"])},
			{"CGNAT flag", dash(ip["cgnatenable"])},
		})
		fmt.Println("\nServices")
		for name, metaAny := range svc {
			meta, ok := metaAny.(map[string]any)
			if !ok {
				continue
			}
			if _, hasEnable := meta["enable"]; !hasEnable {
				continue
			}
			nbrules := meta["nbrules"]
			nbStr := "-"
			if nbrules != nil {
				nbStr = fmt.Sprintf("%v", nbrules)
			}
			fmt.Printf("  %-15s  enable=%s  status=%v  rules=%s\n",
				name, fmtBool(meta["enable"]), dash(meta["status"]), nbStr)
		}
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "WAN + LAN details incl. MAP-T port range",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		wan, err := c().WAN()
		if err != nil {
			return err
		}
		lan, err := c().LAN()
		if err != nil {
			return err
		}
		if emit(map[string]any{"wan": wan, "lan": lan}) {
			return nil
		}
		ip := getMap(wan, "ip")
		fmt.Println("WAN")
		var ipv6 any = "-"
		if arr, ok := ip["ip6address"].([]any); ok && len(arr) > 0 {
			if m, ok := arr[0].(map[string]any); ok {
				if s, ok := m["ipaddress"].(string); ok {
					ipv6 = s
				}
			}
		}
		portRange := toStr(ip["portrange"])
		if portRange == "" {
			portRange = "(full)"
		}
		printKV([][2]any{
			{"public ipv4", firstNonEmpty(ip["address"], "?")},
			{"ipv6", ipv6},
			{"state", firstNonEmpty(ip["state"], "?")},
			{"port range", portRange},
			{"MAP-T", fmtBool(ip["maptenable"])},
		})
		fmt.Println("\nLAN")
		lanIP := getMap(lan, "ip")
		printKV([][2]any{
			{"gateway", lanIP["ipaddress"]},
			{"mtu", lanIP["mtu"]},
		})
		rng := toStr(ip["portrange"])
		if rng != "" && strings.Contains(rng, ":") {
			parts := strings.SplitN(rng, ":", 2)
			fmt.Printf("\n-> Port-forwards MUST use an EXTERNAL port in %s-%s.\n", parts[0], parts[1])
		}
		return nil
	},
}

var wanIPCmd = &cobra.Command{
	Use:   "wan-ip",
	Short: "just print public WAN IPv4 (scriptable)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		wan, err := c().WAN()
		if err != nil {
			return err
		}
		ip, _ := getMap(wan, "ip")["address"].(string)
		if emit(map[string]any{"ip": ip}) {
			return nil
		}
		fmt.Println(ip)
		return nil
	},
}

var logClearCmd = &cobra.Command{
	Use:   "log-clear",
	Short: "Clear device logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().LogClear(); err != nil {
			return err
		}
		fmt.Println("OK: log cleared")
		return nil
	},
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "WAN/LAN traffic counters",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		wanS, _ := c().WANStats()
		lanS, _ := c().LANStats()
		payload := map[string]any{"wan_stats": wanS, "lan_stats": lanS}
		if emit(payload) {
			return nil
		}
		fmt.Println("WAN traffic (bytes)")
		b, _ := json.MarshalIndent(wanS, "", "  ")
		fmt.Println(string(b))
		fmt.Println("\nLAN traffic (bytes)")
		b, _ = json.MarshalIndent(lanS, "", "  ")
		fmt.Println(string(b))
		return nil
	},
}

var exportFile string
var exportDiffPath string
var exportSnapshot bool
var exportCmd = &cobra.Command{
	Use:   "export-config",
	Short: "dump the entire router state to JSON (or diff against a saved snapshot with --diff)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		endpoints := []string{
			"/api/v1/summary", "/api/v1/device", "/api/v1/services",
			"/api/v1/wan/ip", "/api/v1/wan/ip/stats", "/api/v1/wan/backup", "/api/v1/wan/xdsl",
			"/api/v1/lan/ip", "/api/v1/lan/stats",
			"/api/v1/hosts", "/api/v1/hosts/me",
			"/api/v1/nat/rules", "/api/v1/nat/dmz", "/api/v1/upnp/igd", "/api/v1/upnp/igd/rules",
			"/api/v1/firewall", "/api/v1/firewall/rules",
			"/api/v1/dhcp", "/api/v1/dhcp/clients", "/api/v1/dyndns",
			"/api/v1/wireless", "/api/v1/wireless/24", "/api/v1/wireless/5", "/api/v1/wireless/6",
			"/api/v1/wireless/guest", "/api/v1/wireless/scheduler", "/api/v1/wireless/acl", "/api/v1/wireless/wps",
			"/api/v1/parentalcontrol", "/api/v1/hibernate/scheduler",
			"/api/v1/notification", "/api/v1/voip",
			"/api/v1/device/log",
		}
		out := map[string]any{"exported_at": nowUTC()}
		for _, p := range endpoints {
			v, err := c().Get(p)
			if err != nil {
				out[p] = map[string]any{"error": err.Error()}
			} else {
				out[p] = v
			}
		}
		// --diff mode: don't emit the export; print the diff vs the saved snapshot.
		if exportDiffPath != "" {
			prev, err := readExportFile(exportDiffPath)
			if err != nil {
				return err
			}
			rep := diffExports(prev, out)
			if jsonOut {
				b, _ := json.MarshalIndent(rep, "", "  ")
				fmt.Println(string(b))
			} else {
				printDiffHuman(rep)
			}
			return nil
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		if exportSnapshot {
			if exportFile != "" {
				fmt.Fprintln(os.Stderr, "warning: --file was ignored (--snapshot took precedence)")
			}
			path, err := writeSnapshot(b)
			if err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		}
		if exportFile != "" {
			if err := os.WriteFile(exportFile, b, 0644); err != nil {
				return err
			}
			fmt.Printf("OK: wrote %s (%d bytes)\n", exportFile, len(b))
		} else {
			fmt.Println(string(b))
		}
		return nil
	},
}

// writeSnapshot dumps the export blob to ~/.bbox-snapshots/YYYYMMDD-HHMMSS.json,
// creating the directory on demand. Returns the absolute path written.
func writeSnapshot(b []byte) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".bbox-snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := time.Now().UTC().Format("20060102-150405") + ".json"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func init() {
	exportCmd.Flags().StringVarP(&exportFile, "file", "o", "", "write to file instead of stdout")
	exportCmd.Flags().StringVar(&exportDiffPath, "diff", "", "compare current state against this saved JSON export and print a semantic diff")
	exportCmd.Flags().BoolVar(&exportSnapshot, "snapshot", false, "write export to ~/.bbox-snapshots/YYYYMMDD-HHMMSS.json and print the path (drift-tracking friendly)")
	rootCmd.AddCommand(statusCmd, infoCmd, wanIPCmd, logClearCmd, statsCmd, exportCmd)
}

func firstNonEmpty(v any, def string) any {
	if v == nil {
		return def
	}
	if s, ok := v.(string); ok && s == "" {
		return def
	}
	return v
}
