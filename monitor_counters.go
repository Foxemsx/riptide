package main

// counters is the cumulative byte totals for one interface, keyed by
// interface identifier (Windows index, Linux interface name).
type counters struct {
	rx uint64
	tx uint64
}

// safeDelta returns c2 - c1, correctly handling uint32 wraparound (the Windows
// counters wrap at ~4 GB). It also guards against counter resets (e.g. adapter
// was disabled/re-enabled): a negative-looking delta larger than the plausible
// max-per-tick is treated as a reset and reported as 0 to avoid a spurious
// spike. Shared by both the Windows and Linux monitor engines.
func safeDelta(c1, c2 uint64) uint64 {
	const resetThreshold = 1 << 30 // ~1 GiB in a single tick => implausible
	if c2 >= c1 {
		d := c2 - c1
		if d > resetThreshold {
			return 0
		}
		return d
	}
	// Wrapped around the uint32 maximum.
	d := (^uint32(0) - uint32(c1)) + uint32(c2) + 1
	if uint64(d) > resetThreshold {
		return 0
	}
	return uint64(d)
}
