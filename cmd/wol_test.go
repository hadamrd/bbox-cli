package cmd

import (
	"bytes"
	"testing"
)

func TestNormalizeMAC_Formats(t *testing.T) {
	want := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	cases := []string{
		"aa:bb:cc:dd:ee:ff",
		"AA-BB-CC-DD-EE-FF",
		"aabbccddeeff",
		"Aa-Bb-Cc-Dd-Ee-Ff",
	}
	for _, in := range cases {
		got, err := normalizeMAC(in)
		if err != nil {
			t.Errorf("normalizeMAC(%q) unexpected error: %v", in, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("normalizeMAC(%q) = %x, want %x", in, got, want)
		}
	}
}

func TestNormalizeMAC_Invalid(t *testing.T) {
	bad := []string{
		"",
		"not-a-mac",
		"aa:bb:cc:dd:ee",       // too short
		"aa:bb:cc:dd:ee:ff:00", // too long
		"zz:bb:cc:dd:ee:ff",    // non-hex
		"DESKTOP-J18FV15",      // hostname
	}
	for _, in := range bad {
		if _, err := normalizeMAC(in); err == nil {
			t.Errorf("normalizeMAC(%q) should have failed", in)
		}
	}
}

func TestBuildMagicPacket(t *testing.T) {
	mac := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	pkt, err := buildMagicPacket(mac)
	if err != nil {
		t.Fatalf("buildMagicPacket: %v", err)
	}
	if len(pkt) != 102 {
		t.Fatalf("magic packet length = %d, want 102", len(pkt))
	}
	// First 6 bytes: 0xFF.
	for i := 0; i < 6; i++ {
		if pkt[i] != 0xFF {
			t.Errorf("byte %d = %#x, want 0xFF", i, pkt[i])
		}
	}
	// Next 96 bytes: 16 repetitions of the MAC.
	for rep := 0; rep < 16; rep++ {
		off := 6 + rep*6
		if !bytes.Equal(pkt[off:off+6], mac) {
			t.Errorf("MAC repeat %d = %x, want %x", rep, pkt[off:off+6], mac)
		}
	}
}

func TestBuildMagicPacket_WrongLength(t *testing.T) {
	if _, err := buildMagicPacket([]byte{0x01, 0x02, 0x03}); err == nil {
		t.Error("buildMagicPacket with 3-byte MAC should fail")
	}
}

func TestFormatMAC(t *testing.T) {
	mac := []byte{0x00, 0x11, 0x22, 0xaa, 0xbb, 0xcc}
	got := formatMAC(mac)
	want := "00:11:22:aa:bb:cc"
	if got != want {
		t.Errorf("formatMAC = %q, want %q", got, want)
	}
}
