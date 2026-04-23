# Maxclaw Desktop

Electron desktop app for maxclaw.

## Architecture

The desktop app wraps the maxclaw Gateway as an embedded child process:

```
Electron Main Process
  ├─ Window Manager, Tray, Auto-launch
  ├─ GatewayManager (spawns/monitors Go Gateway)
  └─ IPC Bridge (secure preload script)
        ↓ HTTP/WebSocket
Go Gateway (build/maxclaw-gateway)
  ├─ Web UI Server (port 18890)
  ├─ Agent Loop
  ├─ Cron Service
  └─ Channel Registry
```

### Gateway Lifecycle

`GatewayManager` (`src/main/gateway.ts`) handles the full lifecycle:

1. **Port cleanup** — Before spawning, any process on port 18890 is detected and terminated (SIGTERM → wait 1s → SIGKILL). Cross-platform PID lookup: `lsof` (macOS), `fuser`/`ss` (Linux), `netstat` (Windows).

2. **READY protocol** — Gateway prints `READY:127.0.0.1:18890` to stdout once its HTTP server is listening. `start()` waits for this line (30s timeout) before resolving. Legacy keyword fallback (`"Gateway started"`) and health-check fallback are preserved for old Gateway binaries.

3. **Crash recovery** — If Gateway exits while `state === 'running'`, `attemptRestart()` triggers with exponential backoff (5s → 10s → 20s → ...). Each retry pre-cleans the port.

4. **Graceful shutdown** — On app quit, Gateway receives SIGTERM with a 5s timeout, then SIGKILL if still alive.

## Development

### Prerequisites

- Go 1.24+
- Node.js 18+

### Install dependencies

```bash
cd electron
npm install
```

### Development mode (hot reload)

```bash
npm run dev
```

Runs Vite watchers for main/preload/renderer + Electron. Requires `../build/maxclaw-gateway` to exist (run `make build` in repo root first).

### Production build

```bash
npm run build          # build all three entrypoints
npm run start          # run Electron with production build
```

### Create distributable

```bash
npm run dist           # all platforms
npm run dist:mac       # macOS .dmg + .zip
npm run dist:win       # Windows .exe installer
npm run dist:linux     # Linux AppImage + .deb
```

## Project Structure

- `src/main/` — Electron main process (Node.js)
- `src/preload/` — Secure IPC bridge
- `src/renderer/` — React frontend (Chromium)
- `src/shared/` — Shared types

## Verification

### Test READY protocol

```bash
# Terminal 1: start Gateway manually
../build/maxclaw-gateway maxclaw-gateway --port 18892

# Should see "READY:127.0.0.1:18892" in stdout before "Press Ctrl+C to stop"
```

### Test port cleanup

```bash
# Terminal 1: occupy port 18890
python3 -m http.server 18890

# Terminal 2: start Electron
npm run start

# In Electron's main-process terminal you should see:
# "Cleaning up port 18890 by terminating PIDs: <pid>"
# Then the Python process is killed and Gateway starts successfully.
```

### Test crash recovery

```bash
# Start Electron
npm run start

# Find Gateway PID and kill it
pgrep -f "maxclaw-gateway.*18890"
kill -9 <pid>

# Electron terminal should log "Gateway exited with code ..."
# followed by "Attempting Gateway restart 1/5 in 5000ms"
```
