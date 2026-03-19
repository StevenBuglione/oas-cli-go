# Daemon And Remote Runtime Hardening Design

## Goal

Make `ocli` and `oclird` a trustworthy, policy-enforced alternative to direct MCP usage by removing implicit embedded execution from normal workflows, making runtime targeting explicit and mandatory, binding remote authorization to a fixed runtime-owned configuration, and hardening real-world execution and operator UX.

## Scope

This design addresses the concrete gaps identified from live product use and code review:

1. `ocli` can silently behave as embedded even when the operator expects daemon-backed execution.
2. `.cli.json` does not require an explicit runtime contract.
3. Remote runtime requests authorize against caller-selected config paths instead of a fixed runtime-owned config identity.
4. Real OpenAPI specs fail at execution time because relative server URLs are not normalized into an absolute upstream base.
5. `dry-run` is blocked by auth/network resolution instead of being a safe local preview.
6. Audit and status surfaces do not clearly distinguish execution failures, authorization denials, and actual connected runtime mode.
7. Tool introspection and MCP-generated commands are still weaker than they need to be for operator and agent use.

This is an intentionally breaking change. Backward compatibility with config files that omit `runtime` or rely on embedded execution is out of scope.

## Non-Goals

This design does not attempt to:

- preserve support for `embedded` or `auto` runtime modes in normal `.cli.json` workflows
- add a migration helper for legacy configs
- solve token revocation for `oidc_jwks`
- redesign the entire command tree or invent a separate policy language

## Design

### 1. Runtime Contract Becomes Mandatory

Every normal `.cli.json` configuration must declare a `runtime` block with `mode` set to either:

- `local`
- `remote`

Normal configs must no longer accept:

- missing `runtime`
- `runtime.mode: embedded`
- `runtime.mode: auto`

Schema validation and config loading must fail clearly when the runtime block is missing or invalid.

This is the core trust change: every config must declare whether it is daemon-backed or remote-backed, and the CLI must not infer or silently alter that posture.

`--demo` remains a special internal fixture path, not a general runtime mode for user configs.

### 2. No Embedded Execution Path For Normal Configs

`ocli` must stop creating an embedded in-process runtime for ordinary `.cli.json` workflows.

Behavioral contract:

- If a config declares `runtime.mode: local`, `ocli` must connect only to a local daemon runtime.
- If a config declares `runtime.mode: remote`, `ocli` must connect only to the configured remote runtime.
- If the target runtime is unreachable, commands fail hard.
- `ocli` must never silently fall back to embedded execution when a daemon or remote runtime was expected.

Flag semantics:

- `--runtime` may override the runtime URL endpoint within the selected deployment class.
- `--runtime` must not change a `remote` config into `local`, or a `local` config into embedded.
- `--embedded` must be removed or rejected for normal configs.

This ensures the operator can trust that tool execution actually crossed the enforcement boundary they configured.

### 3. Remote Runtime Owns Config Identity

In `remote` mode, the runtime must not honor arbitrary caller-supplied `configPath`.

The runtime process must execute against a fixed runtime-owned config identity established at startup. Requests may optionally carry a config identifier for diagnostics, but not for runtime selection.

Server-side contract:

- remote `GET /v1/catalog`
- remote `POST /v1/tools/execute`
- remote workflow and status endpoints

must all operate against the runtime’s configured config, not a client-provided path.

If a remote request includes a config path or identity that does not match the runtime-owned one, the server should reject it clearly.

This closes the main security gap from the current implementation: auth scopes are evaluated against a fixed catalog boundary rather than whatever config the caller names at request time.

### 4. Runtime Status Must Reflect Actual Connection

`status` and related runtime summaries must report the runtime mode actually in use, not merely the intended config mode.

Required behavior:

- if `ocli` connected to a local daemon, report `local`
- if `ocli` connected to a remote runtime, report `remote`
- if the configured runtime is unreachable, fail instead of reporting a stale or implicit embedded mode

The runtime summary should also expose the connected runtime URL when available.

This is necessary because operator trust depends on knowing whether a given command traversed the daemon/remote control plane.

### 5. Relative OpenAPI Server URLs Must Be Resolved

When a spec is loaded from a URL and its server entry is relative, the system must resolve that server URL against the spec origin and persist or otherwise preserve an absolute upstream base for execution.

Expected behavior:

- `https://petstore3.swagger.io/api/v3/openapi.json` + server `/api/v3`
- resolves to upstream base `https://petstore3.swagger.io/api/v3`

The system may continue to surface a note that the server was relative and normalized, but execution must use the normalized absolute base.

This change applies to both init-time generated configs and direct runtime loading paths so the behavior is consistent.

### 6. `dry-run` Must Be Pure Preview

`dry-run` must render the would-be request shape without requiring:

- auth secret resolution
- token acquisition
- upstream reachability
- daemon-side tool execution

For REST tools, `dry-run` should show:

- method
- absolute URL or normalized path/base composition
- headers that are structurally known
- body if provided

For MCP tools, `dry-run` should show:

- target service/tool
- structured request payload
- any known execution posture such as approval requirement or auth requirement

If a tool requires auth or approval, `dry-run` should report that as metadata, not fail before showing the preview.

### 7. Audit Semantics Must Separate Denial From Failure

Audit events must distinguish:

- authorization or policy denial
- execution failure
- successful execution

An upstream transport failure or runtime-side execution error must not be recorded as an authz denial.

Behavioral contract:

- policy/authz rejection => denial event with denial reason
- execution/network failure => execution error event with failure reason
- empty audit log => `[]`, not `null`

This makes the runtime a more credible enforcement and observability point.

### 8. Introspection Must Work From Operator-Facing References

`explain` and `tool schema` should accept either:

- canonical tool ID (`service:operationId`)
- command-form reference derived from catalog output

Examples:

- `petstore:getPetById`
- `petstore pet get-pet-by-id`

The CLI should resolve operator-facing command references back to the canonical tool. Users should not need JSON output to discover the valid explain identifier.

### 9. MCP Tool Inputs Should Expose Simple Flags

MCP-derived commands should expose first-class flags for simple scalar parameters where the input schema permits it.

Examples:

- string
- boolean
- integer
- number

`--body` remains as a fallback for complex payloads, but common tools should not force raw JSON for one-argument operations such as `--path /tmp`.

This is required if the CLI is meant to be more usable than MCP-native raw tool calling.

## Files

### Config And Schema

- `pkg/config/cli.schema.json`
- `pkg/config/schema.go`
- `pkg/config/load.go`
- `cmd/ocli/internal/config/resolve.go`
- `cmd/ocli/internal/runtime/deployment.go`

Responsibilities:

- require `runtime`
- restrict allowed runtime modes
- reject embedded/auto for normal configs
- stop CLI-side implicit embedded resolution

### CLI Runtime Client And Commands

- `cmd/ocli/internal/runtime/client.go`
- `cmd/ocli/internal/commands/root.go`
- `cmd/ocli/internal/commands/status.go`
- `cmd/ocli/internal/commands/catalog.go`
- `cmd/ocli/internal/commands/dynamic.go`
- `cmd/ocli/internal/commands/dryrun.go`
- `cmd/ocli/internal/commands/init.go`
- `cmd/ocli/internal/commands/commands_test.go`
- `cmd/ocli/main_test.go`

Responsibilities:

- remove embedded fallback from normal config flow
- make runtime mode reporting truthful
- make dry-run independent from live auth/network
- support explain/schema resolution from command-form references
- expose MCP tool fields as flags when possible

### Runtime Server And Audit

- `internal/runtime/server.go`
- `internal/runtime/authn_jwks.go`
- `pkg/audit/store.go`
- `product-tests/tests/capability_refresh_audit_test.go`
- `product-tests/tests/capability_runtime_auth_authentik_test.go`

Responsibilities:

- bind remote execution to runtime-owned config identity
- reject mismatched config selection in remote mode
- preserve current authn support
- fix audit event classification and empty-list response

### OpenAPI Resolution

- `pkg/openapi/load.go`
- `pkg/catalog/build.go`
- `cmd/ocli/internal/commands/init.go`
- relevant OpenAPI/catalog tests

Responsibilities:

- normalize relative server URLs against spec origin
- make both init-generated and runtime-loaded execution honor the normalized base

### Docs

- `README.md`
- runtime/security docs under `website/docs/`

Responsibilities:

- document the breaking runtime contract
- document daemon-only / remote-only normal operation
- remove outdated embedded/auto claims from normal config guidance

## Testing

### Unit / Command Tests

Add or update tests for:

- missing `runtime` rejects config
- `runtime.mode: embedded` rejects config
- `runtime.mode: auto` rejects config
- unreachable configured runtime fails instead of embedding
- status reports actual connected runtime mode
- explain accepts command-form references
- dry-run for auth-gated tools still prints preview
- MCP simple parameters appear as flags when possible

### Runtime / Integration Tests

Add or update tests for:

- remote mode ignores or rejects caller-supplied config path mismatch
- local/remote runtime info reports the expected mode and URL
- execution error is not logged as authz denial
- empty audit endpoint returns `[]`
- relative OpenAPI server URLs execute successfully after normalization

### Product Validation

Re-run live validations using:

- public OpenAPI specs with relative servers
- official MCP filesystem stdio server
- official MCP everything streamable-http server
- local daemon path
- remote auth path where practical

Success criteria:

- no normal workflow succeeds without traversing the configured runtime class
- public spec onboarding produces runnable commands
- MCP operations feel at least as usable as direct tool calling for simple cases

## Risks And Trade-Offs

### Breaking Change Risk

Rejecting legacy configs without `runtime` is disruptive, but keeping the current ambiguity would preserve the core trust problem. This is the correct trade-off.

### Embedded Removal Risk

Removing embedded execution from normal configs may slow simple local workflows. That is acceptable because the project goal is a trustworthy runtime boundary, not convenience through silent bypass.

### Remote Binding Simplicity

Binding remote mode to a fixed runtime-owned config is deliberately restrictive. That is a feature, not a bug, for the security model being claimed.

## Rollout Criteria

This hardening pass is complete only when:

1. Normal `.cli.json` workflows require `runtime.mode: local|remote`.
2. `ocli` cannot silently embed when daemon or remote execution was expected.
3. Remote runtime requests are bound to a fixed config identity.
4. Relative OpenAPI servers execute correctly.
5. `dry-run` is a true preview path.
6. Audit and status reporting align with real execution behavior.
7. Live `ocli` and `oclird` runs against public APIs and official MCP servers demonstrate the intended boundary and usability model.
