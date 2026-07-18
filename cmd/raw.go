package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	rawBody           string
	rawIncludeHeaders bool
	rawSkipBtoken     bool
)

var rawCmd = &cobra.Command{
	Use:   "raw METHOD PATH",
	Args:  cobra.ExactArgs(2),
	Short: "direct API call (debugging)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		method, path := strings.ToUpper(args[0]), args[1]
		// Undo Git-Bash / MSYS mangling: '/Program Files/Git/api/...' -> '/api/...'.
		if strings.Contains(path, "/api/") && !strings.HasPrefix(path, "/api/") {
			path = "/api/" + strings.SplitN(path, "/api/", 2)[1]
		}
		if !rawSkipBtoken && (method == "POST" || method == "PUT" || method == "DELETE" || method == "PATCH") {
			tok, err := c().DeviceToken()
			if err != nil {
				return err
			}
			sep := "?"
			if strings.Contains(path, "?") {
				sep = "&"
			}
			path = path + sep + "btoken=" + url.QueryEscape(tok)
		}
		code, data, hdrs, err := c().Raw(method, path, rawBody)
		if err != nil {
			return err
		}
		if rawIncludeHeaders {
			fmt.Printf("HTTP %d\n", code)
			for k, vs := range hdrs {
				for _, v := range vs {
					fmt.Printf("%s: %s\n", k, v)
				}
			}
			fmt.Println()
		}
		var parsed any
		if err := json.Unmarshal(data, &parsed); err == nil {
			b, _ := json.MarshalIndent(parsed, "", "  ")
			fmt.Println(string(b))
		} else {
			_, _ = os.Stdout.Write(data)
		}
		if code < 200 || code >= 300 {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rawCmd.Flags().StringVar(&rawBody, "body", "", "form-urlencoded body (e.g. 'enable=1&x=y')")
	rawCmd.Flags().BoolVarP(&rawIncludeHeaders, "include-headers", "i", false, "print response headers")
	rawCmd.Flags().BoolVar(&rawSkipBtoken, "skip-btoken", false, "don't auto-append ?btoken=… on write methods")
	rootCmd.AddCommand(rawCmd)
}
