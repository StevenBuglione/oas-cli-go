# oas-cli-go

Go reference implementation for OAS-CLI, including:

- `oascli`: the CLI surface that renders effective tool commands and delegates execution
- `oasclird`: the local runtime that performs discovery, normalization, policy enforcement, auth resolution, and audit logging

## Features

- `.cli.json` scope merging with deny-wins policy semantics
- Managed/User/Project/Local scope file discovery
- RFC 9727 API catalog discovery
- RFC 8631 service discovery via `Link` headers
- OpenAPI ingestion with overlay application
- Normalized Tool Catalog generation with stable tool IDs
- agent profiles, curated mode enforcement, and approval gates
- audited HTTP execution with bearer, basic, and API key auth adapters
- skill manifest guidance and Arazzo workflow loading

## Quick Start

```bash
make test
go run ./cmd/oasclird --addr 127.0.0.1:8765
go run ./cmd/oascli --runtime http://127.0.0.1:8765 --config /path/to/.cli.json catalog list --format pretty
```

## Verification

```bash
make verify
```

## Layout

- `cmd/oascli`: CLI entrypoint
- `cmd/oasclird`: runtime entrypoint
- `internal/runtime`: runtime HTTP API
- `pkg/config`: config loading, merge, and validation
- `pkg/discovery`: RFC 9727 and RFC 8631 discovery
- `pkg/openapi`: OpenAPI document loading
- `pkg/overlay`: overlay parsing and application
- `pkg/catalog`: normalized catalog generation
- `pkg/policy`: execution-time policy decisions
- `pkg/exec`: HTTP request execution
- `pkg/audit`: audit event persistence

## CLI Body Input

`oascli` supports all v1 request body forms required by the profile:

- `--body '{"key":"value"}'`
- `--body @request.json`
- `--body -` to read the body from stdin
