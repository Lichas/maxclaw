import { spawn, ChildProcess, spawnSync } from 'child_process';
import path from 'path';
import os from 'os';
import fs from 'fs';
import { app } from 'electron';
import log from 'electron-log';
import http from 'http';

const GATEWAY_HTTP_ORIGIN = 'http://127.0.0.1:18890';

export interface GatewayStatus {
  state: 'running' | 'stopped' | 'error' | 'starting';
  port: number;
  error?: string;
}

export class GatewayManager {
  private process: ChildProcess | null = null;
  private status: GatewayStatus = { state: 'stopped', port: 18890 };
  private restartAttempts = 0;
  private maxRestartAttempts = 5;
  private restartDelay = 5000;

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

  async startFresh(): Promise<void> {
    log.info('Starting Gateway with fresh restart...');
    await this.stop();
    await this.start();
  }

  async stop(): Promise<void> {
    if (!this.process) {
      return;
    }

    log.info('Stopping Gateway...');

    return new Promise((resolve) => {
      const timeout = setTimeout(() => {
        log.warn('Gateway stop timeout, forcing kill');
        this.process?.kill('SIGKILL');
        resolve();
      }, 5000);

      this.process?.once('exit', () => {
        clearTimeout(timeout);
        this.process = null;
        this.status = { state: 'stopped', port: 18890 };
        resolve();
      });

      this.process?.kill('SIGTERM');
    });
  }

  async restart(): Promise<void> {
    log.info('Restarting Gateway...');
    await this.stop();
    await new Promise(resolve => setTimeout(resolve, 1000));
    await this.start();
  }

  async healthCheck(): Promise<boolean> {
    return new Promise((resolve) => {
      const req = http.get(`${GATEWAY_HTTP_ORIGIN}/api/status`, (res) => {
        resolve(res.statusCode === 200);
      });

      req.on('error', () => {
        resolve(false);
      });

      req.setTimeout(3000, () => {
        req.destroy();
        resolve(false);
      });
    });
  }

  async refreshStatus(): Promise<GatewayStatus> {
    const healthy = await this.healthCheck();

    if (healthy) {
      this.status = { state: 'running', port: 18890 };
      this.restartAttempts = 0;
      return this.getStatus();
    }

    if (this.process && this.status.state === 'starting') {
      return this.getStatus();
    }

    if (this.process) {
      this.status = {
        state: 'error',
        port: 18890,
        error: this.status.error || 'Gateway process exists but health check failed'
      };
      return this.getStatus();
    }

    if (this.status.state === 'error') {
      return this.getStatus();
    }

    this.status = { state: 'stopped', port: 18890 };
    return this.getStatus();
  }

  getStatus(): GatewayStatus {
    return { ...this.status };
  }

  private getBinaryPath(): string {
    const platform = os.platform();
    const ext = platform === 'win32' ? '.exe' : '';
    const gatewayBinaryName = `maxclaw-gateway${ext}`;
    const cliBinaryName = `maxclaw${ext}`;

    if (app.isPackaged) {
      return path.join(process.resourcesPath, 'bin', gatewayBinaryName);
    }

    const overrideBinaryPath = process.env.MAXCLAW_BINARY_PATH || process.env.NANOBOT_BINARY_PATH;
    const appPath = app.getAppPath();
    const candidates = [
      overrideBinaryPath,
      path.resolve(appPath, '..', 'build', gatewayBinaryName),
      path.resolve(appPath, 'build', gatewayBinaryName),
      path.resolve(process.cwd(), '..', 'build', gatewayBinaryName),
      path.resolve(process.cwd(), 'build', gatewayBinaryName),
      path.resolve(appPath, '..', 'build', cliBinaryName),
      path.resolve(appPath, 'build', cliBinaryName),
      path.resolve(process.cwd(), '..', 'build', cliBinaryName),
      path.resolve(process.cwd(), 'build', cliBinaryName)
    ].filter((candidate): candidate is string => Boolean(candidate));

    const existingPath = candidates.find(candidate => fs.existsSync(candidate));
    if (existingPath) {
      return existingPath;
    }

    log.warn('Gateway binary was not found in expected locations:', candidates);
    return candidates[0];
  }

  private getLaunchArgs(binaryPath: string): string[] {
    const binaryName = path.basename(binaryPath).toLowerCase();
    if (binaryName.includes('gateway')) {
      return ['maxclaw-gateway', '-p', '18890'];
    }

    return ['gateway', '-p', '18890'];
  }

  private getConfigPath(): string {
    const maxclawDir = process.env.MAXCLAW_HOME || path.join(os.homedir(), '.maxclaw');
    const legacyDir = process.env.NANOBOT_HOME || path.join(os.homedir(), '.nanobot');

    const maxclawConfigPath = path.join(maxclawDir, 'config.json');
    if (fs.existsSync(maxclawConfigPath)) {
      return maxclawConfigPath;
    }

    const legacyConfigPath = path.join(legacyDir, 'config.json');
    if (fs.existsSync(legacyConfigPath)) {
      return legacyConfigPath;
    }

    return maxclawConfigPath;
  }

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

  private attemptRestart(): void {
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

    setTimeout(() => {
      this.start().catch(error => {
        log.error('Restart failed:', error);
      });
    }, delay);
  }

}
