#!/usr/bin/env bash
# speed — interactive uninstaller
#
# Self-contained bootstrap + Bubble Tea TUI. Removes the `speed` binary
# (and only that — the Go toolchain and your PATH are left untouched).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Foxemsx/speed/main/uninstall.sh | sh
#   bash uninstall.sh

# Re-exec under bash if invoked under a different shell (curl | sh should be fine).
if [ -z "${BASH_VERSION:-}" ]; then
  exec bash "$0" "$@"
fi

set -o pipefail

TMP="$(mktemp -d)"
INSTDIR="$TMP/speed-uninstaller"
LOGFILE="$TMP/uninstall.log"
trap 'rm -rf "$TMP"' EXIT

SPEED_BIN=""
if command -v speed >/dev/null 2>&1; then
  SPEED_BIN="$(command -v speed)"
elif [ -x "$HOME/go/bin/speed" ]; then
  SPEED_BIN="$HOME/go/bin/speed"
fi

# Preflight: if speed isn't installed, there's nothing to do.
if [ -z "$SPEED_BIN" ]; then
  cat <<'MSG'

  speed was not found on your system (no `speed` on PATH or in ~/go/bin).

  Nothing to uninstall. If you built it manually somewhere else, just delete
  that binary yourself.

MSG
  exit 0
fi

# We need `go` only to build the TUI — but the TUI is embedded in Go, so we
# require it. If missing, fall back to a plain message.
GO_CMD=""
if [ -x "$HOME/.local/go/bin/go" ]; then
  GO_CMD="$HOME/.local/go/bin/go"
elif command -v go >/dev/null 2>&1; then
  GO_CMD="go"
fi

if [ -z "$GO_CMD" ]; then
  cat <<'MSG'

  This uninstaller needs the Go toolchain to build its interface. Install Go
  (https://go.dev/dl) or run this instead to remove speed directly:

      rm -f "$(command -v speed || echo "$HOME/go/bin/speed")"

MSG
  exit 1
fi

export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
OS_LC="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
SHELL_NAME="$(basename "${SHELL:-/bin/bash}")"

# ---------------------------------------------------------------------------
# Write the embedded Bubble Tea TUI and build it
# ---------------------------------------------------------------------------
mkdir -p "$INSTDIR"

cat > "$INSTDIR/go.mod" <<'GOMOD_EOF'
module speeduninstaller

go 1.23

require (
	github.com/charmbracelet/bubbletea v1.1.0
	github.com/charmbracelet/lipgloss v0.13.0
)
GOMOD_EOF

cat > "$INSTDIR/main.go" <<'GOEOF'
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	bg        = lipgloss.Color("#0d1117")
	fg        = lipgloss.AdaptiveColor{Light: "#1c2128", Dark: "#e6edf3"}
	muted     = lipgloss.AdaptiveColor{Light: "#57606a", Dark: "#7d8590"}
	borderCol = lipgloss.AdaptiveColor{Light: "#afb8c1", Dark: "#30363d"}
	accent    = lipgloss.AdaptiveColor{Light: "#0a7ea4", Dark: "#39d0d8"}
	green     = lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#7ee787"}
	red       = lipgloss.AdaptiveColor{Light: "#cf222e", Dark: "#ff7b72"}
)

var cardStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(borderCol).
	Padding(1, 3)

var titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accent).MarginBottom(1)
var labelStyle = lipgloss.NewStyle().Foreground(muted)
var valueStyle = lipgloss.NewStyle().Foreground(fg).Bold(true)
var doneStyle  = lipgloss.NewStyle().Foreground(green).Bold(true)
var errStyle   = lipgloss.NewStyle().Foreground(red).Bold(true)
var hintStyle  = lipgloss.NewStyle().Foreground(muted).MarginTop(1)
var logStyle   = lipgloss.NewStyle().Foreground(muted)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type stepState int

const (
	statePending stepState = iota
	stateRunning
	stateOK
	stateFail
)

type step struct {
	name  string
	args  []string
	state stepState
	out   string
}

type stepResultMsg struct {
	index int
	err   error
	out   string
}

type tickMsg struct{ t time.Time }
type confirmMsg struct{ ok bool }
type abortMsg struct{}

type model struct {
	width   int
	height  int
	phase   string // confirm, running, done, failed, aborted
	steps   []step
	spinner int
	log     string
	failOut string
}

func initialModel() model {
	return model{
		phase: "confirm",
		steps: []step{
			{name: "Removing the speed binary", args: []string{"rm", "-f", os.Getenv("SPEED_UNINSTALL_TARGET")}},
		},
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg{t} })
}

func runStep(index int, s step) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, s.args[0], s.args[1:]...)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		return stepResultMsg{index: index, err: err, out: string(out)}
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		if m.phase == "running" {
			m.spinner = (m.spinner + 1) % len(spinnerFrames)
			return m, tickCmd()
		}
		return m, nil
	case confirmMsg:
		if !msg.ok {
			m.phase = "aborted"
			return m, nil
		}
		m.phase = "running"
		m.steps[0].state = stateRunning
		return m, tea.Batch(runStep(0, m.steps[0]), tickCmd())
	case stepResultMsg:
		if msg.err != nil {
			m.steps[msg.index].state = stateFail
			m.steps[msg.index].out = msg.out
			m.phase = "failed"
			m.failOut = msg.out
			return m, nil
		}
		m.steps[msg.index].state = stateOK
		m.steps[msg.index].out = msg.out
		m.log = msg.out
		m.phase = "done"
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "y", "Y":
			if m.phase == "confirm" {
				return m, func() tea.Msg { return confirmMsg{ok: true} }
			}
			if m.phase == "done" || m.phase == "failed" || m.phase == "aborted" {
				return m, tea.Quit
			}
		case "n", "N":
			if m.phase == "confirm" {
				return m, func() tea.Msg { return confirmMsg{ok: false} }
			}
			if m.phase == "done" || m.phase == "failed" || m.phase == "aborted" {
				return m, tea.Quit
			}
		case "enter":
			if m.phase == "confirm" {
				return m, func() tea.Msg { return confirmMsg{ok: true} }
			}
			if m.phase == "done" || m.phase == "failed" || m.phase == "aborted" {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) stepLine(i int) string {
	s := m.steps[i]
	var icon string
	var st lipgloss.Style
	switch s.state {
	case statePending:
		icon, st = "•", labelStyle
	case stateRunning:
		icon, st = spinnerFrames[m.spinner], lipgloss.NewStyle().Foreground(accent)
	case stateOK:
		icon, st = "✓", doneStyle
	case stateFail:
		icon, st = "✗", errStyle
	}
	return fmt.Sprintf("  %s %s", icon, st.Render(s.name))
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func (m model) confirmView() string {
	target := os.Getenv("SPEED_UNINSTALL_TARGET")
	var b strings.Builder
	b.WriteString(titleStyle.Render("⚡  uninstall speed"))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("This will remove:"))
	b.WriteString("\n\n")
	b.WriteString("  • ")
	b.WriteString(valueStyle.Render(target))
	b.WriteString("  ")
	b.WriteString(labelStyle.Render("(the speed binary)"))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("It will NOT touch Go or your PATH."))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("Press Y or Enter to uninstall  ·  N or Esc to cancel"))
	return cardStyle.Render(b.String())
}

func (m model) runningView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Uninstalling speed"))
	b.WriteString("\n")
	for i := range m.steps {
		b.WriteString(m.stepLine(i))
		b.WriteString("\n")
	}
	if m.log != "" {
		b.WriteString("\n")
		b.WriteString(logStyle.Render("› " + strings.ReplaceAll(tail(m.log, 4), "\n", "\n› ")))
		b.WriteString("\n")
	}
	b.WriteString(hintStyle.Render("Working…  ·  Esc to cancel"))
	return cardStyle.Render(b.String())
}

func (m model) doneView() string {
	target := os.Getenv("SPEED_UNINSTALL_TARGET")
	var b strings.Builder
	b.WriteString(doneStyle.Render("✓  speed removed"))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("Removed:  "))
	b.WriteString(valueStyle.Render(target))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("Go and your PATH were left unchanged."))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("Press Enter or q to finish"))
	return cardStyle.Render(b.String())
}

func (m model) failedView() string {
	var b strings.Builder
	b.WriteString(errStyle.Render("✗  Could not remove speed"))
	b.WriteString("\n\n")
	b.WriteString(logStyle.Render(tail(m.failOut, 6)))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("You can delete it manually:  rm -f " + os.Getenv("SPEED_UNINSTALL_TARGET")))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("Press Enter or q to exit"))
	return cardStyle.Render(b.String())
}

func (m model) abortedView() string {
	var b strings.Builder
	b.WriteString(labelStyle.Render("Uninstall cancelled — speed is still installed."))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("Press Enter or q to exit"))
	return cardStyle.Render(b.String())
}

func (m model) View() string {
	var body string
	switch m.phase {
	case "confirm":
		body = m.confirmView()
	case "running":
		body = m.runningView()
	case "done":
		body = m.doneView()
	case "failed":
		body = m.failedView()
	case "aborted":
		body = m.abortedView()
	}
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
	}
	return body
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "uninstaller UI error:", err)
		os.Exit(1)
	}
	if fm, ok := final.(model); ok && fm.phase == "failed" {
		os.Exit(1)
	}
}
GOEOF

echo "Building the uninstaller interface (first run only, this may take a moment)…"

export GOFLAGS=-mod=mod
export GOPATH="${GOPATH:-$HOME/go}"

if ! ( cd "$INSTDIR" && "$GO_CMD" mod tidy >>"$LOGFILE" 2>&1 && "$GO_CMD" build -o "$INSTDIR/ui" . >>"$LOGFILE" 2>&1 ); then
  echo "Failed to build the uninstaller interface." >&2
  echo "Log:" >&2
  tail -n 30 "$LOGFILE" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Hand off to the TUI
# ---------------------------------------------------------------------------
export SPEED_UNINSTALL_TARGET="$SPEED_BIN"

"$INSTDIR/ui"
rc=$?

if [ "$rc" -eq 0 ]; then
  echo ""
  echo "✓ Done. speed has been removed (Go and PATH left unchanged)."
else
  echo "" >&2
  echo "The uninstaller finished with an error. Re-run: bash uninstall.sh" >&2
fi
exit "$rc"
