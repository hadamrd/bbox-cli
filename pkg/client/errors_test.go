package client

import (
	"strings"
	"testing"
)

func TestClassifyError_401Expired(t *testing.T) {
	prev := sessionWasLoaded
	sessionWasLoaded = true
	defer func() { sessionWasLoaded = prev }()

	body := []byte(`{"exception":{"code":"UNAUTHORIZED","message":"session invalid","errors":[{"reason":"Unauthorized","name":"login"}]}}`)
	err := classifyError("GET", "/api/v1/device", 401, body)
	got := err.Error()
	if !strings.Contains(got, "HTTP 401 Unauthorized") {
		t.Errorf("missing status+reason line: %q", got)
	}
	if !strings.Contains(got, "session invalid") {
		t.Errorf("missing message: %q", got)
	}
	if !strings.Contains(got, "session expired") {
		t.Errorf("missing 401 hint: %q", got)
	}
}

func TestClassifyError_403(t *testing.T) {
	body := []byte(`{"exception":{"code":"FORBIDDEN","message":"btoken stale","errors":[{"reason":"Forbidden","name":"nat"}]}}`)
	err := classifyError("PUT", "/api/v1/nat/rules", 403, body)
	got := err.Error()
	if !strings.Contains(got, "PUT /api/v1/nat/rules: HTTP 403 Forbidden") {
		t.Errorf("bad header line: %q", got)
	}
	if !strings.Contains(got, "auto-refreshes") {
		t.Errorf("missing 403 hint: %q", got)
	}
}

func TestClassifyError_404Write(t *testing.T) {
	body := []byte(`{"exception":{"code":"NOT_FOUND","message":"no such route","errors":[{"reason":"NotFound","name":"dyndns"}]}}`)
	err := classifyError("PUT", "/api/v1/dyndns", 404, body)
	got := err.Error()
	if !strings.Contains(got, "may not exist on your Bbox model") {
		t.Errorf("missing 404-write hint: %q", got)
	}
}

func TestClassifyError_404Read(t *testing.T) {
	body := []byte(`{"exception":{"code":"NOT_FOUND","message":"nope","errors":[{"reason":"NotFound","name":"x"}]}}`)
	err := classifyError("GET", "/api/v1/whatever", 404, body)
	got := err.Error()
	if strings.Contains(got, "may not exist on your Bbox model") {
		t.Errorf("404-write hint leaked onto GET: %q", got)
	}
	if !strings.Contains(got, "HTTP 404 NotFound") {
		t.Errorf("bad 404-read format: %q", got)
	}
}

func TestClassifyError_NonJSON(t *testing.T) {
	body := []byte(`<html>gateway timeout</html>`)
	err := classifyError("GET", "/api/v1/x", 502, body)
	got := err.Error()
	// legacy short format
	if !strings.Contains(got, "/api/v1/x: HTTP 502 —") {
		t.Errorf("expected legacy fallback, got: %q", got)
	}
}

func TestClassifyError_NonJSON_WithHint(t *testing.T) {
	body := []byte(`not json`)
	err := classifyError("PUT", "/api/v1/foo", 404, body)
	got := err.Error()
	if !strings.Contains(got, "may not exist on your Bbox model") {
		t.Errorf("expected 404-write hint even for non-JSON body: %q", got)
	}
}

func TestParseBboxError_EmptyEnvelopeRejected(t *testing.T) {
	if _, ok := parseBboxError([]byte(`{"exception":{}}`)); ok {
		t.Errorf("expected empty envelope to be treated as unparseable")
	}
}
