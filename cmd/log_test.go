package cmd

import (
	"testing"
)

func TestFormatLogParam(t *testing.T) {
	cases := []struct {
		event, param, want string
	}{
		{"DEVICE_UP", "68:72:c3:dc:9f:9f;192.168.1.152;Samsung", "mac=68:72:c3:dc:9f:9f ip=192.168.1.152 host=Samsung"},
		{"DEVICE_DOWN", "aa:bb:cc:dd:ee:ff;192.168.1.10;laptop", "mac=aa:bb:cc:dd:ee:ff ip=192.168.1.10 host=laptop"},
		{"LOGIN_LOCAL_LOCKED", "192.168.1.125;13;55;DESKTOP-J18FV15", "ip=192.168.1.125 attempts=13 lockout=55s host=DESKTOP-J18FV15"},
		{"device_up", "mac;ip;host", "mac=mac ip=ip host=host"}, // case-insensitive event
		{"UNKNOWN_EVENT", "some;raw;param", "some;raw;param"},
		{"DEVICE_UP", "malformed", "malformed"}, // fallback when split fails
	}
	for _, tc := range cases {
		got := formatLogParam(tc.event, tc.param)
		if got != tc.want {
			t.Errorf("formatLogParam(%q, %q) = %q, want %q", tc.event, tc.param, got, tc.want)
		}
	}
}

func TestFilterLogs(t *testing.T) {
	entries := []any{
		map[string]any{"date": "1", "log": "DEVICE_UP", "param": "aa;192.168.1.10;alice"},
		map[string]any{"date": "2", "log": "DEVICE_DOWN", "param": "bb;192.168.1.11;bob"},
		map[string]any{"date": "3", "log": "LOGIN_LOCAL_LOCKED", "param": "192.168.1.99;3;30;charlie"},
		map[string]any{"date": "4", "log": "WEIRD_EVENT", "param": "ALICE-was-here"},
	}

	// No filters -> passthrough.
	if got := filterLogs(entries, "", nil); len(got) != 4 {
		t.Errorf("no-filter: got %d, want 4", len(got))
	}

	// --type single.
	if got := filterLogs(entries, "", []string{"DEVICE_UP"}); len(got) != 1 {
		t.Errorf("type=DEVICE_UP: got %d, want 1", len(got))
	}

	// --type case-insensitive.
	if got := filterLogs(entries, "", []string{"device_up"}); len(got) != 1 {
		t.Errorf("type=device_up (lc): got %d, want 1", len(got))
	}

	// --type multiple (OR).
	if got := filterLogs(entries, "", []string{"DEVICE_UP", "DEVICE_DOWN"}); len(got) != 2 {
		t.Errorf("type OR: got %d, want 2", len(got))
	}

	// --search case-insensitive across event+param.
	if got := filterLogs(entries, "alice", nil); len(got) != 2 {
		t.Errorf("search=alice: got %d, want 2", len(got))
	}

	// --search matches event name.
	if got := filterLogs(entries, "LOCKED", nil); len(got) != 1 {
		t.Errorf("search=LOCKED: got %d, want 1", len(got))
	}

	// --search AND --type: type OR narrows first, search filters within.
	got := filterLogs(entries, "alice", []string{"DEVICE_UP", "DEVICE_DOWN", "WEIRD_EVENT"})
	if len(got) != 2 {
		t.Errorf("search=alice type={up,down,weird}: got %d, want 2", len(got))
	}

	// AND semantic: search misses when type excludes.
	if got := filterLogs(entries, "alice", []string{"LOGIN_LOCAL_LOCKED"}); len(got) != 0 {
		t.Errorf("search=alice type=locked: got %d, want 0", len(got))
	}
}
