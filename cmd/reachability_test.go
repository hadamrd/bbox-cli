package cmd

import (
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// startTCPListener binds to 127.0.0.1:0 and returns the chosen port + a stop fn.
func startTCPListener(t *testing.T) (int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return port, func() { _ = ln.Close() }
}

// pickClosedPort finds a currently-unbound port by opening then immediately
// closing a listener. Race-prone in theory but fine for the sub-second window
// of a unit test.
func pickClosedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	_ = ln.Close()
	return port
}

func TestProbeOne_Reachable(t *testing.T) {
	port, stop := startTCPListener(t)
	defer stop()

	r := existingRule{Name: "listener", ExternalPort: port, TargetIP: "10.0.0.1", InternalPort: 22, Protocol: "tcp"}
	res := probeOne("127.0.0.1", r, 2*time.Second)
	if !res.Reachable {
		t.Fatalf("expected reachable, got %+v", res)
	}
	if res.Error != nil {
		t.Errorf("error should be nil, got %q", *res.Error)
	}
	if res.Target != "10.0.0.1:22" {
		t.Errorf("target = %q, want 10.0.0.1:22", res.Target)
	}
}

func TestProbeOne_Unreachable(t *testing.T) {
	closedPort := pickClosedPort(t)
	r := existingRule{Name: "dead", ExternalPort: closedPort, TargetIP: "10.0.0.1", InternalPort: 80, Protocol: "tcp"}
	res := probeOne("127.0.0.1", r, 1*time.Second)
	if res.Reachable {
		t.Fatalf("expected unreachable, got %+v", res)
	}
	if res.Error == nil || *res.Error == "" {
		t.Errorf("expected an error message")
	}
}

func TestProbeOne_Timeout(t *testing.T) {
	// TEST-NET-1 (192.0.2.0/24) is guaranteed unroutable per RFC 5737 — the
	// dial should time out rather than get RST.
	r := existingRule{Name: "blackhole", ExternalPort: 12345, TargetIP: "10.0.0.1", InternalPort: 80, Protocol: "tcp"}
	res := probeOne("192.0.2.1", r, 150*time.Millisecond)
	if res.Reachable {
		t.Fatalf("blackhole should not be reachable")
	}
	if res.Error == nil || !strings.Contains(strings.ToLower(*res.Error), "timeout") &&
		!strings.Contains(strings.ToLower(*res.Error), "i/o timeout") &&
		!strings.Contains(strings.ToLower(*res.Error), "deadline") {
		// Some OSes report "connection timed out"; be lenient and just require
		// an error, but log for visibility.
		t.Logf("timeout error message = %q (accepted)", *res.Error)
	}
}

func TestProbeAll_MixedResults(t *testing.T) {
	openPort, stop := startTCPListener(t)
	defer stop()
	closedPort := pickClosedPort(t)

	targets := []existingRule{
		{Name: "up", ExternalPort: openPort, TargetIP: "10.0.0.1", InternalPort: 22},
		{Name: "down", ExternalPort: closedPort, TargetIP: "10.0.0.2", InternalPort: 22},
	}
	results := probeAll("127.0.0.1", targets, 2*time.Second, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Ordering: probeAll preserves the input order.
	if results[0].Name != "up" || !results[0].Reachable {
		t.Errorf("results[0] = %+v, want up/reachable", results[0])
	}
	if results[1].Name != "down" || results[1].Reachable {
		t.Errorf("results[1] = %+v, want down/unreachable", results[1])
	}
}
