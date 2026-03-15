---
title: Auth Resolution
---

# Auth Resolution

Auth resolution happens inside the runtime just before tool execution.

## Step 1: extract auth requirements from OpenAPI

The catalog builder inspects operation-level or document-level security requirements and records entries such as:

- scheme name (`bearerAuth`)
- type (`http`, `apiKey`, `oauth2`, `openIdConnect`)
- scheme (`bearer`, `basic`, ...)
- location (`header`, `query`, `cookie`)
- parameter name for API keys
- required scopes for OAuth-backed schemes
- OAuth flow metadata such as token and authorization endpoints

OpenAPI alternatives are preserved as alternatives instead of being flattened away. A security block such as:

```yaml
security:
  - oauth: [pets.read]
  - api_key: []
```

is treated as “`oauth` OR `api_key`”, while multiple schemes inside the same object remain an AND requirement.

## Step 2: match by security scheme name

At execution time, the runtime looks up each auth requirement by name in `config.secrets`.

Lookup order is:

1. `secrets["<service>.<scheme>"]`
2. `secrets["<scheme>"]`

Example:

```yaml
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
```

must line up with:

```json
{
  "secrets": {
    "bearerAuth": {
      "type": "env",
      "value": "HELPDESK_TOKEN"
    }
  }
}
```

## Step 3: resolve secret values

The runtime resolves the configured secret source only when a tool is actually executed.

That means:

- missing env vars are not caught during config load
- unreadable files are not caught during config load
- a broken exec secret is not caught during config load

## Step 4: choose an auth alternative

At execution time, the runtime evaluates preserved auth alternatives in two passes:

1. non-interactive first
2. interactive fallback second

That means a satisfiable static API key or client-credentials branch is preferred before the runtime attempts browser-based OAuth.

If an alternative requires browser interaction (`authorizationCode`) and no cached token is already available, the non-interactive pass marks that branch as interactive-required and continues looking for another satisfiable alternative.

## Supported applied auth schemes

The current executor automatically applies:

- HTTP bearer auth
- HTTP basic auth
- API keys in headers
- API keys in query parameters
- OAuth bearer tokens acquired through `oauth2` or `openIdConnect`

## Important limitations

### Cookie auth is not auto-applied

`apiKey` security schemes with `in: cookie` are normalized into catalog metadata, but the execution layer does not currently apply them automatically as auth.

### Unsupported OAuth flows fail explicitly

`authorizationCode` and `clientCredentials` are supported.

`implicit` and `password` are intentionally rejected with runtime errors.

### Secret failures are alternative-specific

If one auth branch cannot be resolved, the runtime does not silently mix partially resolved requirements into the outgoing request. It rejects that alternative and tries the next one.

If no alternative can be satisfied, execution fails explicitly.

`env` secrets still resolve to an empty string when the environment variable is unset, so an auth branch that depends on an unset env var may still produce an empty auth value.

## Recommendation

- keep security scheme names stable
- prefer one clear auth method per operation where possible
- test `tool schema` plus a real execution path together when introducing auth

For source-specific secret behavior, continue with [Secret sources](./secret-sources).
