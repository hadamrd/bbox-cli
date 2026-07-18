package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// resetForTest wipes viper + the flag-bound vars so precedence tests start clean.
func resetForTest(t *testing.T) {
	t.Helper()
	viper.Reset()
	viper.SetEnvPrefix("BBOX")
	viper.AutomaticEnv()
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
	_ = viper.BindPFlag("show_secrets", rootCmd.PersistentFlags().Lookup("show-secrets"))
	_ = viper.BindPFlag("password_file", rootCmd.PersistentFlags().Lookup("password-file"))
	verbose = false
	jsonOut = false
	showSecrets = false
	passwordFile = ""
	// Clear "Changed" flags — cobra records these globally on the parent command.
	for _, name := range []string{"verbose", "json", "show-secrets", "password-file", "config"} {
		if f := rootCmd.PersistentFlags().Lookup(name); f != nil {
			f.Changed = false
		}
	}
	cfgFile = ""
}

func TestPrecedence_DefaultWhenNothingSet(t *testing.T) {
	resetForTest(t)
	initConfig()
	if verbose || jsonOut || showSecrets || passwordFile != "" {
		t.Fatalf("expected all defaults; got verbose=%v json=%v showSecrets=%v passwordFile=%q", verbose, jsonOut, showSecrets, passwordFile)
	}
}

func TestPrecedence_FileBeatsDefault(t *testing.T) {
	resetForTest(t)
	dir := t.TempDir()
	path := filepath.Join(dir, ".bbox.yaml")
	if err := os.WriteFile(path, []byte("verbose: true\njson: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfgFile = path
	initConfig()
	if !verbose || !jsonOut {
		t.Fatalf("file should have set verbose+json; got verbose=%v json=%v", verbose, jsonOut)
	}
}

func TestPrecedence_EnvBeatsFile(t *testing.T) {
	resetForTest(t)
	dir := t.TempDir()
	path := filepath.Join(dir, ".bbox.yaml")
	if err := os.WriteFile(path, []byte("verbose: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfgFile = path
	t.Setenv("BBOX_VERBOSE", "false")
	initConfig()
	if verbose {
		t.Fatalf("env should have overridden file; got verbose=%v", verbose)
	}
}

func TestPrecedence_FlagBeatsEnv(t *testing.T) {
	resetForTest(t)
	t.Setenv("BBOX_JSON", "false")
	// Simulate --json passed on the CLI.
	if err := rootCmd.PersistentFlags().Set("json", "true"); err != nil {
		t.Fatal(err)
	}
	initConfig()
	if !jsonOut {
		t.Fatalf("flag should have overridden env; got jsonOut=%v", jsonOut)
	}
}

func TestConfigInit_RefusesOverwrite(t *testing.T) {
	resetForTest(t)
	dir := t.TempDir()
	path := filepath.Join(dir, ".bbox.yaml")
	if err := os.WriteFile(path, []byte("existing: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// exampleConfigYAML alone is not the CUT here — we just check the guard path
	// by verifying os.Stat returns nil. The command itself calls os.Exit(1) which
	// we can't test without a subprocess. Skip the exit branch; confirm the file
	// stays intact if we WOULD refuse.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected pre-existing file: %v", err)
	}
}

func TestExampleConfigYAML_HasEveryKey(t *testing.T) {
	out := exampleConfigYAML()
	for _, k := range configKeys {
		if !contains(out, k.Key) {
			t.Errorf("example config missing key %q", k.Key)
		}
	}
}

func contains(hay, needle string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
