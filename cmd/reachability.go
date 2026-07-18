package cmd

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

var (
	reachTimeout    time.Duration
	reachConcurrent int
)

// reachResult is one probe outcome. External `json` tags define the --json shape.
type reachResult struct {
	Name         string  `json:"name"`
	ExternalPort int     `json:"external_port"`
	Target       string  `json:"target"`
	Reachable    bool    `json:"reachable"`
	LatencyMS    int64   `json:"latency_ms"`
	Error        *string `json:"error"`
}

var reachabilityCmd = &cobra.Command{
	Use:   "reachability",
	Short: "TCP-probe every NAT rule via the WAN IP",
	Long: `For every configured NAT rule, TCP-dial the WAN IP on the external port and
report whether it accepted the connection.

Note: probing from the LAN. If the Bbox hairpins (default: yes on Bouygues
firmware), 'reachable' matches external reachability. If hairpin is off, expect
false negatives — verify from an outside network.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		wan, err := c().WAN()
		if err != nil {
			return err
		}
		wanIP, _ := getMap(wan, "ip")["address"].(string)
		if wanIP == "" {
			return fmt.Errorf("could not determine WAN IP from /api/v1/wan/ip")
		}
		rules, err := c().NATRules()
		if err != nil {
			return err
		}
		targets := parseCurrent(rules)
		sort.Slice(targets, func(i, j int) bool { return targets[i].ExternalPort < targets[j].ExternalPort })

		results := probeAll(wanIP, targets, reachTimeout, reachConcurrent)

		if emit(results) {
			return nil
		}
		fmt.Printf("Reachability (WAN %s)\n", wanIP)
		fmt.Println("Note: probing from LAN. If the Bbox does hairpin NAT (default: yes),")
		fmt.Println("      'reachable' means external reachability. If it doesn't, false negatives.")
		fmt.Println()
		if len(results) == 0 {
			fmt.Println("(no NAT rules configured)")
			return nil
		}
		nameW := 0
		for _, r := range results {
			if len(r.Name) > nameW {
				nameW = len(r.Name)
			}
		}
		for _, r := range results {
			status := "[ OK ]"
			extra := fmt.Sprintf("%.2fs", float64(r.LatencyMS)/1000)
			if !r.Reachable {
				status = "[FAIL]"
				if r.Error != nil {
					extra = *r.Error
				}
			}
			fmt.Printf("  %-*s  %5d  ->  %-22s  %s   %s\n",
				nameW, r.Name, r.ExternalPort, r.Target, status, extra)
		}
		return nil
	},
}

func init() {
	reachabilityCmd.Flags().DurationVar(&reachTimeout, "timeout", 3*time.Second, "per-probe TCP dial timeout")
	reachabilityCmd.Flags().IntVar(&reachConcurrent, "concurrent", 4, "parallel probes")
	rootCmd.AddCommand(reachabilityCmd)
}

// probeAll runs TCP-dial probes with a bounded worker pool. Results are
// returned in the same order as `targets`.
func probeAll(wanIP string, targets []existingRule, timeout time.Duration, concurrent int) []reachResult {
	if concurrent < 1 {
		concurrent = 1
	}
	results := make([]reachResult, len(targets))
	sem := make(chan struct{}, concurrent)
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, t existingRule) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = probeOne(wanIP, t, timeout)
		}(i, t)
	}
	wg.Wait()
	return results
}

// probeOne TCP-dials WAN_IP:external_port. Success = connection accepted before
// the timeout. Error strings are shortened to net.OpError.Err.Error() where
// possible so the human output stays one line.
func probeOne(wanIP string, r existingRule, timeout time.Duration) reachResult {
	addr := net.JoinHostPort(wanIP, fmt.Sprintf("%d", r.ExternalPort))
	target := fmt.Sprintf("%s:%d", r.TargetIP, r.InternalPort)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	lat := time.Since(start).Milliseconds()
	if err != nil {
		msg := err.Error()
		if op, ok := err.(*net.OpError); ok && op.Err != nil {
			msg = op.Err.Error()
		}
		return reachResult{
			Name: r.Name, ExternalPort: r.ExternalPort, Target: target,
			Reachable: false, LatencyMS: lat, Error: &msg,
		}
	}
	_ = conn.Close()
	return reachResult{
		Name: r.Name, ExternalPort: r.ExternalPort, Target: target,
		Reachable: true, LatencyMS: lat, Error: nil,
	}
}
