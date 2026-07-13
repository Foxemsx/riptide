# riptide

**Measure and watch your internet connection — from the terminal.**

A polished Go TUI with a startup menu, one-shot speed tests, a live bandwidth monitor, themes, and saved test history. Centered cards, smooth graphs, SQLite under the hood.

[![terminal](https://img.shields.io/badge/terminal-TUI-39d0d8?style=flat-square)](https://github.com/Foxemsx/riptide)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Linux](https://img.shields.io/badge/Linux-supported-2ea44f?style=flat-square&logo=linux&logoColor=white)](https://github.com/Foxemsx/riptide)
[![macOS](https://img.shields.io/badge/macOS-supported-A2AAAD?style=flat-square&logo=apple&logoColor=white)](https://github.com/Foxemsx/riptide)
[![Windows](https://img.shields.io/badge/Windows-supported-0078D6?style=flat-square&logo=windows&logoColor=white)](https://github.com/Foxemsx/riptide)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)

<p align="center">
  <img src="assets/showcase.gif?v=3" alt="riptide demo" width="720">
</p>

---

## Modes

| | |
|:---|:---|
| **Speed Test** | One-shot download, upload, and ping. Parallel connections, peak rates, timed phases. Auto-saves runs; average + "Good for" insight; compare the latest 10. |
| **Bandwidth Monitor** | Live view of *real* PC traffic (OS counters only — no test load). Peaks, uptime, pause, and an Apps panel showing what's using bandwidth now. |
| **Settings** | Searchable settings: 20 color themes, About & Support, database reset, uninstall instructions. |

---

## Screenshots

<p align="center">
  <img src="assets/mainmenu.png?v=3" alt="Main menu" width="48%">
  &nbsp;
  <img src="assets/home.png?v=3" alt="Speed test" width="48%">
</p>
<p align="center">
  <sub><b>Main menu</b> · Speed Test, Bandwidth, Settings, Exit &nbsp;&nbsp;|&nbsp;&nbsp; <b>Speed Test</b> · live graphs mid-run</sub>
</p>

<p align="center">
  <img src="assets/finished.png?v=3" alt="Finished summary" width="48%">
  &nbsp;
  <img src="assets/bandwidth.png?v=3" alt="Bandwidth monitor" width="48%">
</p>
<p align="center">
  <sub><b>Finished</b> · peaks + ping + recent history &nbsp;&nbsp;|&nbsp;&nbsp; <b>Bandwidth</b> · live DL/UL of your real connection</sub>
</p>

<p align="center">
  <img src="assets/settings.png?v=3" alt="Settings screen" width="48%">
  &nbsp;
  <img src="assets/save.png?v=3" alt="Save prompt" width="48%">
</p>
<p align="center">
  <sub><b>Settings</b> · theme picker, reset DB, uninstall &nbsp;&nbsp;|&nbsp;&nbsp; <b>Save</b> · name a run after the test</sub>
</p>

<p align="center">
  <img src="assets/helpmenu.png?v=3" alt="Help overlay" width="42%">
</p>
<p align="center">
  <sub><b>Help</b> · press <code>?</code> anytime for controls · <code>esc</code> / <code>m</code> back to menu</sub>
</p>

---

## Features

- **Startup menu** — 2×2 card grid with hotkeys `1`–`4`, keyboard or mouse
- **Speed history** — auto-saves completed tests; press `s` to name a run; latest 10 shown for comparison
- **Average & record insight** — *Your usual average* block summarizes mean ↓/↑/ping across all saved runs
- **"Good for" verdict** — grades your connection Perfect/Good/Fair/Bad for Gaming, 4K, 2K, 1080p and Video calls, plus real transfer-time estimates ("100 GB game ≈ 53m")
- **Copy result** — press `y` to copy `↓248 ↑19 12ms` to the clipboard
- **20 themes** — default, ocean, midnight, sunset, forest, rose, nord, dracula, cyber, ember, arctic, gruvbox, tokyo, catppuccin, solarized, rosepine, monokai, onedark, github, everforest (Settings or `--theme`)
- **Settings search** — filter themes & sections; `enter` jumps to the best match
- **About & Support** — new Settings section with version, GitHub ★, and Buy me a coffee
- **Update checker** — main menu shows an *Up to date* / *Update available* chip (click or `g` opens GitHub)
- **Apps using bandwidth** — Bandwidth monitor can list which apps are active on the network right now (`a`)
- **SQLite store** — preferences + test runs in `riptide.db` (user config dir)
- **High-res graphs** — eighth-block bars, fire gradients, peak spark, age fade
- **Smooth numbers** — lerped display values instead of hard snaps
- **Units on the fly** — `c` cycles Mbps · KB/s · MB/s · GB/s
- **Compact mode** — `t` hides the large logo when space is tight
- **Clean chrome** — themed canvas, rounded cards, accent chips
- **Graceful errors** — no stack traces when the network is down

---

## Quick start

### Linux / macOS (installer — no sudo, bash only)

> The `install.sh` script supports Linux and macOS. It re-execs under bash and, if no
> suitable Go toolchain is present, downloads one locally — it never touches
> your system `go` or needs root.

```sh
curl -fsSL https://raw.githubusercontent.com/Foxemsx/riptide/main/install.sh | sh
riptide
```

Uninstall:

```sh
curl -fsSL https://raw.githubusercontent.com/Foxemsx/riptide/main/uninstall.sh | sh
```

On macOS the same `curl | sh` commands work (the script detects Darwin).

You can also open **Settings → Uninstall** inside the app for the same instructions.

### Windows

The bash installer does **not** run on Windows. Pick one of:

- **Prebuilt release** — download `riptide.exe` from the
  [Releases](https://github.com/Foxemsx/riptide/releases) page and run it.
- **With Go 1.23+** (no release needed):

  ```sh
  go install github.com/Foxemsx/riptide/cmd/riptide@main
  riptide
  ```

### From source (any OS with Go 1.23+)

```sh
git clone https://github.com/Foxemsx/riptide
cd riptide
go build -o riptide ./cmd/riptide    # Windows: go build -o riptide.exe ./cmd/riptide
./riptide
```

> Put `$(go env GOPATH)/bin` on your `PATH` if `riptide` is not found after `go install`.

---

## Usage

```sh
riptide                # main menu
riptide --compact      # skip the large logo
riptide --theme ocean  # start with a palette (also saved as preference)
```

| Flag | Default | Description |
|------|---------|-------------|
| `--compact` | `false` | Tagline only (no large logo) |
| `--theme` | saved / `default` | Color palette (see Themes below) |

### Controls

| Key | Action |
|-----|--------|
| `←` `→` `↑` `↓` / `h j k l` | Move in the menu |
| `1` `2` `3` `4` | Jump to Speed Test / Bandwidth / Settings / Exit |
| `enter` | Select |
| `s` | **Speed Test** — save / rename the current run |
| `y` | **Speed Test** — copy result as `↓248 ↑19 12ms` |
| `c` | Cycle units |
| `r` | Restart test / monitor |
| `p` | Pause / resume (**Bandwidth** only) |
| `a` | **Bandwidth** — toggle the Apps using bandwidth panel |
| `t` | Toggle compact logo |
| `g` | Open the project on GitHub (menu & settings) |
| `?` | Help overlay |
| `esc` / `m` | Back to main menu |
| `q` / `ctrl+c` | Quit |

### Settings

| Key | Action |
|-----|--------|
| type | Filter themes & sections live |
| `enter` | Jump to best match (or apply a matched theme) |
| `tab` | Next section |
| `1`–`4` | Jump to Themes / Reset / Uninstall / About |
| `←` `→` / `j` `k` | Browse themes |
| `enter` on theme | Apply & save theme |
| `enter` on About | Open Buy me a coffee (or `b`) |
| `enter` on Reset | Confirm wipe of saved runs |

---

## Themes

| Name | Vibe |
|------|------|
| `default` | Teal & amber on charcoal |
| `ocean` | Deep sea · cyan foam |
| `midnight` | Electric blue · violet night |
| `sunset` | Coral dusk · warm gold |
| `forest` | Moss · gold canopy |
| `rose` | Blush · soft magenta |
| `nord` | Frost · polar aurora |
| `dracula` | Purple night · neon pink |
| `cyber` | Neon green · hot magenta |
| `ember` | Charcoal fire · molten gold |
| `arctic` | Ice blue · clean slate |
| `gruvbox` | Warm retro · earth tones |
| `tokyo` | Tokyo Night · neon anime dusk |
| `catppuccin` | Soft pastel mocha |
| `solarized` | Classic dark precision |
| `rosepine` | Dusty dawn calm |
| `monokai` | Vivid code-editor heat |
| `onedark` | Editor default comfort |
| `github` | Exactly like the site |
| `everforest` | Calm muted woodland |

Theme preference is stored in the local database (overridden by `--theme` for that launch).

---

## Data & history

Speed tests are stored in **SQLite** as `riptide.db`:

| OS      | Location |
|---------|----------|
| Linux   | `~/.config/riptide/riptide.db` |
| macOS   | `~/Library/Application Support/riptide/riptide.db` (via `os.UserConfigDir()`) |
| Windows | `%AppData%\riptide\riptide.db` |

- Completed speed tests **auto-save** with a timestamped name
- Press **`s`** during/after a speed test to save with a custom name
- The **Recent tests** block (latest 10) lives on the Speed Test screen only
- **Settings → Reset database** clears all saved runs (keeps theme); with confirmation

---

## Uninstall

**Linux / macOS**

```sh
curl -fsSL https://raw.githubusercontent.com/Foxemsx/riptide/main/uninstall.sh | sh
```

**Manual**

```sh
# Linux or macOS
rm -f "$(command -v riptide)"

# Windows (go install)
del %USERPROFILE%\go\bin\riptide.exe
```

Uninstall removes the binary only — not Go, not your PATH entries, and not `riptide.db`. To wipe history first, use **Settings → Reset database**.

---

## License

[MIT](LICENSE) — free to use, modify, and redistribute with the license notice.
