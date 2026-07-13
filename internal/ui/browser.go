package ui

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openURL opens a URL in the default browser (best-effort, non-blocking).
func openURL(url string) error {
	if url == "" {
		return fmt.Errorf("empty url")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
