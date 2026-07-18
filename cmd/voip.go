package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var voipCmd = &cobra.Command{Use: "voip", Short: "VoIP phone status"}

var voipShowCmd = &cobra.Command{
	Use: "show",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		v, err := c().VoIP()
		if err != nil {
			return err
		}
		if emit(v) {
			return nil
		}
		for _, lineAny := range v {
			line, _ := lineAny.(map[string]any)
			printKV([][2]any{
				{"uri", line["uri"]},
				{"status", line["status"]},
				{"callstate", line["callstate"]},
				{"blockanon", line["anoncallstate"]},
				{"mwi", line["mwi"]},
				{"msgs", line["message_count"]},
			})
			fmt.Println()
		}
		return nil
	},
}

func init() {
	voipCmd.AddCommand(voipShowCmd)
	rootCmd.AddCommand(voipCmd)
}
