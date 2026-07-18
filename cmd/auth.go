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
	siVerify bool
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

		if siBtoken == "" {
			fmt.Fprintln(os.Stderr, "warning: no --btoken provided. This session will work for read commands")
			fmt.Fprintln(os.Stderr, "(status, info, host list, ...) but write commands (nat add, wifi key,")
			fmt.Fprintln(os.Stderr, "reboot, ...) will fail with HTTP 401. To fix, re-import with both:")
			fmt.Fprintln(os.Stderr, "  bbox session-import --bbox-id <BBOX_ID> --btoken <btoken>")
			fmt.Fprintln(os.Stderr, "Both cookies are visible in Chrome DevTools → Application → Cookies →")
			fmt.Fprintln(os.Stderr, "https://mabbox.bytel.fr.")
		}

		if siVerify {
			tok, err := c().DeviceToken()
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: device-token verify failed: %v\n", err)
				fmt.Fprintln(os.Stderr, "         writes will likely 401. Re-import with a fresh --btoken.")
			} else if tok == "" {
				fmt.Fprintln(os.Stderr, "warning: device-token endpoint returned empty token")
			} else {
				if siBtoken != "" {
					fmt.Println("verify: device-token OK — writes should work.")
				} else {
					// btoken empty but token endpoint accepted us: unusual, warn anyway.
					fmt.Fprintln(os.Stderr, "warning: btoken cookie may be stale")
				}
			}
		}
		return nil
	},
}

func init() {
	sessionImportCmd.Flags().StringVar(&siBBoxID, "bbox-id", "", "BBOX_ID cookie value from Chrome DevTools")
	sessionImportCmd.Flags().StringVar(&siBtoken, "btoken", "", "optional btoken cookie value (required for writes)")
	sessionImportCmd.Flags().BoolVar(&siVerify, "verify", false, "after writing, probe /api/v1/device/token to confirm writes will work")
	_ = sessionImportCmd.MarkFlagRequired("bbox-id")
	rootCmd.AddCommand(loginCmd, logoutCmd, sessionImportCmd)
}
