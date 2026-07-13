package engine

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// collectActiveProcs enumerates /proc/net/tcp & tcp6 inodes and maps each to a
// process via /proc/<pid>/fd. No root required (reading our own proc tree is
// allowed); processes we cannot read are silently skipped.
func collectActiveProcs(out map[string]int) {
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		collectProcNet(out, f)
	}
}

func collectProcNet(out map[string]int, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 { // header
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		inode := fields[9]
		if pid := pidForInode(inode); pid > 0 {
			if name := commForPid(pid); name != "" {
				out[name]++
			}
		}
	}
}

// pidForInode walks /proc/*/fd and returns the pid owning the socket inode.
func pidForInode(inode string) int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		fdDir := filepath.Join("/proc", e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if target == "socket:["+inode+"]" {
				return pid
			}
		}
	}
	return 0
}

func commForPid(pid int) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
