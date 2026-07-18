package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestRenderPrometheus_IncludesAllMetrics(t *testing.T) {
	s := scrape{
		Up: 1, WANState: 1, WANMapT: 1, UptimeSec: 3661,
		HostsTotal: 5, HostsActive: 3,
		NATRules: 2, FWRules: 1, DHCPLeases: 4,
		WifiBands: map[string]int{"24": 1, "5": 1, "6": 0, "guest": 0},
		At:        time.Now(),
	}
	out := renderPrometheus(s)

	want := []string{
		"# TYPE bbox_up gauge",
		"bbox_up 1",
		"bbox_wan_state 1",
		"bbox_wan_map_t 1",
		"bbox_uptime_seconds 3661",
		"bbox_hosts_total 5",
		"bbox_hosts_active 3",
		"bbox_nat_rules_total 2",
		"bbox_firewall_rules_total 1",
		"bbox_dhcp_leases_total 4",
		"# TYPE bbox_wifi_band_enabled gauge",
		`bbox_wifi_band_enabled{band="24"} 1`,
		`bbox_wifi_band_enabled{band="5"} 1`,
		`bbox_wifi_band_enabled{band="6"} 0`,
		`bbox_wifi_band_enabled{band="guest"} 0`,
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing line %q in:\n%s", w, out)
		}
	}
}

func TestRenderPrometheus_DownState(t *testing.T) {
	out := renderPrometheus(scrape{Up: 0, WifiBands: map[string]int{}})
	if !strings.Contains(out, "bbox_up 0") {
		t.Errorf("expected bbox_up 0, got:\n%s", out)
	}
	// all wifi bands default to 0
	for _, band := range []string{"24", "5", "6", "guest"} {
		want := `bbox_wifi_band_enabled{band="` + band + `"} 0`
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
}
