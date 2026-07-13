package engine

import (
	"os/exec"
	"strings"
)

// collectActiveProcs uses `lsof` (commonly available on macOS) to list
// processes holding network connections. Best-effort; if lsof is missing the
// map stays empty rather than erroring.
func collectActiveProcs(out map[string]int) {
	cmd := exec.Command("lsof", "-nP", "-iTCP", "-iUDP", "-sTCP:ESTABLISHED")
	out2, err := cmd.Output()
	if err != nil {
		// Fall back to any TCP/UDP socket state.
		cmd = exec.Command("lsof", "-nP", "-i")
		out2, err = cmd.Output()
		if err != nil {
			return
		}
	}
	lines := strings.Split(string(out2), "\n")
	for i, line := range lines {
		if i == 0 { // header
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if name == "" || name == "COMMAND" {
			continue
		}
		out[name]++
	}
}
