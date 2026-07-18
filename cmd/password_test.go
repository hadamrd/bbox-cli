package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// setFakeHome makes client.PasswordFileDefault() point at a temp dir.
func setFakeHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	} else {
		t.Setenv("HOME", dir)
	}
	return dir
}

// resetPasswordState clears both flag-var and env, restoring them after the test.
func resetPasswordState(t *testing.T) {
	t.Helper()
	prev := passwordFile
	t.Cleanup(func() { passwordFile = prev })
	passwordFile = ""
	t.Setenv("BBOX_PASSWORD", "")
}

// TestGetPasswordPrecedence: --password-file > $BBOX_PASSWORD > ~/.bbox-password.
func TestGetPasswordPrecedence(t *testing.T) {
	home := setFakeHome(t)
	resetPasswordState(t)

	// Write ~/.bbox-password
	if err := os.WriteFile(filepath.Join(home, ".bbox-password"), []byte("from-home\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Env
	t.Setenv("BBOX_PASSWORD", "from-env")
	// Flag file
	flagFile := filepath.Join(t.TempDir(), "pw.txt")
	if err := os.WriteFile(flagFile, []byte("from-flag\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Flag wins over env + home.
	passwordFile = flagFile
	got, err := getPassword()
	if err != nil || got != "from-flag" {
		t.Fatalf("flag precedence: got=%q err=%v", got, err)
	}

	// Env wins over home.
	passwordFile = ""
	got, err = getPassword()
	if err != nil || got != "from-env" {
		t.Fatalf("env precedence: got=%q err=%v", got, err)
	}

	// Home fallback.
	t.Setenv("BBOX_PASSWORD", "")
	got, err = getPassword()
	if err != nil || got != "from-home" {
		t.Fatalf("home fallback: got=%q err=%v", got, err)
	}
}

// TestGetPasswordEmptyFlagFile: empty --password-file must be an error, not
// an empty-string password (Python bbox.py silently returned "" — bug we won't mirror).
func TestGetPasswordEmptyFlagFile(t *testing.T) {
	setFakeHome(t)
	resetPasswordState(t)

	empty := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(empty, []byte("   \n"), 0600); err != nil {
		t.Fatal(err)
	}
	passwordFile = empty
	_, err := getPassword()
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected 'empty' error, got %v", err)
	}
}

// TestGetPasswordMissing: nothing set, nothing on disk -> error.
func TestGetPasswordMissing(t *testing.T) {
	setFakeHome(t)
	resetPasswordState(t)
	_, err := getPassword()
	if err == nil {
		t.Fatal("expected error when no password source configured")
	}
}
