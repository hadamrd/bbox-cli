package cmd

import (
	"strings"
	"testing"
)

func TestSparkline(t *testing.T) {
	// Series from the task brief. min=1, max=8, range=7.
	// Expected mapping into 8 blocks (▁▂▃▄▅▆▇█):
	//   1 -> idx 0 = ▁
	//   3 -> idx 2 = ▃
	//   7 -> idx 6 = ▇
	//   5 -> idx 4 = ▅
	//   2 -> idx 1 = ▂
	//   8 -> idx 7 = █
	//   4 -> idx 3 = ▄
	got := sparkline([]int64{1, 3, 7, 5, 2, 8, 4})
	want := "▁▃▇▅▂█▄"
	if got != want {
		t.Errorf("sparkline([1,3,7,5,2,8,4]) = %q, want %q", got, want)
	}

	// Empty series -> empty string.
	if got := sparkline(nil); got != "" {
		t.Errorf("sparkline(nil) = %q, want empty", got)
	}

	// Constant series -> all lowest block.
	got = sparkline([]int64{5, 5, 5, 5})
	if got != strings.Repeat("▁", 4) {
		t.Errorf("sparkline(constant) = %q, want 4x lowest block", got)
	}

	// Two-element ascending -> min then max block.
	got = sparkline([]int64{0, 100})
	if got != "▁█" {
		t.Errorf("sparkline([0,100]) = %q, want ▁█", got)
	}
}

func TestRenderStatsGraph(t *testing.T) {
	samples := []statsSample{
		{At: "t1", RxBytes: 100, TxBytes: 10},
		{At: "t2", RxBytes: 200, TxBytes: 40},
		{At: "t3", RxBytes: 150, TxBytes: 20},
	}
	out := renderStatsGraph(samples)
	if !strings.Contains(out, "WAN throughput (last 3 samples)") {
		t.Errorf("missing header: %q", out)
	}
	if !strings.Contains(out, "min=100 max=200 last=150") {
		t.Errorf("missing rx footer in %q", out)
	}
	if !strings.Contains(out, "min=10 max=40 last=20") {
		t.Errorf("missing tx footer in %q", out)
	}
}
