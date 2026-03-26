# Disabled MCP Tool Fail-Closed Validation Design

## Problem

`sources.<name>.disabledTools` currently removes MCP tools before synthetic OpenAPI normalization, but downstream artifacts can still reference those removed tools indirectly:

- Arazzo workflow steps can point at the disabled operation.
- Overlay actions can target the disabled synthetic operation path or operation object.
- Policy entries can name or pattern-match the disabled tool ID.

Today these cases are either handled generically (`workflow` references fail as "no matching tool is available") or may silently no-op (`overlay` targets that no longer match after filtering). That is weaker than the MCP design contract, which requires catalog build to fail closed when disabled MCP tools are still referenced by attached artifacts.

## Goals

- Enforce the MCP design contract at catalog-build time.
- Keep one authoritative failure point: after MCP disabled-tool filtering and normalization context are both known.
- Report actionable errors that identify:
  - source ID
  - service ID
  - disabled MCP tool name
  - artifact type
  - artifact reference/path
  - the referencing operation ID, JSONPath target, or policy pattern
- Avoid false positives for broad policy patterns that still intentionally apply to surviving tools.

## Non-goals

- Generic dead-reference validation for non-MCP services.
- Turning every overlay no-op into an error.
- Changing overlay JSONPath semantics beyond disabled-tool enforcement.
- Reworking the policy engine itself.

## Selected approach

Add a **catalog-time disabled-tool validation pass** for MCP-backed services.

The pass runs with both of these views available:

1. the **pre-filter MCP operation universe** derived from the discovered MCP tool list
2. the **post-filter surviving tool set** after `disabledTools` removal and normal catalog normalization

This lets the builder distinguish:

- a reference that never matched anything
- a reference that still matches surviving tools
- a reference that used to match an MCP tool but now only points at disabled content

Only the third case is in scope for this design, and it must fail closed.

## Why catalog-time validation

### Rejected: config-load validation

This is too early. Workflow bindings, synthetic operation IDs/paths, and overlay targets depend on discovered MCP tools and the generated OpenAPI document.

### Rejected: runtime-only denial

This violates the design requirement that catalog build fail closed when attached artifacts still depend on disabled tools.

## Data model additions

The catalog builder needs a transient source-scoped validation model for MCP services:

- `allToolsByName`: every discovered MCP tool descriptor before filtering
- `disabledToolNames`: the configured `disabledTools` set
- `survivingToolNames`: discovered tool names after filtering
- `allOperationRefs`: synthetic operation metadata for **all** discovered MCP tools before filtering
- `survivingOperationRefs`: synthetic operation metadata after filtering
- `survivingToolIDs`: final normalized tool IDs after `buildTools(...)`

Each synthetic operation ref should retain enough metadata to connect references back to the original MCP tool:

- original MCP tool name
- synthetic operation ID
- synthetic HTTP method/path
- service ID
- source ID

This metadata is validation-only; it does not need to become a long-lived public catalog field.

## Validation stages

### 1. MCP discovery and synthetic OpenAPI generation

For MCP sources, build two synthetic views:

- **unfiltered synthetic document** from the full discovered tool list
- **filtered synthetic document** after applying `disabledTools`

Only the filtered document continues into normal build/normalization output.

The unfiltered document exists only so validation can determine whether later artifact references were removed by `disabledTools`.

### 2. Overlay validation

When applying service overlays to an MCP-backed synthetic document:

- evaluate each overlay action target against the unfiltered synthetic document
- evaluate the same target against the filtered synthetic document

Fail closed when:

- the target matches one or more nodes in the unfiltered document
- every matched node belongs only to disabled MCP tool operations
- the target matches no surviving nodes in the filtered document

Do **not** fail when:

- the target matched nothing in either document
- the target still matches at least one surviving node
- the target matched a mixture of disabled and surviving nodes

Error shape should identify:

- source ID
- service ID
- overlay reference path
- overlay action index
- JSONPath target
- disabled MCP tool name(s)

### 3. Workflow validation

Workflow validation already resolves steps through operation bindings. Extend it for MCP-backed services:

- if a workflow step does not resolve against surviving bindings
- and the referenced `operationId` or `operationPath` resolves in the unfiltered MCP binding set
- and every such match belongs only to disabled MCP tools

then fail with a disabled-tool-specific catalog-build error instead of the current generic "no matching tool is available" message.

If the reference matches nothing in either binding set, preserve the existing generic missing-reference error.

Error shape should identify:

- source ID
- service ID
- workflow reference path
- workflow ID
- step ID
- reference type (`operationId` or `operationPath`)
- referenced value
- disabled MCP tool name

### 4. Policy validation

Policy validation must be source-aware and pattern-aware.

Artifact classes in scope:

- `policy.approvalRequired`
- `policy.deny` / managed deny
- curated tool-set `allow`
- curated tool-set `deny`

For each pattern, evaluate it against:

- the pre-filter synthetic/final candidate tool universe for the MCP-backed service
- the post-filter surviving tool universe

Fail closed when either of these is true:

1. the pattern is an exact tool ID (no glob syntax) that refers to a disabled MCP tool
2. the pattern matches one or more MCP tools in the pre-filter universe, matches **zero** surviving tools, and all matched tools were removed by `disabledTools`

Do **not** fail when:

- the pattern still matches at least one surviving tool
- the pattern matches tools outside the disabled subset as well as disabled ones
- the pattern matches nothing in either universe

That rule keeps broad patterns like `filesystem:*` valid when the operator intentionally wants the rule to continue applying to the surviving portion of the service.

Error shape should identify:

- source ID
- service ID
- artifact type (`approvalRequired`, `managedDeny`, `toolSet.allow`, `toolSet.deny`)
- artifact owner (`policy` or tool-set name)
- offending pattern
- disabled MCP tool name(s) that were the only match set

## Error contract

Introduce a source-scoped catalog build error for this class of failure.

Representative message style:

```text
source "filesystemSource" service "filesystem": overlay "./overlays/files.overlay.yaml" action 2 target "$.paths['/_mcp/filesystem/delete-file'].post" references disabled MCP tool "delete_file"
```

Other examples:

```text
source "filesystemSource" service "filesystem": workflow "./workflows/files.arazzo.yaml" workflow "cleanup" step "delete" references operationId "delete_file", which maps only to disabled MCP tool "delete_file"
```

```text
source "filesystemSource" service "filesystem": policy.approvalRequired pattern "filesystem:delete_file" references disabled MCP tool "delete_file"
```

The implementation may use typed errors internally, but the surfaced message must remain source- and artifact-specific.

## Implementation outline

1. Extend MCP synthetic generation helpers so they can produce both filtered and unfiltered operation metadata.
2. Add transient validation helpers in `pkg/catalog/build.go` for:
   - overlay disabled-target detection
   - workflow disabled-reference detection
   - policy disabled-pattern detection
3. Thread artifact reference paths through workflow and overlay loading so error messages can name the source document.
4. Run the validation pass before final catalog append for MCP-backed services.
5. Keep non-MCP catalog behavior unchanged.

## Testing expectations

Implementation is complete when tests cover:

- workflow `operationId` reference to a disabled MCP tool -> fail closed with artifact-specific error
- workflow `operationPath` reference to a disabled MCP tool -> fail closed with artifact-specific error
- overlay target that only matched disabled MCP operations -> fail closed
- overlay target that never matched anything -> unchanged behavior
- policy exact tool ID reference to a disabled MCP tool -> fail closed
- policy glob pattern that matches only disabled MCP tools -> fail closed
- policy glob pattern that matches both disabled and surviving MCP tools -> allowed
- non-MCP services remain unchanged

## Documentation impact

Update user-facing docs after implementation:

- `website/docs/configuration/overview.md`
- `website/docs/discovery-catalog/service-discovery-and-overlays.md`
- `website/docs/workflows-guidance/overview.md`
- `website/docs/security/policy-and-approval.md`

The docs should explicitly say that `disabledTools` is not only a hiding mechanism; it is a fail-closed contract, and referenced disabled MCP tools make catalog build fail.
