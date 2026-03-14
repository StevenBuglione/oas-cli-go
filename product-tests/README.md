# product-tests

End-to-end product tests for oascli. These tests spin up real infrastructure
(REST API, OAuth server, MCP servers) and exercise the CLI against them.

## Layout

```
product-tests/
  Makefile                  # Main entrypoints
  docker-compose.yml        # Core services (REST API, OAuth server)
  scripts/
    check-prereqs.sh        # Validate required tools are present
    services-up.sh          # Thin wrapper: docker compose up
    services-down.sh        # Thin wrapper: docker compose down
  mcp/
    remote/
      docker-compose.yml    # MCP remote-mode servers
    stdio/
      filesystem.env        # Env vars for MCP filesystem stdio server
      time.env              # Env vars for MCP time stdio server
  fixtures/                 # (future) static test inputs
  tests/                    # (future) test suites
```

## Required Tools

- `go` ≥ 1.22
- `python3` ≥ 3.9
- `docker` with Compose plugin (`docker compose`)
- `npx` (Node.js) for MCP server packages

## Running

```sh
# From repo root
make product-test-smoke    # quick smoke pass
make product-test-full     # full suite

# From this directory
make check-prereqs
make services-up
make smoke
make services-down
```
