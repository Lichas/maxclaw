# Electron Gateway 生命周期增强 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure maxclaw Gateway is reliably ready when Electron starts, handling port conflicts and crashes automatically.

**Architecture:** Port-exclusive mode: Electron cleans up any process on port 18890 before starting Gateway. Go emits a READY protocol on stdout. Electron waits for READY before declaring Gateway running. Crash restarts also pre-clean the port.

**Tech Stack:** Go 1.24, TypeScript/Electron 28, Node.js child_process

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/cli/gateway.go` | Modify | Add READY protocol output after Gateway HTTP server is listening |
| `electron/src/main/gateway.ts` | Modify | Add `cleanupPortLock()`, READY protocol parsing in `start()`, port cleanup in `attemptRestart()` |
| `electron/src/main/index.ts` | Modify | Adjust `initializeApp()` to use `start()` instead of `startFresh()`, notify renderer on startup failure |

---

### Task 1: Go READY Protocol

**Files:**
- Modify: `internal/cli/gateway.go`

Add a goroutine after web server startup that polls the TCP port and prints `READY:127.0.0.1:18890` once the server is accepting connections.

- [ ] **Step 1: Add `net` import to `internal/cli/gateway.go`**

```go
import (
	"context"
	"fmt"
	"net"      // ADD THIS
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	// ... existing imports
)
```

- [ ] **Step 2: Add READY protocol goroutine after web server startup**

Find this block in `internal/cli/gateway.go` (around line 280):

```go
	// 启动 Web UI/API 服务器
	webServer := webui.NewServer(cfg, agentLoop, cronService, channelRegistry)
	go func() {
		if err := webServer.Start(ctx, cfg.Gateway.Host, gatewayPort); err != nil && err != context.Canceled {
			fmt.Printf("⚠ Web UI server error: %v\n", err)
			if lg := logging.Get(); lg != nil && lg.Web != nil {
				lg.Web.Printf("webui error: %v", err)
			}
		}
	}()
```

Replace it with:

```go
	// 启动 Web UI/API 服务器
	webServer := webui.NewServer(cfg, agentLoop, cronService, channelRegistry)
	go func() {
		if err := webServer.Start(ctx, cfg.Gateway.Host, gatewayPort); err != nil && err != context.Canceled {
			fmt.Printf("⚠ Web UI server error: %v\n", err)
			if lg := logging.Get(); lg != nil && lg.Web != nil {
				lg.Web.Printf("webui error: %v", err)
			}
		}
	}()

	// READY protocol: poll TCP port and announce readiness to parent process
	go func() {
		addr := fmt.Sprintf("127.0.0.1:%d", gatewayPort)
		for i := 0; i < 200; i++ {
			conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
			if err == nil {
				conn.Close()
				fmt.Printf("READY:%s\n", addr)
				if lg := logging.Get(); lg != nil && lg.Gateway != nil {
					lg.Gateway.Printf("ready protocol sent: %s", addr)
				}
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		if lg := logging.Get(); lg != nil && lg.Gateway != nil {
			lg.Gateway.Printf("ready protocol timeout: port %d not reachable after 10s", gatewayPort)
		}
	}()
```

- [ ] **Step 3: Build Go binary and verify compilation**

Run: `make build`
Expected: Build succeeds with no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/gateway.go
git commit -m "feat(gateway): emit READY protocol on stdout when HTTP server is listening"
```

---

### Task 2: Electron Port Cleanup

**Files:**
- Modify: `electron/src/main/gateway.ts`

Add cross-platform PID discovery and process termination for port 18890.

- [ ] **Step 1: Add `cleanupPortLock()` and platform-specific PID finders to `GatewayManager`**

Insert these private methods into the `GatewayManager` class in `electron/src/main/gateway.ts`, after `getConfigPath()` and before `attemptRestart()`:

```typescript
  private async cleanupPortLock(port: number): Promise<void> {
    const pids = await this.findPidsForPort(port);
    const excludedPids = new Set<number>();
    excludedPids.add(process.pid);
    if (this.process?.pid) {
      excludedPids.add(this.process.pid);
    }

    const targetPids = pids.filter(pid => !excludedPids.has(pid));
    if (targetPids.length === 0) {
      return;
    }

    log.warn(`Cleaning up port ${port} by terminating PIDs: ${targetPids.join(', ')}`);

    // Phase 1: SIGTERM
    for (const pid of targetPids) {
      try {
        process.kill(pid, 'SIGTERM');
      } catch (error) {
        log.warn(`SIGTERM failed for PID ${pid}:`, error);
      }
    }

    await new Promise(resolve => setTimeout(resolve, 1000));

    // Phase 2: SIGKILL survivors
    const survivors = await this.findPidsForPort(port);
    for (const pid of survivors) {
      if (excludedPids.has(pid)) continue;
      try {
        process.kill(pid, 0); // probe if alive
        try {
          process.kill(pid, 'SIGKILL');
          log.info(`SIGKILL sent to PID ${pid}`);
        } catch (error) {
          log.warn(`SIGKILL failed for PID ${pid}:`, error);
        }
      } catch {
        // already dead
      }
    }
  }

  private async findPidsForPort(port: number): Promise<number[]> {
    const platform = os.platform();
    if (platform === 'darwin') {
      return this.findPidsDarwin(port);
    }
    if (platform === 'linux') {
      return this.findPidsLinux(port);
    }
    if (platform === 'win32') {
      return this.findPidsWindows(port);
    }
    return [];
  }

  private findPidsDarwin(port: number): number[] {
    try {
      const result = spawnSync('lsof', ['-iTCP:' + port, '-sTCP:LISTEN', '-t', '-P'], {
        encoding: 'utf8',
        timeout: 5000
      });
      if (result.status !== 0 || !result.stdout.trim()) {
        return [];
      }
      return result.stdout.split('\n')
        .map(s => parseInt(s.trim(), 10))
        .filter(n => Number.isFinite(n) && n > 0);
    } catch (error) {
      log.error('Failed to find PIDs on macOS:', error);
      return [];
    }
  }

  private findPidsLinux(port: number): number[] {
    // Try fuser first
    try {
      const result = spawnSync('fuser', [port + '/tcp'], {
        encoding: 'utf8',
        timeout: 5000
      });
      if (result.status === 0 && result.stdout.trim()) {
        return result.stdout.trim().split(/\s+/)
          .map(s => parseInt(s.trim(), 10))
          .filter(n => Number.isFinite(n) && n > 0);
      }
    } catch {
      // fuser not available, fall through
    }

    // Fallback: ss
    try {
      const result = spawnSync('ss', ['-tlnp', 'sport = :' + port], {
        encoding: 'utf8',
        timeout: 5000
      });
      if (result.status !== 0 || !result.stdout.trim()) {
        return [];
      }
      const pids: number[] = [];
      for (const line of result.stdout.split('\n')) {
        const match = line.match(/pid=(\d+)/);
        if (match) {
          const pid = parseInt(match[1], 10);
          if (Number.isFinite(pid) && pid > 0) {
            pids.push(pid);
          }
        }
      }
      return pids;
    } catch (error) {
      log.error('Failed to find PIDs on Linux:', error);
      return [];
    }
  }

  private findPidsWindows(port: number): number[] {
    try {
      const result = spawnSync('cmd.exe', ['/c', `netstat -ano | findstr :${port}`], {
        encoding: 'utf8',
        timeout: 5000
      });
      if (result.status !== 0 || !result.stdout.trim()) {
        return [];
      }
      const pids: number[] = [];
      for (const line of result.stdout.split('\n')) {
        const parts = line.trim().split(/\s+/);
        if (parts.length >= 5) {
          const state = parts[3]?.toUpperCase();
          if (state !== 'LISTENING' && state !== 'ESTABLISHED') {
            continue;
          }
          const pid = parseInt(parts[parts.length - 1], 10);
          if (Number.isFinite(pid) && pid > 0) {
            pids.push(pid);
          }
        }
      }
      return [...new Set(pids)];
    } catch (error) {
      log.error('Failed to find PIDs on Windows:', error);
      return [];
    }
  }
```

- [ ] **Step 2: Call `cleanupPortLock()` at the beginning of `start()`**

In `electron/src/main/gateway.ts`, inside `start()`, after the early-return check and before `this.healthCheck()`, add:

```typescript
    // Ensure port is free before starting
    try {
      await this.cleanupPortLock(this.status.port);
    } catch (error) {
      log.error('Failed to cleanup port lock:', error);
      this.status = { state: 'error', port: this.status.port, error: 'Port cleanup failed: ' + (error as Error).message };
      throw error;
    }
```

- [ ] **Step 3: Remove the old `terminateExistingGatewayProcesses()` method and its call in `startFresh()`**

`startFresh()` currently calls `terminateExistingGatewayProcesses()`. Since `cleanupPortLock()` now handles all port cleanup (including non-maxclaw processes), remove `terminateExistingGatewayProcesses()` entirely.

Replace `startFresh()`:

```typescript
  async startFresh(): Promise<void> {
    log.info('Starting Gateway with fresh restart...');
    await this.stop();
    await this.start();
  }
```

Delete the entire `terminateExistingGatewayProcesses()` method (lines 302-352 in the original file).

- [ ] **Step 4: Commit**

```bash
git add electron/src/main/gateway.ts
git commit -m "feat(electron): add cross-platform port cleanup before gateway start"
```

---

### Task 3: Electron READY Protocol Integration

**Files:**
- Modify: `electron/src/main/gateway.ts`

Update `start()` to parse `READY:host:port` from Gateway stdout instead of relying on timeout + keyword heuristics.

- [ ] **Step 1: Rewrite `start()` to use READY protocol**

Replace the `start()` method body with this implementation. Keep the same signature `async start(): Promise<void>`.

Key changes:
- Add `readyResolved` flag
- Listen for `READY:host:port` regex on stdout
- Timeout increased from 10s to 30s
- Keep existing `healthCheck()` fallback for backward compatibility

```typescript
  async start(): Promise<void> {
    if (this.process) {
      log.info('Gateway already running');
      return;
    }

    // Ensure port is free before starting
    try {
      await this.cleanupPortLock(this.status.port);
    } catch (error) {
      log.error('Failed to cleanup port lock:', error);
      this.status = { state: 'error', port: this.status.port, error: 'Port cleanup failed: ' + (error as Error).message };
      throw error;
    }

    if (await this.healthCheck()) {
      log.info('Detected existing healthy Gateway on port 18890, reusing it');
      this.status = { state: 'running', port: 18890 };
      this.restartAttempts = 0;
      return;
    }

    this.status = { state: 'starting', port: 18890 };
    log.info('Starting Gateway...');

    const binaryPath = this.getBinaryPath();
    const configPath = this.getConfigPath();

    if (!fs.existsSync(binaryPath)) {
      const message = `Gateway binary not found: ${binaryPath}. Run "make build" in repository root first.`;
      this.status = { state: 'error', port: 18890, error: message };
      throw new Error(message);
    }

    if (!fs.existsSync(configPath)) {
      log.warn('Config not found, Gateway may fail to start');
    }

    return new Promise((resolve, reject) => {
      const launchArgs = this.getLaunchArgs(binaryPath);
      log.info('Launching gateway binary:', binaryPath, launchArgs.join(' '));

      this.process = spawn(binaryPath, launchArgs, {
        stdio: ['ignore', 'pipe', 'pipe'],
        env: {
          ...process.env,
          MAXCLAW_ELECTRON: '1',
          NANOBOT_ELECTRON: '1'
        }
      });

      let startupTimeout: NodeJS.Timeout;
      let readyResolved = false;

      // READY protocol listener
      this.process.stdout?.on('data', (data: Buffer) => {
        const output = data.toString();
        log.info('[Gateway]', output.trim());

        if (!readyResolved) {
          const readyMatch = output.match(/^READY:(.+):(\d+)$/m);
          if (readyMatch) {
            const host = readyMatch[1];
            const port = parseInt(readyMatch[2], 10);
            readyResolved = true;
            this.status = { state: 'running', port };
            this.restartAttempts = 0;
            clearTimeout(startupTimeout);
            log.info(`Gateway ready at ${host}:${port}`);
            resolve();
            return;
          }
        }

        // Fallback: legacy keyword detection
        if (!readyResolved && (output.includes('Gateway started') || output.includes('listening on'))) {
          readyResolved = true;
          this.status = { state: 'running', port: 18890 };
          this.restartAttempts = 0;
          clearTimeout(startupTimeout);
          resolve();
        }
      });

      this.process.stderr?.on('data', (data: Buffer) => {
        log.error('[Gateway]', data.toString().trim());
      });

      this.process.on('exit', (code) => {
        log.warn(`Gateway exited with code ${code}`);
        this.process = null;

        if (!readyResolved && this.status.state === 'starting') {
          clearTimeout(startupTimeout);
          reject(new Error(`Gateway failed to start (exit code: ${code})`));
        } else if (this.status.state === 'running') {
          this.status = { state: 'stopped', port: 18890 };
          this.attemptRestart();
        }
      });

      this.process.on('error', (error) => {
        log.error('Gateway process error:', error);
        this.status = { state: 'error', port: 18890, error: error.message };
        clearTimeout(startupTimeout);
        reject(error);
      });

      // 30-second timeout with health check fallback
      startupTimeout = setTimeout(() => {
        if (!readyResolved) {
          this.healthCheck().then(healthy => {
            if (healthy && !readyResolved) {
              readyResolved = true;
              this.status = { state: 'running', port: 18890 };
              this.restartAttempts = 0;
              resolve();
            } else if (!readyResolved) {
              reject(new Error('Gateway startup timeout: READY protocol not received'));
            }
          });
        }
      }, 30000);
    });
  }
```

- [ ] **Step 2: Commit**

```bash
git add electron/src/main/gateway.ts
git commit -m "feat(electron): integrate READY protocol for reliable gateway startup"
```

---

### Task 4: Crash Restart Enhancement

**Files:**
- Modify: `electron/src/main/gateway.ts`

Ensure `attemptRestart()` cleans the port before each restart attempt.

- [ ] **Step 1: Update `attemptRestart()` to call `cleanupPortLock()` before restarting**

Replace the `attemptRestart()` method:

```typescript
  private async attemptRestart(): Promise<void> {
    if (this.restartAttempts >= this.maxRestartAttempts) {
      log.error(`Max restart attempts (${this.maxRestartAttempts}) reached`);
      this.status = {
        state: 'error',
        port: 18890,
        error: 'Max restart attempts reached'
      };
      return;
    }

    this.restartAttempts++;
    const delay = this.restartDelay * Math.pow(2, this.restartAttempts - 1);

    log.info(`Attempting Gateway restart ${this.restartAttempts}/${this.maxRestartAttempts} in ${delay}ms`);

    setTimeout(async () => {
      try {
        await this.cleanupPortLock(this.status.port);
      } catch (error) {
        log.error('Port cleanup before restart failed:', error);
      }

      this.start().catch(error => {
        log.error('Restart failed:', error);
      });
    }, delay);
  }
```

- [ ] **Step 2: Commit**

```bash
git add electron/src/main/gateway.ts
git commit -m "feat(electron): cleanup port before crash restart attempts"
```

---

### Task 5: Window Lifecycle Integration

**Files:**
- Modify: `electron/src/main/index.ts`

Use `start()` instead of `startFresh()` (since `start()` now handles port cleanup internally). Notify renderer of Gateway status on startup failure.

- [ ] **Step 1: Replace `startFresh()` with `start()` in `initializeApp()`**

Find this block in `electron/src/main/index.ts`:

```typescript
async function initializeApp(): Promise<void> {
  log.info('Initializing Maxclaw Desktop App');

  // Initialize Gateway Manager
  gatewayManager = new GatewayManager();

  // Start Gateway before creating window
  try {
    await gatewayManager.startFresh();
    log.info('Gateway started successfully');
  } catch (error) {
    log.error('Failed to start Gateway:', error);
    // Continue anyway - will show error in UI
  }

  await ensureMainWindow();
```

Replace with:

```typescript
async function initializeApp(): Promise<void> {
  log.info('Initializing Maxclaw Desktop App');

  // Initialize Gateway Manager
  gatewayManager = new GatewayManager();

  // Start Gateway before creating window (waits for READY protocol)
  try {
    await gatewayManager.start();
    log.info('Gateway started successfully');
  } catch (error) {
    log.error('Failed to start Gateway:', error);
    // Continue to create window so user sees error state
  }

  await ensureMainWindow();

  // Notify renderer of gateway status if startup failed
  if (mainWindow && gatewayManager) {
    const status = gatewayManager.getStatus();
    if (status.state === 'error') {
      mainWindow.webContents.send('gateway:status', status);
    }
  }
```

- [ ] **Step 2: Commit**

```bash
git add electron/src/main/index.ts
git commit -m "feat(electron): use start() with READY protocol, notify renderer on failure"
```

---

### Task 6: Build and End-to-End Verification

**Files:**
- None (build + manual test)

- [ ] **Step 1: Build Go Gateway binary**

Run: `make build`
Expected: `build/maxclaw-gateway` (and `build/maxclaw`) created successfully.

- [ ] **Step 2: Test READY protocol from command line**

Run:
```bash
./build/maxclaw-gateway -p 18891 &
PID=$!
sleep 3
kill $PID
```
Expected: Output contains `READY:127.0.0.1:18891` before "Press Ctrl+C to stop".

- [ ] **Step 3: Test port cleanup scenario**

Terminal 1 (occupy port):
```bash
python3 -m http.server 18890
```

Terminal 2 (build and test Electron — if you have Electron dev env ready):
```bash
cd electron && npm run build
```

Then manually test by running the Electron app and verifying:
1. The Python HTTP server process is terminated
2. Gateway starts successfully
3. Window appears with main UI (not error state)

If you don't have the full Electron dev environment set up, verify the port cleanup logic directly:

```bash
# Terminal 1: occupy port
python3 -m http.server 18890 &
PY_PID=$!

# Terminal 2: run a quick Node.js script to test cleanup logic
node -e "
const { spawnSync } = require('child_process');
const result = spawnSync('lsof', ['-iTCP:18890', '-sTCP:LISTEN', '-t', '-P'], { encoding: 'utf8' });
console.log('PIDs on port 18890:', result.stdout.trim());
"

# Cleanup
kill $PY_PID 2>/dev/null
```

- [ ] **Step 4: Commit final CHANGELOG entry**

Append to `CHANGELOG.md` under `## [Unreleased]`:

```markdown
- feat(electron): Gateway lifecycle enhancement — port-exclusive startup with READY protocol
  - Go gateway emits `READY:host:port` on stdout when HTTP server is listening
  - Electron cleans up any process occupying port 18890 before starting gateway
  - Cross-platform PID detection: lsof (macOS), fuser/ss (Linux), netstat (Windows)
  - `start()` waits for READY protocol (30s timeout) before resolving
  - Crash restarts pre-clean the port to avoid conflict loops
```

- [ ] **Step 5: Final commit**

```bash
git add CHANGELOG.md
git commit -m "docs: update CHANGELOG with gateway lifecycle enhancement"
```

---

## Self-Review Checklist

**1. Spec coverage:**
- [x] Port cleanup for any process on 18890 → Task 2
- [x] READY protocol (Go stdout + Electron parsing) → Task 1 + Task 3
- [x] Window appears after Gateway ready → Task 5 (`start()` resolves before `ensureMainWindow()`)
- [x] Crash restart with port pre-cleanup → Task 4
- [x] Error classification → implicit: `start()` rejects with specific errors, `index.ts` notifies renderer

**2. Placeholder scan:**
- [x] No "TBD", "TODO", "implement later"
- [x] No "add appropriate error handling" — specific error handling shown in code
- [x] No "similar to Task N" — each task is self-contained
- [x] No "write tests for the above" — no new test files needed (E2E verification in Task 6)

**3. Type consistency:**
- [x] `GatewayStatus` interface unchanged (`state: 'running' | 'stopped' | 'error' | 'starting'`)
- [x] `cleanupPortLock(port: number)` consistent across all call sites
- [x] `attemptRestart()` stays `private async` (was `private`, now correctly `private async`)

**4. Backward compatibility:**
- [x] Old `startFresh()` behavior preserved via `cleanupPortLock()` (actually improved — now kills any port occupant, not just maxclaw processes)
- [x] `healthCheck()` fallback in `start()` ensures old Gateways without READY protocol still work
- [x] `terminateExistingGatewayProcesses()` removed — its functionality superseded by `cleanupPortLock()`
