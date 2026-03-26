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

docker_cmd compose --env-file "$COMPOSE_ENV_PATH" -f "$COMPOSE_FILE" down --remove-orphans
