# dbibackend

PC-side server for installing games into Nintendo Switch via USB (DBI0 protocol).

Fork of [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend), rewritten in Go.

## Features

- Single binary — no Python or runtime dependencies
- Cross-platform: macOS, Linux, Windows
- NFC-normalized filenames (fixes Korean/Unicode display on Switch)
- GoReleaser + Homebrew tap distribution

## Requirements

Host:
- [libusb](https://libusb.info/)

Nintendo Switch:
- [DBI](https://github.com/rashevskyv/dbi) v202+

## Install

### Homebrew (macOS)

```bash
brew install kyungw00k/cli/dbibackend
```

### Download

Download the latest binary from [Releases](https://github.com/kyungw00k/dbibackend/releases).

## Usage

```bash
dbibackend <titles_dir> [--debug]
```

1. Run `dbibackend` with the path to your NSP/NSZ/XCI files
2. On Switch, open DBI → Install title from USB
3. Select and install titles

## Build from source

```bash
go build -o dbibackend .
```

## License

MIT
