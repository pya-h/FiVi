package netstats

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

// InterfaceCounters holds raw counters from /proc/net/dev.
type InterfaceCounters struct {
	Interface string
	BytesRecv uint64
	BytesSent uint64
}

// Stats holds computed network rate statistics for a single interface.
type Stats struct {
	Interface  string
	RecvSpeed  uint64 // bytes per second
	SendSpeed  uint64 // bytes per second
	TotalRecv  uint64 // cumulative bytes received
	TotalSent  uint64 // cumulative bytes sent
	SampleTime time.Time
}

// parseNetDev parses /proc/net/dev formatted data from the given reader.
func parseNetDev(r io.Reader) ([]InterfaceCounters, error) {
	scanner := bufio.NewScanner(r)
	lineNo := 0
	var result []InterfaceCounters
	for scanner.Scan() {
		lineNo++
		if lineNo <= 2 {
			continue // skip header lines
		}
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}
		var c InterfaceCounters
		c.Interface = iface
		fmt.Sscan(fields[0], &c.BytesRecv)
		fmt.Sscan(fields[8], &c.BytesSent)
		result = append(result, c)
	}
	return result, scanner.Err()
}

// FormatSpeed returns a human-readable speed string (e.g. "1.23 MB/s").
func FormatSpeed(bytesPerSec uint64) string {
	switch {
	case bytesPerSec >= 1024*1024:
		return fmt.Sprintf("%.2f MB/s", float64(bytesPerSec)/(1024*1024))
	case bytesPerSec >= 1024:
		return fmt.Sprintf("%.1f KB/s", float64(bytesPerSec)/1024)
	default:
		return fmt.Sprintf("%d B/s", bytesPerSec)
	}
}

// FormatTotal returns a human-readable total bytes string (e.g. "1.23 GB").
func FormatTotal(bytes uint64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
