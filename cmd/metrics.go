package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	metricsListen   string
	metricsInterval time.Duration
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Prometheus text-format exporter (scrapes the router at --interval)",
	Long: `Serve /metrics in Prometheus exposition format. Scrapes the router every
--interval and caches the last successful result. /health returns 200 iff the
last scrape succeeded, 503 otherwise. No client_golang dep — the format is
simple enough to hand-write.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureAuth(); err != nil {
			return err
		}
		return runMetrics(metricsListen, metricsInterval)
	},
}

// scrape is the cached snapshot rendered on every /metrics request.
type scrape struct {
	Up          int
	WANState    int
	WANMapT     int
	UptimeSec   int
	HostsTotal  int
	HostsActive int
	NATRules    int
	FWRules     int
	DHCPLeases  int
	WifiBands   map[string]int // "24"|"5"|"6"|"guest" -> 0|1
	At          time.Time
}

func runMetrics(listen string, interval time.Duration) error {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	var mu sync.RWMutex
	var cur scrape

	doScrape := func() {
		s := collectScrape()
		mu.Lock()
		cur = s
		mu.Unlock()
	}
	// Prime the cache before serving so the first scrape observer isn't empty.
	doScrape()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Poller.
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				doScrape()
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		s := cur
		mu.RUnlock()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(renderPrometheus(s)))
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		up := cur.Up
		mu.RUnlock()
		if up == 1 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok\n"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("scrape failed\n"))
	})

	srv := &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	fmt.Printf("listening on http://%s/metrics (scrape every %s)\n", listen, interval)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

// collectScrape runs one poll cycle. Any endpoint failure marks Up=0 but we
// still return whatever partial info we got.
func collectScrape() scrape {
	s := scrape{At: time.Now(), Up: 1, WifiBands: map[string]int{}}
	dev, err := c().Device()
	if err != nil {
		fmt.Fprintf(os.Stderr, "scrape: device: %v\n", err)
		s.Up = 0
	} else {
		s.UptimeSec = toIntAny(dev["uptime"])
	}
	wan, err := c().WAN()
	if err != nil {
		fmt.Fprintf(os.Stderr, "scrape: wan: %v\n", err)
		s.Up = 0
	} else {
		ip := getMap(wan, "ip")
		if strings.EqualFold(toStr(ip["state"]), "up") {
			s.WANState = 1
		}
		if toBoolAny(ip["maptenable"]) {
			s.WANMapT = 1
		}
	}
	if hosts, err := c().Hosts(); err != nil {
		fmt.Fprintf(os.Stderr, "scrape: hosts: %v\n", err)
		s.Up = 0
	} else {
		s.HostsTotal = len(hosts)
		for _, hAny := range hosts {
			h, _ := hAny.(map[string]any)
			if toBoolAny(h["active"]) {
				s.HostsActive++
			}
		}
	}
	if rules, err := c().NATRules(); err != nil {
		fmt.Fprintf(os.Stderr, "scrape: nat: %v\n", err)
		s.Up = 0
	} else {
		s.NATRules = len(rules)
	}
	if rules, err := c().FirewallRules(); err != nil {
		fmt.Fprintf(os.Stderr, "scrape: firewall: %v\n", err)
		s.Up = 0
	} else {
		s.FWRules = len(rules)
	}
	if dhcp, err := c().DHCPClients(); err == nil {
		// Count all top-level list entries as leases.
		for _, v := range dhcp {
			if arr, ok := v.([]any); ok {
				s.DHCPLeases += len(arr)
			}
		}
	}
	// wifi bands
	for _, band := range []string{"24", "5", "6"} {
		if m, err := c().WifiBand(band); err == nil {
			// {"radio": {"enable": 1}} or {"enable": 1}
			radio := getMap(m, "radio")
			en := radio["enable"]
			if en == nil {
				en = m["enable"]
			}
			if toBoolAny(en) {
				s.WifiBands[band] = 1
			}
		}
	}
	if g, err := c().GuestEnable(); err == nil {
		if toBoolAny(g["enable"]) {
			s.WifiBands["guest"] = 1
		}
	}
	return s
}

// renderPrometheus builds the exposition format by hand. Keeps the deps light.
func renderPrometheus(s scrape) string {
	var b strings.Builder
	line := func(help, name, typ string, val int) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n%s %d\n", name, help, name, typ, name, val)
	}
	line("Last scrape succeeded (0|1)", "bbox_up", "gauge", s.Up)
	line("WAN state is Up (0|1)", "bbox_wan_state", "gauge", s.WANState)
	line("MAP-T enabled (0|1)", "bbox_wan_map_t", "gauge", s.WANMapT)
	line("Router uptime seconds", "bbox_uptime_seconds", "counter", s.UptimeSec)
	line("Total known LAN hosts", "bbox_hosts_total", "gauge", s.HostsTotal)
	line("Currently active LAN hosts", "bbox_hosts_active", "gauge", s.HostsActive)
	line("NAT rule count", "bbox_nat_rules_total", "gauge", s.NATRules)
	line("Firewall rule count", "bbox_firewall_rules_total", "gauge", s.FWRules)
	line("DHCP lease/reservation count", "bbox_dhcp_leases_total", "gauge", s.DHCPLeases)

	// wifi band series - one HELP/TYPE, N samples.
	fmt.Fprintln(&b, "# HELP bbox_wifi_band_enabled Radio enabled per band (0|1)")
	fmt.Fprintln(&b, "# TYPE bbox_wifi_band_enabled gauge")
	for _, band := range []string{"24", "5", "6", "guest"} {
		fmt.Fprintf(&b, "bbox_wifi_band_enabled{band=%q} %d\n", band, s.WifiBands[band])
	}
	return b.String()
}

func init() {
	metricsCmd.Flags().StringVar(&metricsListen, "listen", "127.0.0.1:9100", "HTTP bind address")
	metricsCmd.Flags().DurationVar(&metricsInterval, "interval", 30*time.Second, "scrape interval")
	rootCmd.AddCommand(metricsCmd)
}
