---
title: Enterprise Readiness
---

# Enterprise Readiness

This page is the shortest honest route through the parts of `oas-cli-go` that matter during an enterprise evaluation.

## What you can evaluate today

The repository already exposes concrete proof for:

- deployment choices from embedded mode to reusable local daemon to remote runtime
- runtime bearer auth enforced server-side through `runtime.server.auth`
- scoped catalog filtering and execution denial at the runtime boundary
- reproducible fleet validation for CI-safe lanes
- live proof tracking for browser federation and other external-runtime scenarios
- audit logging and runtime instance isolation

## Recommended evaluation order

### 1. Deployment and trust boundary

Read:

- [Deployment models](./deployment-models)
- [Runtime overview](./overview)
- [Security overview](../security/overview)

This establishes where the runtime runs, what the default localhost safety model is, and when you should enable runtime auth.

### 2. Runtime auth proof

Read:

- [Authentik reference proof](./authentik-reference)

That page is the worked example for:

- client-credentials runtime access
- browser-login runtime access
- a broker-neutral contract using Authentik as the reference broker
- Microsoft Entra as the documented upstream federation example

### 3. Reproducible evidence

Read:

- [Fleet validation](../development/fleet-validation)

The fleet matrix shows which product paths are exercised automatically, what artifacts survive each lane, and which enterprise-only slices are tracked as live proof instead of being overclaimed in CI.

### 4. Operations and auditability

Read:

- [Operations overview](../operations/overview)
- [Audit logging](../operations/audit-logging)
- [Tracing and instances](../operations/tracing-and-instances)

These pages explain what operators can inspect after execution and how separate instances keep runtime state isolated.

## Questions this repo can answer

By the time you finish the pages above, you should be able to answer:

- which deployment model fits a workstation, CI runner, or hosted runtime
- how remote callers authenticate to `oasclird`
- how runtime scopes limit what a caller can see and execute
- what proof is reproducible in CI versus what needs a live external environment
- what logs and artifacts remain after a fleet validation run

## What still remains a known gap

The project does **not** currently claim token revocation or introspection-backed runtime auth as a solved, reproducible proof path. Expiry, signature validation, issuer/audience checks, and scope enforcement are covered; revocation remains a tracked gap rather than a hidden assumption.
