package client

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Helpers for the loose-dict JSON shape the Bbox returns everywhere:
// most responses are `[{"<key>": {...}}]`.

func firstMap(v any) map[string]any {
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

// GetInto is a convenience: GET path and return the first array element as map.
func (c *Client) GetFirst(path string) (map[string]any, error) {
	raw, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	return firstMap(raw), nil
}

// unwrap returns m[key] as map, or empty.
func unwrap(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}

func asList(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return nil
}

// ─── READ endpoints ────────────────────────────────────────────────────────

func (c *Client) Summary() (map[string]any, error) { return c.GetFirst("/api/v1/summary") }
func (c *Client) Device() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/device")
	if err != nil {
		return nil, err
	}
	return unwrap(m, "device"), nil
}
func (c *Client) Services() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/services")
	if err != nil {
		return nil, err
	}
	return unwrap(m, "services"), nil
}
func (c *Client) WAN() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/wan/ip")
	if err != nil {
		return nil, err
	}
	return unwrap(m, "wan"), nil
}
func (c *Client) WANStats() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/wan/ip/stats")
	if err != nil {
		return map[string]any{}, nil
	}
	return m, nil
}
func (c *Client) WANBackup() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/wan/backup")
	if err != nil {
		return nil, err
	}
	return unwrap(m, "backup"), nil
}
func (c *Client) WANXDSL() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/wan/xdsl")
	if err != nil {
		return map[string]any{}, nil
	}
	return m, nil
}
func (c *Client) LAN() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/lan/ip")
	if err != nil {
		return nil, err
	}
	return unwrap(m, "lan"), nil
}
func (c *Client) LANStats() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/lan/stats")
	if err != nil {
		return map[string]any{}, nil
	}
	return m, nil
}
func (c *Client) Logs() ([]any, error) {
	m, err := c.GetFirst("/api/v1/device/log")
	if err != nil {
		return nil, err
	}
	return asList(m["log"]), nil
}
func (c *Client) VoIP() ([]any, error) {
	m, err := c.GetFirst("/api/v1/voip")
	if err != nil {
		return nil, err
	}
	return asList(m["voip"]), nil
}
func (c *Client) Parental() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/parentalcontrol")
	if err != nil {
		return nil, err
	}
	return unwrap(m, "parentalcontrol"), nil
}

// Notifications returns the parsed /api/v1/notification body as-is. The
// firmware sometimes returns an array of entries, sometimes an object with a
// "notification" key — callers must handle both shapes.
func (c *Client) Notifications() (any, error) {
	return c.Get("/api/v1/notification")
}

// NotificationsClear tries the collective DELETE first; if that 404s the caller
// is expected to fall back to per-id deletion.
func (c *Client) NotificationsClear() error {
	code, data, _, err := c.Write("DELETE", "/api/v1/notification", nil)
	if err != nil {
		return err
	}
	if code == 200 || code == 204 {
		return nil
	}
	return fmt.Errorf("notification_clear: HTTP %d — %s", code, snippet(data))
}

// NotificationDel deletes a single notification by id.
func (c *Client) NotificationDel(id int) error {
	code, data, _, err := c.Write("DELETE", fmt.Sprintf("/api/v1/notification/%d", id), nil)
	if err != nil {
		return err
	}
	if code == 200 || code == 204 {
		return nil
	}
	return fmt.Errorf("notification_del(%d): HTTP %d — %s", id, code, snippet(data))
}
func (c *Client) Hibernate() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/hibernate/scheduler")
	if err != nil {
		return map[string]any{}, nil
	}
	return m, nil
}

// ─── NAT ───────────────────────────────────────────────────────────────────

func (c *Client) NATRules() ([]any, error) {
	m, err := c.GetFirst("/api/v1/nat/rules")
	if err != nil {
		return nil, err
	}
	return asList(unwrap(m, "nat")["rules"]), nil
}
func (c *Client) NATEnabled() (bool, error) {
	m, err := c.GetFirst("/api/v1/nat/rules")
	if err != nil {
		return false, err
	}
	nat := unwrap(m, "nat")
	return toBool(nat["enable"]), nil
}
func (c *Client) NATToggle(enable bool) error {
	code, data, _, err := c.Write("PUT", "/api/v1/nat/rules", map[string]any{"enable": boolInt(enable)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("nat_toggle: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

type NATAddArgs struct {
	Description  string
	ExternalPort int
	InternalIP   string
	InternalPort int
	Protocol     string
	RemoteIP     string
	IPVersion    string
	// Disabled creates the rule with enable=0. Zero-value (false) preserves the
	// pre-existing behaviour of adding an enabled rule.
	Disabled bool
}

func (c *Client) NATAdd(a NATAddArgs) (int, error) {
	if a.InternalPort == 0 {
		a.InternalPort = a.ExternalPort
	}
	if a.Protocol == "" {
		a.Protocol = "tcp"
	}
	if a.IPVersion == "" {
		a.IPVersion = "IPv4"
	}
	enable := 1
	if a.Disabled {
		enable = 0
	}
	code, _, hdrs, err := c.Write("POST", "/api/v1/nat/rules", map[string]any{
		"enable":        enable,
		"description":   a.Description,
		"protocol":      strings.ToLower(a.Protocol),
		"ipaddress":     a.InternalIP,
		"external_port": a.ExternalPort,
		"ipremote":      a.RemoteIP,
		"internal_port": a.InternalPort,
		"range":         "",
		"ipprotocol":    a.IPVersion,
	})
	if err != nil {
		return 0, err
	}
	if code != 201 {
		return 0, fmt.Errorf("nat_add: HTTP %d", code)
	}
	return locID(hdrs.Get("Location")), nil
}

// NATToggleRule flips the enable flag of a single NAT rule in-place, mirroring
// FirewallRuleToggle. Lets callers avoid a delete+recreate churn when only
// the enable state changes.
func (c *Client) NATToggleRule(ruleID int, enable bool) error {
	code, data, _, err := c.Write("PUT", fmt.Sprintf("/api/v1/nat/rules/%d", ruleID), map[string]any{"enable": boolInt(enable)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("nat_rule_toggle: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

func (c *Client) NATDel(ruleID int) error {
	code, data, _, err := c.Write("DELETE", fmt.Sprintf("/api/v1/nat/rules/%d", ruleID), nil)
	if err != nil {
		return err
	}
	if code != 200 && code != 204 {
		return fmt.Errorf("nat_del(%d): HTTP %d — %s", ruleID, code, snippet(data))
	}
	return nil
}

// ─── DMZ ───────────────────────────────────────────────────────────────────

func (c *Client) DMZ() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/nat/dmz")
	if err != nil {
		return nil, err
	}
	return unwrap(unwrap(m, "nat"), "dmz"), nil
}
func (c *Client) DMZSet(ip string) error {
	code, data, _, err := c.Write("PUT", "/api/v1/nat/dmz", map[string]any{"enable": 1, "ipaddress": ip, "dnsprotect": 1})
	if err != nil {
		return err
	}
	if code != 200 && code != 201 {
		return fmt.Errorf("dmz_set: HTTP %d — %s", code, snippet(data))
	}
	return nil
}
func (c *Client) DMZOff() error {
	code, data, _, err := c.Write("PUT", "/api/v1/nat/dmz", map[string]any{"enable": 0})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("dmz_off: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ─── UPnP ──────────────────────────────────────────────────────────────────

func (c *Client) UPnP() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/upnp/igd")
	if err != nil {
		return nil, err
	}
	return unwrap(unwrap(m, "upnp"), "igd"), nil
}
func (c *Client) UPnPRules() ([]any, error) {
	m, err := c.GetFirst("/api/v1/upnp/igd/rules")
	if err != nil {
		return nil, err
	}
	igd := unwrap(unwrap(m, "upnp"), "igd")
	return asList(igd["rules"]), nil
}
func (c *Client) UPnPToggle(enable bool) error {
	code, data, _, err := c.Write("PUT", "/api/v1/upnp/igd", map[string]any{"enable": boolInt(enable)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("upnp_toggle: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ─── Firewall ──────────────────────────────────────────────────────────────

func (c *Client) Firewall() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/firewall")
	if err != nil {
		return nil, err
	}
	return unwrap(m, "firewall"), nil
}
func (c *Client) FirewallRules() ([]any, error) {
	m, err := c.GetFirst("/api/v1/firewall/rules")
	if err != nil {
		return nil, err
	}
	return asList(unwrap(m, "firewall")["rules"]), nil
}
func (c *Client) FirewallToggle(enable bool) error {
	code, data, _, err := c.Write("PUT", "/api/v1/firewall", map[string]any{"enable": boolInt(enable)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("firewall_toggle: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

type FirewallRuleArgs struct {
	Description string
	Action      string // 'Drop' | 'Accept'
	Protocol    string
	DstIP       string
	DstPort     string
	SrcIP       string
	SrcPort     string
	IPVersion   string
	Enable      bool
}

func (c *Client) FirewallRuleAdd(a FirewallRuleArgs) (int, error) {
	if a.IPVersion == "" {
		a.IPVersion = "IPv4"
	}
	code, _, hdrs, err := c.Write("POST", "/api/v1/firewall/rules", map[string]any{
		"enable":      boolInt(a.Enable),
		"description": a.Description,
		"action":      a.Action,
		"protocol":    a.Protocol,
		"srcipnot":    0,
		"srcip":       a.SrcIP,
		"srcport":     a.SrcPort,
		"dstipnot":    0,
		"dstip":       a.DstIP,
		"dstport":     a.DstPort,
		"ipprotocol":  a.IPVersion,
	})
	if err != nil {
		return 0, err
	}
	if code != 200 && code != 201 {
		return 0, fmt.Errorf("firewall_rule_add: HTTP %d", code)
	}
	return locID(hdrs.Get("Location")), nil
}
func (c *Client) FirewallRuleDel(ruleID int) error {
	code, data, _, err := c.Write("DELETE", fmt.Sprintf("/api/v1/firewall/rules/%d", ruleID), nil)
	if err != nil {
		return err
	}
	if code != 200 && code != 204 {
		return fmt.Errorf("firewall_rule_del(%d): HTTP %d — %s", ruleID, code, snippet(data))
	}
	return nil
}
func (c *Client) FirewallRuleToggle(ruleID int, enable bool) error {
	code, data, _, err := c.Write("PUT", fmt.Sprintf("/api/v1/firewall/rules/%d", ruleID), map[string]any{"enable": boolInt(enable)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("firewall_rule_toggle: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ─── Hosts ─────────────────────────────────────────────────────────────────

func (c *Client) Hosts() ([]any, error) {
	m, err := c.GetFirst("/api/v1/hosts")
	if err != nil {
		return nil, err
	}
	return asList(unwrap(m, "hosts")["list"]), nil
}
func (c *Client) HostMe() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/hosts/me")
	if err != nil {
		return nil, err
	}
	return unwrap(m, "host"), nil
}

// HostBy looks up a host by id, hostname, ip or mac. Case-insensitive.
func (c *Client) HostBy(key string) (map[string]any, error) {
	hosts, err := c.Hosts()
	if err != nil {
		return nil, err
	}
	keyL := strings.ToLower(key)
	keyID, digitErr := strconv.Atoi(key)
	for _, hAny := range hosts {
		h, _ := hAny.(map[string]any)
		if digitErr == nil {
			if toInt(h["id"]) == keyID {
				return h, nil
			}
		}
		if s, _ := h["hostname"].(string); strings.ToLower(s) == keyL {
			return h, nil
		}
		if s, _ := h["ipaddress"].(string); s == key {
			return h, nil
		}
		if s, _ := h["macaddress"].(string); strings.ToLower(s) == keyL {
			return h, nil
		}
	}
	return nil, fmt.Errorf("no host matches '%s'. Try `bbox host list`.", key)
}

func (c *Client) HostRename(hostID int, newName string) error {
	code, data, _, err := c.Write("PUT", fmt.Sprintf("/api/v1/hosts/%d", hostID), map[string]any{"hostname": newName})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("host_rename: HTTP %d — %s", code, snippet(data))
	}
	return nil
}
func (c *Client) HostBlock(hostID int) error {
	code, body, _, err := c.Write("POST", "/api/v1/parentalcontrol/hosts", map[string]any{"macaddress": hostID})
	if err != nil {
		return err
	}
	if code == 200 || code == 201 {
		return nil
	}
	code2, body2, _, err := c.Write("PUT", fmt.Sprintf("/api/v1/hosts/%d", hostID), map[string]any{"blocked": 1})
	if err != nil {
		return err
	}
	if code2 != 200 {
		return fmt.Errorf("host_block: HTTP %d/%d — %s / %s", code, code2, snippet(body), snippet(body2))
	}
	return nil
}
func (c *Client) HostUnblock(hostID int) error {
	code, body, _, err := c.Write("DELETE", fmt.Sprintf("/api/v1/parentalcontrol/hosts/%d", hostID), nil)
	if err != nil {
		return err
	}
	if code == 200 || code == 204 {
		return nil
	}
	code2, body2, _, err := c.Write("PUT", fmt.Sprintf("/api/v1/hosts/%d", hostID), map[string]any{"blocked": 0})
	if err != nil {
		return err
	}
	if code2 != 200 {
		return fmt.Errorf("host_unblock: HTTP %d/%d — %s / %s", code, code2, snippet(body), snippet(body2))
	}
	return nil
}
func (c *Client) HostTrust(hostID int, trusted bool) error {
	method := "DELETE"
	if trusted {
		method = "POST"
	}
	code, data, _, err := c.Write(method, fmt.Sprintf("/api/v1/untouchable/hosts/%d", hostID), nil)
	if err != nil {
		return err
	}
	if code != 200 && code != 201 && code != 204 {
		return fmt.Errorf("host_trust: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ─── Parental control ──────────────────────────────────────────────────────

// ParentalToggle enables/disables parental control globally. Mirrors the other
// resource-level toggles: PUT {enable}. The per-host block/unblock helpers live
// with the Hosts section (HostBlock/HostUnblock use /parentalcontrol/hosts).
func (c *Client) ParentalToggle(enable bool) error {
	code, data, _, err := c.Write("PUT", "/api/v1/parentalcontrol", map[string]any{"enable": boolInt(enable)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("parental_toggle: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ParentalScheduler returns the parental-control scheduler object
// ({enable, defaultpolicy, rules, savedRules}). Prefer this over Parental() when
// you need the pause windows — the base /parentalcontrol read omits them.
func (c *Client) ParentalScheduler() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/parentalcontrol/scheduler")
	if err != nil {
		return map[string]any{}, nil
	}
	return unwrap(unwrap(m, "parentalcontrol"), "scheduler"), nil
}

// ParentalSetPolicy sets the default access policy: "Forbidden" (block outside
// allowed windows) or "Accept". Verified: PUT /parentalcontrol {defaultpolicy}.
func (c *Client) ParentalSetPolicy(policy string) error {
	code, data, _, err := c.Write("PUT", "/api/v1/parentalcontrol", map[string]any{"defaultpolicy": policy})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("parental_set_policy: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ParentalRules returns the active and parked parental-control pause windows.
func (c *Client) ParentalRules() (rules, savedRules []any, err error) {
	s, err := c.ParentalScheduler()
	if err != nil {
		return nil, nil, err
	}
	return asList(s["rules"]), asList(s["savedRules"]), nil
}

// ParentalAddRule adds a parental-control access window and returns its new id.
func (c *Client) ParentalAddRule(a SchedulerRuleArgs) (int, error) {
	return c.addSchedulerRule("/api/v1/parentalcontrol/scheduler", a)
}

// ParentalDelRule removes a parental-control access window by id.
func (c *Client) ParentalDelRule(id int) error {
	return c.delSchedulerRule("/api/v1/parentalcontrol/scheduler", id)
}

// ParentalHostSet enrols (enable=true) or releases (enable=false) a device from
// parental control, keyed by MAC. Verified: PUT /parentalcontrol/hosts
// {enable, macaddress}. This is the per-device switch the Bbox app exposes;
// HostBlock/HostUnblock remain the coarse id-based helpers.
func (c *Client) ParentalHostSet(mac string, enable bool) error {
	code, data, _, err := c.Write("PUT", "/api/v1/parentalcontrol/hosts", map[string]any{
		"enable":     boolInt(enable),
		"macaddress": mac,
	})
	if err != nil {
		return err
	}
	if code != 200 && code != 201 {
		return fmt.Errorf("parental_host_set: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ─── WiFi ──────────────────────────────────────────────────────────────────

func (c *Client) Wifi() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/wireless")
	if err != nil {
		return nil, err
	}
	return unwrap(m, "wireless"), nil
}
func (c *Client) WifiBand(band string) (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/wireless/" + band)
	if err != nil {
		return map[string]any{}, nil
	}
	return unwrap(m, "wireless"), nil
}
func (c *Client) WifiGuest() (map[string]any, error) {
	return c.GetFirst("/api/v1/wireless/guest")
}

// GuestEnable returns {"enable": <0|1>, "radiostatus": <0|1>} for the guest
// WiFi. The plain `/wireless/guest` endpoint only carries SSID/passphrase; the
// on/off state lives on `/wireless/guestenable`.
func (c *Client) GuestEnable() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/wireless/guestenable")
	if err != nil {
		return map[string]any{}, nil
	}
	return unwrap(unwrap(m, "wireless"), "guest"), nil
}
func (c *Client) GuestKeySet(key string) error {
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/guest", map[string]any{"passphrase": key})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("guest_key_set: HTTP %d — %s", code, snippet(data))
	}
	return nil
}
func (c *Client) GuestSSIDSet(ssid string) error {
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/guest", map[string]any{"ssid": ssid})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("guest_ssid_set: HTTP %d — %s", code, snippet(data))
	}
	return nil
}
func (c *Client) WifiWPS() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/wireless/wps")
	if err != nil {
		return map[string]any{}, nil
	}
	return unwrap(m, "wps"), nil
}
func (c *Client) WifiACL() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/wireless/acl")
	if err != nil {
		return map[string]any{}, nil
	}
	return unwrap(m, "acl"), nil
}
func (c *Client) WifiScheduler() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/wireless/scheduler")
	if err != nil {
		return map[string]any{}, nil
	}
	return unwrap(unwrap(m, "wireless"), "scheduler"), nil
}
func (c *Client) WifiSchedulerOff() error {
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/scheduler", map[string]any{"enable": 0})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("wifi_scheduler_off: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// WifiSchedulerOn enables the wireless pause scheduler (the "WiFi pause" feature)
// without touching the configured windows. Exact mirror of WifiSchedulerOff.
func (c *Client) WifiSchedulerOn() error {
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/scheduler", map[string]any{"enable": 1})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("wifi_scheduler_on: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// WifiACLToggle enables/disables WiFi MAC access-control filtering globally.
// Mirrors the other resource-level toggles (UPnP/NAT/firewall): PUT {enable}.
func (c *Client) WifiACLToggle(enable bool) error {
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/acl", map[string]any{"enable": boolInt(enable)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("wifi_acl_toggle: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// WifiACLEnabled reports whether MAC access-control filtering is on.
func (c *Client) WifiACLEnabled() (bool, error) {
	acl, err := c.WifiACL()
	if err != nil {
		return false, err
	}
	return toBool(acl["enable"]), nil
}

// WifiACLRules returns the MAC access-control entries ([{id, enable, macaddress}]).
func (c *Client) WifiACLRules() ([]any, error) {
	acl, err := c.WifiACL()
	if err != nil {
		return nil, err
	}
	return asList(acl["rules"]), nil
}

// WifiACLAddRule adds a MAC to the access-control list and returns the new
// router-assigned rule id. enable controls whether the entry is active.
// Verified against firmware 25.3.20: POST /wireless/acl/rules → 201 + Location.
func (c *Client) WifiACLAddRule(mac string, enable bool) (int, error) {
	code, data, hdrs, err := c.Write("POST", "/api/v1/wireless/acl/rules", map[string]any{
		"macaddress": mac,
		"enable":     boolInt(enable),
	})
	if err != nil {
		return 0, err
	}
	if code != 200 && code != 201 {
		return 0, fmt.Errorf("wifi_acl_add_rule: HTTP %d — %s", code, snippet(data))
	}
	return locID(hdrs.Get("Location")), nil
}

// WifiACLDelRule removes an access-control entry by id.
func (c *Client) WifiACLDelRule(id int) error {
	code, data, _, err := c.Write("DELETE", fmt.Sprintf("/api/v1/wireless/acl/rules/%d", id), nil)
	if err != nil {
		return err
	}
	if code != 200 && code != 204 {
		return fmt.Errorf("wifi_acl_del_rule(%d): HTTP %d — %s", id, code, snippet(data))
	}
	return nil
}

// SchedulerRuleArgs describes a scheduler "pause" window. The wireless-pause and
// parental-control schedulers share this exact body shape (verified against the
// admin-UI bundle + live firmware 25.3.20).
type SchedulerRuleArgs struct {
	Name      string // human label for the pause window
	Occurency string // comma-separated day indices — Mon=1..Sat=6, Sun=0 (e.g. "1,2,3,4,5")
	Intervals string // "HH:MM,HH:MM" — start,end (e.g. "22:00,07:00")
	Enable    bool
}

// addSchedulerRule POSTs a pause window to a scheduler base path
// (…/wireless/scheduler or …/parentalcontrol/scheduler) and returns the new id.
func (c *Client) addSchedulerRule(base string, a SchedulerRuleArgs) (int, error) {
	code, data, hdrs, err := c.Write("POST", base+"/rule", map[string]any{
		"name":      a.Name,
		"occurency": a.Occurency,
		"intervals": a.Intervals,
		"enable":    boolInt(a.Enable),
	})
	if err != nil {
		return 0, err
	}
	if code != 200 && code != 201 {
		return 0, fmt.Errorf("scheduler_add_rule: HTTP %d — %s", code, snippet(data))
	}
	return locID(hdrs.Get("Location")), nil
}

// delSchedulerRule removes a pause window by id from a scheduler base path.
func (c *Client) delSchedulerRule(base string, id int) error {
	code, data, _, err := c.Write("DELETE", fmt.Sprintf("%s/rule/%d", base, id), nil)
	if err != nil {
		return err
	}
	if code != 200 && code != 204 {
		return fmt.Errorf("scheduler_del_rule(%d): HTTP %d — %s", id, code, snippet(data))
	}
	return nil
}

// WifiSchedulerRules returns the runtime and editable pause windows.
// savedRules is the editable set — {id, enable, name, occurency, intervals} —
// and is what you manage/delete by id. rules is a runtime-expanded timeline
// (per day-crossing, id -1, name null, start/end objects) present only while the
// scheduler is enabled; treat it as read-only. Adding an enabled window also
// flips the scheduler's master enable on.
func (c *Client) WifiSchedulerRules() (rules, savedRules []any, err error) {
	s, err := c.WifiScheduler()
	if err != nil {
		return nil, nil, err
	}
	return asList(s["rules"]), asList(s["savedRules"]), nil
}

// WifiSchedulerAddRule adds a WiFi-pause window and returns its new id.
func (c *Client) WifiSchedulerAddRule(a SchedulerRuleArgs) (int, error) {
	return c.addSchedulerRule("/api/v1/wireless/scheduler", a)
}

// WifiSchedulerDelRule removes a WiFi-pause window by id.
func (c *Client) WifiSchedulerDelRule(id int) error {
	return c.delSchedulerRule("/api/v1/wireless/scheduler", id)
}

func (c *Client) WifiBandToggle(band string, enable bool) error {
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/"+band, map[string]any{"radio.enable": boolInt(enable)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("wifi_band_toggle(%s): HTTP %d — %s", band, code, snippet(data))
	}
	return nil
}
func (c *Client) WifiGuestToggle(enable bool) error {
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/guestenable", map[string]any{"enable": boolInt(enable)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("wifi_guest_toggle: HTTP %d — %s", code, snippet(data))
	}
	return nil
}
func (c *Client) WifiAllToggle(enable bool) error {
	// 'deactivation' flag: enable=1 means DEACTIVATED. Invert.
	v := 1
	if enable {
		v = 0
	}
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/deactivation", map[string]any{"enable": v})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("wifi_all_toggle: HTTP %d — %s", code, snippet(data))
	}
	return nil
}
func (c *Client) WifiChannelSet(band, channel string) error {
	ch := 0
	if strings.ToLower(channel) != "auto" {
		n, err := strconv.Atoi(channel)
		if err != nil {
			return fmt.Errorf("invalid channel %q", channel)
		}
		ch = n
	}
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/"+band, map[string]any{"radio.channel": ch})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("wifi_channel_set(%s): HTTP %d — %s", band, code, snippet(data))
	}
	return nil
}
func (c *Client) WifiSSIDSet(band, ssid string) error {
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/"+band, map[string]any{"ssid": ssid})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("wifi_ssid_set(%s): HTTP %d — %s", band, code, snippet(data))
	}
	return nil
}
func (c *Client) WifiKeySet(band, key string) error {
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/"+band, map[string]any{"wpaKey": key})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("wifi_key_set(%s): HTTP %d — %s", band, code, snippet(data))
	}
	return nil
}
func (c *Client) WPSTrigger() error {
	code, data, _, err := c.Write("PUT", "/api/v1/wireless/wps", map[string]any{"enable": 1, "action": "push"})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("wps_trigger: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ─── DHCP ──────────────────────────────────────────────────────────────────

func (c *Client) DHCP() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/dhcp")
	if err != nil {
		return nil, err
	}
	return unwrap(m, "dhcp"), nil
}
func (c *Client) DHCPClients() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/dhcp/clients")
	if err != nil {
		return map[string]any{}, nil
	}
	return m, nil
}
func (c *Client) DHCPReserve(mac, ip, hostname string) error {
	code, data, _, err := c.Write("POST", "/api/v1/dhcp/clients", map[string]any{"enable": 1, "macaddress": mac, "ipaddress": ip, "hostname": hostname})
	if err != nil {
		return err
	}
	if code != 200 && code != 201 {
		return fmt.Errorf("dhcp_reserve: HTTP %d — %s", code, snippet(data))
	}
	return nil
}
func (c *Client) DHCPReservationDel(clientID int) error {
	code, data, _, err := c.Write("DELETE", fmt.Sprintf("/api/v1/dhcp/clients/%d", clientID), nil)
	if err != nil {
		return err
	}
	if code != 200 && code != 204 {
		return fmt.Errorf("dhcp_reservation_del: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ─── DynDNS ────────────────────────────────────────────────────────────────

func (c *Client) DynDNS() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/dyndns")
	if err != nil {
		return nil, err
	}
	return unwrap(m, "dyndns"), nil
}
func (c *Client) DynDNSEnable(provider, hostname, username, password string) error {
	code, data, _, err := c.Write("PUT", "/api/v1/dyndns", map[string]any{
		"enable": 1, "server": provider,
		"hostname": hostname, "username": username, "password": password,
	})
	if err != nil {
		return err
	}
	if code != 200 && code != 201 {
		return fmt.Errorf("dyndns_enable: HTTP %d — %s", code, snippet(data))
	}
	return nil
}
func (c *Client) DynDNSDisable() error {
	code, data, _, err := c.Write("PUT", "/api/v1/dyndns", map[string]any{"enable": 0})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("dyndns_disable: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ─── Hibernate ─────────────────────────────────────────────────────────────

func (c *Client) HibernateOff() error {
	code, data, _, err := c.Write("PUT", "/api/v1/hibernate/scheduler", map[string]any{"enable": 0})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("hibernate_off: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ─── Device ────────────────────────────────────────────────────────────────

func (c *Client) Reboot() error {
	var lastCode int
	var lastData []byte
	for _, method := range []string{"PUT", "POST"} {
		code, data, _, err := c.Write(method, "/api/v1/device/reboot", nil)
		if err != nil {
			return err
		}
		if code == 200 || code == 202 || code == 204 {
			return nil
		}
		lastCode, lastData = code, data
	}
	return fmt.Errorf("reboot: HTTP %d — %s", lastCode, snippet(lastData))
}
func (c *Client) LEDSet(luminosity int) error {
	code, data, _, err := c.Write("PUT", "/api/v1/device/display", map[string]any{"luminosity": luminosity})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("led_set: HTTP %d — %s", code, snippet(data))
	}
	return nil
}
func (c *Client) LogClear() error {
	code, data, _, err := c.Write("DELETE", "/api/v1/device/log", nil)
	if err != nil {
		return err
	}
	if code != 200 && code != 204 {
		return fmt.Errorf("log_clear: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// ─── VoIP (write side; VoIP() read lives in the READ section) ────────────────

// VoIPBlockAnonymous blocks (block=true) or allows anonymous / withheld-number
// calls on a line (1 or 2). Verified: PUT /voip/blockanon/{line} {block}.
func (c *Client) VoIPBlockAnonymous(line int, block bool) error {
	code, data, _, err := c.Write("PUT", fmt.Sprintf("/api/v1/voip/blockanon/%d", line), map[string]any{"block": boolInt(block)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("voip_block_anonymous(%d): HTTP %d — %s", line, code, snippet(data))
	}
	return nil
}

// VoIPAnonBlocked reports whether anonymous calls are blocked on a line. The
// state surfaces as the line's `blockstate` in /voip (the /voip/blockanon/{n}
// endpoint is write-only). Returns false if the line isn't found.
func (c *Client) VoIPAnonBlocked(line int) (bool, error) {
	lines, err := c.VoIP()
	if err != nil {
		return false, err
	}
	for _, lAny := range lines {
		l, _ := lAny.(map[string]any)
		if toInt(l["id"]) == line {
			return toBool(l["blockstate"]), nil
		}
	}
	return false, nil
}

// VoIPScheduler returns the VoIP call-scheduler object
// ({enable, unblock, rules, savedRules}).
func (c *Client) VoIPScheduler() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/voip/scheduler")
	if err != nil {
		return map[string]any{}, nil
	}
	return unwrap(unwrap(m, "voip"), "scheduler"), nil
}

// VoIPSchedulerUnblock flips the scheduler's `unblock` flag (the toggle the Bbox
// app uses): unblock=true lets calls through, overriding the configured windows.
func (c *Client) VoIPSchedulerUnblock(unblock bool) error {
	code, data, _, err := c.Write("PUT", "/api/v1/voip/scheduler", map[string]any{"unblock": boolInt(unblock)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("voip_scheduler_unblock: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// VoIPSchedulerRules returns the VoIP call-block windows (editable savedRules and
// the runtime rules), same shape as the WiFi/parental schedulers.
func (c *Client) VoIPSchedulerRules() (rules, savedRules []any, err error) {
	s, err := c.VoIPScheduler()
	if err != nil {
		return nil, nil, err
	}
	return asList(s["rules"]), asList(s["savedRules"]), nil
}

// VoIPSchedulerAddRule adds a call-block window and returns its new id.
func (c *Client) VoIPSchedulerAddRule(a SchedulerRuleArgs) (int, error) {
	return c.addSchedulerRule("/api/v1/voip/scheduler", a)
}

// VoIPSchedulerDelRule removes a call-block window by id.
func (c *Client) VoIPSchedulerDelRule(id int) error {
	return c.delSchedulerRule("/api/v1/voip/scheduler", id)
}

// ─── USB ───────────────────────────────────────────────────────────────────

// USB returns the USB subsystem state ({usb3:{enable}, parent, child}).
func (c *Client) USB() (map[string]any, error) {
	m, err := c.GetFirst("/api/v1/device/usb")
	if err != nil {
		return map[string]any{}, nil
	}
	return unwrap(m, "usb"), nil
}

// USB3Enabled reports whether USB 3.0 mode is on. (USB 3.0 can interfere with
// 2.4 GHz WiFi, so some setups keep it off.)
func (c *Client) USB3Enabled() (bool, error) {
	u, err := c.USB()
	if err != nil {
		return false, err
	}
	return toBool(unwrap(u, "usb3")["enable"]), nil
}

// USB3Toggle enables/disables USB 3.0 mode. Verified: PUT /device/usb3 {enable}.
func (c *Client) USB3Toggle(enable bool) error {
	code, data, _, err := c.Write("PUT", "/api/v1/device/usb3", map[string]any{"enable": boolInt(enable)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("usb3_toggle: HTTP %d — %s", code, snippet(data))
	}
	return nil
}

// USBPortToggle enables/disables a specific USB port. Verified from the admin
// bundle: PUT /device/usb/{port} {enable}.
func (c *Client) USBPortToggle(port string, enable bool) error {
	code, data, _, err := c.Write("PUT", "/api/v1/device/usb/"+port, map[string]any{"enable": boolInt(enable)})
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("usb_port_toggle(%s): HTTP %d — %s", port, code, snippet(data))
	}
	return nil
}

// ─── helpers ───────────────────────────────────────────────────────────────

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func toBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	case string:
		return x == "1" || strings.EqualFold(x, "on") || strings.EqualFold(x, "up") || strings.EqualFold(x, "true")
	}
	return false
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		n, _ := strconv.Atoi(x)
		return n
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	}
	return 0
}

func locID(loc string) int {
	if loc == "" {
		return -1
	}
	i := strings.LastIndex(loc, "/")
	if i < 0 {
		return -1
	}
	n, err := strconv.Atoi(loc[i+1:])
	if err != nil {
		return -1
	}
	return n
}
