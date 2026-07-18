package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

// bboxErrorEnvelope models the admin-API error shape:
//
//	{"exception":{"code":"...","message":"...","errors":[{"reason":"...","name":"..."}]}}
type bboxErrorEnvelope struct {
	Exception struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Errors  []struct {
			Reason string `json:"reason"`
			Name   string `json:"name"`
		} `json:"errors"`
	} `json:"exception"`
}

// sessionWasLoaded is set by LoadSession() so classifyError can distinguish an
// expired-session 401 from a first-boot 401. Package-scoped rather than
// per-Client because Login/Logout also touch it and single-process CLI use only.
var sessionWasLoaded bool

// classifyError formats an HTTP error with an actionable hint. Falls back to the
// legacy `path: HTTP <code> — <snippet>` format when the body isn't a Bbox
// error envelope.
func classifyError(method, path string, code int, body []byte) error {
	env, ok := parseBboxError(body)
	isWrite := method != "GET"

	// Domain-specific hint.
	var hint string
	switch {
	case code == 401 && sessionWasLoaded:
		hint = "session expired — delete ~/.bbox-session.json and re-login, or run 'bbox session-import --bbox-id <cookie>' with a fresh cookie from Chrome DevTools"
	case code == 403:
		hint = "not authorized — the Bbox often 403s on write endpoints if the device token is stale. Try again; the CLI auto-refreshes it."
	case code == 404 && isWrite:
		hint = "not found — this endpoint may not exist on your Bbox model or firmware version"
	}

	if !ok {
		// Body isn't a parseable Bbox envelope — keep the legacy short format,
		// but append a hint line when we have one.
		base := fmt.Errorf("%s: HTTP %d — %s", path, code, snippet(body))
		if hint == "" {
			return base
		}
		return fmt.Errorf("%s\n  hint: %s", base, hint)
	}

	var b strings.Builder
	reason := ""
	if len(env.Exception.Errors) > 0 {
		reason = env.Exception.Errors[0].Reason
	}
	if reason == "" {
		reason = env.Exception.Code
	}
	fmt.Fprintf(&b, "%s %s: HTTP %d %s", method, path, code, reason)
	if msg := env.Exception.Message; msg != "" {
		fmt.Fprintf(&b, "\n  %s", msg)
	}
	if hint != "" {
		fmt.Fprintf(&b, "\n  hint: %s", hint)
	}
	return fmt.Errorf("%s", b.String())
}

// classifyNetworkError wraps a transport-layer error (DNS, TCP refused, TLS)
// with a reachability hint. Called from req() when http.Client.Do fails.
func classifyNetworkError(err error) error {
	return fmt.Errorf("%w\n  hint: is the router reachable? try 'ping mabbox.bytel.fr' or check you're on the LAN", err)
}

func parseBboxError(body []byte) (bboxErrorEnvelope, bool) {
	var env bboxErrorEnvelope
	trimmed := strings.TrimSpace(string(body))
	if !strings.HasPrefix(trimmed, "{") {
		return env, false
	}
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil {
		return env, false
	}
	if env.Exception.Code == "" && env.Exception.Message == "" && len(env.Exception.Errors) == 0 {
		return env, false
	}
	return env, true
}
