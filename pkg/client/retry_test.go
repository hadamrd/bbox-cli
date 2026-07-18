package client

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// stubClient returns a Client wired to the given test-server URL, skipping
// TLS validation and neutralizing the exponential backoff.
func stubClient(t *testing.T, base string, retries int) *Client {
	t.Helper()
	c := New(true, retries, 5*time.Second)
	// Redirect BaseURL at the transport layer by replacing http.Client Transport.
	// Simpler: rewrite req() by installing a RoundTripper that swaps host.
	target, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = &rewriteTransport{
		base: target,
		inner: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	// Fast retries — no real sleep.
	c.sleep = func(time.Duration) {}
	return c
}

// rewriteTransport swaps every outbound URL's scheme+host to the test server.
type rewriteTransport struct {
	base  *url.URL
	inner http.RoundTripper
}

func (r *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = r.base.Scheme
	req.URL.Host = r.base.Host
	req.Host = r.base.Host
	return r.inner.RoundTrip(req)
}

// TestRetrySucceedsAfterTransientFailures: the server hangs up on the first
// N-1 attempts, then returns 200 on the Nth. Verify Get() succeeds and
// verbose retry lines land on stderr.
func TestRetrySucceedsAfterTransientFailures(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&count, 1)
		if n < 3 {
			// Close the connection mid-flight -> transport error.
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("hijacker not available")
			}
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	// Capture stderr.
	origStderr := os.Stderr
	pr, pw, _ := os.Pipe()
	os.Stderr = pw
	defer func() { os.Stderr = origStderr }()

	c := stubClient(t, srv.URL, 3)
	got, err := c.Get("/api/v1/anything")

	_ = pw.Close()
	buf, _ := io.ReadAll(pr)
	stderr := string(buf)

	if err != nil {
		t.Fatalf("Get returned err after retries: %v (stderr=%s)", err, stderr)
	}
	m, ok := got.(map[string]any)
	if !ok || m["ok"] != true {
		t.Fatalf("unexpected body: %v", got)
	}
	if atomic.LoadInt32(&count) != 3 {
		t.Fatalf("expected 3 attempts, got %d", count)
	}
	if !containsStr(stderr, "retry 1/3") || !containsStr(stderr, "retry 2/3") {
		t.Errorf("expected verbose retry lines on stderr; got:\n%s", stderr)
	}
}

// TestRetryGivesUp: server always fails; after Retries+1 attempts, error surfaces.
func TestRetryGivesUp(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()

	c := stubClient(t, srv.URL, 2)
	_, err := c.Get("/api/v1/anything")
	if err == nil {
		t.Fatal("expected err after exhausting retries")
	}
	// 1 initial + 2 retries = 3 total.
	if got := atomic.LoadInt32(&count); got != 3 {
		t.Fatalf("expected 3 total attempts (1+retries), got %d", got)
	}
}

// TestNoRetryOnHTTPError: a 500 must NOT be retried.
func TestNoRetryOnHTTPError(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(500)
		_, _ = fmt.Fprint(w, `{"exception":{"code":"boom"}}`)
	}))
	defer srv.Close()

	c := stubClient(t, srv.URL, 5)
	_, err := c.Get("/api/v1/anything")
	if err == nil {
		t.Fatal("expected err on 500")
	}
	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("500 must not retry; got %d attempts", got)
	}
}

func containsStr(hay, needle string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
