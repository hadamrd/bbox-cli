package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var voipCmd = &cobra.Command{Use: "voip", Short: "VoIP phone (status + anti-spam)"}

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
				{"line", line["id"]},
				{"uri", line["uri"]},
				{"status", line["status"]},
				{"callstate", line["callstate"]},
				{"block_anon", fmtBool(line["blockstate"])},
				{"mwi", line["mwi"]},
				{"msgs", line["message_count"]},
			})
			fmt.Println()
		}
		return nil
	},
}

var voipBlockAnonCmd = &cobra.Command{
	Use: "block-anon LINE STATE", Args: cobra.ExactArgs(2),
	Short: "Block/allow anonymous (withheld-number) calls on a line (on|off)",
	Long: `Toggle blocking of anonymous / caller-ID-withheld calls on a phone line
(1 or 2) — a simple anti-spam-call measure.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		line, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("LINE must be 1 or 2")
		}
		on := args[1] == "on"
		if err := c().VoIPBlockAnonymous(line, on); err != nil {
			return err
		}
		fmt.Printf("OK: line %d anonymous-call blocking %s\n", line, strings.ToUpper(args[1]))
		return nil
	},
}

func init() {
	voipCmd.AddCommand(voipShowCmd, voipBlockAnonCmd)
	rootCmd.AddCommand(voipCmd)
}
