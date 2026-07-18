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
	code, _, hdrs, err := c.Write("POST", "/api/v1/nat/rules", map[string]any{
		"enable":        1,
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
