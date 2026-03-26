# Local Authentik Reference Setup

This workflow is the repo's local proof for the hosted `open-cli-toolbox` runtime contract. It reuses the validated Authentik stack under `product-tests/authentik/` to prove that local `ocli` can talk to a separately hosted runtime with runtime-enforced auth, scope filtering, and audit behavior.

Unlike the previous repo layout, this flow keeps helper scripts under `examples/local-authentik/` and writes all generated local artifacts under `.open-cli-local/authentik/` so the repository root stays clean.

## What gets generated

Running `./examples/local-authentik/setup.sh` renders:

- `.open-cli-local/authentik/runtime.cli.json` for the Docker-hosted `open-cli-toolbox` runtime
- `.open-cli-local/authentik/client.cli.json` for workload-style `oauthClient` auth from local `ocli`
- `.open-cli-local/authentik/browser.cli.json` for operator `browserLogin` auth from local `ocli`
- `.open-cli-local/authentik/client.env` for the confidential client credentials used by the workload flow
- `.open-cli-local/authentik/docker.env` for Docker Compose overrides

## Quick start

From the repo root:

```bash
./examples/local-authentik/setup.sh
source ./.open-cli-local/authentik/client.env
source ./.open-cli-local/authentik/docker.env
docker compose up -d open-cli-toolbox
```

In a second shell:

```bash
source ./.open-cli-local/authentik/client.env
go run ./cmd/ocli --config ./.open-cli-local/authentik/client.cli.json catalog list --format pretty
```

## Defaults

- Authentik UI/API: `http://127.0.0.1:9100`
- Runtime URL in generated config: `http://127.0.0.1:8765`
- Runtime audience: `open-cli-toolbox`
- Generated runtime scope: `bundle:testapi`
- OpenAPI source: `./product-tests/testdata/openapi/testapi.openapi.yaml`

## Useful overrides

You can customize the generated local config with environment variables before running the setup script:

```bash
OCLI_RUNTIME_SERVICE_ID=petstore \
OCLI_OPENAPI_URI=https://petstore3.swagger.io/api/v3/openapi.json \
./examples/local-authentik/setup.sh
```

Other supported overrides include:

- `OCLI_LOCAL_AUTH_DIR`
- `AUTHENTIK_BASE_URL`
- `OCLI_RUNTIME_URL`
- `OCLI_RUNTIME_AUDIENCE`
- `OCLI_RUNTIME_CONFIG_PATH`
- `OCLI_RUNTIME_AUTHENTIK_BASE_URL`
- `OCLI_RUNTIME_EXTRA_SERVICE_IDS`
- `OCLI_RUNTIME_EXTRA_SCOPES`
- `OCLI_AUTHENTIK_CLIENT_SLUG`
- `OCLI_AUTHENTIK_BROWSER_CLIENT_SLUG`
- `OCLI_LOCAL_CONFIG_PATH`
- `OCLI_BROWSER_CONFIG_PATH`
- `OCLI_LOCAL_ENV_PATH`
- `OCLI_DOCKER_ENV_PATH`
- `OCLI_BROWSER_CALLBACK_PORT`

## Cleanup

Stop the Authentik reference stack:

```bash
cd product-tests && make authentik-down
```

Stop the test API stack if you started it:

```bash
cd product-tests && make services-down
```
