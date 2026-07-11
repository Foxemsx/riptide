#!/usr/bin/env bash
# riptide installer. Usage:
#   curl -fsSL https://raw.githubusercontent.com/Foxemsx/riptide/main/install.sh | sh
#   bash install.sh

# Re-exec under bash. `curl | sh` sets $0 to the sh binary itself, so re-exec
# bash on the script's stdin (copied to a temp file) instead of on $0.
if [ -z "${BASH_VERSION:-}" ]; then
  if [ -f "$0" ] && [ -t 0 ]; then
    exec bash "$0" "$@"
  else
    _riptide_reexec="$(mktemp "${TMPDIR:-/tmp}/riptide-install.XXXXXX.sh")"
    cat > "$_riptide_reexec"
    export _riptide_reexec
    exec bash "$_riptide_reexec" "$@"
  fi
fi

set -o pipefail

TMP="$(mktemp -d)"
INSTDIR="$TMP/riptide-installer"
LOGFILE="$TMP/install.log"
trap 'rm -rf "$TMP"' EXIT

cleanup_and_fail() {
  echo "" >&2
  echo "Installation did not complete. See the log above for details." >&2
  echo "You can retry any time with: bash install.sh" >&2
  exit 1
}

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "This installer needs '$1', but it was not found on your system." >&2
    echo "Please install it (e.g. via your package manager) and run again." >&2
    exit 1
  fi
}

need_cmd curl
need_cmd tar

detect_platform() {
  local os arch
  case "$(uname -s)" in
    Linux)  os="linux" ;;
    Darwin) os="darwin" ;;
    *)      echo "Unsupported OS: $(uname -s). riptide supports Linux, macOS and Windows only." >&2; exit 1 ;;
  esac
  case "$(uname -m)" in
    x86_64 | amd64)   arch="amd64" ;;
    aarch64 | arm64)  arch="arm64" ;;
    armv7l | armv6l)  arch="armv6l" ;;
    i386 | i686)      arch="386" ;;
    *)                arch="amd64" ;;
  esac
  echo "$os $arch"
}

PLATFORM="$(detect_platform)"
OS_LC="${PLATFORM%% *}"
ARCH="${PLATFORM##* }"
SHELL_NAME="$(basename "${SHELL:-/bin/bash}")"

# Pin to the local toolchain so `go` never auto-downloads a newer one over the
# network (GOTOOLCHAIN auto mode), which fails on restricted networks like WSL.
export GOTOOLCHAIN=local

GO_CMD=""
GO_WAS_PRESENT=0

# returns 0 if the go binary $1 reports a version >= 1.23
go_is_new_enough() {
  local ver major minor
  ver="$("$1" version 2>/dev/null | awk '{print $3}')"   # e.g. go1.21.5
  ver="${ver#go}"
  # Unparseable (e.g. "devel ...") or missing -> treat as too old so we
  # install our own known-good toolchain rather than guess.
  case "$ver" in
    [0-9]*.[0-9]*) ;;
    *) return 1 ;;
  esac
  major="${ver%%.*}"
  minor="${ver#*.}"
  minor="${minor%%.*}"
  [ "$major" -eq 1 ] && [ "$minor" -ge 23 ]
}

if [ -x "$HOME/.local/go/bin/go" ] && go_is_new_enough "$HOME/.local/go/bin/go"; then
  GO_CMD="$HOME/.local/go/bin/go"
  GO_WAS_PRESENT=1
elif command -v go >/dev/null 2>&1 && go_is_new_enough "$(command -v go)"; then
  GO_CMD="go"
  GO_WAS_PRESENT=1
fi

# Install Go locally if missing.
install_go() {
  local gover url
  gover="$(curl -fsSL https://go.dev/VERSION?m=text | head -1)"
  if [ -z "$gover" ]; then
    echo "Could not determine the latest Go version from go.dev." >&2
    cleanup_and_fail
  fi
  url="https://go.dev/dl/${gover}.${OS_LC}-${ARCH}.tar.gz"
  echo "Downloading $gover for $OS_LC/$ARCH ..."
  if ! curl -fsSL "$url" -o "$TMP/go.tgz" >>"$LOGFILE" 2>&1; then
    echo "Failed to download Go from $url" >&2
    cleanup_and_fail
  fi
  mkdir -p "$HOME/.local"
  if ! tar -C "$HOME/.local" -xzf "$TMP/go.tgz" >>"$LOGFILE" 2>&1; then
    echo "Failed to extract the Go archive." >&2
    cleanup_and_fail
  fi
  GO_CMD="$HOME/.local/go/bin/go"
  echo "$gover"   # returned to caller via command substitution
}

if [ -z "$GO_CMD" ]; then
  cat <<'MSG'

  riptide is written in Go. Go is a free, open-source programming language made
  by Google. Installing it lets your computer build riptide from its source —
  it is safe, widely used, and only adds a small folder (~150 MB) to your home.

MSG
  ans="Y"
  if [ -t 0 ]; then
    read -r -p "Download and install Go now? [Y/n] " ans
  fi
  case "$ans" in
    ""|Y|y|yes|YES) : ;;
    *) echo "Aborted. Install Go yourself (https://go.dev/dl) and rerun this script."; exit 0 ;;
  esac
  GO_VER="$(install_go)"
  GO_WAS_PRESENT=0
  GO_ACTION="installed"
else
  GO_VER="$("$GO_CMD" version | head -1)"
  GO_ACTION="already-present"
fi

# PATH setup (current shell + the user's shell config).
add_path_entry() {
  local entry="$1"
  export PATH="$entry:$PATH"
  case "$SHELL_NAME" in
    fish)
      fish -c "fish_add_path '$entry'" >/dev/null 2>&1 || true
      ;;
    zsh)
      local rc="$HOME/.zshrc"
      if [ -f "$rc" ] && ! grep -qF "$entry" "$rc"; then
        printf '\nexport PATH="$PATH:%s"\n' "$entry" >> "$rc"
      fi
      ;;
    *)
      local rc="$HOME/.bashrc"
      if [ -f "$rc" ] && ! grep -qF "$entry" "$rc"; then
        printf '\nexport PATH="$PATH:%s"\n' "$entry" >> "$rc"
      fi
      ;;
  esac
}

add_path_entry "$HOME/.local/go/bin"
add_path_entry "$HOME/go/bin"

PATH_ENTRIES="$HOME/.local/go/bin $HOME/go/bin"

# Write the embedded Bubble Tea TUI and build it.
mkdir -p "$INSTDIR"

cat > "$INSTDIR/go.mod" <<'GOMOD_EOF'
module riptideinstaller

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

// ---- styles (mirrors riptide's dark dashboard palette) ----
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
type startMsg struct{}

type model struct {
	width   int
	height  int
	phase   string // intro, running, done, failed
	steps   []step
	current int
	spinner int
	log     string
	failOut string
}

func initialModel() model {
	return model{
		phase: "intro",
		steps: []step{
			{name: "Installing riptide binary (go install)", args: []string{"go", "install", "github.com/Foxemsx/riptide/cmd/riptide@main"}},
			{name: "Verifying riptide is on your PATH", args: []string{"sh", "-c", "command -v riptide"}},
		},
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg{t} })
}

func runStep(index int, s step) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
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
	case startMsg:
		if m.phase == "intro" {
			m.phase = "running"
			m.steps[0].state = stateRunning
			return m, tea.Batch(runStep(0, m.steps[0]), tickCmd())
		}
		return m, nil
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
		next := msg.index + 1
		if next >= len(m.steps) {
			m.phase = "done"
			return m, nil
		}
		m.current = next
		m.steps[next].state = stateRunning
		return m, tea.Batch(runStep(next, m.steps[next]), tickCmd())
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.phase == "intro" {
				return m, func() tea.Msg { return startMsg{} }
			}
			if m.phase == "done" || m.phase == "failed" {
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

func (m model) introView() string {
	osv := os.Getenv("RIPTIDE_INSTALL_OS")
	arch := os.Getenv("RIPTIDE_INSTALL_ARCH")
	gover := os.Getenv("RIPTIDE_INSTALL_GO_VERSION")
	goaction := os.Getenv("RIPTIDE_INSTALL_GO_ACTION")
	shell := os.Getenv("RIPTIDE_INSTALL_SHELL")

	goLine := gover
	if goaction == "installed" {
		goLine = fmt.Sprintf("%s (installed just now)", gover)
	} else {
		goLine = fmt.Sprintf("%s (already installed)", gover)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("⚡  riptide installer"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("System:    "))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%s/%s", osv, arch)))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Go:        "))
	b.WriteString(valueStyle.Render(goLine))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Shell:     "))
	b.WriteString(valueStyle.Render(shell))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("This will:"))
	b.WriteString("\n")
	for i := range m.steps {
		b.WriteString(m.stepLine(i))
		b.WriteString("\n")
	}
	b.WriteString(hintStyle.Render("Press Enter to start  ·  Esc to cancel"))
	return cardStyle.Render(b.String())
}

func (m model) runningView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Installing riptide"))
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
	osv := os.Getenv("RIPTIDE_INSTALL_OS")
	arch := os.Getenv("RIPTIDE_INSTALL_ARCH")
	gover := os.Getenv("RIPTIDE_INSTALL_GO_VERSION")
	goaction := os.Getenv("RIPTIDE_INSTALL_GO_ACTION")
	pathent := os.Getenv("RIPTIDE_INSTALL_PATH_ENTRIES")

	goLine := gover
	if goaction == "installed" {
		goLine = fmt.Sprintf("%s (installed to ~/.local/go)", gover)
	} else {
		goLine = fmt.Sprintf("%s (already installed)", gover)
	}

	var b strings.Builder
	b.WriteString(doneStyle.Render("✓  Installation complete"))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("System:        "))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%s/%s", osv, arch)))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Go:            "))
	b.WriteString(valueStyle.Render(goLine))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("riptide binary:  "))
	b.WriteString(valueStyle.Render("~/go/bin/riptide"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Added to PATH: "))
	b.WriteString(valueStyle.Render(pathent))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("Run it now:  riptide"))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("Press Enter or q to finish"))
	return cardStyle.Render(b.String())
}

func (m model) failedView() string {
	var b strings.Builder
	b.WriteString(errStyle.Render("✗  Something went wrong"))
	b.WriteString("\n\n")
	b.WriteString(logStyle.Render(tail(m.failOut, 6)))
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("Re-run the installer:  bash install.sh"))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("Press Enter or q to exit"))
	return cardStyle.Render(b.String())
}

func (m model) View() string {
	var body string
	switch m.phase {
	case "intro":
		body = m.introView()
	case "running":
		body = m.runningView()
	case "done":
		body = m.doneView()
	case "failed":
		body = m.failedView()
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
		fmt.Fprintln(os.Stderr, "installer UI error:", err)
		os.Exit(1)
	}
	if fm, ok := final.(model); ok && fm.phase == "failed" {
		os.Exit(1)
	}
}
GOEOF

echo "Building the installer interface (first run only, this may take a moment)…"

export GOFLAGS=-mod=mod
export GOPATH="${GOPATH:-$HOME/go}"

if ! ( cd "$INSTDIR" && "$GO_CMD" mod tidy >>"$LOGFILE" 2>&1 && "$GO_CMD" build -o "$INSTDIR/ui" . >>"$LOGFILE" 2>&1 ); then
  echo "Failed to build the installer interface." >&2
  echo "Log:" >&2
  tail -n 30 "$LOGFILE" >&2
  cleanup_and_fail
fi

# Hand off to the TUI.
export RIPTIDE_INSTALL_OS="$OS_LC"
export RIPTIDE_INSTALL_ARCH="$ARCH"
export RIPTIDE_INSTALL_GO_VERSION="$GO_VER"
export RIPTIDE_INSTALL_GO_ACTION="$GO_ACTION"
export RIPTIDE_INSTALL_PATH_ENTRIES="$PATH_ENTRIES"
export RIPTIDE_INSTALL_SHELL="$SHELL_NAME"

"$INSTDIR/ui"
rc=$?

if [ "$rc" -eq 0 ]; then
  echo ""
  echo "✓ riptide installed! Start it with:  riptide"
else
  echo "" >&2
  echo "The installer finished with an error. Re-run: bash install.sh" >&2
fi
exit "$rc"
