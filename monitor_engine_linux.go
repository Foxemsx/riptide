//go:build linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RunMonitor is the real Bandwidth Monitor engine for Linux. It generates no
// traffic: it reads the kernel's per-interface byte counters from
// /sys/class/net/<iface>/statistics/{rx,tx}_bytes and derives the actual
// download / upload throughput of the whole PC from byte deltas between
// samples. It runs until ctx is cancelled.
//
// Scope: every "up" interface except loopback (lo) is summed, so the numbers
// reflect total PC throughput (Wi-Fi, Ethernet, VPN, LAN, ...).
func RunMonitor(ctx context.Context, p *Progress, sampleInterval time.Duration) {
	// Set the source label before signalling "connected" so the UI's
	// phaseMsg handler sees the name and renders the "watching <adapters>"
	// header (it would otherwise drop it on a to-the-tick race).
	p.ServerName = discoverAdaptersLinux()
	sendPhase(p, PhaseConnected)

	prev := map[string]counters{}

	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			if last.IsZero() {
				_, _ = readCountersLinux(prev)
				last = t
				continue
			}
			elapsed := t.Sub(last).Seconds()
			last = t
			if elapsed <= 0 {
				continue
			}

			total, names := readCountersLinux(nil)
			if len(names) > 0 {
				p.ServerName = strings.Join(names, ", ")
			}

			var rx, tx uint64
			for iface, c := range total {
				base, ok := prev[iface]
				if !ok {
					prev[iface] = c
					continue
				}
				rx += safeDelta(base.rx, c.rx)
				tx += safeDelta(base.tx, c.tx)
				prev[iface] = c
			}

			_ = sendSample(p, Sample{Phase: PhaseDownload, Rate: float64(rx) / elapsed, At: t})
			_ = sendSample(p, Sample{Phase: PhaseUpload, Rate: float64(tx) / elapsed, At: t})
		}
	}
}

// readCountersLinux reads rx/tx byte counters for every up, non-loopback
// interface from sysfs. When seen is non-nil it is populated with the
// counters; it always returns the full map and the interface names.
func readCountersLinux(seen map[string]counters) (map[string]counters, []string) {
	total := map[string]counters{}
	var names []string

	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return total, names
	}
	for _, e := range entries {
		iface := e.Name()
		if iface == "lo" {
			continue
		}
		operstate, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "operstate"))
		if err != nil {
			// Interface may have vanished; skip it.
			continue
		}
		if strings.TrimSpace(string(operstate)) != "up" {
			continue
		}

		rx, err := readSysfsUint64(filepath.Join("/sys/class/net", iface, "statistics", "rx_bytes"))
		if err != nil {
			continue
		}
		tx, err := readSysfsUint64(filepath.Join("/sys/class/net", iface, "statistics", "tx_bytes"))
		if err != nil {
			continue
		}

		c := counters{rx: rx, tx: tx}
		total[iface] = c
		if seen != nil {
			seen[iface] = c
		}
		names = append(names, iface)
	}
	return total, names
}

// discoverAdaptersLinux returns a friendly comma-separated label of the up,
// non-loopback interfaces (used as the monitor's "source" name).
func discoverAdaptersLinux() string {
	_, names := readCountersLinux(nil)
	if len(names) == 0 {
		return "no active interfaces"
	}
	return strings.Join(names, ", ")
}

// readSysfsUint64 reads a single unsigned integer from a sysfs file.
func readSysfsUint64(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}
