# Maxclaw Desktop

Electron desktop app for maxclaw.

## Development

```bash
# Install dependencies
npm install

# Run in development mode (requires Go gateway running or use integrated mode)
npm run dev

# Build for production
npm run build

# Create distributable
npm run dist
```

## Project Structure

- `src/main/` - Electron main process (Node.js)
- `src/renderer/` - React frontend (Chromium)
- `src/preload/` - Secure IPC bridge
- `src/shared/` - Shared types

## Architecture

The desktop app wraps the maxclaw Gateway:
1. Main process starts Gateway as a child process
2. Renderer communicates with Gateway via HTTP/WebSocket
3. Main process provides system integrations (tray, notifications, etc.)
