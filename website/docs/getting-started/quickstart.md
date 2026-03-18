---
title: Quickstart
---

# Quickstart

**Goal:** get a generated command tree running in under 5 minutes.

This quickstart uses **embedded mode** because it removes one moving part — you do not need a separate daemon just to inspect the catalog. You will need `ocli` from [Installation](./installation) before continuing.

:::tip First win
After step 1, you will already have a working demo. **If you only need a first win, stop at step 1** and return when you are ready to point at your own API.
:::

## 1. Try the built-in demo — zero config ✓

No config file, no OpenAPI document, nothing to create:

```bash
ocli --demo catalog list --format pretty
```

This uses a built-in sample API to show how open-cli works. If you see catalog output, `ocli` is installed and working.

## 2. Set up your own API with `ocli init`

Point `ocli init` at any OpenAPI spec to generate a `.cli.json` automatically:

```bash
ocli init https://petstore3.swagger.io/api/v3/openapi.json
```

This creates a `.cli.json` in the current directory. You can also pass a local file path:

```bash
ocli init ./my-api.openapi.yaml
```

:::info Manual config
If you prefer to create the config by hand, see the [Configuration overview](../configuration/overview) for the full schema. A minimal example:

```json title=".cli.json"
{
  "cli": "1.0.0",
  "mode": { "default": "discover" },
  "sources": {
    "ticketsSource": {
      "type": "openapi",
      "uri": "./tickets.openapi.yaml",
      "enabled": true
    }
  },
  "services": {
    "tickets": {
      "source": "ticketsSource",
      "alias": "helpdesk"
    }
  }
}
```
:::

## 3. Inspect the catalog

```bash
ocli --embedded --config ./.cli.json catalog list --format pretty
```

**What to expect:**

- The response contains a full `catalog` plus the selected `view`.
- Service aliases become top-level command names.
- Operations become tools such as `tickets:listTickets` and `tickets:getTicket`.
- Generated commands are based on `operationId`: `list-tickets` and `get-ticket`.

If you see catalog output, discovery succeeded.

## 4. Inspect tool metadata before executing anything

These are the safest first commands because they do **not** call the upstream API:

```bash
ocli --embedded --config ./.cli.json tool schema tickets:listTickets --format pretty
ocli --embedded --config ./.cli.json explain tickets:listTickets --format pretty
```

## 5. Preview the dynamic command tree

```bash
ocli --embedded --config ./.cli.json helpdesk tickets --help
```

This help renders correctly because the runtime and config are already available. Without `--embedded` and a valid config path, top-level help can fail before Cobra renders anything — that is expected behavior, not a bug.

## 6. Start the daemon when you want a reusable runtime

```bash
oclird --config ./.cli.json --addr 127.0.0.1:8765
```

In another shell:

```bash
ocli --runtime http://127.0.0.1:8765 --config ./.cli.json catalog list --format pretty
```

`oclird` writes instance metadata to `runtime.json`, so later `ocli` commands can resolve the runtime automatically when the instance ID and state directory line up. See [Deployment models](../runtime/deployment-models) and [Tracing and instances](../operations/tracing-and-instances).

## 7. Execute a real tool only when the upstream API is reachable

With the sample config, a dynamic command looks like this:

```bash
ocli --runtime http://127.0.0.1:8765 --config ./.cli.json helpdesk tickets list-tickets --status open
```

This calls the first OpenAPI server URL (`https://api.example.com` in this sample). Replace it with a real service before expecting a successful response.

## Where to go next

- [Choose your path](./choose-your-path) — pick the runtime model and reading order that fits your goal.
- [Configuration overview](../configuration/overview) — add overlays, skill manifests, and workflows.
- [CLI overview](../cli/overview) — learn the full command tree model.
- [Discovery & Catalog overview](../discovery-catalog/overview) — understand how discovery and normalization work.
