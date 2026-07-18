// Package cmd contains the cobra commands for the bbox CLI.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Hexalgo/bbox-cli/internal/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	verbose      bool
	jsonOut      bool
	passwordFile string

	// bx is the shared Client instance. Constructed on Execute.
	bx *client.Client
)

var rootCmd = &cobra.Command{
	Use:   "bbox",
	Short: "CLI for Bouygues Bbox admin (reversed from mabbox.bytel.fr on 2026-07-17).",
	Long: `bbox — CLI for Bouygues Bbox admin (reversed from mabbox.bytel.fr on 2026-07-17).

Auth:
    password from --password-file, $BBOX_PASSWORD, or ~/.bbox-password.
    Session cached at ~/.bbox-session.json (auto-refreshed).

Bouygues MAP-T quirk: with 'cgnatenable=0 maptenable=1', your router only owns a
specific WAN port RANGE (e.g. 40960:49151). Port-forwards on ports outside that
range silently drop. 'bbox info' prints yours. 'bbox nat add' refuses outside-range
ports unless --skip-port-check.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute is the entry point called by main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print HTTP calls to stderr")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "emit JSON for read commands (scriptable)")
	rootCmd.PersistentFlags().StringVar(&passwordFile, "password-file", "",
		fmt.Sprintf("path to password file (default: env BBOX_PASSWORD, then %s)", client.PasswordFileDefault()))

	_ = viper.BindPFlag("password-file", rootCmd.PersistentFlags().Lookup("password-file"))
	viper.SetEnvPrefix("BBOX")
	viper.AutomaticEnv()
}

func initConfig() {
	// Nothing config-file-based; viper is used only for env/flag layering.
}

// clientOnce builds bx lazily so --help doesn't touch the network.
func c() *client.Client {
	if bx == nil {
		bx = client.New(verbose)
	}
	return bx
}

// getPassword mirrors the Python get_password: --password-file → $BBOX_PASSWORD → ~/.bbox-password.
func getPassword() (string, error) {
	if passwordFile != "" {
		data, err := os.ReadFile(passwordFile)
		if err != nil {
			return "", fmt.Errorf("read password file: %w", err)
		}
		s := strings.TrimSpace(string(data))
		if s == "" {
			return "", fmt.Errorf("password file %s is empty", passwordFile)
		}
		return s, nil
	}
	if env := os.Getenv("BBOX_PASSWORD"); env != "" {
		return strings.TrimSpace(env), nil
	}
	def := client.PasswordFileDefault()
	if data, err := os.ReadFile(def); err == nil {
		return strings.TrimSpace(string(data)), nil
	}
	return "", fmt.Errorf("no password. Set BBOX_PASSWORD or --password-file, or create %s", def)
}

func ensureAuth() error {
	return c().EnsureAuth(getPassword)
}

// ─── output helpers ────────────────────────────────────────────────────────

// emit prints the given payload as JSON if --json is set. Returns true iff it did.
func emit(v any) bool {
	if !jsonOut {
		return false
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
	return true
}

// fmtBool mirrors the Python `fmt_bool`.
func fmtBool(v any) string {
	switch x := v.(type) {
	case bool:
		if x {
			return "on"
		}
		return "off"
	case float64:
		if x != 0 {
			return "on"
		}
		return "off"
	case int:
		if x != 0 {
			return "on"
		}
		return "off"
	case string:
		switch strings.ToLower(x) {
		case "1", "on", "up":
			return "on"
		case "0", "off", "down":
			return "off"
		}
		return x
	case nil:
		return "off"
	}
	return fmt.Sprintf("%v", v)
}

// printKV mirrors the Python `print_kv`.
func printKV(pairs [][2]any) {
	if len(pairs) == 0 {
		return
	}
	w := 0
	for _, p := range pairs {
		k := fmt.Sprintf("%v", p[0])
		if len(k) > w {
			w = len(k)
		}
	}
	for _, p := range pairs {
		k := fmt.Sprintf("%v", p[0])
		fmt.Printf("  %-*s  %v\n", w, k, p[1])
	}
}

// toStr converts any to a display string (empty for nil).
func toStr(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// truncate returns s truncated to n runes.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// getMap returns m[k] as map or empty.
func getMap(m map[string]any, k string) map[string]any {
	if v, ok := m[k].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}
