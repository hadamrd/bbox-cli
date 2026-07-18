// Package client wraps the reversed Bouygues Bbox admin API at mabbox.bytel.fr.
//
// Endpoint pattern (reversed 2026-07-17):
//
//	Reads   -> GET    /api/v1/<resource>
//	Writes  -> PUT    /api/v1/<resource>?btoken=<device_token>  (form-urlencoded)
//	Create  -> POST   /api/v1/<resource>?btoken=<device_token>
//	Delete  -> DELETE /api/v1/<resource>/<id>?btoken=<device_token>
//
// The device token is short-lived; fetched from /api/v1/device/token and cached
// until 30s before expiry.
package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	BaseURL   = "https://mabbox.bytel.fr"
	UserAgent = "Mozilla/5.0 bbox-cli"
)

// SessionFile is the path to the cached cookie session (matches Python layout).
func SessionFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bbox-session.json")
}

// PasswordFileDefault is the fallback password file if none provided.
func PasswordFileDefault() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bbox-password")
}

// WANHistoryFile is the default JSONL history file for `watch-ip`.
func WANHistoryFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bbox-wan-history.jsonl")
}

// Client is the transport wrapper. Mirrors Python `Bbox` class.
type Client struct {
	Verbose      bool
	Retries      int // max retry attempts on transient network errors (default 0 = no retry)
	http         *http.Client
	jar          http.CookieJar
	token        string
	tokenExpires time.Time
	// sleep is the backoff sleep, overridable in tests.
	sleep func(time.Duration)
}

// New constructs a Client with an InsecureSkipVerify HTTPS transport (Bbox is self-signed).
// retries=0 disables retry; timeout<=0 falls back to 15s.
func New(verbose bool, retries int, timeout time.Duration) *Client {
	jar, _ := cookiejar.New(nil)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if retries < 0 {
		retries = 0
	}
	return &Client{
		Verbose: verbose,
		Retries: retries,
		jar:     jar,
		http: &http.Client{
			Transport: tr,
			Jar:       jar,
			Timeout:   timeout,
		},
		sleep: time.Sleep,
	}
}

// req sends a raw HTTP request with the mandatory Bbox headers. Retries on
// transport-layer errors (DNS, TCP refused, EOF, TLS handshake) up to
// c.Retries times with exponential backoff (500ms, 1s, 2s...). HTTP status
// codes (incl. 5xx) are never retried — they are semantically final.
func (c *Client) req(method, path string, body []byte, ctype string) (int, []byte, http.Header, error) {
	u := BaseURL + path
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "-> %s %s\n", method, u)
		if body != nil {
			s := string(body)
			if len(s) > 200 {
				s = s[:200]
			}
			fmt.Fprintf(os.Stderr, "   body: %s\n", s)
		}
	}
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		var rdr io.Reader
		if body != nil {
			rdr = strings.NewReader(string(body))
		}
		req, err := http.NewRequest(method, u, rdr)
		if err != nil {
			return 0, nil, nil, err
		}
		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("X-Requested-With", "XmlHttpRequest")
		req.Header.Set("Origin", BaseURL)
		req.Header.Set("Referer", BaseURL+"/login")
		if body != nil {
			req.Header.Set("Content-Type", ctype)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.Retries {
				backoff := time.Duration(500<<attempt) * time.Millisecond
				if c.Verbose {
					fmt.Fprintf(os.Stderr, "   retry %d/%d after %v: %v\n", attempt+1, c.Retries, backoff, err)
				}
				if c.sleep != nil {
					c.sleep(backoff)
				} else {
					time.Sleep(backoff)
				}
				continue
			}
			return 0, nil, nil, err
		}
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if c.Verbose {
			fmt.Fprintf(os.Stderr, "<- %d (%dB)\n", resp.StatusCode, len(data))
		}
		return resp.StatusCode, data, resp.Header, nil
	}
	return 0, nil, nil, lastErr
}

// Get performs a JSON GET and unmarshals the response into any.
func (c *Client) Get(path string) (any, error) {
	code, data, _, err := c.req("GET", path, nil, "")
	if err != nil {
		return nil, classifyNetworkError(err)
	}
	if code >= 400 {
		return nil, classifyError("GET", path, code, data)
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: HTTP %d — %s", path, code, snippet(data))
	}
	if len(data) == 0 {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Write performs a write (POST/PUT/DELETE/PATCH) with auto btoken and form-urlencoded body.
func (c *Client) Write(method, path string, form map[string]any) (int, []byte, http.Header, error) {
	tok, err := c.DeviceToken()
	if err != nil {
		return 0, nil, nil, err
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	full := path + sep + "btoken=" + url.QueryEscape(tok)
	var body []byte
	if form != nil {
		v := url.Values{}
		for k, val := range form {
			if val == nil {
				v.Set(k, "")
			} else {
				v.Set(k, fmt.Sprintf("%v", val))
			}
		}
		body = []byte(v.Encode())
	}
	code, data, hdrs, err := c.req(method, full, body, "application/x-www-form-urlencoded; charset=UTF-8")
	if err != nil {
		return code, data, hdrs, classifyNetworkError(err)
	}
	if code >= 400 {
		return code, data, hdrs, classifyError(method, path, code, data)
	}
	return code, data, hdrs, nil
}

// Raw performs a direct API call with no auto-btoken (used by `bbox raw`).
func (c *Client) Raw(method, path string, body string) (int, []byte, http.Header, error) {
	var b []byte
	if body != "" {
		b = []byte(body)
	}
	return c.req(strings.ToUpper(method), path, b, "application/x-www-form-urlencoded; charset=UTF-8")
}

// ── session ─────────────────────────────────────────────────────────────────

// sessionEntry mirrors the Python bbox.py session file. Python writes
// `expires` via datetime.timestamp() which is a float; accept it as float64
// so on-disk sessions round-trip between the two implementations.
type sessionEntry struct {
	Value   string   `json:"value"`
	Expires *float64 `json:"expires"`
}

// SaveSession writes the cookie jar to the session file (Python-compatible layout).
func (c *Client) SaveSession() error {
	u, _ := url.Parse(BaseURL)
	out := map[string]sessionEntry{}
	for _, ck := range c.jar.Cookies(u) {
		var exp *float64
		if !ck.Expires.IsZero() {
			e := float64(ck.Expires.Unix())
			exp = &e
		}
		out[ck.Name] = sessionEntry{Value: ck.Value, Expires: exp}
	}
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	if err := os.WriteFile(SessionFile(), data, 0600); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(SessionFile(), 0600)
	}
	return nil
}

// LoadSession restores cookies from the on-disk session file. Returns true on success.
func (c *Client) LoadSession() bool {
	data, err := os.ReadFile(SessionFile())
	if err != nil {
		return false
	}
	var raw map[string]sessionEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	now := float64(time.Now().Unix())
	u, _ := url.Parse(BaseURL)
	var cookies []*http.Cookie
	for name, meta := range raw {
		if meta.Expires != nil && *meta.Expires < now {
			return false
		}
		ck := &http.Cookie{
			Name:   name,
			Value:  meta.Value,
			Domain: "mabbox.bytel.fr",
			Path:   "/",
			Secure: true,
		}
		if meta.Expires != nil {
			ck.Expires = time.Unix(int64(*meta.Expires), 0)
		}
		cookies = append(cookies, ck)
	}
	c.jar.SetCookies(u, cookies)
	sessionWasLoaded = true
	return true
}

// Login sends a PUT /api/v1/login with the given password and saves the resulting cookies.
func (c *Client) Login(password string) error {
	body := url.Values{"password": {password}, "remember": {"1"}}.Encode()
	code, data, _, err := c.req("PUT", "/api/v1/login", []byte(body), "application/x-www-form-urlencoded; charset=UTF-8")
	if err != nil {
		return err
	}
	if code == 200 {
		return c.SaveSession()
	}
	if code == 429 {
		return fmt.Errorf("Bbox rate-limited login. Every failed retry EXTENDS the lockout. Wait 15+ min, or use `bbox session-import --bbox-id <cookie>` with the value from Chrome DevTools.")
	}
	if code == 401 {
		return fmt.Errorf("Bbox login rejected (wrong password?).")
	}
	return fmt.Errorf("login failed: HTTP %d — %s", code, snippet(data))
}

// Logout hits /api/v1/logout and deletes the local session file.
func (c *Client) Logout() error {
	_, _, _, _ = c.req("POST", "/api/v1/logout", nil, "")
	_ = os.Remove(SessionFile())
	return nil
}

// EnsureAuth loads the cached session or logs in (via password getter) if missing/expired.
func (c *Client) EnsureAuth(passwordGetter func() (string, error)) error {
	if c.LoadSession() {
		code, _, _, err := c.req("GET", "/api/v1/device", nil, "")
		if err == nil && code == 200 {
			return nil
		}
		// Clear cookies
		jar, _ := cookiejar.New(nil)
		c.jar = jar
		c.http.Jar = jar
	}
	pw, err := passwordGetter()
	if err != nil {
		return err
	}
	return c.Login(pw)
}

// DeviceToken fetches (or returns cached) short-lived write token.
func (c *Client) DeviceToken() (string, error) {
	now := time.Now()
	if c.token != "" && now.Before(c.tokenExpires.Add(-30*time.Second)) {
		return c.token, nil
	}
	raw, err := c.Get("/api/v1/device/token")
	if err != nil {
		return "", err
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return "", fmt.Errorf("device token: unexpected shape")
	}
	m, _ := arr[0].(map[string]any)
	dev, _ := m["device"].(map[string]any)
	tok, _ := dev["token"].(string)
	if tok == "" {
		return "", fmt.Errorf("device token: missing")
	}
	c.token = tok
	// Try to parse expires; Bbox returns ISO8601. Fall back to +5 min.
	if s, ok := dev["expires"].(string); ok {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			c.tokenExpires = t
		} else if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
			c.tokenExpires = t
		} else {
			c.tokenExpires = now.Add(5 * time.Minute)
		}
	} else {
		c.tokenExpires = now.Add(5 * time.Minute)
	}
	return c.token, nil
}

func snippet(b []byte) string {
	if len(b) > 200 {
		b = b[:200]
	}
	return fmt.Sprintf("%q", string(b))
}
