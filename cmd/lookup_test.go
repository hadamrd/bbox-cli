package cmd

import (
	"strings"
	"testing"
)

// TestLookup_FiltersAllFourSections feeds synthetic fixtures directly to the
// filter routines (no HTTP), verifying the four match paths.
func TestLookup_FiltersAllFourSections(t *testing.T) {
	match := func(s string) bool {
		if s == "" {
			return false
		}
		return strings.Contains(strings.ToLower(s), "desktop")
	}

	hosts := []any{
		map[string]any{"id": float64(9), "hostname": "DESKTOP-J18FV15", "ipaddress": "192.168.1.125", "macaddress": "28:ee:52:00:0a:44", "link": "Wifi 2.4", "active": true},
		map[string]any{"id": float64(10), "hostname": "iphone", "ipaddress": "192.168.1.50", "macaddress": "aa:bb:cc:dd:ee:ff", "link": "Wifi 5"},
	}
	got := filterHosts(hosts, match)
	if len(got) != 1 || got[0]["hostname"] != "DESKTOP-J18FV15" {
		t.Fatalf("hosts filter: want 1 DESKTOP, got %#v", got)
	}

	nat := []any{
		map[string]any{"id": float64(1), "description": "retrobot-desktop", "protocol": "tcp", "externalport": float64(45080), "internalip": "192.168.1.125", "internalport": float64(45080)},
		map[string]any{"id": float64(2), "description": "unrelated", "externalport": float64(22), "internalip": "192.168.1.50"},
	}
	if got := filterNAT(nat, match); len(got) != 1 || got[0]["id"].(float64) != 1 {
		t.Fatalf("nat filter: want 1 rule id=1, got %#v", got)
	}

	fw := []any{
		map[string]any{"id": float64(3), "description": "block-desktop", "action": "Drop", "protocol": "tcp", "dstip": "192.168.1.125", "dstport": "22"},
		map[string]any{"id": float64(4), "description": "other", "dstip": "192.168.1.50"},
	}
	if got := filterFirewall(fw, match); len(got) != 1 || got[0]["id"].(float64) != 3 {
		t.Fatalf("firewall filter: want 1 rule id=3, got %#v", got)
	}

	dhcp := map[string]any{
		"clients": []any{
			map[string]any{"hostname": "DESKTOP-J18FV15", "ipaddress": "192.168.1.125", "macaddress": "28:ee:52:00:0a:44"},
			map[string]any{"hostname": "iphone", "ipaddress": "192.168.1.50"},
		},
	}
	if got := filterDHCP(dhcp, match); len(got) != 1 || got[0]["hostname"] != "DESKTOP-J18FV15" {
		t.Fatalf("dhcp filter: want 1 reservation DESKTOP, got %#v", got)
	}
}

// TestLookup_NoMatch verifies empty-result behavior.
func TestLookup_NoMatch(t *testing.T) {
	no := func(s string) bool { return false }
	if got := filterHosts([]any{map[string]any{"hostname": "x"}}, no); got != nil {
		t.Errorf("expected nil, got %#v", got)
	}
	if got := filterNAT([]any{map[string]any{"description": "x"}}, no); got != nil {
		t.Errorf("expected nil, got %#v", got)
	}
	if got := filterFirewall([]any{map[string]any{"description": "x"}}, no); got != nil {
		t.Errorf("expected nil, got %#v", got)
	}
	if got := filterDHCP(map[string]any{"clients": []any{map[string]any{"hostname": "x"}}}, no); got != nil {
		t.Errorf("expected nil, got %#v", got)
	}
}

// TestLookup_NestedDHCPShape ensures the {"clients": {"list": [...]}} variant works.
func TestLookup_NestedDHCPShape(t *testing.T) {
	match := func(s string) bool { return strings.Contains(s, "192.168.1.42") }
	dhcp := map[string]any{
		"reserved": map[string]any{
			"list": []any{
				map[string]any{"hostname": "nas", "ipaddress": "192.168.1.42", "macaddress": "11:22:33:44:55:66"},
			},
		},
	}
	got := filterDHCP(dhcp, match)
	if len(got) != 1 || got[0]["ipaddress"] != "192.168.1.42" {
		t.Fatalf("nested dhcp filter: want 1 hit, got %#v", got)
	}
}
