# Complete CLI Experience Design

## Problem

ocli has strong governance, auth, and multi-source normalization — genuine advantages over MCP. But the CLI surface is incomplete: `init` doesn't validate specs or detect auth, there's no way to manage config or auth without editing JSON by hand, no tool search, no filtering, no dry-run, and 48% of error paths give no guidance. New users hit walls; power users reach for workarounds.

## Goal

Make ocli the best CLI for both audiences:
- **New users**: `ocli init <url>` just works — parses spec, detects auth, tells you what to do next. No manual JSON editing required.
- **Power users**: search, filter, dry-run, interactive prompts — daily workflow is fast and predictable.

## Architecture

All new features are CLI-layer additions in `cmd/ocli/internal/commands/`. No changes to the runtime API (`/v1/*` endpoints), NTC schema, .cli.json schema, or policy model. New commands follow the existing pattern: Cobra commands registered in `root.go`, output via `WriteOutput()`, errors via `FormatError()`/`NewUserError()`.

## New Files

| File | Responsibility |
|------|---------------|
| `cmd/ocli/internal/commands/config.go` | `ocli config show`, `add-source`, `remove-source`, `add-secret` |
| `cmd/ocli/internal/commands/auth.go` | `ocli auth login`, `status`, `logout` |
| `cmd/ocli/internal/commands/search.go` | `ocli search <pattern>` fuzzy tool search |
| `cmd/ocli/internal/commands/status.go` | `ocli status` health check |

## Modified Files

| File | Changes |
|------|---------|
| `cmd/ocli/internal/commands/init.go` | Parse spec with kin-openapi, detect securitySchemes, validate content, warn about relative servers, show tool count, print auth-aware next steps, support `--type mcp` |
| `cmd/ocli/internal/commands/catalog.go` | Add `--service`, `--group`, `--safety` filter flags to `catalog list` |
| `cmd/ocli/internal/commands/dynamic.go` | Add `--dry-run` flag, interactive TTY prompts for missing path params |
| `cmd/ocli/internal/commands/root.go` | Register new commands (config, auth, search, status) |
| `cmd/ocli/internal/commands/errors.go` | Add error types for remaining unstructured error paths |
| `cmd/ocli/main.go` | Wire new commands into root command |

---

## Feature 1: Smart Init

### Current Behavior
`ocli init <url>` does a HEAD request to check reachability, derives a service name from the filename, writes a minimal `.cli.json`, and prints next steps that assume no auth.

### New Behavior

```
$ ocli init https://petstore3.swagger.io/api/v3/openapi.json

Parsing spec... ✓ OpenAPI 3.0.3
Found 19 tools across 3 groups (pet, store, user)

⚠ This API requires authentication:
  • petstore_auth (oauth2) — authorization code flow
  • api_key (apiKey) — header: api_key

⚠ Server URL is relative: /api/v3
  Resolved against spec host: https://petstore3.swagger.io/api/v3

Created .cli.json

Next steps:
  ocli catalog list                                     List available tools
  ocli config add-secret petstore_auth --type oauth2    Configure OAuth credentials
  ocli auth login                                       Start browser login flow
```

### MCP Source Support

```
$ ocli init --type mcp --transport stdio \
    --command "npx" --args "-y,@modelcontextprotocol/server-filesystem,/workspace" \
    filesystem

Created .cli.json with MCP source "filesystem" (stdio transport)

Next steps:
  ocli catalog list             List available tools
  ocli filesystem --help        See filesystem commands
```

### Implementation Details

1. After reachability check, fetch full spec body (GET, not just HEAD)
2. Parse with `openapi3.Loader{}` from `github.com/getkin/kin-openapi` (already in go.mod)
3. Validate: check `doc.OpenAPI`, `doc.Info`, `doc.Paths` are present
4. Detect auth: iterate `doc.Components.SecuritySchemes` — report type (oauth2, apiKey, http) and flow details
5. Detect relative servers: check `doc.Servers[].URL` for relative paths, resolve against spec host
6. Count tools: count operations across all paths
7. Generate config: same schema as today, but include auth hints in next-steps output
8. For `--type mcp`: generate MCP source config with transport block

### New Flags on `init`
- `--type` (string, default "openapi"): Source type — "openapi" or "mcp"
- `--transport` (string): MCP transport type — "stdio", "sse", "streamable-http"
- `--command` (string): MCP stdio command
- `--args` (string): Comma-separated MCP stdio args
- `--url` (string): MCP sse/streamable-http URL
- `--global` (bool, existing): Write to user scope

---

## Feature 2: Config Management

### `ocli config show`

Loads effective config (merged across all scopes) and renders sources + config file locations.

**Table output (TTY):**
```
SOURCE           TYPE     URI                                            ENABLED
openapiSource    openapi  https://petstore3.swagger.io/.../openapi.json  true
filesystem       mcp      stdio: npx -y @modelcontextprotocol/...        true

Config files:
  /etc/oas-cli/.cli.json                 (not found)
  /home/user/.config/oas-cli/.cli.json   (not found)
  .cli.json                              (active)
  .cli.local.json                        (not found)
```

Note: The effective config is a merge of all scopes. Individual source origin tracking is not available — the "Config files" section shows which scope files exist so the user can determine provenance by inspection.

**JSON output (piped):** Full effective config as JSON.

### `ocli config add-source <name> [flags]`

Adds a source + matching service entry to the project-scope `.cli.json` (or user-scope with `--global`).

**Flags:**
- `--type` (string, default "openapi"): "openapi", "mcp", "apiCatalog", "serviceRoot"
- `--uri` (string): URI for openapi/apiCatalog/serviceRoot sources
- `--transport` (string): MCP transport type
- `--command` (string): MCP stdio command
- `--args` (string): Comma-separated MCP stdio args
- `--url` (string): MCP sse/streamable-http URL
- `--alias` (string, default same as name): Service alias
- `--global` (bool): Write to user scope

**Behavior:**
1. Read existing `.cli.json` as `map[string]any` (preserves unknown fields). If `.cli.json` doesn't exist, create a minimal skeleton (`cli: "1.0.0"`, `mode: {default: "discover"}`) before adding the source.
2. Add to `sources` map and `services` map
3. Write back with `json.MarshalIndent`
4. Error if source name already exists (suggest `remove-source` first)

### `ocli config add-secret <name> [flags]`

Adds a secret entry to the project-scope `.cli.json` (or user-scope with `--global`).

**Flags:**
- `--type` (string, required): "oauth2", "env", "file", "osKeychain"
- `--mode` (string): OAuth mode — "authorizationCode", "clientCredentials"
- `--token-url` (string): OAuth token URL
- `--client-id-env` (string): Env var name for client ID
- `--client-secret-env` (string): Env var name for client secret
- `--scopes` (string): Comma-separated scopes
- `--global` (bool): Write to user scope

**Example:**
```
ocli config add-secret petstore_auth --type oauth2 --mode clientCredentials \
  --token-url https://auth.example.com/token \
  --client-id-env PETSTORE_CLIENT_ID \
  --client-secret-env PETSTORE_CLIENT_SECRET \
  --scopes "pets.read,pets.write"
```

**Behavior:**
1. Read existing `.cli.json` as `map[string]any`
2. Add to `secrets` map with the appropriate structure based on `--type`
3. For `--type oauth2`: generate `{ type: "oauth2", mode, tokenURL, clientId: {type: "env", value}, clientSecret: {type: "env", value}, scopes }`
4. For `--type env`: generate `{ type: "env", value: <env-var-name> }`
5. Write back with `json.MarshalIndent`
6. Error if secret name already exists

### `ocli config remove-source <name>`

Removes a source and its corresponding service from `.cli.json`.

**Flags:**
- `--global` (bool): Modify user-scope config

**Behavior:**
1. Read existing `.cli.json` as `map[string]any`
2. Delete from `sources` map
3. Delete matching service (where `service.source == name`)
4. Write back
5. Error with suggestion if source not found

---

## Feature 3: Auth Commands

### Architecture Note

The CLI authenticates to the **runtime daemon** (not to individual upstream APIs). The runtime manages per-service token resolution internally. Therefore, auth commands operate at the CLI→runtime level, not per-service.

### `ocli auth login`

Triggers the runtime's browser-based OAuth login flow. This authenticates the CLI session to the runtime daemon.

**Behavior:**
1. Check if runtime is reachable (handshake via `/v1/runtime/info`)
2. If runtime supports browser login: trigger via existing `auth.AcquireBrowserLoginToken()`
3. Store token in runtime session
4. Print confirmation: "✓ Logged in to runtime (token expires in 1h)"

**Error cases:**
- Runtime not running → "Runtime not available. Use `--embedded` or start with `oclird`."
- Runtime doesn't require auth → "Runtime doesn't require authentication."
- Browser flow fails → structured error with suggestion

### `ocli auth status`

Shows authentication configuration derived from the local config (not live token state, since the runtime manages tokens internally).

**Table output:**
```
SERVICE    AUTH TYPE         CONFIGURED
openapi    oauth2            ✓ secret: petstore_auth
filesystem none              no auth required
remote     apiKey            ✗ missing secret
```

**Behavior:**
1. Load config, enumerate services and their sources
2. For each service, check source auth requirements (OpenAPI securitySchemes, MCP oauth)
3. Check if matching secrets exist in config
4. Report configured vs missing — this is a **config audit**, not live token state
5. Runtime is NOT required for this command (works offline)

### `ocli auth logout`

Closes the runtime session and clears all cached authentication state.

**Behavior:**
1. Call `/v1/runtime/session-close` to close the entire session
2. Print confirmation: "✓ Session closed. Cached tokens cleared."
3. Note: this clears ALL session state, not per-service. This is correct because the runtime manages tokens as a unit.

**Error cases:**
- Runtime not running → "No active session to close."

---

## Feature 4: Tool Search

### `ocli search <pattern>`

Case-insensitive fuzzy search across tool ID, command name, summary, and description.

**Example:**
```
$ ocli search "create"
SERVICE  GROUP       COMMAND           METHOD  SUMMARY
demo     items       create-item       POST    Create an item
demo     operations  create-operation  POST    Start a long-running operation
```

**Behavior:**
1. Load catalog (same as any runtime-dependent command)
2. For each tool, check if pattern appears in: ID, Command, Summary, Description (case-insensitive `strings.Contains`)
3. Render matches using same table format as `catalog list`
4. If no matches: "No tools matching '<pattern>'. Run `ocli catalog list` to see all tools."

**Flags:**
- `--service` (string): Limit search to one service
- Inherits `--format` from global flags

---

## Feature 5: Catalog Filtering

### New Flags on `catalog list`

- `--service` (string): Filter by service alias (exact match)
- `--group` (string): Filter by group name (exact match)
- `--safety` (string): Filter by safety level — "read-only", "destructive", "requires-approval", "idempotent"

**Examples:**
```
$ ocli catalog list --service demo --group items
$ ocli catalog list --safety read-only
$ ocli catalog list --safety destructive
```

**Behavior:**
1. After loading catalog, apply filters to `view.Tools` array
2. `--service`: keep tools where `tool.ServiceID` matches (or service alias matches)
3. `--group`: keep tools where `tool.Group` matches
4. `--safety read-only`: keep tools where `tool.Safety.ReadOnly == true`
5. `--safety destructive`: keep tools where `tool.Safety.Destructive == true`
6. `--safety requires-approval`: keep tools where `tool.Safety.RequiresApproval == true`
7. `--safety idempotent`: keep tools where `tool.Safety.Idempotent == true`
8. Filters are AND-combined

---

## Feature 6: Dry-Run

### `--dry-run` Flag on Dynamic Tool Commands

Shows the HTTP request that would be sent, without executing it. Since auth resolution happens inside the runtime (not the CLI), auth headers are not shown.

**Example:**
```
$ ocli demo items create-item --body '{"name":"test"}' --dry-run

POST http://localhost:8080/items
Content-Type: application/json

{"name":"test"}
```

**Behavior:**
1. Add `--dry-run` bool flag to every dynamic tool command (in `dynamic.go` command builder)
2. When set: resolve path parameters into the URL, resolve query params, determine method
3. Print: method, full URL (with path params and query params resolved), Content-Type header, body
4. Auth headers are NOT shown (they're resolved inside the runtime, not available to the CLI)
5. Do NOT send the HTTP request — do NOT call `client.Execute()`
6. Exit with success

**Implementation:**
- Build URL from: tool's servers[0] + tool.Path with pathArgs substituted + query params from flags
- Use the tool metadata already available in dynamic.go (tool.Method, tool.Path, tool.PathParams, tool.Flags)
- No runtime interaction needed — this is pure CLI-side URL construction

---

## Feature 7: Interactive Prompts

### TTY-Aware Parameter Prompts

When running a tool command interactively and missing required path parameters, prompt for them instead of erroring.

**Example:**
```
$ ocli demo items get-item
Enter id: 42
{"id":"42","name":"Widget",...}
```

**Behavior:**
1. Change Cobra `Args` from `ExactArgs(N)` to custom validator
2. In the validator: if args < required AND stdin is a terminal, prompt for each missing param
3. TTY detection for stdin: add `IsTerminalReader(r io.Reader) bool` helper that checks if the underlying `*os.File` has `ModeCharDevice` (same pattern as existing `IsTerminal()` in table.go but for `io.Reader`)
4. Use `fmt.Fprintf(cmd.ErrOrStderr(), "Enter %s: ", param.Name)` + `bufio.Scanner` on stdin
5. If NOT TTY (piped), return normal "expected N args" error
6. Prompted values are appended to args and execution continues normally

**No new dependencies** — uses stdlib `bufio` and a new `IsTerminalReader()` helper alongside existing `IsTerminal()` from `table.go`.

---

## Feature 8: Status Command

### `ocli status`

Quick health check. Registered unconditionally (available even when runtime is down — that's exactly when you need it most).

**Table output (runtime available):**
```
Runtime:  ✓ embedded (v0.0.1)
Config:   .cli.json (project scope)
Sources:  2 active (1 openapi, 1 mcp)
```

**Behavior:**
1. Load config (always works, no runtime needed)
2. Count sources by type from config
3. List which config files exist (via `DiscoverScopePaths()`)
4. Try to reach runtime (handshake via `/v1/runtime/info`) — report success/failure
5. Render summary

**Error case (no runtime):**
```
Runtime:  ✗ not running
Config:   .cli.json (project scope)
Sources:  2 configured

Suggestion: Run with --embedded or start the daemon with oclird
```

**Registration:** This command and `search` are registered unconditionally in root.go (like `init`), NOT inside the `if !runtimeUnavailable` block. When the runtime is down, `search` shows "Runtime unavailable — cannot load catalog." while `status` degrades gracefully as shown above.

---

## Feature 9: Structured Errors Everywhere

### Current State
52% of error paths use `FormatError()` or `NewUserError()`. The remaining 48% use bare `fmt.Errorf()`.

### Target
100% of user-facing errors include Error + Cause + Suggestion.

### Error Categories to Cover

| Category | Current | Target |
|----------|---------|--------|
| Tool not found | ✓ structured | keep |
| Runtime unreachable | ✓ structured | keep |
| Init validation | ✓ structured | keep |
| Auth failures | ✗ raw | Add: "Check credentials with `ocli auth status`" |
| Invalid JSON body | ✗ raw | Add: "Body must be valid JSON. Use --body @file.json for file input" |
| Unsupported format | ✗ raw | Add: "Supported formats: json, yaml, pretty, table" |
| MCP process crash | ✗ raw | Add: "MCP server process exited. Check command: <cmd>" |
| Relative server URL | ✗ silent | Add: warning during init |
| Missing config | ✓ structured | keep |

---

## Feature 10: Test Coverage

### New Test Files

| File | Tests |
|------|-------|
| `cmd/ocli/internal/commands/init_test.go` | Spec parsing, auth detection, MCP init, name derivation, validation errors |
| `cmd/ocli/internal/commands/config_test.go` | Add/remove source, show config, scope handling |
| `cmd/ocli/internal/commands/auth_test.go` | Login flow, status display, logout |
| `cmd/ocli/internal/commands/search_test.go` | Fuzzy matching, no results, service filter |
| `cmd/ocli/internal/commands/catalog_test.go` | Filter by service/group/safety |
| `cmd/ocli/internal/commands/dynamic_test.go` | Dry-run output, interactive prompts, format handling |

### Test Strategy
- Unit tests for each command using `bytes.Buffer` for stdout capture
- Use `httptest.Server` for runtime API mocking (pattern exists in `main_test.go`)
- Test both TTY and non-TTY output paths
- Test error formatting for all new error paths

---

## Non-Goals (Explicitly Out of Scope)

- **Streaming responses** — Requires runtime API changes (`/v1/*` contract is frozen)
- **Resources primitive** — New spec concept, not a CLI-layer addition
- **Workflow composition** — Spec-level change (branching, loops, data-passing)
- **LLM prompt templates** — Spec extension, not CLI feature
- **Cost/rate metadata** — Requires new tool schema fields (NTC change)

---

## Dependencies

- `github.com/getkin/kin-openapi` — Already in go.mod, used for spec parsing in smart init
- `bufio` (stdlib) — For interactive prompts
- No new external dependencies required
