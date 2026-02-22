#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${MAXCLAW_GITHUB_REPO:-${NANOBOT_GITHUB_REPO:-Lichas/maxclaw}}"
REF="${MAXCLAW_INSTALL_REF:-${NANOBOT_INSTALL_REF:-main}}"

run_local() {
  local script_name="$1"
  shift
  if [ -x "$SCRIPT_DIR/$script_name" ]; then
    exec "$SCRIPT_DIR/$script_name" "$@"
  fi

  local url="https://raw.githubusercontent.com/${REPO}/${REF}/${script_name}"
  if command -v curl >/dev/null 2>&1; then
    exec bash <(curl -fsSL "$url") "$@"
  elif command -v wget >/dev/null 2>&1; then
    exec bash <(wget -qO- "$url") "$@"
  else
    echo "Error: curl or wget is required to download installer" >&2
    exit 1
  fi
}

OS="$(uname -s)"
case "$OS" in
  Linux)
    run_local "install_linux.sh" "$@"
    ;;
  Darwin)
    run_local "install_mac.sh" "$@"
    ;;
  *)
    echo "Unsupported OS: $OS" >&2
    exit 1
    ;;
esac
