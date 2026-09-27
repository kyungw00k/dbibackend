# dbibackend

PC-side server for installing games into Nintendo Switch via USB (DBI0 protocol).

Fork of [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend), rewritten in Go.

![Screenshot](docs/screenshot.png)

## Features

- System tray menu bar app (default) with start/stop control
- CLI mode for headless usage (`--cli`)
- Multiple directory support — scan all your title folders at once
- Cross-platform: macOS, Linux, Windows
- NFC-normalized filenames (fixes Korean/Unicode display on Switch)
- Single binary — no Python or runtime dependencies
- GoReleaser + Homebrew tap distribution

## Requirements

Host:
- macOS / Windows — no additional dependencies (libusb is embedded)
- Linux — [libusb](https://libusb.info/) (`sudo apt install libusb-1.0-0-dev` or equivalent)

Nintendo Switch:
- [DBI](https://github.com/rashevskyv/dbi) v202+

## Install

### Homebrew (macOS & Linux)

```bash
brew install kyungw00k/tap/dbibackend
```

> Linux also needs libusb: `sudo apt install libusb-1.0-0-dev` (or equivalent).

### Scoop (Windows)

```bash
scoop bucket add kyungw00k https://github.com/kyungw00k/scoop-bucket
scoop install dbibackend
```

### Download

Download the latest binary from [Releases](https://github.com/kyungw00k/dbibackend/releases).

### After installing

Run `dbibackend` — the app starts and lives in your menu bar / system tray,
then see [Usage](#usage). No arguments needed.

## Usage

### Menu bar mode (default)

```bash
dbibackend [--debug]
```

The app lives in your system tray. Click the DBI icon to:

1. **Start** — begin waiting for a Switch USB connection
2. **Add Directory** — add folders containing NSP/NSZ/XCI files
3. **Stop** — disconnect and stop the server
4. On Switch, open DBI → Install title from USB
5. Select and install titles from all configured directories

### CLI mode

```bash
dbibackend --cli <titles_dir> [--debug]
```

## Build from source

```bash
make build    # dist/dbibackend, version stamped from git tags
make test     # go test ./...
```

## License

MIT
