# speed

> A polished terminal internet speed test, written in Go.

`speed` measures **download**, **upload**, and **latency/ping** in a single
run, showing a centered card UI with live progress and compact speed-history
graphs.

![Showcase GIF](assets/showcase.gif)

[![terminal speed test](https://img.shields.io/badge/terminal-speed%20test-39d0d8?style=flat-square)](https://github.com/Foxemsx/speed)
[![Linux](https://img.shields.io/badge/Linux-supported-2ea44f?style=flat-square&logo=linux&logoColor=white)](https://github.com/Foxemsx/speed)
[![Windows](https://img.shields.io/badge/Windows-supported-0078D6?style=flat-square&logo=windows&logoColor=white)](https://github.com/Foxemsx/speed)
Runs on Linux and Windows terminals. Any other OS supported by Go should work
too.

---

## Features

- **All-in-one test** — download, upload, and ping in a single run.
- **Parallel connections** (~5) to saturate your link, like fast.com.
- **Centered, rounded-border card** that reflows on resize and looks good at
  any terminal size.
- **Custom full-screen background** via `--bg <hex>` (falls back to the
  terminal's default background when omitted).
- **Distinct accent colors** for download (teal) vs upload (amber), with a
  small reskinnable theme.
- **Compact live sparklines** beside each speed readout, refreshed every
  ~100 ms.
- **Smooth interpolation** (lerp) so the numbers and bars glide instead of
  snapping.
- **Phase progression**: finding servers (spinner) → download → upload →
  latency → a one-line summary with peak values.
- **Graceful errors**: no internet / network failures show a clear message,
  never a stack trace.
- Quits cleanly on **q / esc / ctrl+c**, cancelling in-flight transfers.

---

## Installation

`speed` is distributed as a single static binary — no runtime dependencies.

### Prerequisites

You only need the **Go toolchain (1.23 or newer)** to install. Download it
from <https://go.dev/dl/>. Verify the install with:

```sh
go version   # should print go1.23 or later
```

> After installing Go, make sure `$GOPATH/bin` (usually `~/go/bin` on Linux,
> `%USERPROFILE%\go\bin` on Windows) is on your `PATH`, so the `speed`
> command is reachable after install. On most setups the official Go
> installer adds it for you.

### Option 1 — `go install` (recommended)

This compiles and installs the latest release into your Go bin directory in
one step.

**Linux / macOS**

```sh
go install github.com/Foxemsx/speed@latest
```

Then run it from anywhere:

```sh
speed
```

If `speed` isn't found, add Go's bin directory to your `PATH`:

```sh
# add to ~/.bashrc, ~/.zshrc, etc.
export PATH="$PATH:$(go env GOPATH)/bin"
```

**Windows (PowerShell)**

```powershell
go install github.com/Foxemsx/speed@latest
```

Go installs the binary to `%USERPROFILE%\go\bin\speed.exe`. To run it from any
folder, add that directory to your `PATH`:

```powershell
# Run once in an admin PowerShell, then restart the terminal
$env:Path += ";$env:USERPROFILE\go\bin"
[Environment]::SetEnvironmentVariable("Path", $env:Path, "User")
```

Then:

```powershell
speed
```

### Option 2 — Build from source

Use this if you don't have `go install` set up, want a local tweak, or prefer
to build manually.

```sh
git clone https://github.com/Foxemsx/speed
cd speed
go build -o speed .      # on Windows: go build -o speed.exe .
```

Run the binary from the folder you built it in:

```sh
./speed        # Linux / macOS
.\speed.exe    # Windows
```

To make it available everywhere, copy the resulting binary into a folder on
your `PATH` (for example `/usr/local/bin` on Linux).

### Arch Linux

There is no Arch package in the repository yet — use **Option 1** or
**Option 2** above.

---

## Usage

```sh
speed                 # run the test with the terminal's default background
speed --bg "#0d1117"  # run with a custom full-screen background color
speed --bg 0d1117     # the leading '#' is optional
```

### Flags

| Flag      | Description                                                                       | Default   |
| --------- | --------------------------------------------------------------------------------- | --------- |
| `--bg`    | Full-screen background color as hex (`#rrggbb`). Omit to use the terminal native background. | _(none)_   |
| `--theme` | Color theme name. Currently only `default`. Reserved for future palettes.          | `default` |

After a run completes, press **q**, **esc**, or **ctrl+c** to quit — you can
quit at any time to abort an in-progress test.

---

## License

Released under the [MIT License](LICENSE). Free to use, modify, and
redistribute, provided the copyright notice and license text are included.