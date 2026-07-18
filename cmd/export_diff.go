package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// diffReport is what `export-config --diff` emits in JSON mode. Nested per
// endpoint / domain so downstream scripts can inspect specific areas without
// walking a free-form recursive diff.
type diffReport struct {
	Added   map[string]any `json:"added"`
	Removed map[string]any `json:"removed"`
	Changed map[string]any `json:"changed"`
}

func newDiffReport() diffReport {
	return diffReport{
		Added:   map[string]any{},
		Removed: map[string]any{},
		Changed: map[string]any{},
	}
}

// isEmpty is true when the report has no findings.
func (d diffReport) isEmpty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// diffExports compares two export-config snapshots and returns a semantic
// diff for the domains we care about (NAT, hosts, WiFi, DHCP, firewall).
// Unknown endpoint keys get shallow "endpoint added/removed" treatment.
func diffExports(prev, cur map[string]any) diffReport {
	rep := newDiffReport()

	// Shallow endpoint add/remove first.
	for k := range cur {
		if k == "exported_at" {
			continue
		}
		if _, ok := prev[k]; !ok {
			rep.Added["endpoint:"+k] = true
		}
	}
	for k := range prev {
		if k == "exported_at" {
			continue
		}
		if _, ok := cur[k]; !ok {
			rep.Removed["endpoint:"+k] = true
		}
	}

	// NAT rules — keyed by id.
	if p, c := extractNATRules(prev), extractNATRules(cur); p != nil || c != nil {
		diffNATRules(p, c, &rep)
	}
	// Hosts — keyed by mac.
	if p, c := extractHosts(prev), extractHosts(cur); p != nil || c != nil {
		diffHosts(p, c, &rep)
	}
	// WiFi — per band.
	diffWiFi(prev, cur, &rep)
	// Firewall rules — keyed by id.
	if p, c := extractFirewallRules(prev), extractFirewallRules(cur); p != nil || c != nil {
		diffFirewallRules(p, c, &rep)
	}
	// DHCP clients — keyed by mac.
	if p, c := extractDHCPClients(prev), extractDHCPClients(cur); p != nil || c != nil {
		diffDHCPClients(p, c, &rep)
	}
	return rep
}

// ─── extractors ────────────────────────────────────────────────────────────

func extractNATRules(exp map[string]any) []map[string]any {
	v, ok := exp["/api/v1/nat/rules"]
	if !ok {
		return nil
	}
	m := firstOfAny(v)
	nat, _ := m["nat"].(map[string]any)
	arr, _ := nat["rules"].([]any)
	return toMapSlice(arr)
}

func extractFirewallRules(exp map[string]any) []map[string]any {
	v, ok := exp["/api/v1/firewall/rules"]
	if !ok {
		return nil
	}
	m := firstOfAny(v)
	fw, _ := m["firewall"].(map[string]any)
	arr, _ := fw["rules"].([]any)
	return toMapSlice(arr)
}

func extractHosts(exp map[string]any) []map[string]any {
	v, ok := exp["/api/v1/hosts"]
	if !ok {
		return nil
	}
	m := firstOfAny(v)
	hosts, _ := m["hosts"].(map[string]any)
	arr, _ := hosts["list"].([]any)
	return toMapSlice(arr)
}

func extractDHCPClients(exp map[string]any) []map[string]any {
	v, ok := exp["/api/v1/dhcp/clients"]
	if !ok {
		return nil
	}
	m := firstOfAny(v)
	arr, _ := m["clients"].([]any)
	return toMapSlice(arr)
}

// firstOfAny mirrors client.firstMap for the export shape ([{...}] or {...}).
func firstOfAny(v any) map[string]any {
	if arr, ok := v.([]any); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]any); ok {
			return m
		}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func toMapSlice(arr []any) []map[string]any {
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// ─── diff routines ─────────────────────────────────────────────────────────

func diffNATRules(prev, cur []map[string]any, rep *diffReport) {
	prevByID := indexByField(prev, "id")
	curByID := indexByField(cur, "id")
	added, removed, changed := []any{}, []any{}, []any{}
	for id, r := range curByID {
		if _, ok := prevByID[id]; !ok {
			added = append(added, natSummary(r))
		}
	}
	for id, r := range prevByID {
		if _, ok := curByID[id]; !ok {
			removed = append(removed, natSummary(r))
		}
	}
	for id, r := range curByID {
		p, ok := prevByID[id]
		if !ok {
			continue
		}
		delta := fieldDelta(p, r, []string{"enable", "description", "protocol", "ipaddress", "external_port", "internal_port", "ipremote"})
		if len(delta) > 0 {
			delta["id"] = id
			changed = append(changed, delta)
		}
	}
	if len(added) > 0 {
		rep.Added["nat_rules"] = added
	}
	if len(removed) > 0 {
		rep.Removed["nat_rules"] = removed
	}
	if len(changed) > 0 {
		rep.Changed["nat_rules"] = changed
	}
}

func diffFirewallRules(prev, cur []map[string]any, rep *diffReport) {
	prevByID := indexByField(prev, "id")
	curByID := indexByField(cur, "id")
	added, removed, changed := []any{}, []any{}, []any{}
	for id, r := range curByID {
		if _, ok := prevByID[id]; !ok {
			added = append(added, r)
		}
	}
	for id, r := range prevByID {
		if _, ok := curByID[id]; !ok {
			removed = append(removed, r)
		}
	}
	for id, r := range curByID {
		p, ok := prevByID[id]
		if !ok {
			continue
		}
		delta := fieldDelta(p, r, []string{"enable", "action", "protocol", "srcip", "srcport", "dstip", "dstport"})
		if len(delta) > 0 {
			delta["id"] = id
			changed = append(changed, delta)
		}
	}
	if len(added) > 0 {
		rep.Added["firewall_rules"] = added
	}
	if len(removed) > 0 {
		rep.Removed["firewall_rules"] = removed
	}
	if len(changed) > 0 {
		rep.Changed["firewall_rules"] = changed
	}
}

func diffHosts(prev, cur []map[string]any, rep *diffReport) {
	prevByMAC := indexByStringField(prev, "macaddress")
	curByMAC := indexByStringField(cur, "macaddress")
	added, removed, renamed, ipChanged := []any{}, []any{}, []any{}, []any{}
	for mac, h := range curByMAC {
		if _, ok := prevByMAC[mac]; !ok {
			added = append(added, map[string]any{
				"mac": mac, "hostname": toStr(h["hostname"]), "ip": toStr(h["ipaddress"]),
			})
		}
	}
	for mac, h := range prevByMAC {
		if _, ok := curByMAC[mac]; !ok {
			removed = append(removed, map[string]any{
				"mac": mac, "hostname": toStr(h["hostname"]), "ip": toStr(h["ipaddress"]),
			})
		}
	}
	for mac, h := range curByMAC {
		p, ok := prevByMAC[mac]
		if !ok {
			continue
		}
		if toStr(p["hostname"]) != toStr(h["hostname"]) {
			renamed = append(renamed, map[string]any{
				"mac":  mac,
				"from": toStr(p["hostname"]),
				"to":   toStr(h["hostname"]),
			})
		}
		if toStr(p["ipaddress"]) != toStr(h["ipaddress"]) {
			ipChanged = append(ipChanged, map[string]any{
				"mac":  mac,
				"from": toStr(p["ipaddress"]),
				"to":   toStr(h["ipaddress"]),
			})
		}
	}
	if len(added) > 0 {
		rep.Added["hosts"] = added
	}
	if len(removed) > 0 {
		rep.Removed["hosts"] = removed
	}
	if len(renamed) > 0 {
		rep.Changed["hosts_renamed"] = renamed
	}
	if len(ipChanged) > 0 {
		rep.Changed["hosts_ip_changed"] = ipChanged
	}
}

func diffDHCPClients(prev, cur []map[string]any, rep *diffReport) {
	prevByMAC := indexByStringField(prev, "macaddress")
	curByMAC := indexByStringField(cur, "macaddress")
	added, removed := []any{}, []any{}
	for mac, h := range curByMAC {
		if _, ok := prevByMAC[mac]; !ok {
			added = append(added, h)
		}
	}
	for mac, h := range prevByMAC {
		if _, ok := curByMAC[mac]; !ok {
			removed = append(removed, h)
		}
	}
	if len(added) > 0 {
		rep.Added["dhcp_reservations"] = added
	}
	if len(removed) > 0 {
		rep.Removed["dhcp_reservations"] = removed
	}
}

func diffWiFi(prev, cur map[string]any, rep *diffReport) {
	changes := map[string]any{}
	for _, band := range []string{"24", "5", "6", "guest"} {
		p := extractWifiBand(prev, band)
		c := extractWifiBand(cur, band)
		if p == nil && c == nil {
			continue
		}
		if p == nil {
			continue
		}
		if c == nil {
			continue
		}
		delta := fieldDelta(p, c, []string{"enable", "ssid", "channel"})
		// Also probe nested radio.channel field which some firmwares expose.
		pRadio, _ := p["radio"].(map[string]any)
		cRadio, _ := c["radio"].(map[string]any)
		if len(pRadio) > 0 && len(cRadio) > 0 {
			for _, f := range []string{"channel", "enable"} {
				if toStr(pRadio[f]) != toStr(cRadio[f]) {
					delta["radio."+f] = map[string]any{"from": pRadio[f], "to": cRadio[f]}
				}
			}
		}
		if len(delta) > 0 {
			changes["wifi_"+band] = delta
		}
	}
	for k, v := range changes {
		rep.Changed[k] = v
	}
}

func extractWifiBand(exp map[string]any, band string) map[string]any {
	v, ok := exp["/api/v1/wireless/"+band]
	if !ok {
		return nil
	}
	m := firstOfAny(v)
	if w, ok := m["wireless"].(map[string]any); ok {
		return w
	}
	return m
}

// ─── indexing helpers ──────────────────────────────────────────────────────

func indexByField(items []map[string]any, field string) map[int]map[string]any {
	out := map[int]map[string]any{}
	for _, it := range items {
		id := toIntAny(it[field])
		out[id] = it
	}
	return out
}

func indexByStringField(items []map[string]any, field string) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, it := range items {
		k := strings.ToLower(toStr(it[field]))
		if k == "" {
			continue
		}
		out[k] = it
	}
	return out
}

func fieldDelta(prev, cur map[string]any, fields []string) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		if toStr(prev[f]) != toStr(cur[f]) {
			out[f] = map[string]any{"from": prev[f], "to": cur[f]}
		}
	}
	return out
}

func natSummary(r map[string]any) map[string]any {
	return map[string]any{
		"id":            toIntAny(r["id"]),
		"description":   toStr(r["description"]),
		"protocol":      toStr(r["protocol"]),
		"external_port": r["external_port"],
		"ipaddress":     toStr(r["ipaddress"]),
	}
}

// ─── human printer ─────────────────────────────────────────────────────────

func printDiffHuman(rep diffReport) {
	if rep.isEmpty() {
		fmt.Println("no changes")
		return
	}
	printSection := func(label string, m map[string]any) {
		if len(m) == 0 {
			return
		}
		fmt.Println(label)
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b, _ := json.MarshalIndent(m[k], "  ", "  ")
			fmt.Printf("  %s: %s\n", k, string(b))
		}
	}
	printSection("added:", rep.Added)
	printSection("removed:", rep.Removed)
	printSection("changed:", rep.Changed)
}

// readExportFile loads a previously-written export-config JSON blob.
func readExportFile(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return out, nil
}
