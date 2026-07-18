package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var hibernateCmd = &cobra.Command{Use: "hibernate", Short: "Router power scheduler"}

var hibernateShowCmd = &cobra.Command{
	Use: "show",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		h, _ := c().Hibernate()
		if emit(h) {
			return nil
		}
		b, _ := json.MarshalIndent(h, "", "  ")
		fmt.Println(string(b))
		return nil
	},
}

var hibernateOffCmd = &cobra.Command{
	Use: "off",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().HibernateOff(); err != nil {
			return err
		}
		fmt.Println("OK: hibernation scheduler off")
		return nil
	},
}

func init() {
	hibernateCmd.AddCommand(hibernateShowCmd, hibernateOffCmd)
	rootCmd.AddCommand(hibernateCmd)
}
