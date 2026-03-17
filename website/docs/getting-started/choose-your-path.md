---
title: Choose Your Path
---

# Choose Your Path

Use this page when you know **why** you are here, but not yet which runtime path or proof path to follow.

## 1. I want a first successful run quickly

Start here if you want the smallest possible setup:

- [Installation](./installation)
- [Quickstart](./quickstart)
- [CLI overview](../cli/overview)

This path keeps the runtime embedded or local and is the fastest way to see generated commands and execute a tool.

## 2. I want a reusable local daemon

Choose this when you expect repeated CLI use, a warmed cache, or multiple commands against the same config:

- [Runtime overview](../runtime/overview)
- [Deployment models](../runtime/deployment-models)
- [Operations overview](../operations/overview)

This is the right path for developers and operators who want `oasclird` running as a local control plane.

## 3. I want remote runtime auth and scoped access

Choose this when the runtime is hosted separately and access to it must be authenticated:

- [Runtime overview](../runtime/overview)
- [Security overview](../security/overview)
- [Authentik reference proof](../runtime/authentik-reference)

This path shows how runtime bearer auth, catalog filtering, and browser-login metadata work together.

## 4. I want MCP integrations

Choose this when you care about MCP stdio or streamable HTTP servers:

- [Discovery & Catalog overview](../discovery-catalog/overview)
- [Deployment models](../runtime/deployment-models)
- [Fleet validation](../development/fleet-validation)

The fleet page is especially useful here because it shows which MCP paths are reproducible in CI and which need live proof.

## 5. I am evaluating enterprise readiness

Choose this when you need a review path you can hand to operators, security reviewers, or buyers:

- [Enterprise readiness](../runtime/enterprise-readiness)
- [Authentik reference proof](../runtime/authentik-reference)
- [Fleet validation](../development/fleet-validation)
- [Security overview](../security/overview)

This path is for people asking, “How would we deploy this safely, prove it works, and audit it?”
