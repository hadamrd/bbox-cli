package client

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestGetAutoRefreshesOn401: the server 401s once, then 200s after "login".
// Verifies the transparent retry via WithPasswordGetter.
func TestGetAutoRefreshesOn401(t *testing.T) {
	var loginHits, dataHits int32
	var loggedIn atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/login" {
			atomic.AddInt32(&loginHits, 1)
			loggedIn.Store(true)
			w.WriteHeader(200)
			return
		}
		atomic.AddInt32(&dataHits, 1)
		if !loggedIn.Load() {
			w.WriteHeader(401)
			_, _ = fmt.Fprint(w, `{"exception":{"code":"unauthorized"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := stubClient(t, srv.URL, 0)
	c.WithPasswordGetter(func() (string, error) { return "pw", nil })
	// Simulate a loaded-but-expired session: the flag is what gates refresh.
	sessionWasLoaded = true
	defer func() { sessionWasLoaded = false }()

	got, err := c.Get("/api/v1/device")
	if err != nil {
		t.Fatalf("Get returned err: %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok || m["ok"] != true {
		t.Fatalf("unexpected body: %v", got)
	}
	if n := atomic.LoadInt32(&loginHits); n != 1 {
		t.Errorf("expected 1 login call, got %d", n)
	}
	if n := atomic.LoadInt32(&dataHits); n != 2 {
		t.Errorf("expected 2 data calls (initial 401 + retry 200), got %d", n)
	}
}

// TestGetNoRefreshWhenPasswordGetterNil: with no getter installed, a 401
// surfaces to the caller and no login attempt is made.
func TestGetNoRefreshWhenPasswordGetterNil(t *testing.T) {
	var loginHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/login" {
			atomic.AddInt32(&loginHits, 1)
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(401)
	}))
	defer srv.Close()

	c := stubClient(t, srv.URL, 0)
	// passwordGetter left nil.
	sessionWasLoaded = true
	defer func() { sessionWasLoaded = false }()

	_, err := c.Get("/api/v1/device")
	if err == nil {
		t.Fatal("expected 401 to surface when no passwordGetter is set")
	}
	if n := atomic.LoadInt32(&loginHits); n != 0 {
		t.Errorf("expected 0 login calls without passwordGetter, got %d", n)
	}
}

// TestGetNoRefreshWhenPasswordGetterErrors: if the getter returns an error,
// the original 401 must surface unchanged and no login retry is attempted.
func TestGetNoRefreshWhenPasswordGetterErrors(t *testing.T) {
	var loginHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/login" {
			atomic.AddInt32(&loginHits, 1)
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(401)
	}))
	defer srv.Close()

	c := stubClient(t, srv.URL, 0)
	c.WithPasswordGetter(func() (string, error) { return "", fmt.Errorf("no password source") })
	sessionWasLoaded = true
	defer func() { sessionWasLoaded = false }()

	_, err := c.Get("/api/v1/device")
	if err == nil {
		t.Fatal("expected 401 to surface when passwordGetter errors")
	}
	if n := atomic.LoadInt32(&loginHits); n != 0 {
		t.Errorf("expected Login to be skipped when passwordGetter errors, got %d hits", n)
	}
}

// TestLoginItselfNeverAutoRefreshes: calling Login() must not trigger the
// refresh path even if it returns 401 (would infinite-loop).
func TestLoginItselfNeverAutoRefreshes(t *testing.T) {
	var loginHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&loginHits, 1)
		w.WriteHeader(401)
	}))
	defer srv.Close()

	c := stubClient(t, srv.URL, 0)
	c.WithPasswordGetter(func() (string, error) { return "pw", nil })
	sessionWasLoaded = true
	defer func() { sessionWasLoaded = false }()

	if err := c.Login("pw"); err == nil {
		t.Fatal("expected 401 login error")
	}
	// Exactly 1 login attempt — no auto-retry.
	if n := atomic.LoadInt32(&loginHits); n != 1 {
		t.Errorf("Login must not auto-refresh; got %d hits", n)
	}
}
