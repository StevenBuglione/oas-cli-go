---
title: CLI Overview
---

# CLI Overview

**Read this if** you are deploying or scripting `open-cli` and need to understand its command shape, flag surface, and runtime dependency. This page answers: how are commands named, what flags exist everywhere, and why does `open-cli --help` need a reachable runtime.

`open-cli` is a **runtime-backed client**. It does not ship a fixed set of service commands. Instead, it asks `open-cli-toolbox` for the effective catalog, then builds a Cobra command tree from the returned services and tools.

## Command families

Built-in command families are always present after catalog resolution:

- `catalog list`
- `tool schema <tool-id>`
- `explain <tool-id>`
- `workflow run <workflow-id>`

Everything else is dynamic and comes from the current catalog view.

## Dynamic command shape

The generated hierarchy is:

```text
open-cli <service-alias> <group> <command> [path-args] [flags]
```

How each segment is chosen:

- **service alias**: `services.<id>.alias`, or the service ID if no alias is set
- **group**: `x-cli-group`, otherwise the first OpenAPI tag, otherwise the first path segment, otherwise `misc`
- **command**: `x-cli-name`, otherwise a slugified `operationId`, otherwise an inferred verb such as `list`, `get`, `create`, `update`, `patch`, or `delete`

### Example

With this config:

```json
{
  "services": {
    "tickets": {
      "source": "ticketsSource",
      "alias": "helpdesk"
    }
  }
}
```

and an operation tagged `tickets` with `operationId: listTickets`, the generated path can be:

```bash
open-cli helpdesk tickets list-tickets
```

If the alias and group are the same, you will see both segments. For example, alias `tickets` plus group `tickets` produces `open-cli tickets tickets list-tickets`.

## Persistent flags

These flags exist on the root command and apply to built-in and dynamic commands alike:

| Flag | Meaning |
| --- | --- |
| `--runtime` | Runtime base URL. If omitted, `open-cli` tries `OPEN_CLI_RUNTIME_URL`, then `runtime.remote.url`, otherwise it errors. |
| `--config` | Path to the project `.cli.json`. |
| `--mode` | Requested mode, typically `discover` or `curated`. |
| `--agent-profile` | Agent profile name used for the selected effective view. |
| `--format` | Output format: `json` (default), `yaml`, or `pretty`. |
| `--approval` | Grants approval for tools that need it. |
| `--instance-id` | Selects an isolated runtime instance for cache, audit, and auth state. |
| `--state-dir` | Overrides the state root used for runtime metadata and derived cache paths. |

## Output behavior

- `catalog list`, `tool schema`, `explain`, and `workflow run` all respect `--format`.
- Dynamic tool execution has one extra nuance:
  - if the runtime returns JSON and `--format json`, `open-cli` prints the JSON body directly
  - if the runtime returns non-JSON text, `open-cli` prints the text directly
  - otherwise it serializes the execution wrapper (`statusCode`, `body`, or `text`)

## Help behavior quirk

:::warning Dynamic help needs a catalog
`open-cli` resolves the runtime and fetches the catalog **before** Cobra renders help. In practice, `open-cli --help` can fail if no runtime/config is available.
:::

Safe help patterns:

```bash
# explicit runtime URL
./bin/open-cli --runtime http://127.0.0.1:8765 --config ./.cli.json --help

# config-driven runtime URL
./bin/open-cli --config ./.cli.json --help
```

By contrast, `open-cli-toolbox --help` is static because it uses Go's `flag` package and does not fetch a catalog.

## Supported HTTP operations

The current catalog builder creates tools for these OpenAPI methods:

- `GET`
- `POST`
- `PUT`
- `PATCH`
- `DELETE`

Do not assume that `HEAD`, `OPTIONS`, or `TRACE` operations become CLI tools in the current implementation.

## If you are trying to…

| Goal | Go to |
| --- | --- |
| Understand how commands are named from your OpenAPI | [Catalog and explain](./catalog-and-explain) |
| Map flags and request body fields to HTTP calls | [Tool execution](./tool-execution) |
| Choose or debug how `open-cli` finds the runtime | [Deployment models](../runtime/deployment-models) |
| Run multi-step operations as a single command | [Workflow run](./workflow-run) |
| Configure which tools different agents can see | [Configuration overview](../configuration/overview) |

## Learn the next layer

- Use [Catalog and explain](./catalog-and-explain) to inspect the catalog.
- Use [Tool execution](./tool-execution) for request mapping rules.
- Use [Deployment models](../runtime/deployment-models) to understand hosted runtime deployment and resolution.
