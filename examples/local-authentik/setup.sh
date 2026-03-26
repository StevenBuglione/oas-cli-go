#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yaml"
BOOTSTRAP_SCRIPT="$SCRIPT_DIR/bootstrap.py"

LOCAL_AUTH_DIR="${OPEN_CLI_LOCAL_AUTH_DIR:-$REPO_ROOT/.open-cli-local/authentik}"
RUNTIME_CONFIG_PATH="${OPEN_CLI_RUNTIME_CONFIG_PATH:-${OPEN_CLI_DAEMON_CONFIG_PATH:-$LOCAL_AUTH_DIR/runtime.cli.json}}"
CONFIG_PATH="${OPEN_CLI_LOCAL_CONFIG_PATH:-$LOCAL_AUTH_DIR/client.cli.json}"
BROWSER_CONFIG_PATH="${OPEN_CLI_BROWSER_CONFIG_PATH:-$LOCAL_AUTH_DIR/browser.cli.json}"
ENV_PATH="${OPEN_CLI_LOCAL_ENV_PATH:-$LOCAL_AUTH_DIR/client.env}"
DOCKER_ENV_PATH="${OPEN_CLI_DOCKER_ENV_PATH:-$LOCAL_AUTH_DIR/compose.env}"
AUTHENTIK_BASE_URL="${AUTHENTIK_BASE_URL:-http://127.0.0.1:9100}"
DEFAULT_RUNTIME_AUTHENTIK_BASE_URL="${OPEN_CLI_RUNTIME_AUTHENTIK_BASE_URL_DEFAULT:-http://server:9000}"
RUNTIME_AUTHENTIK_BASE_URL="${OPEN_CLI_RUNTIME_AUTHENTIK_BASE_URL:-${OPEN_CLI_DAEMON_AUTHENTIK_BASE_URL:-$DEFAULT_RUNTIME_AUTHENTIK_BASE_URL}}"
RUNTIME_URL="${OPEN_CLI_RUNTIME_URL:-http://127.0.0.1:8765}"
RUNTIME_AUDIENCE="${OPEN_CLI_RUNTIME_AUDIENCE:-open-cli-toolbox}"
RUNTIME_SERVICE_ID="${OPEN_CLI_RUNTIME_SERVICE_ID:-testapi}"
RUNTIME_EXTRA_SERVICE_IDS="${OPEN_CLI_RUNTIME_EXTRA_SERVICE_IDS:-}"
RUNTIME_EXTRA_SCOPES="${OPEN_CLI_RUNTIME_EXTRA_SCOPES:-}"
OPENAPI_URI="${OPEN_CLI_OPENAPI_URI:-./product-tests/testdata/openapi/testapi.openapi.yaml}"
RUNTIME_OPENAPI_SERVER_URL="${OPEN_CLI_RUNTIME_OPENAPI_SERVER_URL:-http://testapi:8080}"
AUTHENTIK_CLIENT_SLUG="${OPEN_CLI_AUTHENTIK_CLIENT_SLUG:-open-cli-runtime-local}"
AUTHENTIK_PROVIDER_NAME="${OPEN_CLI_AUTHENTIK_PROVIDER_NAME:-open-cli Runtime Local Provider}"
AUTHENTIK_APPLICATION_NAME="${OPEN_CLI_AUTHENTIK_APPLICATION_NAME:-open-cli Runtime Local}"
AUTHENTIK_REDIRECT_URI="${OPEN_CLI_AUTHENTIK_REDIRECT_URI:-http://127.0.0.1:8787/callback}"
BROWSER_CALLBACK_PORT="${OPEN_CLI_BROWSER_CALLBACK_PORT:-8787}"
AUTHENTIK_BROWSER_CLIENT_SLUG="${OPEN_CLI_AUTHENTIK_BROWSER_CLIENT_SLUG:-open-cli-runtime-browser-local}"
AUTHENTIK_BROWSER_PROVIDER_NAME="${OPEN_CLI_AUTHENTIK_BROWSER_PROVIDER_NAME:-open-cli Runtime Browser Local Provider}"
AUTHENTIK_BROWSER_APPLICATION_NAME="${OPEN_CLI_AUTHENTIK_BROWSER_APPLICATION_NAME:-open-cli Runtime Browser Local}"
AUTHENTIK_BROWSER_REDIRECT_URI="${OPEN_CLI_AUTHENTIK_BROWSER_REDIRECT_URI:-http://127.0.0.1:${BROWSER_CALLBACK_PORT}/callback}"
AUTHENTIK_ACCESS_TOKEN_VALIDITY="${OPEN_CLI_AUTHENTIK_ACCESS_TOKEN_VALIDITY:-hours=1}"
RUNTIME_BUNDLE_SCOPE="bundle:${RUNTIME_SERVICE_ID}"
BOOTSTRAP_EXTRA_SCOPES="$(python3 - <<'PY' "$RUNTIME_EXTRA_SERVICE_IDS" "$RUNTIME_EXTRA_SCOPES"
import sys

extra_service_ids = [item for item in sys.argv[1].replace(",", " ").split() if item]
extra_scopes = [item for item in sys.argv[2].split() if item]
scopes = [*(f"bundle:{service_id}" for service_id in extra_service_ids), *extra_scopes]
print(" ".join(scopes))
PY
)"
DOCKER_WITH_SG=0

docker_cmd() {
  if [[ "$DOCKER_WITH_SG" -eq 1 ]]; then
    local command
    printf -v command '%q ' docker "$@"
    sg docker -c "$command"
    return
  fi
  docker "$@"
}

if ! docker info >/dev/null 2>&1; then
  if sg docker -c 'docker info >/dev/null 2>&1'; then
    DOCKER_WITH_SG=1
  else
    echo "docker is installed but this shell cannot access the Docker daemon" >&2
    exit 1
  fi
fi

wait_for_worker() {
  local deadline=$((SECONDS + 120))
  local worker_id=""
  while [[ $SECONDS -lt $deadline ]]; do
    worker_id="$(docker_cmd compose -f "$COMPOSE_FILE" ps -q worker)"
    if [[ -n "$worker_id" ]]; then
      printf '%s\n' "$worker_id"
      return 0
    fi
    sleep 2
  done
  return 1
}

extract_bootstrap_json() {
  python3 -c 'import json, sys
text = sys.stdin.read()
marker = "__OPEN_CLI_JSON__="
index = text.rfind(marker)
if index == -1:
    raise SystemExit(1)
payload = json.loads(text[index + len(marker):].strip())
print(json.dumps(payload))'
}

json_field() {
  local field="$1"
  python3 -c 'import json, sys
payload = json.load(sys.stdin)
print(payload[sys.argv[1]])' "$field"
}

fetch_discovery() {
  local discovery_url="$1"
  local deadline=$((SECONDS + 120))
  while [[ $SECONDS -lt $deadline ]]; do
    if curl --fail --silent --show-error "$discovery_url"; then
      return 0
    fi
    sleep 2
  done
  return 1
}

bootstrap_provider() {
  local worker_id="$1"
  local client_type="$2"
  local client_slug="$3"
  local provider_name="$4"
  local application_name="$5"
  local redirect_uri="$6"
  local encoded_script
  encoded_script="$(base64 -w0 "$BOOTSTRAP_SCRIPT")"
  docker_cmd exec \
    -e OPEN_CLI_BOOTSTRAP_SCRIPT_B64="$encoded_script" \
    -e OPEN_CLI_AUTHENTIK_CLIENT_TYPE="$client_type" \
    -e OPEN_CLI_RUNTIME_AUDIENCE="$RUNTIME_AUDIENCE" \
    -e OPEN_CLI_RUNTIME_SERVICE_ID="$RUNTIME_SERVICE_ID" \
    -e OPEN_CLI_RUNTIME_EXTRA_SCOPES="$BOOTSTRAP_EXTRA_SCOPES" \
    -e OPEN_CLI_AUTHENTIK_CLIENT_SLUG="$client_slug" \
    -e OPEN_CLI_AUTHENTIK_PROVIDER_NAME="$provider_name" \
    -e OPEN_CLI_AUTHENTIK_APPLICATION_NAME="$application_name" \
    -e OPEN_CLI_AUTHENTIK_REDIRECT_URI="$redirect_uri" \
    -e OPEN_CLI_AUTHENTIK_ACCESS_TOKEN_VALIDITY="$AUTHENTIK_ACCESS_TOKEN_VALIDITY" \
    "$worker_id" \
    /ak-root/venv/bin/python /manage.py shell -c "import base64, os; exec(base64.b64decode(os.environ['OPEN_CLI_BOOTSTRAP_SCRIPT_B64']).decode())"
}

bootstrap_provider_json() {
  local worker_id="$1"
  local client_type="$2"
  local client_slug="$3"
  local provider_name="$4"
  local application_name="$5"
  local redirect_uri="$6"
  local bootstrap_output=""
  local bootstrap_json=""
  local bootstrap_deadline=$((SECONDS + 120))
  while [[ $SECONDS -lt $bootstrap_deadline ]]; do
    if bootstrap_output="$(bootstrap_provider "$worker_id" "$client_type" "$client_slug" "$provider_name" "$application_name" "$redirect_uri" 2>&1)"; then
      if bootstrap_json="$(printf '%s' "$bootstrap_output" | extract_bootstrap_json 2>/dev/null)"; then
        printf '%s\n' "$bootstrap_json"
        return 0
      fi
    fi
    sleep 2
  done
  echo "failed to bootstrap Authentik provider" >&2
  printf '%s\n' "$bootstrap_output" >&2
  return 1
}

echo "==> Starting Authentik reference stack..."
docker_cmd compose -f "$COMPOSE_FILE" up -d postgresql redis server worker >/dev/null

echo "==> Waiting for Authentik worker..."
worker_id="$(wait_for_worker)" || {
  echo "failed to find Authentik worker container" >&2
  exit 1
}

echo "==> Bootstrapping local Authentik runtime providers..."
bootstrap_json="$(bootstrap_provider_json "$worker_id" "confidential" "$AUTHENTIK_CLIENT_SLUG" "$AUTHENTIK_PROVIDER_NAME" "$AUTHENTIK_APPLICATION_NAME" "$AUTHENTIK_REDIRECT_URI")"
browser_bootstrap_json="$(bootstrap_provider_json "$worker_id" "public" "$AUTHENTIK_BROWSER_CLIENT_SLUG" "$AUTHENTIK_BROWSER_PROVIDER_NAME" "$AUTHENTIK_BROWSER_APPLICATION_NAME" "$AUTHENTIK_BROWSER_REDIRECT_URI")"

client_slug="$(printf '%s' "$bootstrap_json" | json_field slug)"
client_id="$(printf '%s' "$bootstrap_json" | json_field client_id)"
client_secret="$(printf '%s' "$bootstrap_json" | json_field client_secret)"
browser_client_slug="$(printf '%s' "$browser_bootstrap_json" | json_field slug)"
browser_client_id="$(printf '%s' "$browser_bootstrap_json" | json_field client_id)"

discovery_url="${AUTHENTIK_BASE_URL%/}/application/o/${client_slug}/.well-known/openid-configuration"
echo "==> Waiting for discovery: $discovery_url"
discovery_json="$(fetch_discovery "$discovery_url")" || {
  echo "failed to fetch Authentik discovery document" >&2
  exit 1
}
browse_discovery_url="${AUTHENTIK_BASE_URL%/}/application/o/${browser_client_slug}/.well-known/openid-configuration"
echo "==> Waiting for browser discovery: $browse_discovery_url"
browser_discovery_json="$(fetch_discovery "$browse_discovery_url")" || {
  echo "failed to fetch Authentik browser discovery document" >&2
  exit 1
}

issuer="$(printf '%s' "$discovery_json" | json_field issuer)"
jwks_url="$(printf '%s' "$discovery_json" | json_field jwks_uri)"
token_url="$(printf '%s' "$discovery_json" | json_field token_endpoint)"
browser_issuer="$(printf '%s' "$browser_discovery_json" | json_field issuer)"
browser_jwks_url="$(printf '%s' "$browser_discovery_json" | json_field jwks_uri)"
browser_token_url="$(printf '%s' "$browser_discovery_json" | json_field token_endpoint)"
browser_authorization_url="$(printf '%s' "$browser_discovery_json" | json_field authorization_endpoint)"

mkdir -p "$LOCAL_AUTH_DIR" "$(dirname "$RUNTIME_CONFIG_PATH")" "$(dirname "$CONFIG_PATH")" "$(dirname "$BROWSER_CONFIG_PATH")" "$(dirname "$ENV_PATH")" "$(dirname "$DOCKER_ENV_PATH")"

python3 - <<'PY' "$ENV_PATH" "$client_id" "$client_secret"
import pathlib
import shlex
import sys

path = pathlib.Path(sys.argv[1])
client_id = sys.argv[2]
client_secret = sys.argv[3]
path.write_text(
    "export OPEN_CLI_REMOTE_CLIENT_ID={}\nexport OPEN_CLI_REMOTE_CLIENT_SECRET={}\n".format(
        shlex.quote(client_id),
        shlex.quote(client_secret),
    ),
    encoding="utf-8",
)
path.chmod(0o600)
PY

python3 - <<'PY' "$RUNTIME_CONFIG_PATH" "$issuer" "$jwks_url" "$token_url" "$browser_authorization_url" "$browser_client_id" "$RUNTIME_AUDIENCE" "$RUNTIME_URL" "$RUNTIME_BUNDLE_SCOPE" "$RUNTIME_SERVICE_ID" "$OPENAPI_URI" "$REPO_ROOT" "$RUNTIME_EXTRA_SERVICE_IDS" "$BOOTSTRAP_EXTRA_SCOPES" "$RUNTIME_AUTHENTIK_BASE_URL" "$LOCAL_AUTH_DIR" "$RUNTIME_OPENAPI_SERVER_URL"
import json
import os
import pathlib
import sys
from urllib.parse import urlparse, urlunparse

path = pathlib.Path(sys.argv[1])
issuer = sys.argv[2]
jwks_url = sys.argv[3]
token_url = sys.argv[4]
authorization_url = sys.argv[5]
browser_client_id = sys.argv[6]
runtime_audience = sys.argv[7]
runtime_url = sys.argv[8]
bundle_scope = sys.argv[9]
service_id = sys.argv[10]
openapi_uri = sys.argv[11]
repo_root = pathlib.Path(sys.argv[12]).resolve()
extra_service_ids = [item for item in sys.argv[13].replace(",", " ").split() if item]
extra_scopes = [item for item in sys.argv[14].split() if item]
runtime_authentik_base_url = sys.argv[15]
local_auth_dir = pathlib.Path(sys.argv[16]).resolve()
runtime_openapi_server_url = sys.argv[17]
mounted_runtime_root = pathlib.Path("/config")
mounted_runtime_config_path = mounted_runtime_root / path.name

try:
    mounted_runtime_root = pathlib.Path("/workspace") / local_auth_dir.relative_to(repo_root)
    mounted_runtime_config_path = mounted_runtime_root / path.name
except ValueError:
    pass

def remap_authentik_url(url: str) -> str:
    parsed = urlparse(url)
    base = urlparse(runtime_authentik_base_url)
    return urlunparse(parsed._replace(scheme=base.scheme, netloc=base.netloc))

def rendered_uri(source_id: str, uri: str) -> str:
    if "://" in uri:
        return uri
    source_path = pathlib.Path(uri)
    if not source_path.is_absolute():
        source_path = repo_root / source_path
    source_path = source_path.resolve()
    rendered_path = local_auth_dir / f"{source_id}.openapi.yaml"
    rendered_text = source_path.read_text(encoding="utf-8")
    rendered_text = rendered_text.replace("http://localhost:8080", runtime_openapi_server_url)
    rendered_path.write_text(rendered_text, encoding="utf-8")
    return str(mounted_runtime_root / rendered_path.name)

def unique(items):
    seen = set()
    ordered = []
    for item in items:
        if item in seen:
            continue
        seen.add(item)
        ordered.append(item)
    return ordered

service_ids = unique([service_id, *extra_service_ids])
scopes = unique([bundle_scope, *extra_scopes])
sources = {}
services = {}
for current_service_id in service_ids:
    current_source_id = f"{current_service_id}Source"
    sources[current_source_id] = {
        "type": "openapi",
        "uri": rendered_uri(current_source_id, openapi_uri),
        "enabled": True,
    }
    services[current_service_id] = {
        "source": current_source_id,
        "alias": current_service_id,
    }

config = {
    "cli": "1.0.0",
    "mode": {"default": "discover"},
    "runtime": {
        "mode": "remote",
        "server": {
            "auth": {
                "validationProfile": "oidc_jwks",
                "issuer": issuer,
                "jwksURL": remap_authentik_url(jwks_url),
                "audience": runtime_audience,
                "authorizationURL": authorization_url,
                "tokenURL": token_url,
                "browserClientId": browser_client_id,
            }
        },
        "remote": {
            "url": runtime_url,
            "requestConfigPath": str(mounted_runtime_config_path),
            "oauth": {
                "mode": "oauthClient",
                "audience": runtime_audience,
                "scopes": scopes,
                "client": {
                    "tokenURL": token_url,
                    "clientId": {"type": "env", "value": "OPEN_CLI_REMOTE_CLIENT_ID"},
                    "clientSecret": {"type": "env", "value": "OPEN_CLI_REMOTE_CLIENT_SECRET"},
                },
            },
        },
    },
    "sources": sources,
    "services": services,
}
path.write_text(json.dumps(config, indent=2) + "\n", encoding="utf-8")
PY

MOUNTED_RUNTIME_CONFIG_PATH="$(python3 - <<'PY' "$REPO_ROOT" "$LOCAL_AUTH_DIR" "$RUNTIME_CONFIG_PATH"
import pathlib
import sys

repo_root = pathlib.Path(sys.argv[1]).resolve()
local_auth_dir = pathlib.Path(sys.argv[2]).resolve()
runtime_config_path = pathlib.Path(sys.argv[3]).resolve()
try:
    print((pathlib.Path("/workspace") / runtime_config_path.relative_to(repo_root)).as_posix())
except ValueError:
    print((pathlib.Path("/config") / runtime_config_path.relative_to(local_auth_dir)).as_posix())
PY
)"

python3 - <<'PY' "$CONFIG_PATH" "$token_url" "$RUNTIME_AUDIENCE" "$RUNTIME_URL" "$RUNTIME_BUNDLE_SCOPE" "$BOOTSTRAP_EXTRA_SCOPES" "$MOUNTED_RUNTIME_CONFIG_PATH"
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
token_url = sys.argv[2]
runtime_audience = sys.argv[3]
runtime_url = sys.argv[4]
bundle_scope = sys.argv[5]
extra_scopes = [item for item in sys.argv[6].split() if item]
request_config_path = sys.argv[7]

def unique(items):
    seen = set()
    ordered = []
    for item in items:
        if item in seen:
            continue
        seen.add(item)
        ordered.append(item)
    return ordered

config = {
    "cli": "1.0.0",
    "mode": {"default": "discover"},
    "runtime": {
        "mode": "remote",
        "remote": {
            "url": runtime_url,
            "requestConfigPath": request_config_path,
            "oauth": {
                "mode": "oauthClient",
                "audience": runtime_audience,
                "scopes": unique([bundle_scope, *extra_scopes]),
                "client": {
                    "tokenURL": token_url,
                    "clientId": {"type": "env", "value": "OPEN_CLI_REMOTE_CLIENT_ID"},
                    "clientSecret": {"type": "env", "value": "OPEN_CLI_REMOTE_CLIENT_SECRET"},
                },
            },
        },
    },
}
path.write_text(json.dumps(config, indent=2) + "\n", encoding="utf-8")
PY

python3 - <<'PY' "$BROWSER_CONFIG_PATH" "$RUNTIME_AUDIENCE" "$RUNTIME_URL" "$RUNTIME_BUNDLE_SCOPE" "$BROWSER_CALLBACK_PORT" "$BOOTSTRAP_EXTRA_SCOPES" "$MOUNTED_RUNTIME_CONFIG_PATH"
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
runtime_audience = sys.argv[2]
runtime_url = sys.argv[3]
bundle_scope = sys.argv[4]
callback_port = int(sys.argv[5])
extra_scopes = [item for item in sys.argv[6].split() if item]
request_config_path = sys.argv[7]

def unique(items):
    seen = set()
    ordered = []
    for item in items:
        if item in seen:
            continue
        seen.add(item)
        ordered.append(item)
    return ordered

config = {
    "cli": "1.0.0",
    "mode": {"default": "discover"},
    "runtime": {
        "mode": "remote",
        "remote": {
            "url": runtime_url,
            "requestConfigPath": request_config_path,
            "oauth": {
                "mode": "browserLogin",
                "audience": runtime_audience,
                "scopes": unique([bundle_scope, *extra_scopes]),
                "browserLogin": {
                    "callbackPort": callback_port,
                },
            },
        },
    },
}
path.write_text(json.dumps(config, indent=2) + "\n", encoding="utf-8")
PY

python3 - <<'PY' "$DOCKER_ENV_PATH" "$LOCAL_AUTH_DIR" "$RUNTIME_CONFIG_PATH" "$AUTHENTIK_BASE_URL" "$RUNTIME_URL" "$REPO_ROOT"
import pathlib
import sys
from urllib.parse import urlparse

path = pathlib.Path(sys.argv[1])
mounted_config_dir = pathlib.Path(sys.argv[2]).resolve()
runtime_config_path = pathlib.Path(sys.argv[3]).resolve()
authentik_base_url = urlparse(sys.argv[4])
runtime_url = urlparse(sys.argv[5])
repo_root = pathlib.Path(sys.argv[6]).resolve()
try:
    relative_config_path = runtime_config_path.relative_to(mounted_config_dir)
except ValueError as exc:
    raise SystemExit(f"runtime config path must live under {mounted_config_dir}: {runtime_config_path}") from exc

mounted_runtime_root = pathlib.Path("/config")
try:
    mounted_runtime_root = pathlib.Path("/workspace") / mounted_config_dir.relative_to(repo_root)
except ValueError:
    pass

values = {
    "OPEN_CLI_TOOLBOX_CONFIG_DIR": str(mounted_config_dir),
    "OPEN_CLI_TOOLBOX_CONFIG_PATH": str(mounted_runtime_root / relative_config_path),
    "OPEN_CLI_TOOLBOX_PORT": str(runtime_url.port or 8765),
    "AUTHENTIK_PORT_HTTP": str(authentik_base_url.port or 9100),
}
path.write_text(
    "".join(f"{key}={value}\n" for key, value in values.items()),
    encoding="utf-8",
)
path.chmod(0o600)
PY

echo "==> Wrote hosted runtime config: $RUNTIME_CONFIG_PATH"
echo "==> Wrote workload client config: $CONFIG_PATH"
echo "==> Wrote browser client config: $BROWSER_CONFIG_PATH"
echo "==> Wrote client credential exports: $ENV_PATH"
echo "==> Wrote Docker Compose env file: $DOCKER_ENV_PATH"
echo
echo "Next steps:"
echo "  source \"$ENV_PATH\""
echo "  ./examples/local-authentik/up.sh"
echo "  source \"$ENV_PATH\" && open-cli --config \"$CONFIG_PATH\" catalog list --format pretty"
echo "  # browser client config written to: \"$BROWSER_CONFIG_PATH\""
echo "  # browser client config is exercised against the same compose stack"
echo
echo "Stop the stack with:"
echo "  ./examples/local-authentik/down.sh"
