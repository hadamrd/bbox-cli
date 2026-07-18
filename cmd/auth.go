package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/hadamrd/bbox-cli/internal/client"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in with password from --password-file / $BBOX_PASSWORD / ~/.bbox-password",
	RunE: func(cmd *cobra.Command, args []string) error {
		pw, err := getPassword()
		if err != nil {
			return err
		}
		if err := c().Login(pw); err != nil {
			return err
		}
		fmt.Println("OK:", client.SessionFile())
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and delete cached session",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		if err := c().Logout(); err != nil {
			return err
		}
		fmt.Println("OK: logged out")
		return nil
	},
}

var (
	siBBoxID string
	siBtoken string
)

var sessionImportCmd = &cobra.Command{
	Use:   "session-import",
	Short: "bootstrap session from Chrome DevTools cookies",
	Long: `Escape hatch for when the Bbox login endpoint is rate-limited (every failed
retry EXTENDS the lockout). Log in via the router web UI in Chrome, copy the
BBOX_ID cookie from DevTools > Application > Cookies, and paste it here. The
CLI writes it to ~/.bbox-session.json so subsequent commands see a valid
session without hitting /api/v1/login again.`,
	Example: `  # Bare minimum: paste the BBOX_ID cookie from Chrome DevTools
  bbox session-import --bbox-id 1a2b3c4d5e6f...

  # Optionally include a btoken cookie (needed on some firmwares for writes)
  bbox session-import --bbox-id 1a2b... --btoken 0011...

  # Verify
  bbox status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		expires := time.Now().UTC().Add(8 * time.Hour).Unix()
		entries := map[string]map[string]any{
			"BBOX_ID": {"value": siBBoxID, "expires": expires},
		}
		if siBtoken != "" {
			entries["btoken"] = map[string]any{"value": siBtoken, "expires": expires}
		}
		data, _ := json.Marshal(entries)
		if err := os.WriteFile(client.SessionFile(), data, 0600); err != nil {
			return err
		}
		if runtime.GOOS != "windows" {
			_ = os.Chmod(client.SessionFile(), 0600)
		}
		fmt.Printf("OK: wrote %s — try `bbox status` to verify.\n", client.SessionFile())
		return nil
	},
}

func init() {
	sessionImportCmd.Flags().StringVar(&siBBoxID, "bbox-id", "", "BBOX_ID cookie value from Chrome DevTools")
	sessionImportCmd.Flags().StringVar(&siBtoken, "btoken", "", "optional btoken cookie value")
	_ = sessionImportCmd.MarkFlagRequired("bbox-id")
	rootCmd.AddCommand(loginCmd, logoutCmd, sessionImportCmd)
}
