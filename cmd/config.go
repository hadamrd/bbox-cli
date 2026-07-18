package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the bbox config file (~/.bbox.yaml)",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write an example config file to ~/.bbox.yaml (or --config PATH)",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := cfgFile
		if path == "" {
			path = defaultConfigPath()
		}
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(os.Stderr, "refusing to overwrite %s — delete it first\n", path)
			os.Exit(1)
		}
		if err := os.WriteFile(path, []byte(exampleConfigYAML()), 0600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Printf("wrote %s\n", path)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the effective config (value + source per key)",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := cfgFile
		if path == "" {
			path = defaultConfigPath()
		}
		fileLoaded := false
		if _, err := os.Stat(path); err == nil {
			fileLoaded = true
		}

		fmt.Printf("config file: %s", path)
		if !fileLoaded {
			fmt.Printf(" (not present)")
		}
		fmt.Println()
		fmt.Println()

		pairs := [][2]any{}
		for _, k := range configKeys {
			flagName := strings.ReplaceAll(k.Key, "_", "-")
			src := "default"
			val := k.Default
			if fileLoaded && viper.InConfig(k.Key) {
				src = "file"
				val = fmt.Sprintf("%v", viper.Get(k.Key))
			}
			if envVal, ok := os.LookupEnv("BBOX_" + strings.ToUpper(k.Key)); ok {
				src = "env"
				val = envVal
			}
			if f := rootCmd.PersistentFlags().Lookup(flagName); f != nil && f.Changed {
				src = "flag"
				val = f.Value.String()
			}
			pairs = append(pairs, [2]any{k.Key, fmt.Sprintf("%-10s  [%s]", val, src)})
		}
		printKV(pairs)
		return nil
	},
}

// exampleConfigYAML returns a well-commented default config document.
func exampleConfigYAML() string {
	var b strings.Builder
	b.WriteString("# bbox-cli config — see `bbox config show` for the effective values.\n")
	b.WriteString("# Precedence: CLI flag > BBOX_* env var > this file > built-in default.\n")
	b.WriteString("#\n")
	for _, k := range configKeys {
		fmt.Fprintf(&b, "# %s (%s) — %s\n", k.Key, k.Type, k.Comment)
		fmt.Fprintf(&b, "# %s: %s\n\n", k.Key, k.Default)
	}
	return b.String()
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}
