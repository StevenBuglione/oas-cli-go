#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LOCAL_AUTH_DIR="${OPEN_CLI_LOCAL_AUTH_DIR:-$REPO_ROOT/.open-cli-local/authentik}"
COMPOSE_ENV_PATH="${OPEN_CLI_DOCKER_ENV_PATH:-$LOCAL_AUTH_DIR/compose.env}"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yaml"

docker_cmd() {
  if docker info >/dev/null 2>&1; then
    docker "$@"
    return
  fi
  printf -v command '%q ' docker "$@"
  sg docker -c "$command"
}

if [[ ! -f "$COMPOSE_ENV_PATH" ]]; then
  echo "missing $COMPOSE_ENV_PATH; run ./examples/local-authentik/setup.sh first" >&2
  exit 1
fi

docker_cmd compose --env-file "$COMPOSE_ENV_PATH" -f "$COMPOSE_FILE" up --build -d

cat <<EOF
==> Local Authentik + open-cli-toolbox stack is up.

Next steps:
1. source $LOCAL_AUTH_DIR/client.env
2. open-cli --config $LOCAL_AUTH_DIR/client.cli.json catalog list --format pretty

Browser-login proof:
  open-cli --config $LOCAL_AUTH_DIR/browser.cli.json catalog list --format pretty

Stop everything with:
  ./examples/local-authentik/down.sh
EOF
