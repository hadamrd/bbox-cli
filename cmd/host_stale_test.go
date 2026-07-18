package cmd

import (
	"testing"
	"time"
)

func TestComputeStaleHosts(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	hosts := []any{
		// Recently seen — 5 days ago.
		map[string]any{"id": 1, "hostname": "recent", "ipaddress": "192.168.1.10", "macaddress": "AA:AA:AA:AA:AA:01"},
		// Stale — last DEVICE_UP 45 days ago.
		map[string]any{"id": 2, "hostname": "old", "ipaddress": "192.168.1.11", "macaddress": "AA:AA:AA:AA:AA:02"},
		// Never seen in log window.
		map[string]any{"id": 3, "hostname": "ghost", "ipaddress": "192.168.1.12", "macaddress": "AA:AA:AA:AA:AA:03"},
	}
	logs := []any{
		map[string]any{"date": now.AddDate(0, 0, -5).Format(time.RFC3339), "log": "DEVICE_UP", "param": "aa:aa:aa:aa:aa:01;192.168.1.10;recent"},
		map[string]any{"date": now.AddDate(0, 0, -45).Format(time.RFC3339), "log": "DEVICE_UP", "param": "aa:aa:aa:aa:aa:02;192.168.1.11;old"},
	}
	stale, _ := computeStaleHosts(hosts, logs, 30, now)
	if len(stale) != 2 {
		t.Fatalf("expected 2 stale hosts, got %d: %+v", len(stale), stale)
	}
	// Expect "old" (45 days) + "ghost" (never).
	byMAC := map[string]staleHost{}
	for _, s := range stale {
		byMAC[s.MAC] = s
	}
	old, ok := byMAC["AA:AA:AA:AA:AA:02"]
	if !ok {
		t.Fatalf("expected 'old' host in results")
	}
	if old.DaysAgo == nil || *old.DaysAgo != 45 {
		t.Errorf("old.DaysAgo = %v, want 45", old.DaysAgo)
	}
	ghost, ok := byMAC["AA:AA:AA:AA:AA:03"]
	if !ok {
		t.Fatalf("expected 'ghost' host in results")
	}
	if ghost.LastSeen != nil || ghost.DaysAgo != nil {
		t.Errorf("ghost should have nil LastSeen/DaysAgo, got %+v", ghost)
	}

	// Tighter window — 'recent' (5d) drops out of "fresh" only if days<5.
	stale2, _ := computeStaleHosts(hosts, logs, 3, now)
	if len(stale2) != 3 {
		t.Errorf("with --days 3, expected all 3 hosts stale, got %d", len(stale2))
	}
}

func TestComputeStaleHostsWarn(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	// Log window only covers 10 days but the user asked --days 30.
	logs := []any{
		map[string]any{"date": now.AddDate(0, 0, -10).Format(time.RFC3339), "log": "DEVICE_UP", "param": "aa;ip;host"},
	}
	_, warn := computeStaleHosts(nil, logs, 30, now)
	if warn == "" {
		t.Errorf("expected a warning about the log window being too short")
	}
}
