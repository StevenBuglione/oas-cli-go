# spec/examples/

Example configuration files for the Open CLI specification.

## Configuration examples

| File | Purpose |
|------|---------|
| `project.cli.json` | Full-featured config with overlays, skills, curation, and agent profiles. Use as a reference for all supported top-level keys. |
| `minimal.cli.json` | Absolute bare-minimum config — one source, one service, no optional fields. Start here when creating a new config from scratch. |
| `invalid-openapi-oauth.cli.json` | Incorrect OAuth configuration on an OpenAPI source. Used by validation tests to verify that the validator rejects `oauth` at source level (OAuth belongs in the OpenAPI document's security schemes, not in `.cli.json`). |
| `invalid-mcp-uri.cli.json` | Incorrect URI on an MCP source. The `uri` field contains an `https://` URL while the transport is `stdio`, which is invalid. Used by validation tests. |
| `invalid-openapi-transport.cli.json` | Incorrect transport on an OpenAPI source. OpenAPI sources do not support a `transport` block. Used by validation tests. |

## Other specification artifacts

| File | Purpose |
|------|---------|
| `ntc.json` | Normalized Tool Catalog (NTC) example showing the runtime output of discovery and normalization. |
| `skill-manifest.json` | Skill manifest example with tool guidance, usage hints, and example commands. |
| `compatibility-matrix.json` | Implementation compatibility matrix tracking feature support across implementations. |
