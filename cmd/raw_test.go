package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hadamrd/bbox-cli/pkg/client"
)

// rawTestServer returns a TLS server that answers login (200), device (200),
// /api/v1/ok (200) and /api/v1/missing (404).
func rawTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "BBOX_ID", Value: "x", Path: "/"})
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/v1/device", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"device":{"modelname":"Bbox Miami"}}]`))
	})
	mux.HandleFunc("/api/v1/ok", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hello":"world"}`))
	})
	mux.HandleFunc("/api/v1/missing", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"exception":"not found"}`, http.StatusNotFound)
	})
	return httptest.NewTLSServer(mux)
}

func setupRawTest(t *testing.T) func() {
	t.Helper()
	srv := rawTestServer(t)

	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	} else {
		t.Setenv("HOME", dir)
	}
	if err := os.WriteFile(filepath.Join(dir, ".bbox-password"), []byte("pw"), 0600); err != nil {
		t.Fatal(err)
	}
	prev := client.BaseURL
	client.BaseURL = srv.URL
	bx = nil
	return func() {
		srv.Close()
		client.BaseURL = prev
		bx = nil
	}
}

func TestRaw_2xxReturnsNil(t *testing.T) {
	defer setupRawTest(t)()
	out, err := captureStdout(t, func() error {
		return rawCmd.RunE(rawCmd, []string{"GET", "/api/v1/ok"})
	})
	if err != nil {
		t.Fatalf("expected nil error on 200, got %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected body in stdout, got %q", out)
	}
}

func TestRaw_404ReturnsError(t *testing.T) {
	defer setupRawTest(t)()
	_, err := captureStdout(t, func() error {
		return rawCmd.RunE(rawCmd, []string{"GET", "/api/v1/missing"})
	})
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected HTTP 404 in error message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "raw request failed") {
		t.Errorf("expected 'raw request failed' prefix, got %q", err.Error())
	}
}
