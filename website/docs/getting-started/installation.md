---
title: Installation
---

# Installation

`open-cli` is a two-product repository: `ocli` is the client, and `open-cli-toolbox` is the reference hosted runtime. The supported model is remote-only: `ocli` always talks to a reachable runtime.

## npm (Recommended for the client)

```bash
npm install -g @sbuglione/open-cli
```

The npm package installs **`ocli` only**. During `postinstall`, it downloads the correct pre-built client binary for your platform.

If you also want the reference hosted runtime, install `open-cli-toolbox` separately from the same GitHub Releases page or via Docker from this repo.

## Binary Download

Pre-built binaries for every supported platform are attached to each [GitHub Release](https://github.com/StevenBuglione/open-cli/releases). The release publishes separate archives for each product:

- `ocli_<version>_<os>_<arch>.tar.gz|zip`
- `open-cli-toolbox_<version>_<os>_<arch>.tar.gz|zip`

Install only the binary you need.

**macOS / Linux:**

```bash
tar xzf ocli_<version>_<os>_<arch>.tar.gz
sudo mv ocli /usr/local/bin/

# Optional: install the reference runtime separately
tar xzf open-cli-toolbox_<version>_<os>_<arch>.tar.gz
sudo mv open-cli-toolbox /usr/local/bin/
```

**Windows:**

Extract the `.zip` archive for the product you want and add the folder containing `ocli.exe` and/or `open-cli-toolbox.exe` to your system `PATH`.

## From Source

Requires **Go 1.25.1+**.

**Install into your Go bin directory:**

```bash
go install github.com/StevenBuglione/open-cli/cmd/ocli@latest
go install github.com/StevenBuglione/open-cli/cmd/open-cli-toolbox@latest
```

**Or build from a local clone (for contributors):**

```bash
git clone https://github.com/StevenBuglione/open-cli.git
cd open-cli
go build -o ./bin/ocli ./cmd/ocli
go build -o ./bin/open-cli-toolbox ./cmd/open-cli-toolbox
```

## Verify Installation

```bash
ocli --version
```

If you also installed the reference runtime:

```bash
open-cli-toolbox --help
```

If those commands work, continue to [Quickstart](./quickstart).

## Platform Support

| OS | x64 | arm64 |
|----|-----|-------|
| macOS | ✅ | ✅ |
| Linux | ✅ | ✅ |
| Windows | ✅ | ✅ |

## Troubleshooting

- **`npm install` fails behind a proxy** — set `HTTPS_PROXY` before installing. The postinstall script follows standard proxy environment variables.
- **Permission denied on global install** — use `sudo npm install -g @sbuglione/open-cli` or configure npm to use a user-writable prefix (`npm config set prefix ~/.npm-global`).
- **Client binary not found after install** — ensure your npm global `bin` directory is on your `PATH` (`npm bin -g`).
- **Toolbox missing after npm install** — expected behavior; install `open-cli-toolbox` separately from GitHub Releases or Docker.
- **Go build fails** — verify your Go version with `go version`; the minimum required is Go 1.25.1.
