package cmd

import (
	"testing"
)

func TestDiffPlan_CreateKeepReplaceDelete(t *testing.T) {
	current := []existingRule{
		{ID: 1, Name: "gost", ExternalPort: 40080, TargetIP: "192.168.1.99", InternalPort: 40080, Protocol: "tcp"},
		{ID: 2, Name: "keeper", ExternalPort: 40090, TargetIP: "192.168.1.42", InternalPort: 22, Protocol: "tcp"},
		{ID: 3, Name: "legacy", ExternalPort: 40100, TargetIP: "192.168.1.5", InternalPort: 80, Protocol: "tcp"},
	}
	desired := normalizeDesired([]desiredRule{
		{Name: "ssh", ExternalPort: 40022, TargetIP: "192.168.1.42", InternalPort: 22},
		{Name: "gost", ExternalPort: 40080, TargetIP: "192.168.1.125"}, // ip changed
		{Name: "keeper", ExternalPort: 40090, TargetIP: "192.168.1.42", InternalPort: 22, Protocol: "tcp"},
	})

	plan := diffPlan(current, desired, true)

	// Expect: create ssh, replace gost (ip), keep keeper, delete legacy
	if len(plan) != 4 {
		t.Fatalf("plan length = %d, want 4: %+v", len(plan), plan)
	}
	if plan[0].Op != opCreate || plan[0].Name != "ssh" {
		t.Errorf("plan[0] = %+v, want create ssh", plan[0])
	}
	if plan[1].Op != opReplace || plan[1].Name != "gost" {
		t.Errorf("plan[1] = %+v, want replace gost", plan[1])
	}
	if plan[1].Reason == "" {
		t.Errorf("replace gost should have a reason, got empty")
	}
	if plan[2].Op != opKeep || plan[2].Name != "keeper" {
		t.Errorf("plan[2] = %+v, want keep keeper", plan[2])
	}
	if plan[3].Op != opDelete || plan[3].Name != "legacy" {
		t.Errorf("plan[3] = %+v, want delete legacy", plan[3])
	}
}

func TestDiffPlan_PruneOff(t *testing.T) {
	current := []existingRule{
		{ID: 1, Name: "legacy", ExternalPort: 40100, TargetIP: "192.168.1.5", InternalPort: 80, Protocol: "tcp"},
	}
	desired := normalizeDesired([]desiredRule{
		{Name: "ssh", ExternalPort: 40022, TargetIP: "192.168.1.42"},
	})

	plan := diffPlan(current, desired, false)

	if len(plan) != 1 {
		t.Fatalf("without --prune, plan should only have the create; got %d", len(plan))
	}
	if plan[0].Op != opCreate {
		t.Errorf("plan[0] = %+v, want create", plan[0])
	}
}

func TestDiffPlan_KeepAcrossDefaults(t *testing.T) {
	// Router returns a rule with tcp+internalport=externalport; YAML omits both.
	// Should be a KEEP, not a replace.
	current := []existingRule{
		{ID: 1, Name: "web", ExternalPort: 40080, TargetIP: "192.168.1.10", InternalPort: 40080, Protocol: "tcp"},
	}
	desired := normalizeDesired([]desiredRule{
		{Name: "web", ExternalPort: 40080, TargetIP: "192.168.1.10"},
	})

	plan := diffPlan(current, desired, false)
	if len(plan) != 1 || plan[0].Op != opKeep {
		t.Fatalf("want single keep, got %+v", plan)
	}
}

func TestDiffPlan_ReplaceReason(t *testing.T) {
	current := []existingRule{
		{ID: 1, Name: "svc", ExternalPort: 40080, TargetIP: "192.168.1.10", InternalPort: 8080, Protocol: "tcp"},
	}
	desired := normalizeDesired([]desiredRule{
		{Name: "svc", ExternalPort: 40080, TargetIP: "192.168.1.10", InternalPort: 9090, Protocol: "udp"},
	})

	plan := diffPlan(current, desired, false)
	if len(plan) != 1 || plan[0].Op != opReplace {
		t.Fatalf("want replace, got %+v", plan)
	}
	// Reason should mention both changed fields.
	if plan[0].Reason == "" {
		t.Errorf("reason should be non-empty")
	}
}

func TestValidateDesired(t *testing.T) {
	cases := []struct {
		name    string
		rules   []desiredRule
		wantErr bool
	}{
		{"ok", []desiredRule{{Name: "a", ExternalPort: 40000, TargetIP: "1.2.3.4"}}, false},
		{"missing name", []desiredRule{{ExternalPort: 40000, TargetIP: "1.2.3.4"}}, true},
		{"missing ip", []desiredRule{{Name: "a", ExternalPort: 40000}}, true},
		{"bad port", []desiredRule{{Name: "a", ExternalPort: 0, TargetIP: "1.2.3.4"}}, true},
		{"port too high", []desiredRule{{Name: "a", ExternalPort: 70000, TargetIP: "1.2.3.4"}}, true},
		{"dup names", []desiredRule{
			{Name: "a", ExternalPort: 40000, TargetIP: "1.2.3.4"},
			{Name: "a", ExternalPort: 40001, TargetIP: "1.2.3.5"},
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDesired(tc.rules)
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestNormalizeDesired_Defaults(t *testing.T) {
	in := []desiredRule{{Name: "x", ExternalPort: 40080, TargetIP: "1.2.3.4"}}
	out := normalizeDesired(in)
	if out[0].InternalPort != 40080 {
		t.Errorf("internal_port should default to external_port, got %d", out[0].InternalPort)
	}
	if out[0].Protocol != "tcp" {
		t.Errorf("protocol should default to tcp, got %q", out[0].Protocol)
	}
}

func TestParseCurrent(t *testing.T) {
	raw := []any{
		map[string]any{
			"id":           float64(7),
			"description":  "web",
			"protocol":     "TCP",
			"externalport": float64(40080),
			"internalip":   "192.168.1.10",
			"internalport": float64(80),
			"ipremote":     "",
		},
	}
	cur := parseCurrent(raw)
	if len(cur) != 1 {
		t.Fatalf("len = %d", len(cur))
	}
	if cur[0].ID != 7 || cur[0].Name != "web" || cur[0].Protocol != "tcp" || cur[0].ExternalPort != 40080 {
		t.Errorf("parsed = %+v", cur[0])
	}
}
