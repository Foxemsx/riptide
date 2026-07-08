//go:build windows

package main

import (
	"context"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// RunMonitor is the real Bandwidth Monitor engine. Unlike the Speed Test
// (speed.go), it generates no traffic: it reads the operating system's network
// interface counters and derives the actual download / upload throughput of the
// whole PC from byte deltas between samples. It runs until ctx is cancelled.
//
// Scope: every active, non-loopback adapter is summed, so the numbers reflect
// total PC throughput (Wi-Fi, Ethernet, VPN, LAN, ...).
func RunMonitor(ctx context.Context, p *Progress, sampleInterval time.Duration) {
	// Set the source label before signalling "connected" so the UI's
	// phaseMsg handler sees the name and renders the "watching <adapters>"
	// header (it would otherwise drop it on a to-the-tick race).
	p.ServerName = discoverAdapters()
	sendPhase(p, PhaseConnected)

	// prev holds the last cumulative byte counters keyed by interface index.
	prev := map[uint32]counters{}

	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			if last.IsZero() {
				// First tick: just record baseline counters, no rate yet.
				_, _ = readCounters(prev)
				last = t
				continue
			}
			elapsed := t.Sub(last).Seconds()
			last = t
			if elapsed <= 0 {
				continue
			}

			total, names := readCounters(nil)
			if len(names) > 0 {
				p.ServerName = strings.Join(names, ", ")
			}

			var rx, tx uint64
			for idx, c := range total {
				base, ok := prev[idx]
				if !ok {
					// New adapter since baseline; seed it and skip this tick.
					prev[idx] = c
					continue
				}
				rx += safeDelta(base.rx, c.rx)
				tx += safeDelta(base.tx, c.tx)
				prev[idx] = c
			}

			_ = sendSample(p, Sample{Phase: PhaseDownload, Rate: float64(rx) / elapsed, At: t})
			_ = sendSample(p, Sample{Phase: PhaseUpload, Rate: float64(tx) / elapsed, At: t})
		}
	}
}

// discoverAdapters returns a friendly comma-separated label of the active,
// non-loopback adapters (used as the monitor's "source" name).
func discoverAdapters() string {
	_, names := readCounters(nil)
	if len(names) == 0 {
		return "no active adapters"
	}
	return strings.Join(names, ", ")
}

// readCounters reads cumulative In/Out octets for every active, non-loopback
// adapter. When seen is non-nil it is populated with the per-interface
// counters (used to seed the baseline). It always returns the full cumulative
// map and the friendly names of active adapters.
func readCounters(seen map[uint32]counters) (map[uint32]counters, []string) {
	total := map[uint32]counters{}
	var names []string

	// Growable buffer for GetAdaptersAddresses.
	size := uint32(16 * 1024)
	for {
		buf := make([]byte, size)
		rc := windows.GetAdaptersAddresses(
			windows.AF_UNSPEC,
			windows.GAA_FLAG_SKIP_ANYCAST|windows.GAA_FLAG_SKIP_MULTICAST|windows.GAA_FLAG_SKIP_DNS_SERVER,
			0,
			(*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])),
			&size,
		)
		if rc == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if rc != nil {
			return total, names
		}
		for addr := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); addr != nil; addr = addr.Next {
			// Skip loopback and adapters that are not operationally up.
			if addr.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK {
				continue
			}
			if addr.OperStatus != windows.IfOperStatusUp {
				continue
			}

			var row windows.MibIfRow
			row.Index = addr.IfIndex
			if err := windows.GetIfEntry(&row); err != nil {
				continue
			}

			c := counters{rx: uint64(row.InOctets), tx: uint64(row.OutOctets)}
			total[addr.IfIndex] = c
			if seen != nil {
				seen[addr.IfIndex] = c
			}
			if name := windows.UTF16PtrToString(addr.FriendlyName); name != "" {
				names = append(names, name)
			}
		}
		return total, names
	}
}
