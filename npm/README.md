# @sbuglione/open-cli

**`ocli`** and **`oclird`** turn OpenAPI descriptions and MCP servers into a local, policy-aware command surface.

## Install

```bash
npm install -g @sbuglione/open-cli
```

After installation, both `ocli` and `oclird` commands are available globally.

## Usage

```bash
# Inspect your API catalog
ocli --embedded --config ./.cli.json catalog list --format pretty

# Start the runtime daemon
oclird --config ./.cli.json --addr 127.0.0.1:8765
```

## Documentation

Full docs: https://open-cli.dev/

## License

GPL-3.0
