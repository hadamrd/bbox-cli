package cmd

import (
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	wolInterface string
	wolBroadcast string
	wolPort      int
	wolRepeat    int
)

var hostWolCmd = &cobra.Command{
	Use:     "wake-on-lan MAC",
	Aliases: []string{"wol", "wake"},
	Short:   "Send a Wake-on-LAN magic packet",
	Long: `Send a Wake-on-LAN magic packet (6× 0xFF followed by 16× the target MAC) via
UDP broadcast. The MAC may be given in aa:bb:cc:dd:ee:ff, AA-BB-CC-DD-EE-FF or
aabbccddeeff form. If the argument is not a MAC, it is resolved via the host
table (same as 'bbox host rename/block/unblock') and the MAC of the matching
host is used.`,
	Example: `  bbox host wol aa:bb:cc:dd:ee:ff
  bbox host wake DESKTOP-J18FV15
  bbox host wol aa-bb-cc-dd-ee-ff --broadcast 192.168.1.255 --port 7 --repeat 5`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		arg := args[0]
		mac, err := normalizeMAC(arg)
		var displayHost string
		if err != nil {
			// Fall back to host lookup — needs auth.
			if err := ensureAuth(); err != nil {
				return err
			}
			h, err := c().HostBy(arg)
			if err != nil {
				return err
			}
			hostMac, _ := h["macaddress"].(string)
			mac, err = normalizeMAC(hostMac)
			if err != nil {
				return fmt.Errorf("host %q has invalid MAC %q: %v", arg, hostMac, err)
			}
			if hn, _ := h["hostname"].(string); hn != "" {
				displayHost = hn
			}
		}

		pkt, err := buildMagicPacket(mac)
		if err != nil {
			return err
		}
		if wolRepeat < 1 {
			wolRepeat = 1
		}
		if wolPort <= 0 || wolPort > 65535 {
			return fmt.Errorf("invalid --port %d", wolPort)
		}

		laddr := &net.UDPAddr{IP: net.IPv4zero, Port: 0}
		if wolInterface != "" {
			ip := net.ParseIP(wolInterface)
			if ip == nil {
				return fmt.Errorf("invalid --interface %q (expected IP)", wolInterface)
			}
			laddr = &net.UDPAddr{IP: ip, Port: 0}
		}
		bcast := net.ParseIP(wolBroadcast)
		if bcast == nil {
			return fmt.Errorf("invalid --broadcast %q", wolBroadcast)
		}
		raddr := &net.UDPAddr{IP: bcast, Port: wolPort}

		conn, err := net.DialUDP("udp", laddr, raddr)
		if err != nil {
			return fmt.Errorf("udp dial: %w", err)
		}
		defer conn.Close()

		for i := 0; i < wolRepeat; i++ {
			if _, err := conn.Write(pkt); err != nil {
				return fmt.Errorf("udp write: %w", err)
			}
			if i < wolRepeat-1 {
				time.Sleep(100 * time.Millisecond)
			}
		}

		prettyMAC := formatMAC(mac)
		src := "0.0.0.0"
		if wolInterface != "" {
			src = wolInterface
		}
		if displayHost != "" {
			fmt.Printf("OK: sent %d WoL packet(s) to %s (%s) (%s → %s:%d)\n",
				wolRepeat, prettyMAC, displayHost, src, bcast.String(), wolPort)
		} else {
			fmt.Printf("OK: sent %d WoL packet(s) to %s (%s → %s:%d)\n",
				wolRepeat, prettyMAC, src, bcast.String(), wolPort)
		}
		return nil
	},
}

// normalizeMAC accepts aa:bb:cc:dd:ee:ff, AA-BB-CC-DD-EE-FF or aabbccddeeff
// and returns the 6-byte MAC. Returns error on any other shape.
func normalizeMAC(s string) ([]byte, error) {
	stripped := strings.Map(func(r rune) rune {
		switch r {
		case ':', '-', '.', ' ':
			return -1
		}
		return r
	}, s)
	if len(stripped) != 12 {
		return nil, fmt.Errorf("invalid MAC %q: want 12 hex chars, got %d", s, len(stripped))
	}
	if !regexp.MustCompile(`^[0-9a-fA-F]{12}$`).MatchString(stripped) {
		return nil, fmt.Errorf("invalid MAC %q: non-hex characters", s)
	}
	b, err := hex.DecodeString(stripped)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC %q: %v", s, err)
	}
	return b, nil
}

// formatMAC renders the 6-byte MAC as aa:bb:cc:dd:ee:ff.
func formatMAC(b []byte) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5])
}

// buildMagicPacket returns the 102-byte WoL payload: 6× 0xFF followed by
// 16× the target MAC.
func buildMagicPacket(mac []byte) ([]byte, error) {
	if len(mac) != 6 {
		return nil, fmt.Errorf("magic packet: MAC must be 6 bytes, got %d", len(mac))
	}
	pkt := make([]byte, 0, 102)
	for i := 0; i < 6; i++ {
		pkt = append(pkt, 0xFF)
	}
	for i := 0; i < 16; i++ {
		pkt = append(pkt, mac...)
	}
	return pkt, nil
}

func init() {
	hostWolCmd.Flags().StringVar(&wolInterface, "interface", "", "local interface IP to send from (default: any / 0.0.0.0)")
	hostWolCmd.Flags().StringVar(&wolBroadcast, "broadcast", "255.255.255.255", "broadcast address to send to")
	hostWolCmd.Flags().IntVar(&wolPort, "port", 9, "UDP port (some devices use 7)")
	hostWolCmd.Flags().IntVar(&wolRepeat, "repeat", 3, "send the packet N times, 100ms apart")
	hostWolCmd.ValidArgsFunction = completeHost
	hostCmd.AddCommand(hostWolCmd)
}
