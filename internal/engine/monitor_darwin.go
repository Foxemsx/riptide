//go:build darwin

package engine

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// RunMonitor is the real Bandwidth Monitor engine for macOS. It generates no
// traffic: it reads OS interface byte counters via `netstat -ib -n` and
// derives the actual download / upload throughput of the whole machine from
// deltas between samples. It runs until ctx is cancelled.
//
// Scope: every non-loopback interface reported by netstat is summed
// (en0, en1, utun* for VPNs, etc.), matching the spirit of Linux and Windows
// implementations.
func RunMonitor(ctx context.Context, p *Progress, sampleInterval time.Duration) {
	// Set the source label before signalling "connected" so the UI's
	// phaseMsg handler sees the name and renders the "watching <adapters>"
	// header (it would otherwise drop it on a to-the-tick race).
	p.ServerName = discoverAdaptersDarwin()
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
				_, _ = readCountersDarwin(prev)
				last = t
				continue
			}
			elapsed := t.Sub(last).Seconds()
			last = t
			if elapsed <= 0 {
				continue
			}

			total, names := readCountersDarwin(nil)
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

// readCountersDarwin executes netstat and parses cumulative rx/tx byte counters.
// It returns the map of per-interface counters and a list of friendly interface names.
// When seen is non-nil it is also populated (used for baseline seeding).
// Parsing is defensive: wrong column indices were the main defect in earlier attempts.
func readCountersDarwin(seen map[string]counters) (map[string]counters, []string) {
	total := map[string]counters{}
	var names []string

	// Short timeout so a stuck netstat cannot freeze the monitor goroutine.
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	out, err := exec.CommandContext(ctx, "netstat", "-ib", "-n").Output()
	if err != nil {
		// netstat missing or permission issue — degrade gracefully (no crash).
		return total, names
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		// Typical header + data lines for macOS netstat -ib -n have >= 11 fields.
		// We require 11 to safely access indices 6 and 9.
		if len(fields) < 11 {
			continue
		}

		iface := fields[0]
		// Skip loopback (lo0 on macOS) and anything that looks like loopback.
		if iface == "lo0" || strings.HasPrefix(iface, "lo") {
			continue
		}

		// On macOS netstat -ib -n for <Link#> lines the layout after split is:
		// [0]Name [1]Mtu [2]Network [3]Address [4]Ipkts [5]Ierrs [6]Ibytes [7]Opkts [8]Oerrs [9]Obytes ...
		// We deliberately use indices 6/9 (this was the primary bug in the prior external PR).
		ibytes, err1 := strconv.ParseUint(fields[6], 10, 64)
		obytes, err2 := strconv.ParseUint(fields[9], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}

		c := counters{rx: ibytes, tx: obytes}
		total[iface] = c
		if seen != nil {
			seen[iface] = c
		}
		names = append(names, iface)
	}

	return total, names
}

// discoverAdaptersDarwin returns a friendly comma-separated label of active
// non-loopback interfaces (used as the monitor's "source" name).
func discoverAdaptersDarwin() string {
	_, names := readCountersDarwin(nil)
	if len(names) == 0 {
		return "no active interfaces"
	}
	return strings.Join(names, ", ")
}
