package cmd

import (
	"encoding/json"
	"testing"
)

// mockExport builds an export-config-shaped map with the given NAT rules and
// hosts. Any nil slice is omitted.
func mockExport(natRules []map[string]any, hosts []map[string]any) map[string]any {
	out := map[string]any{"exported_at": "2026-07-18T00:00:00Z"}
	if natRules != nil {
		rules := make([]any, len(natRules))
		for i, r := range natRules {
			rules[i] = r
		}
		out["/api/v1/nat/rules"] = []any{
			map[string]any{"nat": map[string]any{"rules": rules, "enable": 1}},
		}
	}
	if hosts != nil {
		list := make([]any, len(hosts))
		for i, h := range hosts {
			list[i] = h
		}
		out["/api/v1/hosts"] = []any{
			map[string]any{"hosts": map[string]any{"list": list}},
		}
	}
	return out
}

func TestDiff_NewNATRuleDetected(t *testing.T) {
	prev := mockExport(
		[]map[string]any{
			{"id": float64(1), "description": "ssh", "external_port": float64(22), "ipaddress": "192.168.1.10"},
		},
		nil,
	)
	cur := mockExport(
		[]map[string]any{
			{"id": float64(1), "description": "ssh", "external_port": float64(22), "ipaddress": "192.168.1.10"},
			{"id": float64(2), "description": "https", "external_port": float64(443), "ipaddress": "192.168.1.20"},
		},
		nil,
	)
	rep := diffExports(prev, cur)
	added, ok := rep.Added["nat_rules"].([]any)
	if !ok || len(added) != 1 {
		t.Fatalf("expected 1 added nat rule, got %#v", rep.Added["nat_rules"])
	}
	m := added[0].(map[string]any)
	if m["id"] != 2 {
		t.Errorf("added rule id: want 2, got %v", m["id"])
	}
	if len(rep.Removed) != 0 || len(rep.Changed) != 0 {
		t.Errorf("expected only .Added populated; got removed=%v changed=%v", rep.Removed, rep.Changed)
	}
}

func TestDiff_MACRenameDetected(t *testing.T) {
	prev := mockExport(nil, []map[string]any{
		{"macaddress": "aa:bb:cc:dd:ee:ff", "hostname": "old-name", "ipaddress": "192.168.1.10"},
	})
	cur := mockExport(nil, []map[string]any{
		{"macaddress": "aa:bb:cc:dd:ee:ff", "hostname": "new-name", "ipaddress": "192.168.1.10"},
	})
	rep := diffExports(prev, cur)
	renames, ok := rep.Changed["hosts_renamed"].([]any)
	if !ok || len(renames) != 1 {
		t.Fatalf("expected 1 rename event, got %#v", rep.Changed["hosts_renamed"])
	}
	r := renames[0].(map[string]any)
	if r["from"] != "old-name" || r["to"] != "new-name" {
		t.Errorf("rename event wrong: %v", r)
	}
	if _, ok := rep.Changed["hosts_ip_changed"]; ok {
		t.Errorf("did not expect ip-change event on rename-only diff")
	}
}

func TestDiff_NoChangeReportsEmpty(t *testing.T) {
	rules := []map[string]any{
		{"id": float64(1), "description": "ssh", "external_port": float64(22), "ipaddress": "192.168.1.10"},
	}
	hosts := []map[string]any{
		{"macaddress": "aa:bb:cc:dd:ee:ff", "hostname": "same", "ipaddress": "192.168.1.10"},
	}
	// Deep-clone via JSON to avoid map-identity noise.
	clone := func(v any) map[string]any {
		b, _ := json.Marshal(v)
		var out map[string]any
		_ = json.Unmarshal(b, &out)
		return out
	}
	prev := mockExport(rules, hosts)
	cur := clone(prev)
	rep := diffExports(prev, cur)
	if !rep.isEmpty() {
		t.Fatalf("expected empty diff, got added=%v removed=%v changed=%v",
			rep.Added, rep.Removed, rep.Changed)
	}
}
