package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hadamrd/bbox-cli/internal/client"
)

// mockBboxServer returns an httptest TLS server mimicking a subset of the
// mabbox.bytel.fr admin API — just enough to exercise the read/write happy
// paths for status, wan-ip, host list, nat list, nat add.
func mockBboxServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "BBOX_ID", Value: "fake-cookie", Path: "/", HttpOnly: true})
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/v1/device", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []any{map[string]any{
			"device": map[string]any{
				"modelname":     "Bbox Miami",
				"serialnumber":  "AB12345678",
				"main":          map[string]any{"version": "20.5.9"},
				"numberofboots": float64(42),
				"now":           "2026-07-18T10:00:00Z",
				"uptime":        float64(86400),
			},
		}})
	})
	mux.HandleFunc("/api/v1/wan/ip", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []any{map[string]any{
			"wan": map[string]any{
				"ip": map[string]any{
					"address":     "1.2.3.4",
					"state":       "Up",
					"mac":         "aa:bb:cc:dd:ee:ff",
					"portrange":   "40960:49151",
					"maptenable":  float64(1),
					"cgnatenable": float64(0),
				},
			},
		}})
	})
	mux.HandleFunc("/api/v1/services", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []any{map[string]any{
			"services": map[string]any{
				"nat": map[string]any{"enable": float64(1), "status": "Up", "nbrules": float64(1)},
			},
		}})
	})
	mux.HandleFunc("/api/v1/hosts", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []any{map[string]any{
			"hosts": map[string]any{
				"list": []any{
					map[string]any{"id": float64(1), "hostname": "DESKTOP-1", "ipaddress": "192.168.1.10", "macaddress": "11:22:33:44:55:66", "link": "Wifi 5", "active": true},
					map[string]any{"id": float64(2), "hostname": "phone", "ipaddress": "192.168.1.20", "macaddress": "77:88:99:aa:bb:cc", "link": "Wifi 2.4", "active": false},
				},
			},
		}})
	})
	mux.HandleFunc("/api/v1/device/token", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []any{map[string]any{
			"device": map[string]any{
				"token":   "fake-token",
				"expires": time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339),
			},
		}})
	})
	mux.HandleFunc("/api/v1/firewall/rules", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []any{map[string]any{
			"firewall": map[string]any{"rules": []any{}},
		}})
	})
	mux.HandleFunc("/api/v1/dhcp/clients", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []any{map[string]any{"clients": []any{}}})
	})
	mux.HandleFunc("/api/v1/nat/rules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, []any{map[string]any{
				"nat": map[string]any{
					"enable": float64(1),
					"rules": []any{
						map[string]any{
							"id": float64(1), "description": "retrobot-socks5", "protocol": "tcp",
							"externalport": float64(45080), "internalip": "192.168.1.10", "internalport": float64(45080),
						},
					},
				},
			}})
		case http.MethodPost:
			w.Header().Set("Location", "/api/v1/nat/rules/2")
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})

	return httptest.NewTLSServer(mux)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(v)
	_, _ = w.Write(b)
}

// setupIntegration points BaseURL at the mock server, isolates HOME so the
// session file lives in a temp dir, resets the shared client, and returns a
// cleanup hook.
func setupIntegration(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	srv := mockBboxServer(t)

	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	} else {
		t.Setenv("HOME", dir)
	}
	// Password source: seed a file the default password-getter reads.
	pwPath := filepath.Join(dir, ".bbox-password")
	if err := os.WriteFile(pwPath, []byte("test-password"), 0600); err != nil {
		t.Fatalf("write password: %v", err)
	}

	prevURL := client.BaseURL
	client.BaseURL = srv.URL
	bx = nil // reset the cached shared client so it picks up the new URL

	cleanup := func() {
		srv.Close()
		client.BaseURL = prevURL
		bx = nil
	}
	return srv, cleanup
}

// captureStdout redirects os.Stdout, runs fn, returns whatever fn printed.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := fn()
	_ = w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), err
}

func TestIntegration_Status(t *testing.T) {
	_, cleanup := setupIntegration(t)
	defer cleanup()
	out, err := captureStdout(t, func() error { return statusCmd.RunE(statusCmd, nil) })
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "1.2.3.4") {
		t.Errorf("expected 1.2.3.4 in status output, got:\n%s", out)
	}
	if !strings.Contains(out, "Bbox Miami") {
		t.Errorf("expected model in output, got:\n%s", out)
	}
}

func TestIntegration_WANIP(t *testing.T) {
	_, cleanup := setupIntegration(t)
	defer cleanup()
	out, err := captureStdout(t, func() error { return wanIPCmd.RunE(wanIPCmd, nil) })
	if err != nil {
		t.Fatalf("wan-ip: %v", err)
	}
	if strings.TrimSpace(out) != "1.2.3.4" {
		t.Errorf("expected exactly '1.2.3.4', got %q", out)
	}
}

func TestIntegration_HostList(t *testing.T) {
	_, cleanup := setupIntegration(t)
	defer cleanup()
	out, err := captureStdout(t, func() error { return hostListCmd.RunE(hostListCmd, nil) })
	if err != nil {
		t.Fatalf("host list: %v", err)
	}
	if !strings.Contains(out, "DESKTOP-1") || !strings.Contains(out, "phone") {
		t.Errorf("expected both hosts in output, got:\n%s", out)
	}
}

func TestIntegration_NATList(t *testing.T) {
	_, cleanup := setupIntegration(t)
	defer cleanup()
	out, err := captureStdout(t, func() error { return natListCmd.RunE(natListCmd, nil) })
	if err != nil {
		t.Fatalf("nat list: %v", err)
	}
	if !strings.Contains(out, "retrobot-socks5") {
		t.Errorf("expected NAT rule in output, got:\n%s", out)
	}
	if !strings.Contains(out, "1 rule") {
		t.Errorf("expected '1 rule' count in output, got:\n%s", out)
	}
}

func TestIntegration_NATAdd(t *testing.T) {
	_, cleanup := setupIntegration(t)
	defer cleanup()
	// Skip the MAP-T check so we don't have to model the exact port math.
	prev := natAddSkipPortCheck
	natAddSkipPortCheck = true
	defer func() { natAddSkipPortCheck = prev }()

	out, err := captureStdout(t, func() error {
		return natAddCmd.RunE(natAddCmd, []string{"testrule", "45081", "192.168.1.99"})
	})
	if err != nil {
		t.Fatalf("nat add: %v", err)
	}
	if !strings.Contains(out, "id=2") {
		t.Errorf("expected new rule id=2 from Location header, got:\n%s", out)
	}
}

// TestIntegration_Lookup exercises the new lookup command against the mock.
func TestIntegration_Lookup(t *testing.T) {
	_, cleanup := setupIntegration(t)
	defer cleanup()
	res := runLookup("DESKTOP", true)
	if len(res.Hosts) != 1 {
		t.Fatalf("expected 1 host hit, got %d", len(res.Hosts))
	}
	if fmt.Sprint(res.Hosts[0]["hostname"]) != "DESKTOP-1" {
		t.Errorf("unexpected host: %v", res.Hosts[0])
	}
}
