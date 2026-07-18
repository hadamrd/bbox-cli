package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var dhcpCmd = &cobra.Command{Use: "dhcp", Short: "DHCP"}

var dhcpShowCmd = &cobra.Command{
	Use: "show",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		d, err := c().DHCP()
		if err != nil {
			return err
		}
		if emit(d) {
			return nil
		}
		printKV([][2]any{
			{"enable", fmtBool(d["enable"])},
			{"min", d["minaddress"]},
			{"max", d["maxaddress"]},
			{"lease", fmt.Sprintf("%vs", d["leasetime"])},
		})
		return nil
	},
}

var dhcpLeasesCmd = &cobra.Command{
	Use: "leases",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		c1, _ := c().DHCPClients()
		if emit(c1) {
			return nil
		}
		b, _ := json.MarshalIndent(c1, "", "  ")
		fmt.Println(string(b))
		return nil
	},
}

var dhcpReserveHostname string
var dhcpReserveCmd = &cobra.Command{
	Use: "reserve MAC IP", Args: cobra.ExactArgs(2), Short: "Reserve a DHCP lease",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().DHCPReserve(args[0], args[1], dhcpReserveHostname); err != nil {
			return err
		}
		fmt.Printf("OK: reserved %s for %s\n", args[1], args[0])
		return nil
	},
}

func init() {
	dhcpReserveCmd.Flags().StringVar(&dhcpReserveHostname, "hostname", "", "optional hostname")
	dhcpCmd.AddCommand(dhcpShowCmd, dhcpLeasesCmd, dhcpReserveCmd)
	rootCmd.AddCommand(dhcpCmd)
}
