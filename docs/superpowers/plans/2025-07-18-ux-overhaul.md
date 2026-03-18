# UX Overhaul & Architecture Cleanup — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform open-cli from a tool that crashes on first run into one that guides new users to success in under 60 seconds, while cleaning up the monolithic main.go and hardening the npm distribution.

**Architecture:** Four parallel work streams. Stream A (npm hardening) and Stream D (documentation) are fully independent. Stream B (main.go refactoring phases 1–3) extracts existing code into subpackages. Stream C (CLI UX features) builds new features on the clean structure from Stream B.

**Tech Stack:** Go 1.25.1, Cobra CLI, Node.js 16+, go:embed, npm postinstall

**Spec:** `docs/superpowers/specs/2025-07-18-ux-overhaul-design.md`

---

## Stream Dependencies

```
Stream A (npm hardening)     ──── independent ────────────────────────►
Stream D (documentation)     ──── independent (update after C done) ──►
Stream B (refactor phases 1-3) ──► Stream C (UX features + phases 4-5)
```

Streams A and D can run in parallel with everything. Stream C depends on Stream B completing phases 1–3.

---

## Stream A: npm Package Hardening

### Task 1: Rewrite `npm/install.js` with retry, timeout, and validation

**Files:**
- Modify: `npm/install.js` (full rewrite, currently 161 lines)

- [ ] **Step 1: Read current install.js and understand the flow**

Read `npm/install.js`. Current flow:
1. `platformConfig()` maps os/arch (line 10–40)
2. `followRedirects()` downloads archive (line 59–83)
3. `extractArchive()` unpacks tar.gz/zip (line 85–125)
4. `main()` orchestrates (line 127–160)

- [ ] **Step 2: Rewrite install.js with hardened download**

Replace `npm/install.js` with a rewrite that adds:

**Signal handlers** (top of file):
```javascript
let tempDir = null;
const cleanup = () => { if (tempDir) try { fs.rmSync(tempDir, { recursive: true }); } catch {} };
process.on("SIGINT", () => { cleanup(); process.exit(130); });
process.on("SIGTERM", () => { cleanup(); process.exit(143); });
```

**Timeout support** in download:
```javascript
const timeout = parseInt(process.env.OPEN_CLI_DOWNLOAD_TIMEOUT || "30", 10) * 1000;
// Apply to each https.get() call via setTimeout + req.destroy()
```

**Retry with exponential backoff:**
```javascript
async function downloadWithRetry(url, maxRetries = 3) {
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try { return await download(url, timeout); }
    catch (err) {
      if (attempt === maxRetries) throw err;
      const delay = Math.pow(2, attempt - 1) * 1000;
      console.error(`open-cli: attempt ${attempt}/${maxRetries} failed: ${err.message}. Retrying in ${delay/1000}s...`);
      await new Promise(r => setTimeout(r, delay));
    }
  }
}
```

**Strict binary validation after extraction:**
```javascript
const expectedBinaries = ["ocli", "oclird"];
for (const name of expectedBinaries) {
  const bin = path.join(binDir, name + ext);
  if (!fs.existsSync(bin)) {
    console.error(`open-cli: binary "${name}" missing after extraction`);
    process.exit(1);
  }
}
```

**Post-download version check:**
```javascript
const { execFileSync } = require("child_process");
try {
  const out = execFileSync(path.join(binDir, "ocli" + ext), ["--version"], { timeout: 5000 }).toString().trim();
  console.log(`open-cli: ✓ installed ${out} (${os}-${arch})`);
} catch {
  console.error("open-cli: ⚠ binary downloaded but --version check failed");
}
```

**Progress messaging:**
```javascript
console.log(`open-cli: downloading v${version} for ${os}-${arch}...`);
// ... after success:
console.log(`open-cli: run \`ocli --help\` to get started`);
```

- [ ] **Step 3: Test locally**

```bash
cd npm && node install.js
# Verify: downloads binary, prints success message, validates --version
```

- [ ] **Step 4: Commit**

```bash
git add npm/install.js
git commit -m "feat(npm): rewrite install.js with retry, timeout, validation, and signal handlers"
```

---

### Task 2: Improve wrapper scripts

**Files:**
- Modify: `npm/bin/ocli.js` (currently 24 lines)
- Modify: `npm/bin/oclird.js` (currently 24 lines)

- [ ] **Step 1: Add pre-flight binary check and differentiated errors to ocli.js**

Add before the `try/execFileSync` block:
```javascript
const fs = require("fs");
const bin = path.join(__dirname, "ocli" + ext);
if (!fs.existsSync(bin)) {
  console.error(`open-cli: binary not found at ${bin}`);
  console.error(`open-cli: run "npm install -g @sbuglione/open-cli" to reinstall`);
  process.exit(1);
}
```

Update the catch block to differentiate EACCES:
```javascript
} catch (e) {
  if (e.status !== null) process.exit(e.status);
  if (e.code === "EACCES") {
    console.error(`open-cli: permission denied on ${bin}`);
    console.error(`open-cli: try: chmod +x ${bin}`);
  } else {
    console.error(`open-cli: failed to execute ${bin}: ${e.message}`);
  }
  process.exit(1);
}
```

- [ ] **Step 2: Apply same changes to oclird.js**

Mirror the ocli.js changes, replacing `"ocli"` with `"oclird"` in paths and messages.

- [ ] **Step 3: Commit**

```bash
git add npm/bin/ocli.js npm/bin/oclird.js
git commit -m "feat(npm): add pre-flight binary check and differentiated errors to wrapper scripts"
```

---

### Task 3: Update `npm/package.json`

**Files:**
- Modify: `npm/package.json` (currently 45 lines)

- [ ] **Step 1: Add `files` field and `engines.npm`**

Add `"files"` array after `"description"`:
```json
"files": ["bin/ocli.js", "bin/oclird.js", "install.js", "README.md"],
```

Update `"engines"` to include npm:
```json
"engines": { "node": ">=16", "npm": ">=5.2.0" },
```

- [ ] **Step 2: Commit**

```bash
git add npm/package.json
git commit -m "feat(npm): add explicit files field and engines.npm requirement"
```

---

### Task 4: Rewrite `npm/README.md`

**Files:**
- Modify: `npm/README.md` (currently 30 lines → ~80 lines)

- [ ] **Step 1: Rewrite with complete documentation**

Replace the entire file with comprehensive documentation following this structure:

1. **Header** — package name + one-line description
2. **What is open-cli** — 2-sentence conceptual explanation
3. **Platform Support** — table (macOS/Linux/Windows × x64/arm64)
4. **Installation** — `npm install -g @sbuglione/open-cli`
5. **What happens during install** — explains postinstall binary download from GitHub Releases
6. **Quick Start** — 3 commands: install → `ocli --demo` → `ocli init <your-api>`
7. **Troubleshooting** — table of common errors (ENOENT, 404, SSL, timeout) with solutions
8. **Configuration** — brief overview + link to https://open-cli.dev/
9. **License** — GPL-3.0

- [ ] **Step 2: Commit**

```bash
git add npm/README.md
git commit -m "docs(npm): comprehensive README with platform support, quickstart, and troubleshooting"
```

---

## Stream B: main.go Refactoring (Phases 1–3)

### Prerequisite: Verify baseline

- [ ] **Step 0: Run existing tests to establish baseline**

```bash
cd /home/olfa/oas-cli-go
make build && make test
```

All tests must pass before any refactoring begins. Record the test count.

---

### Task 5: Create `runtime/http.go` — extract HTTP helpers

**Files:**
- Create: `cmd/ocli/internal/runtime/http.go`
- Modify: `cmd/ocli/main.go` (remove extracted functions, add imports)

These are the lowest-dependency functions, making them the safest to extract first.

- [ ] **Step 1: Create the `runtime` package with HTTP helpers**

Create `cmd/ocli/internal/runtime/http.go` with package declaration and these functions extracted from `main.go`:

| Function | Source Line | New Name (exported) |
|----------|-------------|---------------------|
| `postJSON[T any]()` | 730 | `PostJSON[T any]()` |
| `getJSON[T any]()` | 758 | `GetJSON[T any]()` |

These are generic HTTP helpers with no dependencies on other main.go types. Extract them with their imports (`bytes`, `encoding/json`, `fmt`, `io`, `net/http`, `strings`).

Also move `runtimeHTTPError` (line 118) and its `Error()` method (line 123) since `postJSON`/`getJSON` return this error type.

- [ ] **Step 2: Update main.go to import and use runtime package**

Replace calls to `postJSON()`/`getJSON()` in main.go with `runtime.PostJSON()`/`runtime.GetJSON()`. Update the import block. Remove the extracted function bodies and the `runtimeHTTPError` type.

- [ ] **Step 3: Build and test**

```bash
make build && make test
```

All tests must pass. If tests reference `runtimeHTTPError` directly, update those references.

- [ ] **Step 4: Commit**

```bash
git add cmd/ocli/internal/runtime/http.go cmd/ocli/main.go
git commit -m "refactor: extract postJSON, getJSON, runtimeHTTPError to internal/runtime/http"
```

---

### Task 6: Extract `runtime/deployment.go` — mode resolution

**Files:**
- Create: `cmd/ocli/internal/runtime/deployment.go`
- Modify: `cmd/ocli/main.go`

- [ ] **Step 1: Extract deployment resolution functions**

Move to `cmd/ocli/internal/runtime/deployment.go`:

| Function | Source Line | New Name |
|----------|-------------|----------|
| `resolveRuntimeDeployment()` | 969 | `ResolveDeployment()` |
| `loadRuntimeConfig()` | 998 | `LoadConfig()` |
| `hasLocalMCPSource()` | 1012 | `HasLocalMCPSource()` |

These depend on `configpkg` (already an external package). Define a minimal `DeploymentOptions` struct to replace the `CommandOptions` parameter — only pass the fields these functions actually use (`ConfigPath`, `Mode`, `Embedded`).

- [ ] **Step 2: Update main.go callers**

Replace `resolveRuntimeDeployment(options)` → `runtime.ResolveDeployment(...)` etc. The `resolveCommandOptions()` function (line 878) is the primary caller.

- [ ] **Step 3: Build and test**

```bash
make build && make test
```

- [ ] **Step 4: Commit**

```bash
git add cmd/ocli/internal/runtime/deployment.go cmd/ocli/main.go
git commit -m "refactor: extract resolveRuntimeDeployment, loadRuntimeConfig to internal/runtime/deployment"
```

---

### Task 7: Extract `runtime/client.go` and `runtime/session.go`

**Files:**
- Create: `cmd/ocli/internal/runtime/client.go`
- Create: `cmd/ocli/internal/runtime/session.go`
- Modify: `cmd/ocli/main.go`

- [ ] **Step 1: Extract the `runtimeClient` interface and implementations**

Move to `cmd/ocli/internal/runtime/client.go`:

| Item | Source Line | Notes |
|------|-------------|-------|
| `runtimeClient` interface | 94 | Export as `Client` |
| `httpRuntimeClient` struct | 171 | Export as `HTTPClient` |
| All `httpRuntimeClient` methods | 546–645 | `FetchCatalog`, `Execute`, `RunWorkflow`, `RuntimeInfo`, `Heartbeat`, `Stop`, `SessionClose`, `do` |
| `embeddedRuntimeClient` struct | 178 | Export as `EmbeddedClient` |
| All `embeddedRuntimeClient` methods | 648–728 | Same method set |
| `newRuntimeClient()` | 1539 | Export as `NewClient()` |
| `fetchCatalogHTTP()` | 509 | Keep as internal helper if only used by HTTPClient |

Move to `cmd/ocli/internal/runtime/session.go`:

| Item | Source Line | Notes |
|------|-------------|-------|
| `runtimeSessionToken` struct | 106 | Export as `SessionToken` |
| `runtimeTokenSession` struct | 111 | Export as `TokenSession` |
| `newRuntimeTokenSession()` | 125 | Export as `NewTokenSession()` |
| `tokenForPreflight()` | 129 | Keep method |
| `handleAuthnFailed()` | 151 | Keep method |
| `isExpiring()` | 164 | Keep method |
| `tokenRefreshGrace` constant | 104 | Export as `TokenRefreshGrace` |
| `performLocalSessionHandshake()` | 1222 | Export as `PerformLocalHandshake()` |
| `validateRuntimeContract()` | 1255 | Internal helper |
| `lifecycleCapabilityEnabled()` | 1288 | Internal helper |
| `localRuntimeConfigFingerprint()` | 1309 | Export as `ConfigFingerprint()` |
| `stringSlice()` | 1271 | Internal helper |
| `shouldSendLocalHeartbeat()` | 1642 | Export as `ShouldSendHeartbeat()` |

Also move related types: `runtimeCatalogResponse` (line 56), `executeRequest` (line 76), `executeResponse` (line 87).

- [ ] **Step 2: Update main.go to import runtime package types**

Replace all references to the moved types and functions. `NewRootCommand()` will now call `runtime.NewClient()`, use `runtime.Client` interface, etc.

- [ ] **Step 3: Update main_test.go imports**

Tests that construct `httpRuntimeClient` or `embeddedRuntimeClient` directly need to reference `runtime.HTTPClient` / `runtime.EmbeddedClient`. Tests that use `runtimeSessionToken` → `runtime.SessionToken`, etc.

- [ ] **Step 4: Build and test**

```bash
make build && make test
```

- [ ] **Step 5: Commit**

```bash
git add cmd/ocli/internal/runtime/ cmd/ocli/main.go cmd/ocli/main_test.go
git commit -m "refactor: extract runtime client, session, and types to internal/runtime/"
```

---

### Task 8: Extract `runtime/managed.go` and `runtime/instance.go`

**Files:**
- Create: `cmd/ocli/internal/runtime/managed.go`
- Create: `cmd/ocli/internal/runtime/instance.go`
- Modify: `cmd/ocli/main.go`

- [ ] **Step 1: Extract managed runtime functions**

Move to `cmd/ocli/internal/runtime/managed.go`:

| Function | Source Line |
|----------|-------------|
| `startManagedRuntime()` | 1454 |
| `managedRuntimeArgs()` | 1489 |
| `resolveDaemonBinary()` | 1524 |
| `configureManagedRuntimeCommand()` | 1638 |

Move to `cmd/ocli/internal/runtime/instance.go`:

| Function | Source Line |
|----------|-------------|
| `resolveRuntimeURLFromInstance()` | 1556 |
| `runtimeInfoReachable()` | 1577 |
| `runtimeURLReachable()` | 1587 |
| `resolveInstancePaths()` | 1600 |
| `cacheRootForState()` | 1609 |
| `resolveLocalRuntimeInstanceID()` | 1170 |
| `resolveLocalSessionID()` | 1198 |
| `detectTerminalSessionIdentity()` | 1616 |
| `detectAgentSessionIdentity()` | 1629 |

- [ ] **Step 2: Update main.go and main_test.go**

Replace all calls. Update test imports.

- [ ] **Step 3: Build and test**

```bash
make build && make test
```

- [ ] **Step 4: Commit**

```bash
git add cmd/ocli/internal/runtime/ cmd/ocli/main.go cmd/ocli/main_test.go
git commit -m "refactor: extract managed runtime and instance resolution to internal/runtime/"
```

---

### Task 9: Extract `auth/` package

**Files:**
- Create: `cmd/ocli/internal/auth/token.go`
- Create: `cmd/ocli/internal/auth/browser.go`
- Modify: `cmd/ocli/main.go`

- [ ] **Step 1: Extract token resolution**

Move to `cmd/ocli/internal/auth/token.go`:

| Function | Source Line | New Name |
|----------|-------------|----------|
| `resolveRuntimeToken()` | 1024 | `ResolveToken()` |
| `resolveRuntimeOAuthClientToken()` | 1105 | `ResolveOAuthClientToken()` |
| `resolveRuntimeOAuthSecret()` | 1159 | `ResolveOAuthSecret()` |

Move to `cmd/ocli/internal/auth/browser.go`:

| Function | Source Line | New Name |
|----------|-------------|----------|
| `fetchRuntimeBrowserLoginMetadata()` | 1356 | `FetchBrowserLoginMetadata()` |
| `resolveRuntimeEndpointURL()` | 1384 | `ResolveEndpointURL()` |
| `validateRuntimeBrowserLoginMetadata()` | 1400 | `ValidateBrowserLoginMetadata()` |
| `acquireRuntimeBrowserLoginToken()` | 1413 | `AcquireBrowserLoginToken()` |
| `runtimeBrowserLoginMetadata` type | 61 | `BrowserLoginMetadata` |
| `runtimeBrowserLoginRequest` type | 68 | `BrowserLoginRequest` |
| `fetchRuntimeHandshake()` | 1352 | `FetchHandshake()` |

The `auth` package imports `runtime` for `GetJSON` and session types. This one-directional dependency is documented in the spec.

- [ ] **Step 2: Update main.go and main_test.go**

Replace all auth-related calls. Update test imports for auth types.

- [ ] **Step 3: Build and test**

```bash
make build && make test
```

- [ ] **Step 4: Commit**

```bash
git add cmd/ocli/internal/auth/ cmd/ocli/main.go cmd/ocli/main_test.go
git commit -m "refactor: extract auth token and browser login to internal/auth/"
```

---

### Task 10: Extract `config/resolve.go`

**Files:**
- Create: `cmd/ocli/internal/config/resolve.go`
- Modify: `cmd/ocli/main.go`

- [ ] **Step 1: Extract command options resolution**

Move to `cmd/ocli/internal/config/resolve.go`:

| Function | Source Line | New Name |
|----------|-------------|----------|
| `bootstrapFromArgs()` | 809 | `BootstrapFromArgs()` |
| `resolveCommandOptions()` | 878 | `ResolveCommandOptions()` |
| `envBool()` | 1654 | `EnvBool()` |
| `CommandOptions` struct | 35 | `Options` |

`ResolveCommandOptions()` is the orchestrator that calls into `runtime.ResolveDeployment()`, `auth.ResolveToken()`, `runtime.PerformLocalHandshake()`, etc. It now imports from the extracted subpackages.

- [ ] **Step 2: Update main.go**

`main()` now calls `config.BootstrapFromArgs()` → `config.ResolveCommandOptions()` → `commands.NewRootCommand()`. main.go becomes a thin entry point.

- [ ] **Step 3: Update main_test.go**

Tests that construct `CommandOptions` directly switch to `config.Options`. Tests that call `bootstrapFromArgs` switch to `config.BootstrapFromArgs`.

- [ ] **Step 4: Build and test**

```bash
make build && make test
```

- [ ] **Step 5: Commit**

```bash
git add cmd/ocli/internal/config/ cmd/ocli/main.go cmd/ocli/main_test.go
git commit -m "refactor: extract bootstrapFromArgs, resolveCommandOptions to internal/config/"
```

---

### Task 11: Extract `commands/` package — existing commands

**Files:**
- Create: `cmd/ocli/internal/commands/root.go`
- Create: `cmd/ocli/internal/commands/dynamic.go`
- Create: `cmd/ocli/internal/commands/catalog.go`
- Create: `cmd/ocli/internal/commands/util.go`
- Modify: `cmd/ocli/main.go`

- [ ] **Step 1: Extract utility functions to commands/util.go**

| Function | Source Line | New Name |
|----------|-------------|----------|
| `writeOutput()` | 781 | `WriteOutput()` |
| `findTool()` | 860 | `FindTool()` |
| `commandSummary()` | 489 | `CommandSummary()` |
| `loadBody()` | 496 | `LoadBody()` |
| `sortedServiceAliases()` | 869 | `SortedServiceAliases()` |

- [ ] **Step 2: Extract command constructors to commands/catalog.go**

| Function | Source Line | New Name |
|----------|-------------|----------|
| `newCatalogCommand()` | 290 | `NewCatalogCommand()` |
| `newToolCommand()` | 304 | `NewToolSchemaCommand()` |
| `newExplainCommand()` | 321 | `NewExplainCommand()` |
| `newWorkflowCommand()` | 344 | `NewWorkflowCommand()` |
| `newRuntimeCommand()` | 367 | `NewRuntimeCommand()` |

- [ ] **Step 3: Extract dynamic commands to commands/dynamic.go**

| Function | Source Line | New Name |
|----------|-------------|----------|
| `addDynamicToolCommands()` | 405 | `AddDynamicToolCommands()` |

- [ ] **Step 4: Create commands/root.go with NewRootCommand**

Move `NewRootCommand()` (line 216) to `commands/root.go`. It now imports from `runtime`, `auth`, `config` subpackages. This is the thin orchestrator that:
1. Calls `config.ResolveCommandOptions()`
2. Calls `runtime.NewClient()`
3. Fetches catalog
4. Adds all subcommands
5. Returns the cobra.Command

- [ ] **Step 5: Slim down main.go to ~200 lines**

`main.go` becomes:
```go
package main

import (
    "fmt"
    "os"
    "github.com/StevenBuglione/open-cli/cmd/ocli/internal/commands"
    "github.com/StevenBuglione/open-cli/cmd/ocli/internal/config"
    "github.com/StevenBuglione/open-cli/internal/version"
)

func main() {
    // Early --version check (existing logic, lines 195-204)
    for _, arg := range os.Args[1:] {
        if arg == "--version" || arg == "-v" {
            fmt.Println(version.String())
            os.Exit(0)
        }
        if arg == "--" { break }
    }

    options := config.BootstrapFromArgs(os.Args[1:])
    command, err := commands.NewRootCommand(options, os.Args[1:])
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    if err := command.Execute(); err != nil {
        os.Exit(1)
    }
}
```

Plus `proc_linux.go`/`proc_other.go` (unchanged), and the platform-specific `configureManagedRuntimePlatform()` function.

- [ ] **Step 6: Update main_test.go**

Update all imports. Tests that call `NewRootCommand` now call `commands.NewRootCommand`. Tests that use `CommandOptions` now use `config.Options`. Integration tests remain in `main_test.go` since they test the assembled system.

- [ ] **Step 7: Build and test**

```bash
make build && make test
```

- [ ] **Step 8: Commit**

```bash
git add cmd/ocli/internal/commands/ cmd/ocli/main.go cmd/ocli/main_test.go
git commit -m "refactor: extract commands to internal/commands/, slim main.go to ~200 lines"
```

---

## Stream C: CLI UX Features

> **Depends on:** Stream B tasks 5–11 must be complete before starting Stream C.

### Task 12: Create `demo/embed.go` — embedded demo spec

**Files:**
- Create: `cmd/ocli/internal/demo/embed.go`

- [ ] **Step 1: Create the embed package**

```go
package demo

import _ "embed"

//go:embed testapi.openapi.yaml
var Spec []byte
```

- [ ] **Step 2: Copy the test spec into the demo package**

```bash
cp product-tests/testdata/openapi/testapi.openapi.yaml cmd/ocli/internal/demo/testapi.openapi.yaml
```

The file must live in or under the package directory for `go:embed` to work.

- [ ] **Step 3: Verify it compiles**

```bash
make build
```

- [ ] **Step 4: Commit**

```bash
git add cmd/ocli/internal/demo/
git commit -m "feat: embed demo API spec for ocli --demo mode"
```

---

### Task 13: Graceful degradation — help without runtime

**Files:**
- Modify: `cmd/ocli/internal/commands/root.go`
- Modify: `cmd/ocli/main.go`

This is the critical UX fix: `ocli --help` and `ocli` with no args must work without config or runtime.

- [ ] **Step 1: Write failing test — help with no config**

Add to `cmd/ocli/main_test.go`:
```go
func TestRootCommandShowsHelpWithoutConfig(t *testing.T) {
    options := config.Options{
        Stdout: new(bytes.Buffer),
        Stderr: new(bytes.Buffer),
    }
    cmd, err := commands.NewRootCommand(options, []string{"--help"})
    if err != nil {
        t.Fatalf("NewRootCommand failed: %v", err)
    }
    // Should not error — help should work without config
    err = cmd.Execute()
    if err != nil {
        t.Fatalf("--help should work without config: %v", err)
    }
    output := options.Stdout.(*bytes.Buffer).String()
    if !strings.Contains(output, "init") {
        t.Error("help output should mention 'init' command")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -run TestRootCommandShowsHelpWithoutConfig -v ./cmd/ocli/
```

Expected: FAIL (currently crashes trying to connect to runtime).

- [ ] **Step 3: Implement graceful degradation in NewRootCommand**

Modify `commands/root.go` `NewRootCommand()`:

1. Detect if we're in a "no-config" state (no ConfigPath, not Embedded, no RuntimeURL)
2. If no-config: build a root command with only built-in subcommands (`init`, `completion`, `version`) and a welcome `RunE` that prints guidance
3. If config exists: proceed with full runtime bootstrap (existing behavior)

```go
func NewRootCommand(options config.Options, args []string) (*cobra.Command, error) {
    root := &cobra.Command{
        Use:   "ocli",
        Short: "Turn OpenAPI descriptions and MCP servers into a local, policy-aware command surface",
    }

    // Always add built-in commands (no runtime needed)
    root.AddCommand(NewInitCommand())
    root.AddCommand(NewCompletionCommand())

    // Check if we have config/runtime available
    if !hasRuntimeConfig(options) {
        // No config — add welcome message as default action
        root.RunE = func(cmd *cobra.Command, args []string) error {
            fmt.Fprintln(options.Stderr, "No configuration found.")
            fmt.Fprintln(options.Stderr, "")
            fmt.Fprintln(options.Stderr, "Get started:")
            fmt.Fprintln(options.Stderr, "  ocli init <url-or-file>   Set up a new project from an OpenAPI spec")
            fmt.Fprintln(options.Stderr, "  ocli --demo               Try a built-in demo API")
            fmt.Fprintln(options.Stderr, "  ocli --help               Show all commands")
            fmt.Fprintln(options.Stderr, "")
            fmt.Fprintln(options.Stderr, "Full docs: https://open-cli.dev/")
            return nil
        }
        return root, nil
    }

    // Existing flow: resolve options, create client, fetch catalog, add dynamic commands...
    // (moved from current NewRootCommand logic)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test -run TestRootCommandShowsHelpWithoutConfig -v ./cmd/ocli/
```

- [ ] **Step 5: Write test — no args, no config shows welcome**

```go
func TestRootCommandShowsWelcomeWithoutConfigOrArgs(t *testing.T) {
    // Similar to above but without --help flag
    // Verify welcome message includes "ocli init" and "ocli --demo"
}
```

- [ ] **Step 6: Run full test suite**

```bash
make test
```

All existing tests must still pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/ocli/internal/commands/root.go cmd/ocli/main.go cmd/ocli/main_test.go
git commit -m "feat: graceful degradation — help and welcome work without config or runtime"
```

---

### Task 14: Implement `ocli init` command

**Files:**
- Create: `cmd/ocli/internal/commands/init.go`
- Create: `cmd/ocli/internal/commands/init_test.go`

- [ ] **Step 1: Write failing test — init with local OpenAPI file**

```go
func TestInitCommandCreatesConfigFromLocalFile(t *testing.T) {
    // Create temp dir with a sample OpenAPI spec
    // Run init command pointing to the spec
    // Verify .cli.json was created with correct structure
    // Verify output includes "Created .cli.json"
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -run TestInitCommandCreatesConfigFromLocalFile -v ./cmd/ocli/internal/commands/
```

- [ ] **Step 3: Implement init command core**

Create `cmd/ocli/internal/commands/init.go`:

```go
package commands

// NewInitCommand creates the `ocli init` command.
func NewInitCommand() *cobra.Command {
    var forceFlag bool
    var typeFlag string

    cmd := &cobra.Command{
        Use:   "init [url-or-file]",
        Short: "Initialize a new .cli.json configuration",
        Long:  "Create a .cli.json configuration from an OpenAPI spec URL or file path.",
        Args:  cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            source := ""
            if len(args) > 0 {
                source = args[0]
            } else {
                // Interactive prompt
                fmt.Fprint(os.Stderr, "Enter the URL or file path of your API spec: ")
                reader := bufio.NewReader(os.Stdin)
                line, _ := reader.ReadString('\n')
                source = strings.TrimSpace(line)
            }
            return runInit(source, typeFlag, forceFlag)
        },
    }
    cmd.Flags().BoolVar(&forceFlag, "force", false, "Overwrite existing .cli.json")
    cmd.Flags().StringVar(&typeFlag, "type", "", "Source type: openapi or mcp")
    return cmd
}
```

Implement `runInit()`:
1. Check for existing `.cli.json` (error unless `--force`)
2. Detect source type (file extension, URL content-type, `--type` override)
3. For OpenAPI: read/fetch spec, validate `openapi` field exists, extract `info.title`
4. Generate `.cli.json` with minimal config
5. Validate by loading catalog in embedded mode
6. Print success message with tool count

- [ ] **Step 4: Run test to verify it passes**

```bash
go test -run TestInitCommandCreatesConfigFromLocalFile -v ./cmd/ocli/internal/commands/
```

- [ ] **Step 5: Write additional tests**

- `TestInitCommandRejectsNonOpenAPIYaml` — YAML without `openapi` field
- `TestInitCommandRejectsHTMLResponse` — URL returning HTML
- `TestInitCommandRefusesOverwriteWithoutForce` — existing .cli.json
- `TestInitCommandAcceptsForceFlag` — overwrites with --force
- `TestInitCommandDetectsSourceType` — various inputs auto-detected correctly

- [ ] **Step 6: Run full test suite**

```bash
make test
```

- [ ] **Step 7: Commit**

```bash
git add cmd/ocli/internal/commands/init.go cmd/ocli/internal/commands/init_test.go
git commit -m "feat: add ocli init command for config scaffolding from OpenAPI specs"
```

---

### Task 15: Implement `ocli --demo` mode

**Files:**
- Create: `cmd/ocli/internal/commands/demo.go`
- Modify: `cmd/ocli/internal/commands/root.go`

- [ ] **Step 1: Write failing test**

```go
func TestDemoModeFetchesCatalogFromEmbeddedSpec(t *testing.T) {
    // Set --demo flag
    // Verify catalog is fetched successfully
    // Verify output includes "Demo mode" header
    // Verify temp file is cleaned up
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Implement demo mode**

Create `cmd/ocli/internal/commands/demo.go`:

```go
package commands

import (
    "os"
    "path/filepath"
    "github.com/StevenBuglione/open-cli/cmd/ocli/internal/demo"
)

// SetupDemoMode writes the embedded spec to a temp file and returns
// options configured for embedded demo mode.
func SetupDemoMode(options config.Options) (config.Options, func(), error) {
    // Write embedded spec to temp file
    tmpDir, err := os.MkdirTemp("", "ocli-demo-*")
    if err != nil {
        return options, nil, fmt.Errorf("cannot create temp directory: %w", err)
    }
    cleanup := func() { os.RemoveAll(tmpDir) }

    specPath := filepath.Join(tmpDir, "demo.openapi.yaml")
    if err := os.WriteFile(specPath, demo.Spec, 0644); err != nil {
        cleanup()
        return options, nil, fmt.Errorf("cannot write demo spec: %w", err)
    }

    // Generate demo config
    configPath := filepath.Join(tmpDir, ".cli.json")
    demoConfig := fmt.Sprintf(`{
        "cli": "1.0.0",
        "mode": {"default": "discover"},
        "sources": {"demo": {"type": "openapi", "uri": "%s"}},
        "services": {"demo": {"source": "demo"}}
    }`, specPath)
    if err := os.WriteFile(configPath, []byte(demoConfig), 0644); err != nil {
        cleanup()
        return options, nil, fmt.Errorf("cannot write demo config: %w", err)
    }

    options.ConfigPath = configPath
    options.Embedded = true
    return options, cleanup, nil
}
```

Add `--demo` persistent flag to root command in `root.go`. When set, call `SetupDemoMode()` before runtime bootstrap. Print demo header to stderr.

Register signal handler for cleanup in the root command's `PersistentPreRunE`:
```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigCh
    cleanup()
    os.Exit(130)
}()
```
This ensures the demo temp file is cleaned up even on Ctrl+C (Go's default SIGINT handler does not run deferred functions).

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Run full test suite**

```bash
make test
```

- [ ] **Step 6: Commit**

```bash
git add cmd/ocli/internal/commands/demo.go cmd/ocli/internal/commands/root.go cmd/ocli/main_test.go
git commit -m "feat: add --demo mode with embedded API spec for zero-config exploration"
```

---

### Task 16: Improve error messages

**Files:**
- Modify: `cmd/ocli/internal/commands/root.go`
- Modify: `cmd/ocli/internal/config/resolve.go`

- [ ] **Step 1: Write failing test — runtime connection error includes suggestion**

```go
func TestRuntimeConnectionErrorIncludesSuggestion(t *testing.T) {
    // Set up options pointing to non-existent runtime
    // Verify error includes "Suggestion:" with recovery steps
}
```

- [ ] **Step 2: Create error wrapping helpers**

Add to `cmd/ocli/internal/commands/util.go`:

```go
// CLIError wraps errors with user-facing context.
type CLIError struct {
    Message    string // What happened
    Cause      error  // Technical details
    Suggestion string // What to try next
}

func (e *CLIError) Error() string {
    s := fmt.Sprintf("Error: %s\n", e.Message)
    if e.Cause != nil {
        s += fmt.Sprintf("Cause: %s\n", e.Cause)
    }
    if e.Suggestion != "" {
        s += fmt.Sprintf("Suggestion: %s\n", e.Suggestion)
    }
    return s
}
```

- [ ] **Step 3: Wrap existing errors in resolution pipeline**

Update `config/resolve.go` `ResolveCommandOptions()` to wrap errors:
- Runtime connection failure → suggest `--embedded` or starting oclird
- Config not found → suggest `ocli init` or `ocli --demo`
- Config parse error → suggest checking config format, link to docs

Update `commands/root.go` to format `CLIError` when printing errors.

- [ ] **Step 4: Run tests**

```bash
make test
```

- [ ] **Step 5: Commit**

```bash
git add cmd/ocli/internal/commands/ cmd/ocli/internal/config/
git commit -m "feat: wrap errors with actionable context (Error/Cause/Suggestion format)"
```

---

## Stream D: Documentation Updates

### Task 17: Fix `website/docs/getting-started/installation.md`

**Files:**
- Modify: `website/docs/getting-started/installation.md` (currently 68 lines)

- [ ] **Step 1: Rewrite installation.md**

Replace the content. New structure:

1. **npm (Recommended)** — `npm install -g @sbuglione/open-cli`, explains postinstall download
2. **Binary Download** — link to GitHub Releases, instructions for each OS
3. **From Source** — `go install` command, Go version requirement
4. **Verify Installation** — `ocli --version`
5. **Platform Support** — table: macOS/Linux/Windows × x64/arm64
6. **Troubleshooting** — common install issues

Remove the claim "This repository uses source-based installation. There are no packaged installers or pre-built release binaries."

- [ ] **Step 2: Commit**

```bash
git add website/docs/getting-started/installation.md
git commit -m "docs: fix installation.md — add npm as recommended method, remove source-only claim"
```

---

### Task 18: Update `website/docs/getting-started/quickstart.md`

**Files:**
- Modify: `website/docs/getting-started/quickstart.md` (currently 138 lines)

- [ ] **Step 1: Update quickstart for npm users**

Changes:
- Replace all `./bin/ocli` with `ocli` (6+ occurrences)
- Replace all `./bin/oclird` with `oclird`
- Add `ocli --demo` as a zero-config first step before creating a config
- Add `ocli init` as the method for creating `.cli.json` (instead of manual creation)
- Update flow: install → `ocli --demo` → `ocli init <url>` → catalog list → explain → execute
- Keep advanced daemon section unchanged

- [ ] **Step 2: Commit**

```bash
git add website/docs/getting-started/quickstart.md
git commit -m "docs: update quickstart for npm users — use ocli directly, add --demo and init"
```

---

### Task 19: Update root `README.md`

**Files:**
- Modify: `README.md` (currently 216 lines)

- [ ] **Step 1: Add demo and init to getting started**

In the Quick Start section, add before the manual `.cli.json` creation:

```markdown
### Try the demo (no setup needed)

    ocli --demo catalog list --format pretty

This uses a built-in sample API to show how open-cli works.

### Set up your own API

    ocli init https://petstore3.swagger.io/api/v3/openapi.json

This creates a `.cli.json` configuration from your OpenAPI spec.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add ocli --demo and ocli init to README quick start"
```

---

### Task 20: Add `spec/examples/` annotations

**Files:**
- Create: `spec/examples/README.md`
- Create: `spec/examples/minimal.cli.json`

- [ ] **Step 1: Create annotated examples README**

Create `spec/examples/README.md` explaining each example config file:
- `project.cli.json` — Full-featured config with overlays, skills, curation, agent profiles
- `minimal.cli.json` — Absolute bare minimum config (NEW)
- `invalid-openapi-oauth.cli.json` — Example of incorrect OAuth on OpenAPI (validation test)
- `invalid-mcp-uri.cli.json` — Example of incorrect URI on MCP (validation test)

- [ ] **Step 2: Create minimal example config**

Create `spec/examples/minimal.cli.json`:
```json
{
  "cli": "1.0.0",
  "mode": {
    "default": "discover"
  },
  "sources": {
    "myapi": {
      "type": "openapi",
      "uri": "./my-api.openapi.yaml"
    }
  },
  "services": {
    "myapi": {
      "source": "myapi"
    }
  }
}
```

- [ ] **Step 3: Commit**

```bash
git add spec/examples/README.md spec/examples/minimal.cli.json
git commit -m "docs: add annotated examples README and minimal.cli.json to spec/examples/"
```

---

## Final Verification

### Task 21: Full build, test, and integration check

- [ ] **Step 1: Run full test suite**

```bash
make build && make test
```

- [ ] **Step 2: Run product smoke tests (if available)**

```bash
make product-test-smoke
```

- [ ] **Step 3: Verify new UX behaviors manually**

```bash
# Help without config
ocli --help

# Welcome with no args
ocli

# Version
ocli --version

# Demo mode
ocli --demo catalog list --format pretty

# Init (with a public API spec)
cd /tmp && ocli init https://petstore3.swagger.io/api/v3/openapi.json
cat .cli.json
```

- [ ] **Step 4: Verify main.go line count**

```bash
wc -l cmd/ocli/main.go
# Target: under 250 lines
```

- [ ] **Step 5: Final commit if any cleanup needed**

```bash
git add -A && git commit -m "chore: final cleanup after UX overhaul"
```

---

## Task Dependency Summary

```
Stream A (independent):     Task 1 → Task 2 → Task 3 → Task 4
Stream B (foundation):      Task 5 → Task 6 → Task 7 → Task 8 → Task 9 → Task 10 → Task 11
Stream C (depends on B):    Task 12 → Task 13 → Task 14 → Task 15 → Task 16
Stream D (independent):     Task 17, Task 18, Task 19, Task 20 (all independent of each other)
Final:                      Task 21 (depends on all above)
```

**Parallelization opportunities:**
- Stream A tasks 1–4 run in parallel with Stream B tasks 5–11
- Stream D tasks 17–20 run in parallel with everything (update after C features land for accuracy)
- Within Stream B, tasks are strictly sequential (each builds on the previous extraction)
- Within Stream C, tasks are strictly sequential
