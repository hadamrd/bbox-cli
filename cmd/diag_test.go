package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// isolateHome points HOME/USERPROFILE at a temp dir so SessionFile() +
// defaultConfigPath() resolve inside t.TempDir.
func isolateHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	} else {
		t.Setenv("HOME", dir)
	}
	return dir
}

func TestDiag_ConfigFileMissing(t *testing.T) {
	isolateHome(t)
	prev := cfgFile
	cfgFile = ""
	defer func() { cfgFile = prev }()

	c := runDiag()[0]
	if c.Check != "config file present" {
		t.Fatalf("check[0] = %+v", c)
	}
	if c.Status != diagWarn {
		t.Errorf("missing config should be WARN, got %s", c.Status)
	}
}

func TestDiag_ConfigFilePresent(t *testing.T) {
	dir := isolateHome(t)
	path := filepath.Join(dir, ".bbox.yaml")
	if err := os.WriteFile(path, []byte("verbose: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	prev := cfgFile
	cfgFile = ""
	defer func() { cfgFile = prev }()

	c := runDiag()[0]
	if c.Status != diagOK {
		t.Errorf("present config should be OK, got %s (%s)", c.Status, c.Detail)
	}
}

func TestDiag_SessionMissing_CascadesSkip(t *testing.T) {
	isolateHome(t)
	// No session file, no config file. Expect checks 3..10 skipped.
	results := runDiag()
	if len(results) != 10 {
		t.Fatalf("expected 10 checks, got %d", len(results))
	}
	if results[1].Check != "session cached" || results[1].Status != diagFail {
		t.Errorf("session should FAIL when missing, got %+v", results[1])
	}
	for _, r := range results[2:] {
		if r.Status != diagSkip {
			t.Errorf("expected SKIP after missing session, got %s for %q", r.Status, r.Check)
		}
	}
}

func TestDiag_SessionExpired(t *testing.T) {
	dir := isolateHome(t)
	// Write a session file with an expired cookie.
	past := float64(time.Now().Add(-1 * time.Hour).Unix())
	body := map[string]any{"BBOX_ID": map[string]any{"value": "x", "expires": past}}
	b, _ := json.Marshal(body)
	if err := os.WriteFile(filepath.Join(dir, ".bbox-session.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	results := runDiag()
	if results[1].Status != diagFail {
		t.Errorf("expired session should FAIL, got %s (%s)", results[1].Status, results[1].Detail)
	}
}

func TestDiag_SessionValid_LocalOnly(t *testing.T) {
	dir := isolateHome(t)
	// Non-expiring cookie.
	body := map[string]any{"BBOX_ID": map[string]any{"value": "x", "expires": nil}}
	b, _ := json.Marshal(body)
	if err := os.WriteFile(filepath.Join(dir, ".bbox-session.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	results := runDiag()
	if results[1].Status != diagOK {
		t.Errorf("valid session should be OK, got %s (%s)", results[1].Status, results[1].Detail)
	}
	// Router-reachability checks may go FAIL (real router unreachable from CI)
	// or OK — but they must run past the session gate, not skip on session.
	for _, r := range results[2:] {
		if r.Status == diagSkip && r.Detail == "no valid session" {
			t.Errorf("check %q wrongly skipped on session (should skip on network)", r.Check)
		}
	}
}

func TestPadStatus(t *testing.T) {
	if padStatus(diagOK) != " OK " {
		t.Errorf("OK glyph wrong: %q", padStatus(diagOK))
	}
	if padStatus(diagFail) != "FAIL" {
		t.Errorf("FAIL glyph wrong: %q", padStatus(diagFail))
	}
}

func TestHostPort(t *testing.T) {
	h, p := hostPort("https://mabbox.bytel.fr")
	if h != "mabbox.bytel.fr" || p != "443" {
		t.Errorf("default https: got %s:%s", h, p)
	}
	h, p = hostPort("http://192.168.1.1:8080/x")
	if h != "192.168.1.1" || p != "8080" {
		t.Errorf("explicit port: got %s:%s", h, p)
	}
}
