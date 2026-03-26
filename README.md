# open-cli

**`open-cli`** is the core client product: a remote-only, policy-aware command surface for OpenAPI descriptions and MCP servers. **`open-cli-toolbox`** is the reference hosted runtime/server implementation that `open-cli` talks to, but it is installed separately and can be replaced by other server implementations that honor the same contract.

This is the Go reference implementation of the [Open CLI specification](spec/).

**Full docs:** https://open-cli.dev/

---

## Installation

### npm (recommended for the client)

```bash
npm install -g @sbuglione/open-cli
```

This installs **`open-cli` only**. The package automatically downloads the correct pre-built client binary for your platform (macOS, Linux, Windows — x64 and arm64).

### Install `open-cli-toolbox` separately

`open-cli-toolbox` is intentionally **not** bundled in the npm package. Install it separately from the same [GitHub Releases](https://github.com/StevenBuglione/open-cli/releases) page or run it via Docker from this repo.

```bash
docker pull ghcr.io/stevenbuglione/open-cli-toolbox:latest
```

### Download release binaries

Each GitHub Release publishes separate archives for the two products:

- `open-cli_<version>_<os>_<arch>.tar.gz|zip`
- `open-cli-toolbox_<version>_<os>_<arch>.tar.gz|zip`

Download only the product you need, extract it, and place the binary on your `PATH`.

### Build from source

Requires Go 1.25.1+:

```bash
go install github.com/StevenBuglione/open-cli/cmd/open-cli@latest
go install github.com/StevenBuglione/open-cli/cmd/open-cli-toolbox@latest
```

---

## Two products, one remote-only model

| Product | Role | Install surface |
|--------|------|-----------------|
| `open-cli` | Operator-facing client. Renders the effective catalog, exposes dynamic commands derived from your OpenAPI or MCP sources, and forwards execution requests to the hosted runtime. | npm, release binary, or source build |
| `open-cli-toolbox` | Reference hosted runtime/server. Loads config, performs discovery, normalizes catalogs, resolves auth, enforces policy, executes upstream HTTP requests, and records audit events. | Release binary, Docker, or source build |

`open-cli` always needs a remote runtime. `open-cli-toolbox` is the default reference server for that hosted boundary: operators host it, secure it, and point `open-cli` at it with `--runtime` or `runtime.remote.url`.

---

## Quick Start

### Set up your own API

    open-cli init https://petstore3.swagger.io/api/v3/openapi.json

This creates a `.cli.json` configuration from your OpenAPI spec.

### Manual configuration

**Prerequisites:** install `open-cli`, then ensure you have a reachable hosted runtime such as `open-cli-toolbox`.

Create a minimal `.cli.json` pointing at an OpenAPI document:

```json
{
  "cli": "1.0.0",
  "mode": { "default": "discover" },
  "runtime": {
    "mode": "remote",
    "remote": {
      "url": "https://toolbox.example.com"
    }
  },
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

Inspect the catalog through your hosted runtime:

```bash
open-cli --config ./.cli.json catalog list --format pretty
```

This prints the normalized catalog visible through the hosted runtime: service aliases, tools, and generated command names derived from your OpenAPI document.

Inspect a specific tool before executing it:

```bash
open-cli --config ./.cli.json tool schema tickets:listTickets --format pretty
open-cli --config ./.cli.json explain tickets:listTickets --format pretty
```

Preview the generated command tree:

```bash
open-cli --config ./.cli.json helpdesk tickets --help
```

For a complete walkthrough including a sample OpenAPI document, see the [quickstart](https://open-cli.dev/docs/getting-started/quickstart).

---

## Runtime deployment

`open-cli-toolbox` is the default reference runtime/server implementation.

You can host it anywhere you control. Even for localhost evaluation, treat it as a hosted security boundary and enable runtime auth instead of using an unauthenticated shortcut.

```bash
./examples/local-authentik/setup.sh
./examples/local-authentik/up.sh
source ./.open-cli-local/authentik/client.env
open-cli --config ./.open-cli-local/authentik/client.cli.json catalog list --format pretty
```

That local Docker Compose flow keeps localhost simple while still enforcing:

- `runtime.server.auth.validationProfile: "oidc_jwks"` on `open-cli-toolbox`
- Authentik-issued runtime tokens
- workload-style `oauthClient` auth from `open-cli`

**Config-driven selection** — avoid flags by declaring the remote runtime in `.cli.json`:

```json
{
  "runtime": {
    "mode": "remote",
    "remote": {
      "url": "http://127.0.0.1:8765"
    }
  }
}
```

Manual config can still be restrictive, but runtime reachability and tool exposure are resolved from the hosted runtime plus the token scopes it accepts.

The repo's authenticated local runtime flow lives in [`examples/local-authentik/`](examples/local-authentik/README.md).

---

## Auth, policy, and audit

Auth and policy enforcement live inside the runtime, not in the CLI layer.

**Per-request auth resolution** — OpenAPI `oauth2` and `openIdConnect` flows; MCP `streamable-http` with `clientCredentials` OAuth and `headerSecrets`; MCP transports `stdio`, legacy `sse`, and `streamable-http`; per-instance token caching under the runtime state directory.

**Remote runtime bearer auth** — when `open-cli-toolbox` is deployed with `runtime.server.auth` configured, it:

- validates bearer tokens against `oidc_jwks` or `oauth2_introspection`
- filters the visible catalog by `bundle:*`, `profile:*`, and `tool:*` scopes
- re-checks execution against the resolved authorization envelope
- records audit events for connect, auth failures, authz denials, token refresh, session lifecycle, and tool execution
- exposes the audit log at `GET /v1/audit/events`

Remote client auth modes — `providedToken` (forward a bearer token from an env reference), `oauthClient` (acquire a client-credentials token), and `browserLogin` (authorization-code + PKCE flow against the runtime's broker).

**Reference proof** — the repository ships an Authentik-based reference proof for both `oauthClient` and `browserLogin` runtime auth paths:

- Repo assets: `examples/runtime-auth-broker/authentik/`
- Local hosted-runtime proof: `examples/local-authentik/`
- Docs: [Authentik reference proof](https://open-cli.dev/docs/runtime/authentik-reference)
- Microsoft Entra is documented as an upstream federation target in that same proof

---

## Where to go next

| Goal | Link |
|------|------|
| Quickstart with a sample OpenAPI document | [Quickstart](https://open-cli.dev/docs/getting-started/quickstart) |
| Understand the hosted runtime model | [Deployment models](https://open-cli.dev/docs/runtime/deployment-models) |
| Configuration reference | [Configuration overview](https://open-cli.dev/docs/configuration/overview) |
| Full CLI command model | [CLI overview](https://open-cli.dev/docs/cli/overview) |
| Auth, policy, and secret sources | [Security overview](https://open-cli.dev/docs/security/overview) |
| Enterprise readiness checklist | [Enterprise overview](https://open-cli.dev/docs/enterprise/overview) |
| Contributing and development | [Development guide](https://open-cli.dev/docs/development/overview) |

---

## Repository layout

```
cmd/open-cli                 CLI entrypoint and runtime client
cmd/open-cli-toolbox     Reference hosted runtime entrypoint
internal/runtime         Runtime HTTP API and wiring
pkg/                     Shared config, discovery, catalog, policy, execution, audit, and observability code
examples/                Reference deployments, broker examples, and local operator workflows
spec/                    Normative Open CLI specification and JSON schemas
conformance/             Language-neutral conformance fixtures and expected outputs
product-tests/           End-to-end product validation lanes and fixtures
website/                 Docusaurus site content, navigation, and landing page
.github/                 CI, release, and Pages automation
```

---

## Verification

```bash
make verify             # format Go code, run tests, build both binaries
make verify-spec        # validate spec examples against schemas
make verify-conformance # run conformance fixtures against spec/schemas
make verify-all         # all three
```

Spec and conformance targets create a `.venv` and install Python dependencies automatically — no system-level pip required.

**Product tests** — end-to-end capability tests in `product-tests/`:

```bash
make product-test-smoke  # validate infra configs only, no services started (runs in CI)
make product-test-full   # current full-lane placeholder: smoke + service bring-up/tear-down (requires Docker)
```

**Docs site** — when `website/` or repo-facing docs change:

```bash
cd website && npm ci && npm run build
```
