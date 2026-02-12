#!/usr/bin/env bash
set -euo pipefail

REPO="${NANOBOT_GITHUB_REPO:-Lichas/nanobot-go}"
VERSION="${NANOBOT_VERSION:-latest}"
INSTALL_DIR="${NANOBOT_INSTALL_DIR:-$HOME/.local/share/nanobot-go}"
SETUP_LAUNCHD=1
BRIDGE_PORT="${BRIDGE_PORT:-3001}"
GATEWAY_PORT="${GATEWAY_PORT:-18890}"
BRIDGE_PROXY="${BRIDGE_PROXY:-}"

usage() {
  cat <<USAGE
nanobot-go macOS installer

Usage: ./install_mac.sh [options]

Options:
  --version <tag|latest>    Release version (default: latest)
  --dir <path>              Install directory (default: ~/.local/share/nanobot-go)
  --bridge-port <port>      Bridge port (default: 3001)
  --gateway-port <port>     Gateway port (default: 18890)
  --bridge-proxy <url>      Bridge proxy url (optional)
  --no-launchd              Skip launchd setup
  -h, --help                Show this help
USAGE
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version)
      VERSION="$2"; shift 2 ;;
    --dir)
      INSTALL_DIR="$2"; shift 2 ;;
    --bridge-port)
      BRIDGE_PORT="$2"; shift 2 ;;
    --gateway-port)
      GATEWAY_PORT="$2"; shift 2 ;;
    --bridge-proxy)
      BRIDGE_PROXY="$2"; shift 2 ;;
    --no-launchd)
      SETUP_LAUNCHD=0; shift ;;
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
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported macOS arch: $ARCH_RAW" >&2
    exit 1 ;;
esac

ASSET="nanobot-go_darwin_${ARCH}.tar.gz"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
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
mkdir -p "$INSTALL_DIR"

if command -v rsync >/dev/null 2>&1; then
  rsync -a --delete "$TMP_DIR/payload/" "$INSTALL_DIR/"
else
  rm -rf "$INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
  tar -C "$TMP_DIR/payload" -cf - . | tar -C "$INSTALL_DIR" -xf -
fi

echo "Installing bridge runtime dependencies"
(cd "$INSTALL_DIR/bridge" && npm ci --omit=dev --no-audit --no-fund)

if [ "$SETUP_LAUNCHD" -eq 1 ]; then
  need_cmd launchctl
  LAUNCH_DIR="$HOME/Library/LaunchAgents"
  mkdir -p "$LAUNCH_DIR"

  cat > "$LAUNCH_DIR/com.nanobot.bridge.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.nanobot.bridge</string>
  <key>ProgramArguments</key>
  <array><string>$INSTALL_DIR/scripts/run_bridge.sh</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>EnvironmentVariables</key>
  <dict>
    <key>BRIDGE_PORT</key><string>$BRIDGE_PORT</string>
    <key>BRIDGE_PROXY</key><string>$BRIDGE_PROXY</string>
  </dict>
  <key>StandardOutPath</key><string>$HOME/Library/Logs/nanobot-bridge.log</string>
  <key>StandardErrorPath</key><string>$HOME/Library/Logs/nanobot-bridge.log</string>
</dict>
</plist>
PLIST

  cat > "$LAUNCH_DIR/com.nanobot.gateway.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.nanobot.gateway</string>
  <key>ProgramArguments</key>
  <array><string>$INSTALL_DIR/scripts/run_gateway.sh</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>EnvironmentVariables</key>
  <dict>
    <key>GATEWAY_PORT</key><string>$GATEWAY_PORT</string>
  </dict>
  <key>StandardOutPath</key><string>$HOME/Library/Logs/nanobot-gateway.log</string>
  <key>StandardErrorPath</key><string>$HOME/Library/Logs/nanobot-gateway.log</string>
</dict>
</plist>
PLIST

  launchctl bootout "gui/$(id -u)/com.nanobot.bridge" >/dev/null 2>&1 || true
  launchctl bootout "gui/$(id -u)/com.nanobot.gateway" >/dev/null 2>&1 || true
  launchctl bootstrap "gui/$(id -u)" "$LAUNCH_DIR/com.nanobot.bridge.plist"
  launchctl bootstrap "gui/$(id -u)" "$LAUNCH_DIR/com.nanobot.gateway.plist"

  echo "Installed launchd agents:"
  echo "  com.nanobot.bridge"
  echo "  com.nanobot.gateway"
  echo "Status: launchctl print gui/$(id -u)/com.nanobot.gateway"
else
  echo "Installed at $INSTALL_DIR"
  echo "Run manually:"
  echo "  $INSTALL_DIR/scripts/run_bridge.sh"
  echo "  $INSTALL_DIR/scripts/run_gateway.sh"
fi

echo "Web UI: http://localhost:${GATEWAY_PORT}"
