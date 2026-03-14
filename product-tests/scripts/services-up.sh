#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

echo "==> Starting services..."
docker compose up -d
docker compose -f mcp/remote/docker-compose.yml up -d
echo "==> Services started."
