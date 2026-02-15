#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "==> Restarting nanobot daemon services"

"$ROOT_DIR/scripts/stop_daemon.sh"
sleep 0.5
"$ROOT_DIR/scripts/start_daemon.sh"
