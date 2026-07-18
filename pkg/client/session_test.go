package client

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

// setHome makes SessionFile() point at a temp dir on both Windows and POSIX.
func setHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	} else {
		t.Setenv("HOME", dir)
	}
	return dir
}

// TestSessionEntryRoundTrip pins the on-disk shape shared with the Python CLI:
// Expires is a *float64 (seconds since epoch), preserving sub-second precision.
func TestSessionEntryRoundTrip(t *testing.T) {
	exp := 1784396386.123
	in := map[string]sessionEntry{
		"BBOX_ID": {Value: "abc123", Expires: &exp},
		"NO_EXP":  {Value: "session-only", Expires: nil},
	}
	blob, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]sessionEntry
	if err := json.Unmarshal(blob, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch\nin=%#v\nout=%#v", in, out)
	}
	if *out["BBOX_ID"].Expires != exp {
		t.Fatalf("precision lost: got %v want %v", *out["BBOX_ID"].Expires, exp)
	}
}

// TestLoadSessionPythonFixture: a session file written by the Python CLI must
// restore into the Go cookie jar.
func TestLoadSessionPythonFixture(t *testing.T) {
	home := setHome(t)
	// Expiry well into the future.
	future := float64(time.Now().Add(24 * time.Hour).Unix())
	fixture := map[string]sessionEntry{
		"BBOX_ID": {Value: "python-cookie", Expires: &future},
	}
	blob, _ := json.Marshal(fixture)
	if err := os.WriteFile(filepath.Join(home, ".bbox-session.json"), blob, 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	c := New(false, 0, 0)
	if !c.LoadSession() {
		t.Fatal("LoadSession returned false for valid Python-format fixture")
	}
	u, _ := url.Parse(BaseURL)
	cookies := c.jar.Cookies(u)
	found := false
	for _, ck := range cookies {
		if ck.Name == "BBOX_ID" && ck.Value == "python-cookie" {
			found = true
		}
	}
	if !found {
		t.Fatalf("cookie not restored into jar; got %+v", cookies)
	}
}

// TestLoadSessionExpired: any past-expiry cookie invalidates the whole file.
func TestLoadSessionExpired(t *testing.T) {
	home := setHome(t)
	past := float64(time.Now().Add(-1 * time.Hour).Unix())
	fixture := map[string]sessionEntry{
		"BBOX_ID": {Value: "stale", Expires: &past},
	}
	blob, _ := json.Marshal(fixture)
	if err := os.WriteFile(filepath.Join(home, ".bbox-session.json"), blob, 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	c := New(false, 0, 0)
	if c.LoadSession() {
		t.Fatal("LoadSession returned true for expired session; expected false")
	}
}
