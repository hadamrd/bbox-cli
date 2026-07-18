package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// completeHost is a cobra ValidArgsFunction that offers LAN hostnames + IPs
// pulled from GET /api/v1/hosts. Only the first positional argument is
// completed. Cobra swallows errors during completion, so failures return
// ShellCompDirectiveError.
func completeHost(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if bx == nil {
		return nil, cobra.ShellCompDirectiveError
	}
	if err := ensureAuth(); err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	hosts, err := c().Hosts()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var out []string
	for _, hAny := range hosts {
		h, _ := hAny.(map[string]any)
		if s, _ := h["hostname"].(string); s != "" {
			out = append(out, s)
		}
		if s, _ := h["ipaddress"].(string); s != "" {
			out = append(out, s)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeNATRule offers descriptions + numeric IDs of existing NAT rules.
func completeNATRule(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if bx == nil {
		return nil, cobra.ShellCompDirectiveError
	}
	if err := ensureAuth(); err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	rules, err := c().NATRules()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var out []string
	for _, rAny := range rules {
		r, _ := rAny.(map[string]any)
		if s, _ := r["description"].(string); s != "" {
			out = append(out, s)
		}
		if id := toIntAny(r["id"]); id != 0 {
			out = append(out, strconv.Itoa(id))
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeNATDescription offers only descriptions (used by retrobot teardown).
func completeNATDescription(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if bx == nil {
		return nil, cobra.ShellCompDirectiveError
	}
	if err := ensureAuth(); err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	rules, err := c().NATRules()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var out []string
	for _, rAny := range rules {
		r, _ := rAny.(map[string]any)
		if s, _ := r["description"].(string); s != "" {
			out = append(out, s)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeFirewallRuleID offers numeric firewall rule IDs.
func completeFirewallRuleID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if bx == nil {
		return nil, cobra.ShellCompDirectiveError
	}
	if err := ensureAuth(); err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	rules, err := c().FirewallRules()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var out []string
	for _, rAny := range rules {
		r, _ := rAny.(map[string]any)
		if id := toIntAny(r["id"]); id != 0 {
			desc, _ := r["description"].(string)
			if desc != "" {
				out = append(out, fmt.Sprintf("%d\t%s", id, desc))
			} else {
				out = append(out, strconv.Itoa(id))
			}
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	hostRenameCmd.ValidArgsFunction = completeHost
	hostBlockCmd.ValidArgsFunction = completeHost
	hostUnblockCmd.ValidArgsFunction = completeHost
	natDelCmd.ValidArgsFunction = completeNATRule
	firewallDelCmd.ValidArgsFunction = completeFirewallRuleID
	firewallToggleRuleCmd.ValidArgsFunction = completeFirewallRuleID
	retrobotTeardownCmd.ValidArgsFunction = completeNATDescription
}
