package cmd

import (
	"strings"
	"testing"
)

func TestFormatNotification_Full(t *testing.T) {
	e := map[string]any{
		"date":     "2026-07-18T12:00:00Z",
		"severity": "warning",
		"title":    "Firmware update",
		"message":  "A new version is available",
	}
	got := formatNotification(e)
	want := "2026-07-18T12:00:00Z  warning  Firmware update: A new version is available"
	if got != want {
		t.Errorf("format mismatch:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestFormatNotification_TypeFallback(t *testing.T) {
	e := map[string]any{
		"date":    "2026-07-18",
		"type":    "info",
		"message": "hi",
	}
	got := formatNotification(e)
	if !strings.Contains(got, "info") || !strings.Contains(got, "hi") {
		t.Errorf("expected severity fallback to type field; got %q", got)
	}
}

func TestFormatNotification_UnknownShapeFallsBackToJSON(t *testing.T) {
	e := map[string]any{"weird_key": "value"}
	got := formatNotification(e)
	if !strings.HasPrefix(got, "{") {
		t.Errorf("expected JSON dump fallback; got %q", got)
	}
}

func TestNotificationEntries_FlatArray(t *testing.T) {
	raw := []any{
		map[string]any{"id": float64(1), "title": "a"},
		map[string]any{"id": float64(2), "title": "b"},
	}
	got := notificationEntries(raw)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
}

func TestNotificationEntries_WrappedShape(t *testing.T) {
	raw := []any{
		map[string]any{"notification": []any{
			map[string]any{"id": float64(1), "title": "a"},
		}},
	}
	got := notificationEntries(raw)
	if len(got) != 1 || toStr(got[0]["title"]) != "a" {
		t.Fatalf("unwrap failed; got %#v", got)
	}
}
