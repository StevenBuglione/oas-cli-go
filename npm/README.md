# @sbuglione/open-cli

> Turn OpenAPI descriptions and MCP servers into a local, policy-aware command surface.

## What is open-cli?

**open-cli** (`ocli`) converts OpenAPI specs and MCP server definitions into executable CLI commands, so you can explore, test, and audit APIs from your terminal. A companion daemon (`oclird`) runs alongside to manage runtime state and policy evaluation.

## Platform Support

| OS      | x64 | arm64 |
|---------|-----|-------|
| macOS   | ✓   | ✓     |
| Linux   | ✓   | ✓     |
| Windows | ✓   | ✓     |

## Installation

```bash
npm install -g @sbuglione/open-cli
```

### What happens during install

The `postinstall` script automatically downloads the correct pre-built `ocli` and `oclird` binaries for your platform from [GitHub Releases](https://github.com/StevenBuglione/open-cli/releases). No compiler or Go toolchain is needed.

## Quick Start

```bash
# 1. Install globally
npm install -g @sbuglione/open-cli

# 2. Explore the built-in demo API
ocli --demo

# 3. Point at your own API
ocli init <your-api>
```

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `binary not found at …` | `postinstall` didn't run or failed | Run `npm install -g @sbuglione/open-cli` again |
| `HTTP 404` during install | Version/platform mismatch or unpublished release | Check [releases](https://github.com/StevenBuglione/open-cli/releases) for your platform |
| SSL / proxy errors | Corporate proxy or firewall blocking GitHub | Set `https_proxy` env var or download the binary manually |
| Download timeout | Slow connection | Set `OPEN_CLI_DOWNLOAD_TIMEOUT=120` (seconds) before install |
| `permission denied` | Binary lacks execute permission | Run `chmod +x $(npm prefix -g)/lib/node_modules/@sbuglione/open-cli/bin/ocli` |

## Configuration

See the full documentation at [https://open-cli.dev/](https://open-cli.dev/) for configuration options, policy files, and advanced usage.

## License

[GPL-3.0](https://github.com/StevenBuglione/open-cli/blob/main/LICENSE)
