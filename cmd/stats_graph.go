package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// statsSample is one row of the ~/.bbox-stats.jsonl history file.
type statsSample struct {
	At      string `json:"at"`
	RxBytes int64  `json:"rx_bytes"`
	TxBytes int64  `json:"tx_bytes"`
}

// sparkBlocks is the 8-step block-character ramp used by classic `spark`.
var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// sparkline renders a series as a string of block characters. Empty series -> "".
// Constant series -> all lowest block. NaN-free int64 input.
func sparkline(series []int64) string {
	if len(series) == 0 {
		return ""
	}
	min, max := series[0], series[0]
	for _, v := range series {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	rng := max - min
	var b strings.Builder
	for _, v := range series {
		var idx int
		if rng == 0 {
			idx = 0
		} else {
			// scale into [0, len(sparkBlocks)-1]
			idx = int(float64(v-min) / float64(rng) * float64(len(sparkBlocks)-1))
			if idx < 0 {
				idx = 0
			}
			if idx > len(sparkBlocks)-1 {
				idx = len(sparkBlocks) - 1
			}
		}
		b.WriteRune(sparkBlocks[idx])
	}
	return b.String()
}

// defaultHistoryPath is ~/.bbox-stats.jsonl (best-effort; empty on failure).
func defaultHistoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".bbox-stats.jsonl")
}

// appendStatsSample writes one JSONL record to path (created if missing).
func appendStatsSample(path string, s statsSample) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

// readStatsHistory returns the last `tail` samples from path (all if tail<=0).
func readStatsHistory(path string, tail int) ([]statsSample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []statsSample
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var s statsSample
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			continue // skip corrupt lines
		}
		out = append(out, s)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if tail > 0 && len(out) > tail {
		out = out[len(out)-tail:]
	}
	return out, nil
}

// extractWANBytes walks the loose WANStats map and returns (rx, tx) bytes.
// The Bbox nests counters under wan.ip.stats.rx.bytes / .tx.bytes; also try
// flat rxbytes/txbytes as a fallback.
func extractWANBytes(m map[string]any) (int64, int64) {
	// nested: {"wan":{"ip":{"stats":{"rx":{"bytes":..},"tx":{"bytes":..}}}}}
	if wan, ok := m["wan"].(map[string]any); ok {
		if ip, ok := wan["ip"].(map[string]any); ok {
			if st, ok := ip["stats"].(map[string]any); ok {
				return pickBytes(st, "rx"), pickBytes(st, "tx")
			}
		}
	}
	// direct: {"rx":{"bytes":..},"tx":{"bytes":..}}
	if _, ok := m["rx"]; ok {
		return pickBytes(m, "rx"), pickBytes(m, "tx")
	}
	// flat: {"rxbytes":..,"txbytes":..}
	return int64(toIntAny(m["rxbytes"])), int64(toIntAny(m["txbytes"]))
}

func pickBytes(m map[string]any, dir string) int64 {
	if sub, ok := m[dir].(map[string]any); ok {
		return int64(toIntAny(sub["bytes"]))
	}
	return int64(toIntAny(m[dir+"bytes"]))
}

// renderStatsGraph builds the human-readable sparkline block. Assumes >= 2 samples.
func renderStatsGraph(samples []statsSample) string {
	rx := make([]int64, len(samples))
	tx := make([]int64, len(samples))
	for i, s := range samples {
		rx[i] = s.RxBytes
		tx[i] = s.TxBytes
	}
	rxMin, rxMax := minMax(rx)
	txMin, txMax := minMax(tx)
	var b strings.Builder
	fmt.Fprintf(&b, "WAN throughput (last %d samples)\n", len(samples))
	fmt.Fprintf(&b, "  rx  %s\n", sparkline(rx))
	fmt.Fprintf(&b, "      min=%d max=%d last=%d\n", rxMin, rxMax, rx[len(rx)-1])
	fmt.Fprintf(&b, "  tx  %s\n", sparkline(tx))
	fmt.Fprintf(&b, "      min=%d max=%d last=%d\n", txMin, txMax, tx[len(tx)-1])
	return b.String()
}

func minMax(xs []int64) (int64, int64) {
	if len(xs) == 0 {
		return 0, 0
	}
	mn, mx := xs[0], xs[0]
	for _, v := range xs {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	return mn, mx
}

// nowUTCRFC3339 returns the current UTC timestamp for the JSONL "at" field.
func nowUTCRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
