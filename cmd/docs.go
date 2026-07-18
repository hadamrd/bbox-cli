package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docsCmd = &cobra.Command{
	Use:    "docs",
	Short:  "Generate man pages / markdown docs (maintainer tool)",
	Hidden: true,
}

var docsManCmd = &cobra.Command{
	Use:   "man DIR",
	Short: "Generate groff man pages, one per command node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := args[0]
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		header := &doc.GenManHeader{Title: "BBOX", Section: "1"}
		if err := doc.GenManTree(rootCmd, header, dir); err != nil {
			return err
		}
		fmt.Printf("OK: man pages written to %s\n", dir)
		return nil
	},
}

var docsMDCmd = &cobra.Command{
	Use:   "md DIR",
	Short: "Generate Markdown docs, one per command node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := args[0]
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := doc.GenMarkdownTree(rootCmd, dir); err != nil {
			return err
		}
		fmt.Printf("OK: markdown docs written to %s\n", dir)
		return nil
	},
}

func init() {
	docsCmd.AddCommand(docsManCmd, docsMDCmd)
	rootCmd.AddCommand(docsCmd)
}
