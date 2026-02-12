#!/usr/bin/env bash
set -euo pipefail

REPO="${NANOBOT_GITHUB_REPO:-Lichas/nanobot-go}"
VERSION="${NANOBOT_VERSION:-latest}"
INSTALL_DIR="${NANOBOT_INSTALL_DIR:-/opt/nanobot-go}"
SERVICE_USER="${NANOBOT_SERVICE_USER:-$(id -un)}"
SETUP_SERVICE=1
BRIDGE_PORT="${BRIDGE_PORT:-3001}"
GATEWAY_PORT="${GATEWAY_PORT:-18890}"
BRIDGE_PROXY="${BRIDGE_PROXY:-}"

usage() {
  cat <<USAGE
nanobot-go Linux installer

Usage: ./install_linux.sh [options]

Options:
  --version <tag|latest>    Release version (default: latest)
  --dir <path>              Install directory (default: /opt/nanobot-go)
  --user <name>             Service user (default: current user)
  --bridge-port <port>      Bridge port (default: 3001)
  --gateway-port <port>     Gateway port (default: 18890)
  --bridge-proxy <url>      Bridge proxy url (optional)
  --no-service              Skip systemd setup
  -h, --help                Show this help
USAGE
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version)
      VERSION="$2"; shift 2 ;;
    --dir)
      INSTALL_DIR="$2"; shift 2 ;;
    --user)
      SERVICE_USER="$2"; shift 2 ;;
    --bridge-port)
      BRIDGE_PORT="$2"; shift 2 ;;
    --gateway-port)
      GATEWAY_PORT="$2"; shift 2 ;;
    --bridge-proxy)
      BRIDGE_PROXY="$2"; shift 2 ;;
    --no-service)
      SETUP_SERVICE=0; shift ;;
    -h|--help)
      usage; exit 0 ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1 ;;
  esac
done

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: required command not found: $1" >&2
    exit 1
  fi
}

need_cmd tar
need_cmd git
need_cmd node
need_cmd npm

if command -v curl >/dev/null 2>&1; then
  DL_BIN="curl"
elif command -v wget >/dev/null 2>&1; then
  DL_BIN="wget"
else
  echo "Error: curl or wget is required" >&2
  exit 1
fi

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported Linux arch: $ARCH_RAW" >&2
    exit 1 ;;
esac

ASSET="nanobot-go_linux_${ARCH}.tar.gz"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

SUDO=""
if [ "$(id -u)" -ne 0 ] && [ ! -w "$(dirname "$INSTALL_DIR")" ]; then
  if command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
  else
    echo "Error: install dir requires root, but sudo is unavailable" >&2
    exit 1
  fi
fi

TMP_DIR="$(mktemp -d)"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

ARCHIVE="$TMP_DIR/$ASSET"
echo "Downloading $URL"
if [ "$DL_BIN" = "curl" ]; then
  curl -fL "$URL" -o "$ARCHIVE"
else
  wget -qO "$ARCHIVE" "$URL"
fi

mkdir -p "$TMP_DIR/payload"
tar -xzf "$ARCHIVE" -C "$TMP_DIR/payload"

$SUDO mkdir -p "$INSTALL_DIR"
if command -v rsync >/dev/null 2>&1; then
  $SUDO rsync -a --delete "$TMP_DIR/payload/" "$INSTALL_DIR/"
else
  $SUDO rm -rf "$INSTALL_DIR"
  $SUDO mkdir -p "$INSTALL_DIR"
  tar -C "$TMP_DIR/payload" -cf - . | $SUDO tar -C "$INSTALL_DIR" -xf -
fi

echo "Installing bridge runtime dependencies"
if [ -n "$SUDO" ]; then
  $SUDO bash -lc "cd '$INSTALL_DIR/bridge' && npm ci --omit=dev --no-audit --no-fund"
else
  (cd "$INSTALL_DIR/bridge" && npm ci --omit=dev --no-audit --no-fund)
fi

if [ "$SETUP_SERVICE" -eq 1 ]; then
  need_cmd systemctl

  HOME_DIR="$(getent passwd "$SERVICE_USER" | cut -d: -f6 || true)"
  if [ -z "$HOME_DIR" ]; then
    if [ "$SERVICE_USER" = "root" ]; then
      HOME_DIR="/root"
    else
      HOME_DIR="/home/$SERVICE_USER"
    fi
  fi

  ENV_FILE="/etc/default/nanobot-go"
  $SUDO bash -lc "cat > '$ENV_FILE' <<ENV
BRIDGE_PORT=$BRIDGE_PORT
GATEWAY_PORT=$GATEWAY_PORT
BRIDGE_PROXY=$BRIDGE_PROXY
ENV"

  for svc in nanobot-bridge nanobot-gateway; do
    src="$INSTALL_DIR/deploy/systemd/${svc}.service"
    dst="/etc/systemd/system/${svc}.service"
    $SUDO sed \
      -e "s|__APP_DIR__|$INSTALL_DIR|g" \
      -e "s|__SERVICE_USER__|$SERVICE_USER|g" \
      -e "s|__HOME_DIR__|$HOME_DIR|g" \
      "$src" | $SUDO tee "$dst" >/dev/null
  done

  $SUDO systemctl daemon-reload
  $SUDO systemctl enable --now nanobot-bridge.service
  $SUDO systemctl enable --now nanobot-gateway.service

  echo "Installed with systemd services:"
  echo "  nanobot-bridge.service"
  echo "  nanobot-gateway.service"
  echo "Status: systemctl status nanobot-gateway --no-pager"
else
  echo "Installed at $INSTALL_DIR"
  echo "Run manually:"
  echo "  $INSTALL_DIR/scripts/run_bridge.sh"
  echo "  $INSTALL_DIR/scripts/run_gateway.sh"
fi

echo "Web UI: http://localhost:${GATEWAY_PORT}"
