package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hadamrd/bbox-cli/pkg/client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// desiredRule is one entry in the YAML input file.
type desiredRule struct {
	Name         string `yaml:"name"`
	ExternalPort int    `yaml:"external_port"`
	TargetIP     string `yaml:"target_ip"`
	InternalPort int    `yaml:"internal_port,omitempty"`
	Protocol     string `yaml:"protocol,omitempty"`
	RemoteIP     string `yaml:"remote_ip,omitempty"`
}

type natBatchFile struct {
	Rules []desiredRule `yaml:"rules"`
}

// existingRule is the normalized view of a router rule for diffing.
type existingRule struct {
	ID           int
	Name         string
	ExternalPort int
	TargetIP     string
	InternalPort int
	Protocol     string
	RemoteIP     string
}

type planOp int

const (
	opCreate planOp = iota
	opKeep
	opReplace
	opDelete
)

type planEntry struct {
	Op       planOp
	Name     string
	Desired  *desiredRule  // nil for pure delete
	Existing *existingRule // nil for pure create
	Reason   string        // for replace: what field(s) changed
}

var (
	natBatchFrom   string
	natBatchDryRun bool
	natBatchPrune  bool
	natBatchYes    bool
)

var natBatchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Apply NAT rules from a YAML file (idempotent create/replace/prune)",
	Long: `Reconcile the router's NAT rules against a YAML manifest. Missing rules are
created, mismatched rules are deleted+recreated, matching rules are left alone.
With --prune, rules whose names are absent from the YAML are deleted too.

Rule name (YAML 'name') maps to the router's 'description' field and is the
primary key for matching. Each planned create/replace is MAP-T-range-validated;
if any violation is found, the whole apply is refused.`,
	Example: `  # Preview
  bbox nat batch --from nat.yaml --dry-run

  # Apply, deleting rules missing from the YAML, no prompt
  bbox nat batch --from nat.yaml --prune -y`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if natBatchFrom == "" {
			return fmt.Errorf("--from is required")
		}
		data, err := os.ReadFile(natBatchFrom)
		if err != nil {
			return fmt.Errorf("read %s: %w", natBatchFrom, err)
		}
		var file natBatchFile
		if err := yaml.Unmarshal(data, &file); err != nil {
			return fmt.Errorf("parse %s: %w", natBatchFrom, err)
		}
		if err := validateDesired(file.Rules); err != nil {
			return err
		}
		normalized := normalizeDesired(file.Rules)

		if err := ensureAuth(); err != nil {
			return err
		}
		rawRules, err := c().NATRules()
		if err != nil {
			return err
		}
		current := parseCurrent(rawRules)

		plan := diffPlan(current, normalized, natBatchPrune)

		// MAP-T range validation across all create/replace targets.
		var violations []string
		wan, _ := c().WAN()
		lo, hi := readMAPTRange(wan)
		if lo > 0 && hi > 0 {
			for _, p := range plan {
				if p.Op == opCreate || p.Op == opReplace {
					ep := p.Desired.ExternalPort
					if ep < lo || ep > hi {
						violations = append(violations,
							fmt.Sprintf("  %s: external_port %d outside MAP-T range %d-%d", p.Name, ep, lo, hi))
					}
				}
			}
		}
		if len(violations) > 0 {
			return fmt.Errorf("MAP-T range violations — refusing to apply:\n%s", strings.Join(violations, "\n"))
		}

		if emit(map[string]any{"plan": plan2json(plan), "dry_run": natBatchDryRun}) {
			if natBatchDryRun {
				return nil
			}
		} else {
			printPlan(plan)
		}

		if natBatchDryRun {
			return nil
		}

		nCreate, nReplace, nDelete := 0, 0, 0
		for _, p := range plan {
			switch p.Op {
			case opCreate:
				nCreate++
			case opReplace:
				nReplace++
			case opDelete:
				nDelete++
			}
		}
		if nCreate+nReplace+nDelete == 0 {
			if !jsonOut {
				fmt.Println("nothing to do.")
			}
			return nil
		}
		if !natBatchYes && !jsonOut {
			fmt.Print("Confirm apply? [y/N] ")
			r := bufio.NewReader(os.Stdin)
			line, _ := r.ReadString('\n')
			line = strings.TrimSpace(strings.ToLower(line))
			if line != "y" && line != "yes" {
				fmt.Println("aborted")
				return nil
			}
		}

		return applyPlan(plan)
	},
}

func init() {
	natBatchCmd.Flags().StringVar(&natBatchFrom, "from", "", "YAML manifest (required)")
	natBatchCmd.Flags().BoolVar(&natBatchDryRun, "dry-run", false, "print plan without applying")
	natBatchCmd.Flags().BoolVar(&natBatchPrune, "prune", false, "delete existing rules whose name is absent from the YAML")
	natBatchCmd.Flags().BoolVarP(&natBatchYes, "yes", "y", false, "skip confirmation prompt")
	_ = natBatchCmd.MarkFlagRequired("from")
	natCmd.AddCommand(natBatchCmd)
}

// ─── plan-diff (pure) ──────────────────────────────────────────────────────

// normalizeDesired fills in defaults (internal_port, protocol) and lowercases
// protocol so downstream compares are stable.
func normalizeDesired(rules []desiredRule) []desiredRule {
	out := make([]desiredRule, len(rules))
	for i, r := range rules {
		if r.InternalPort == 0 {
			r.InternalPort = r.ExternalPort
		}
		if r.Protocol == "" {
			r.Protocol = "tcp"
		}
		r.Protocol = strings.ToLower(r.Protocol)
		out[i] = r
	}
	return out
}

func validateDesired(rules []desiredRule) error {
	seen := map[string]bool{}
	for i, r := range rules {
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("rules[%d]: name is required", i)
		}
		if seen[r.Name] {
			return fmt.Errorf("duplicate rule name %q", r.Name)
		}
		seen[r.Name] = true
		if r.ExternalPort <= 0 || r.ExternalPort > 65535 {
			return fmt.Errorf("rules[%d] %s: invalid external_port %d", i, r.Name, r.ExternalPort)
		}
		if strings.TrimSpace(r.TargetIP) == "" {
			return fmt.Errorf("rules[%d] %s: target_ip is required", i, r.Name)
		}
	}
	return nil
}

func parseCurrent(raw []any) []existingRule {
	out := make([]existingRule, 0, len(raw))
	for _, rAny := range raw {
		r, _ := rAny.(map[string]any)
		desc, _ := r["description"].(string)
		proto, _ := r["protocol"].(string)
		ip, _ := r["internalip"].(string)
		remote, _ := r["ipremote"].(string)
		out = append(out, existingRule{
			ID:           toIntAny(r["id"]),
			Name:         desc,
			ExternalPort: toIntAny(r["externalport"]),
			TargetIP:     ip,
			InternalPort: toIntAny(r["internalport"]),
			Protocol:     strings.ToLower(proto),
			RemoteIP:     remote,
		})
	}
	return out
}

// diffPlan produces an apply-style plan. Order: creates/keeps/replaces first
// (sorted by name), deletes last (sorted by name) so operators see additions
// before subtractions in the printout.
func diffPlan(current []existingRule, desired []desiredRule, prune bool) []planEntry {
	byName := map[string]*existingRule{}
	for i := range current {
		byName[current[i].Name] = &current[i]
	}
	seen := map[string]bool{}

	// Preserve desired order for creates/keeps/replaces so users can eyeball
	// their YAML top-to-bottom.
	var out []planEntry
	for i := range desired {
		d := desired[i]
		seen[d.Name] = true
		ex, ok := byName[d.Name]
		if !ok {
			out = append(out, planEntry{Op: opCreate, Name: d.Name, Desired: &desired[i]})
			continue
		}
		if reason := diffFields(ex, &d); reason != "" {
			out = append(out, planEntry{Op: opReplace, Name: d.Name, Desired: &desired[i], Existing: ex, Reason: reason})
		} else {
			out = append(out, planEntry{Op: opKeep, Name: d.Name, Desired: &desired[i], Existing: ex})
		}
	}
	if prune {
		var stale []existingRule
		for _, e := range current {
			if !seen[e.Name] {
				stale = append(stale, e)
			}
		}
		sort.Slice(stale, func(i, j int) bool { return stale[i].Name < stale[j].Name })
		for i := range stale {
			out = append(out, planEntry{Op: opDelete, Name: stale[i].Name, Existing: &stale[i]})
		}
	}
	return out
}

// diffFields returns a short reason string when e and d differ, "" when equal.
func diffFields(e *existingRule, d *desiredRule) string {
	var diffs []string
	if e.ExternalPort != d.ExternalPort {
		diffs = append(diffs, "port")
	}
	if e.TargetIP != d.TargetIP {
		diffs = append(diffs, "ip")
	}
	if e.InternalPort != d.InternalPort {
		diffs = append(diffs, "internal_port")
	}
	if e.Protocol != d.Protocol {
		diffs = append(diffs, "protocol")
	}
	if e.RemoteIP != d.RemoteIP {
		diffs = append(diffs, "remote_ip")
	}
	if len(diffs) == 0 {
		return ""
	}
	if len(diffs) == 1 {
		return diffs[0] + " changed"
	}
	return strings.Join(diffs, ",") + " changed"
}

// readMAPTRange extracts the WAN port-range from the wan payload. Returns
// (0, 0) if unavailable — caller should skip validation in that case.
func readMAPTRange(wan map[string]any) (int, int) {
	rng, _ := getMap(wan, "ip")["portrange"].(string)
	if rng == "" || !strings.Contains(rng, ":") {
		return 0, 0
	}
	parts := strings.SplitN(rng, ":", 2)
	lo := toIntAny(parts[0])
	hi := toIntAny(parts[1])
	return lo, hi
}

// ─── output + apply ────────────────────────────────────────────────────────

func printPlan(plan []planEntry) {
	fmt.Println("Plan:")
	if len(plan) == 0 {
		fmt.Println("  (empty)")
		return
	}
	for _, p := range plan {
		switch p.Op {
		case opCreate:
			d := p.Desired
			fmt.Printf("  + create   %-20s  %-3s   %d -> %s:%d\n",
				p.Name, d.Protocol, d.ExternalPort, d.TargetIP, d.InternalPort)
		case opKeep:
			d := p.Desired
			fmt.Printf("  = keep     %-20s  %-3s   %d -> %s:%d\n",
				p.Name, d.Protocol, d.ExternalPort, d.TargetIP, d.InternalPort)
		case opReplace:
			d := p.Desired
			fmt.Printf("  ~ replace  %-20s  %-3s   %d -> %s:%d   [%s]\n",
				p.Name, d.Protocol, d.ExternalPort, d.TargetIP, d.InternalPort, p.Reason)
		case opDelete:
			e := p.Existing
			fmt.Printf("  - delete   %-20s  %-3s   %d -> %s:%d   [pruned]\n",
				p.Name, e.Protocol, e.ExternalPort, e.TargetIP, e.InternalPort)
		}
	}
	fmt.Println()
}

func plan2json(plan []planEntry) []map[string]any {
	out := make([]map[string]any, 0, len(plan))
	for _, p := range plan {
		m := map[string]any{"name": p.Name, "op": opString(p.Op)}
		if p.Desired != nil {
			m["desired"] = map[string]any{
				"external_port": p.Desired.ExternalPort,
				"target_ip":     p.Desired.TargetIP,
				"internal_port": p.Desired.InternalPort,
				"protocol":      p.Desired.Protocol,
				"remote_ip":     p.Desired.RemoteIP,
			}
		}
		if p.Existing != nil {
			m["existing"] = map[string]any{
				"id":            p.Existing.ID,
				"external_port": p.Existing.ExternalPort,
				"target_ip":     p.Existing.TargetIP,
				"internal_port": p.Existing.InternalPort,
				"protocol":      p.Existing.Protocol,
				"remote_ip":     p.Existing.RemoteIP,
			}
		}
		if p.Reason != "" {
			m["reason"] = p.Reason
		}
		out = append(out, m)
	}
	return out
}

func opString(o planOp) string {
	switch o {
	case opCreate:
		return "create"
	case opKeep:
		return "keep"
	case opReplace:
		return "replace"
	case opDelete:
		return "delete"
	}
	return "?"
}

// applyPlan executes creates/replaces/deletes in order: deletes first (frees
// external ports the router may otherwise reject as duplicates), then replaces
// (delete+create), then creates.
func applyPlan(plan []planEntry) error {
	// Deletes (both pruned and the delete-half of replaces).
	for _, p := range plan {
		if p.Op == opDelete {
			if err := c().NATDel(p.Existing.ID); err != nil {
				return fmt.Errorf("delete %s (id=%d): %w", p.Name, p.Existing.ID, err)
			}
			fmt.Printf("  deleted   %s (id=%d)\n", p.Name, p.Existing.ID)
		}
	}
	for _, p := range plan {
		if p.Op == opReplace {
			if err := c().NATDel(p.Existing.ID); err != nil {
				return fmt.Errorf("replace(del) %s (id=%d): %w", p.Name, p.Existing.ID, err)
			}
			d := p.Desired
			id, err := c().NATAdd(client.NATAddArgs{
				Description:  d.Name,
				ExternalPort: d.ExternalPort,
				InternalIP:   d.TargetIP,
				InternalPort: d.InternalPort,
				Protocol:     d.Protocol,
				RemoteIP:     d.RemoteIP,
			})
			if err != nil {
				return fmt.Errorf("replace(add) %s: %w", p.Name, err)
			}
			fmt.Printf("  replaced  %s (new id=%d)\n", p.Name, id)
		}
	}
	for _, p := range plan {
		if p.Op == opCreate {
			d := p.Desired
			id, err := c().NATAdd(client.NATAddArgs{
				Description:  d.Name,
				ExternalPort: d.ExternalPort,
				InternalIP:   d.TargetIP,
				InternalPort: d.InternalPort,
				Protocol:     d.Protocol,
				RemoteIP:     d.RemoteIP,
			})
			if err != nil {
				return fmt.Errorf("create %s: %w", p.Name, err)
			}
			fmt.Printf("  created   %s (id=%d)\n", p.Name, id)
		}
	}
	return nil
}
