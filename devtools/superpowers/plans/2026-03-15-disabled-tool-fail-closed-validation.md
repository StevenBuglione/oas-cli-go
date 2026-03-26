# Disabled MCP Tool Fail-Closed Validation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make catalog build fail closed when an MCP `disabledTools` entry removes a tool that is still referenced by workflows, overlays, or policy patterns.

**Architecture:** Extend MCP synthetic OpenAPI generation so catalog build can compare the full pre-filter MCP operation universe with the post-filter surviving tool set. Keep the actual disabled-reference logic in a focused catalog validation helper so MCP transport code stays separate from artifact-validation policy, then surface source- and artifact-specific build errors.

**Tech Stack:** Go, `kin-openapi`, existing `pkg/catalog`, existing `pkg/mcp/openapi`, existing website docs, Go test, `make verify`, Docusaurus

---

## File structure

**Create**

- `pkg/mcp/openapi/adapter_test.go` — focused tests for pre-filter vs post-filter MCP synthetic operation metadata.
- `pkg/catalog/mcp_disabled_validation.go` — transient MCP-only validation helpers for workflows, overlays, and policy references.

**Modify**

- `pkg/mcp/openapi/adapter.go` — return both filtered synthetic document output and transient operation metadata for disabled-tool validation.
- `pkg/catalog/build.go` — wire MCP build flow to use the new adapter result, run the disabled-reference validation pass, and preserve current non-MCP behavior.
- `pkg/catalog/catalog_test.go` — add catalog-level regression tests for workflow, overlay, and policy disabled-tool cases.
- `website/docs/configuration/overview.md` — document that `disabledTools` is fail-closed, not only cosmetic.
- `website/docs/discovery-catalog/service-discovery-and-overlays.md` — document overlay failures when targets only exist on disabled MCP operations.
- `website/docs/workflows-guidance/overview.md` — document workflow validation failure for disabled MCP tools.
- `website/docs/security/policy-and-approval.md` — document policy pattern validation for disabled MCP tools.

**Keep unchanged unless forced**

- `pkg/overlay/*` — do not change generic overlay JSONPath semantics unless a test proves it is required.
- `pkg/policy/policy.go` — prefer catalog-time validation helpers over runtime policy-engine changes.

## Chunk 1: MCP disabled-tool fail-closed validation

### Task 1: Add MCP synthetic metadata plumbing

**Files:**
- Create: `pkg/mcp/openapi/adapter_test.go`
- Modify: `pkg/mcp/openapi/adapter.go`
- Test: `pkg/mcp/openapi/adapter_test.go`

- [ ] **Step 1: Write the failing adapter metadata test**

Add `TestBuildDocumentResultPreservesAllAndFilteredMCPToolRefs` in `pkg/mcp/openapi/adapter_test.go` that:

```go
func TestBuildDocumentResultPreservesAllAndFilteredMCPToolRefs(t *testing.T) {
    result, err := openapi.BuildDocumentResult(
        "filesystem",
        "filesystemSource",
        "streamable-http",
        []mcpclient.ToolDescriptor{
            {Name: "list_files"},
            {Name: "delete_file"},
        },
        []string{"delete_file"},
    )
    if err != nil {
        t.Fatalf("BuildDocumentResult returned error: %v", err)
    }
    if len(result.AllOperations) != 2 {
        t.Fatalf("expected 2 total MCP operation refs, got %d", len(result.AllOperations))
    }
    if len(result.FilteredOperations) != 1 {
        t.Fatalf("expected 1 surviving MCP operation ref, got %d", len(result.FilteredOperations))
    }
    if result.FilteredOperations[0].ToolName != "list_files" {
        t.Fatalf("expected surviving tool list_files, got %#v", result.FilteredOperations[0])
    }
}
```

- [ ] **Step 2: Run the adapter test to verify it fails**

Run: `go test ./pkg/mcp/openapi -run TestBuildDocumentResultPreservesAllAndFilteredMCPToolRefs -count=1`

Expected: FAIL because `BuildDocumentResult` and/or the metadata fields do not exist yet.

- [ ] **Step 3: Implement minimal adapter result plumbing**

In `pkg/mcp/openapi/adapter.go`, introduce a result type such as:

```go
type BuildResult struct {
    Document           *openapi3.T
    AllOperations      []OperationRef
    FilteredOperations []OperationRef
}

type OperationRef struct {
    ToolName    string
    OperationID string
    Method      string
    Path        string
    SourceID    string
    ServiceID   string
}
```

Then:

- build the synthetic operation refs for the full discovered MCP tool list
- apply `disabledTools`
- build the filtered document from the surviving subset
- keep the existing `BuildDocument(...)` API as a thin wrapper if that avoids broader churn

- [ ] **Step 4: Run the adapter test to verify it passes**

Run: `go test ./pkg/mcp/openapi -run TestBuildDocumentResultPreservesAllAndFilteredMCPToolRefs -count=1`

Expected: PASS

- [ ] **Step 5: Commit the metadata plumbing**

```bash
git add pkg/mcp/openapi/adapter.go pkg/mcp/openapi/adapter_test.go
git commit -m "feat: add mcp synthetic operation metadata"
```

### Task 2: Fail closed for workflow and overlay references

**Files:**
- Create: `pkg/catalog/mcp_disabled_validation.go`
- Modify: `pkg/catalog/build.go`
- Modify: `pkg/catalog/catalog_test.go`
- Test: `pkg/catalog/catalog_test.go`

- [ ] **Step 1: Write the failing workflow and overlay tests**

Add these tests in `pkg/catalog/catalog_test.go`:

- `TestBuildRejectsWorkflowReferencingDisabledMCPToolOperationID`
- `TestBuildRejectsWorkflowReferencingDisabledMCPToolOperationPath`
- `TestBuildRejectsOverlayTargetingDisabledMCPTool`
- `TestBuildAllowsOverlayTargetThatStillMatchesSurvivingMCPTool`

Use an MCP-backed config fixture pattern like:

```go
cfg := config.Config{
    CLI: "1.0.0",
    Mode: config.ModeConfig{Default: "discover"},
    Sources: map[string]config.Source{
        "filesystemSource": {
            Type: "mcp",
            Enabled: true,
            DisabledTools: []string{"delete_file"},
            Transport: &config.MCPTransport{
                Type: "stdio",
                Command: "ignored-in-test",
            },
        },
    },
    Services: map[string]config.Service{
        "filesystem": {
            Source: "filesystemSource",
            Overlays: []string{"./overlays/files.overlay.yaml"},
            Workflows: []string{"./workflows/files.arazzo.yaml"},
        },
    },
}
```

Mock the MCP client seam the same way existing MCP catalog tests do, then assert errors include:

- source ID
- workflow/overlay reference path
- operation ID or JSONPath target
- disabled MCP tool name

- [ ] **Step 2: Run the workflow/overlay tests to verify they fail**

Run:

```bash
go test ./pkg/catalog -run 'TestBuildRejects(WorkflowReferencingDisabledMCPToolOperationID|WorkflowReferencingDisabledMCPToolOperationPath|OverlayTargetingDisabledMCPTool)|TestBuildAllowsOverlayTargetThatStillMatchesSurvivingMCPTool' -count=1
```

Expected: FAIL because catalog build does not yet distinguish disabled MCP references from generic misses.

- [ ] **Step 3: Implement workflow/overlay disabled-reference validation**

In `pkg/catalog/mcp_disabled_validation.go`:

- add helpers that compare:
  - pre-filter MCP operation refs
  - filtered MCP operation refs
  - final surviving tool bindings
- detect when a workflow reference resolves only in the disabled universe
- detect when an overlay JSONPath target matched only disabled MCP operations in the unfiltered synthetic document and no surviving operations in the filtered document
- return source-scoped, artifact-specific errors

In `pkg/catalog/build.go`:

- switch MCP build flow to the new `BuildDocumentResult(...)`
- run overlay validation before final tool append
- run workflow validation with access to both surviving bindings and pre-filter MCP bindings
- keep non-MCP service flow unchanged

- [ ] **Step 4: Run the workflow/overlay tests to verify they pass**

Run:

```bash
go test ./pkg/catalog -run 'TestBuildRejects(WorkflowReferencingDisabledMCPToolOperationID|WorkflowReferencingDisabledMCPToolOperationPath|OverlayTargetingDisabledMCPTool)|TestBuildAllowsOverlayTargetThatStillMatchesSurvivingMCPTool' -count=1
```

Expected: PASS

- [ ] **Step 5: Commit the workflow/overlay validation slice**

```bash
git add pkg/catalog/build.go pkg/catalog/mcp_disabled_validation.go pkg/catalog/catalog_test.go
git commit -m "feat: fail closed on disabled mcp workflow and overlay refs"
```

### Task 3: Fail closed for policy references

**Files:**
- Modify: `pkg/catalog/mcp_disabled_validation.go`
- Modify: `pkg/catalog/catalog_test.go`
- Test: `pkg/catalog/catalog_test.go`

- [ ] **Step 1: Write the failing policy tests**

Add these tests in `pkg/catalog/catalog_test.go`:

- `TestBuildRejectsApprovalRequiredPatternReferencingOnlyDisabledMCPTool`
- `TestBuildRejectsManagedDenyPatternReferencingOnlyDisabledMCPTool`
- `TestBuildRejectsCuratedAllowPatternReferencingOnlyDisabledMCPTool`
- `TestBuildAllowsPolicyPatternMatchingDisabledAndSurvivingMCPTools`

Use exact-ID and glob-pattern cases, for example:

```go
Policy: config.PolicyConfig{
    ApprovalRequired: []string{"filesystem:delete_file"},
    ManagedDeny:      []string{"filesystem:delete_*"},
},
Curation: config.CurationConfig{
    ToolSets: map[string]config.ToolSet{
        "filesystem-set": {
            Allow: []string{"filesystem:delete_*"},
            Deny:  []string{"filesystem:*"},
        },
    },
},
```

Assertions:

- exact or glob patterns that match only disabled MCP tools -> fail
- patterns that still match at least one surviving MCP tool -> allowed

- [ ] **Step 2: Run the policy tests to verify they fail**

Run:

```bash
go test ./pkg/catalog -run 'TestBuildRejects(ApprovalRequiredPatternReferencingOnlyDisabledMCPTool|ManagedDenyPatternReferencingOnlyDisabledMCPTool|CuratedAllowPatternReferencingOnlyDisabledMCPTool)|TestBuildAllowsPolicyPatternMatchingDisabledAndSurvivingMCPTools' -count=1
```

Expected: FAIL because catalog build does not yet validate policy references against disabled MCP tools.

- [ ] **Step 3: Implement policy pattern validation**

Extend `pkg/catalog/mcp_disabled_validation.go` to:

- expand each relevant policy pattern against:
  - the pre-filter MCP candidate tool ID universe
  - the post-filter surviving MCP tool ID universe
- fail when:
  - an exact tool ID points at a disabled MCP tool, or
  - a pattern matches one or more pre-filter MCP tools and zero surviving tools, with all matches removed by `disabledTools`
- allow broad patterns that still match surviving tools

Validate all of:

- `cfg.Policy.ApprovalRequired`
- `cfg.Policy.ManagedDeny`
- every `cfg.Curation.ToolSets[*].Allow`
- every `cfg.Curation.ToolSets[*].Deny`

- [ ] **Step 4: Run the policy tests to verify they pass**

Run:

```bash
go test ./pkg/catalog -run 'TestBuildRejects(ApprovalRequiredPatternReferencingOnlyDisabledMCPTool|ManagedDenyPatternReferencingOnlyDisabledMCPTool|CuratedAllowPatternReferencingOnlyDisabledMCPTool)|TestBuildAllowsPolicyPatternMatchingDisabledAndSurvivingMCPTools' -count=1
```

Expected: PASS

- [ ] **Step 5: Commit the policy validation slice**

```bash
git add pkg/catalog/mcp_disabled_validation.go pkg/catalog/catalog_test.go
git commit -m "feat: validate policy refs against disabled mcp tools"
```

### Task 4: Update docs and run full verification

**Files:**
- Modify: `website/docs/configuration/overview.md`
- Modify: `website/docs/discovery-catalog/service-discovery-and-overlays.md`
- Modify: `website/docs/workflows-guidance/overview.md`
- Modify: `website/docs/security/policy-and-approval.md`
- Test: `pkg/mcp/openapi/adapter_test.go`
- Test: `pkg/catalog/catalog_test.go`

- [ ] **Step 1: Update the docs**

Document these rules:

- `disabledTools` is fail-closed for MCP-backed artifacts
- workflows referencing disabled MCP tools fail catalog build
- overlay targets that only matched disabled MCP synthetic operations fail catalog build
- policy patterns that only refer to disabled MCP tools fail catalog build

- [ ] **Step 2: Run focused verification**

Run:

```bash
gofmt -w pkg/mcp/openapi/adapter.go pkg/mcp/openapi/adapter_test.go pkg/catalog/build.go pkg/catalog/mcp_disabled_validation.go pkg/catalog/catalog_test.go
go test ./pkg/mcp/openapi ./pkg/catalog -count=1
```

Expected: PASS

- [ ] **Step 3: Run full repository verification**

Run:

```bash
go test ./...
make verify
cd website && npm run build
```

Expected:

- `go test ./...` PASS
- `make verify` PASS
- website build completes successfully

- [ ] **Step 4: Commit docs and verification-safe cleanup**

```bash
git add pkg/mcp/openapi/adapter.go pkg/mcp/openapi/adapter_test.go pkg/catalog/build.go pkg/catalog/mcp_disabled_validation.go pkg/catalog/catalog_test.go \
  website/docs/configuration/overview.md website/docs/discovery-catalog/service-discovery-and-overlays.md \
  website/docs/workflows-guidance/overview.md website/docs/security/policy-and-approval.md
git commit -m "feat: fail closed on disabled mcp tool references"
```

- [ ] **Step 5: Request implementation review before merge handoff**

Use `@superpowers/requesting-code-review` after the execution worker finishes this plan and before any merge/push step.
