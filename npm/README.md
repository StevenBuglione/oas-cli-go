# @sbuglione/open-cli

> Remote-only API and MCP command tooling with a separately installed hosted runtime.

## What is open-cli?

**open-cli** converts OpenAPI specs and MCP server definitions into executable CLI commands, so you can explore, test, and audit APIs from your terminal. `open-cli` always connects to a separately hosted runtime server (`open-cli-toolbox`), which handles catalog execution, token-scoped tool exposure, policy evaluation, and enterprise deployment concerns.

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

The `postinstall` script automatically downloads the correct pre-built **`open-cli`** binary for your platform from [GitHub Releases](https://github.com/StevenBuglione/open-cli/releases). No compiler or Go toolchain is needed.

`open-cli-toolbox` is **not** bundled in the npm package. Install it separately from the same GitHub Releases page or build/run it via the repo's Docker flow if you need the reference hosted runtime.

## Quick Start

```bash
# 1. Install the client
npm install -g @sbuglione/open-cli

# 2. Point open-cli at your hosted runtime
open-cli --runtime https://toolbox.example.com

# 3. Initialize your own API catalog
open-cli init <your-api>
```

`open-cli` does not embed a local execution daemon. Operators host `open-cli-toolbox`, secure it, and decide which tools are visible to each token.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `binary not found at …` | `postinstall` didn't run or failed | Run `npm install -g @sbuglione/open-cli` again |
| `HTTP 404` during install | Version/platform mismatch or unpublished release | Check [releases](https://github.com/StevenBuglione/open-cli/releases) for your platform |
| SSL / proxy errors | Corporate proxy or firewall blocking GitHub | Set `https_proxy` env var or download the binary manually |
| Download timeout | Slow connection | Set `OPEN_CLI_DOWNLOAD_TIMEOUT=120` (seconds) before install |
| `permission denied` | Binary lacks execute permission | Run `chmod +x $(npm prefix -g)/lib/node_modules/@sbuglione/open-cli/bin/open-cli` |

## Configuration

See the full documentation at [https://open-cli.dev/](https://open-cli.dev/) for configuration options, policy files, and advanced usage.

## License

[GPL-3.0](https://github.com/StevenBuglione/open-cli/blob/main/LICENSE)
