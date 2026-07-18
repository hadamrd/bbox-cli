package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version, commit, build date and Go version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(versionLine())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
