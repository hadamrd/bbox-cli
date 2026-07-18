package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var hostCmd = &cobra.Command{Use: "host", Short: "LAN hosts"}

var (
	hostListActiveOnly bool
	hostListSearch     string
)
var hostListCmd = &cobra.Command{
	Use: "list", Short: "List LAN hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		hosts, err := c().Hosts()
		if err != nil {
			return err
		}
		var filtered []any
		for _, hAny := range hosts {
			h, _ := hAny.(map[string]any)
			if hostListActiveOnly && !toBoolAny(h["active"]) {
				continue
			}
			if hostListSearch != "" {
				s := strings.ToLower(hostListSearch)
				joined := strings.ToLower(toStr(h["hostname"]) + toStr(h["ipaddress"]) + toStr(h["macaddress"]))
				if !strings.Contains(joined, s) {
					continue
				}
			}
			filtered = append(filtered, h)
		}
		if emit(filtered) {
			return nil
		}
		fmt.Printf("%-4s %-3s %-25s %-15s %-19s %-10s\n", "ID", "act", "hostname", "ip", "mac", "link")
		for _, hAny := range filtered {
			h, _ := hAny.(map[string]any)
			act := "-"
			if toBoolAny(h["active"]) {
				act = "*"
			}
			hn := toStr(h["hostname"])
			if hn == "" {
				hn = "?"
			}
			hn = truncate(hn, 25)
			fmt.Printf("%-4v %-3s %-25s %-15v %-19v %-10v\n",
				firstOr(h["id"], "?"), act, hn,
				firstOr(h["ipaddress"], "?"), firstOr(h["macaddress"], "?"), firstOr(h["link"], "?"))
		}
		return nil
	},
}

var hostMeCmd = &cobra.Command{
	Use: "me", Short: "Show this machine's LAN entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		h, err := c().HostMe()
		if err != nil {
			return err
		}
		if emit(h) {
			return nil
		}
		pr := toStr(h["portrange"])
		if pr == "" {
			pr = "(none reserved)"
		}
		printKV([][2]any{
			{"id", h["id"]},
			{"hostname", h["hostname"]},
			{"ip", h["ipaddress"]},
			{"mac", h["macaddress"]},
			{"link", h["link"]},
			{"port range", pr},
		})
		return nil
	},
}

var hostRenameCmd = &cobra.Command{
	Use: "rename HOST NEW_NAME", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		h, err := c().HostBy(args[0])
		if err != nil {
			return err
		}
		id := toIntAny(h["id"])
		if err := c().HostRename(id, args[1]); err != nil {
			return err
		}
		fmt.Printf("OK: host id=%d renamed '%v' -> '%s'\n", id, h["hostname"], args[1])
		return nil
	},
}

var hostBlockCmd = &cobra.Command{
	Use: "block HOST", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		h, err := c().HostBy(args[0])
		if err != nil {
			return err
		}
		if err := c().HostBlock(toIntAny(h["id"])); err != nil {
			return err
		}
		fmt.Printf("OK: blocked %v (%v)\n", h["hostname"], h["macaddress"])
		return nil
	},
}

var hostUnblockCmd = &cobra.Command{
	Use: "unblock HOST", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		h, err := c().HostBy(args[0])
		if err != nil {
			return err
		}
		if err := c().HostUnblock(toIntAny(h["id"])); err != nil {
			return err
		}
		fmt.Printf("OK: unblocked %v (%v)\n", h["hostname"], h["macaddress"])
		return nil
	},
}

func init() {
	hostListCmd.Flags().BoolVar(&hostListActiveOnly, "active-only", false, "only show active hosts")
	hostListCmd.Flags().StringVarP(&hostListSearch, "search", "s", "", "filter by name/ip/mac substring")
	hostCmd.AddCommand(hostListCmd, hostMeCmd, hostRenameCmd, hostBlockCmd, hostUnblockCmd)
	rootCmd.AddCommand(hostCmd)
}
