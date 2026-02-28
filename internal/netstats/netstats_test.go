package netstats

import (
	"strings"
	"testing"
)

const sampleProcNetDev = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo frame compressed
    lo: 1234567    1234    0    0    0     0          0         0  1234567    1234    0    0    0     0       0          0
  eth0: 987654321  987654    5    0    0     0          0         0 123456789  123456    0    0    0     0       0          0
 wlan0:     54321      50    0    0    0     0          0         0     12345      30    0    0    0     0       0          0
`

func TestParseNetDev(t *testing.T) {
	counters, err := parseNetDev(strings.NewReader(sampleProcNetDev))
	if err != nil {
		t.Fatalf("parseNetDev: %v", err)
	}
	if len(counters) != 3 {
		t.Fatalf("expected 3 interfaces, got %d", len(counters))
	}

	// lo
	if counters[0].Interface != "lo" {
		t.Errorf("iface[0] = %q, want lo", counters[0].Interface)
	}
	if counters[0].BytesRecv != 1234567 {
		t.Errorf("lo recv = %d, want 1234567", counters[0].BytesRecv)
	}
	if counters[0].BytesSent != 1234567 {
		t.Errorf("lo sent = %d, want 1234567", counters[0].BytesSent)
	}

	// eth0
	if counters[1].Interface != "eth0" {
		t.Errorf("iface[1] = %q, want eth0", counters[1].Interface)
	}
	if counters[1].BytesRecv != 987654321 {
		t.Errorf("eth0 recv = %d, want 987654321", counters[1].BytesRecv)
	}
	if counters[1].BytesSent != 123456789 {
		t.Errorf("eth0 sent = %d, want 123456789", counters[1].BytesSent)
	}

	// wlan0
	if counters[2].Interface != "wlan0" {
		t.Errorf("iface[2] = %q, want wlan0", counters[2].Interface)
	}
	if counters[2].BytesRecv != 54321 {
		t.Errorf("wlan0 recv = %d, want 54321", counters[2].BytesRecv)
	}
	if counters[2].BytesSent != 12345 {
		t.Errorf("wlan0 sent = %d, want 12345", counters[2].BytesSent)
	}
}

func TestParseNetDevEmpty(t *testing.T) {
	counters, err := parseNetDev(strings.NewReader(""))
	if err != nil {
		t.Fatalf("parseNetDev: %v", err)
	}
	if len(counters) != 0 {
		t.Fatalf("expected 0 interfaces, got %d", len(counters))
	}
}

func TestParseNetDevHeaderOnly(t *testing.T) {
	data := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo frame compressed
`
	counters, err := parseNetDev(strings.NewReader(data))
	if err != nil {
		t.Fatalf("parseNetDev: %v", err)
	}
	if len(counters) != 0 {
		t.Fatalf("expected 0 interfaces, got %d", len(counters))
	}
}

func TestParseNetDevMalformedLine(t *testing.T) {
	data := `Inter-|   Receive
 face |bytes
    lo: 100
  eth0: 200 300 400 500 600 700 800 900 1000 1100 1200 1300 1400 1500 1600 1700
  bad_no_colon
`
	counters, err := parseNetDev(strings.NewReader(data))
	if err != nil {
		t.Fatalf("parseNetDev: %v", err)
	}
	// lo has too few fields (only 1), should be skipped
	// eth0 has enough fields
	// bad_no_colon has no colon, should be skipped
	if len(counters) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(counters))
	}
	if counters[0].Interface != "eth0" {
		t.Errorf("iface = %q, want eth0", counters[0].Interface)
	}
	if counters[0].BytesRecv != 200 {
		t.Errorf("eth0 recv = %d, want 200", counters[0].BytesRecv)
	}
	if counters[0].BytesSent != 1000 {
		t.Errorf("eth0 sent = %d, want 1000", counters[0].BytesSent)
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		bps  uint64
		want string
	}{
		{0, "0 B/s"},
		{512, "512 B/s"},
		{1024, "1.0 KB/s"},
		{50 * 1024, "50.0 KB/s"},
		{1024 * 1024, "1.00 MB/s"},
		{15 * 1024 * 1024, "15.00 MB/s"},
	}
	for _, tt := range tests {
		got := FormatSpeed(tt.bps)
		if got != tt.want {
			t.Errorf("FormatSpeed(%d) = %q, want %q", tt.bps, got, tt.want)
		}
	}
}

func TestFormatTotal(t *testing.T) {
	tests := []struct {
		b    uint64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{3 * 1024 * 1024 * 1024, "3.00 GB"},
	}
	for _, tt := range tests {
		got := FormatTotal(tt.b)
		if got != tt.want {
			t.Errorf("FormatTotal(%d) = %q, want %q", tt.b, got, tt.want)
		}
	}
}
