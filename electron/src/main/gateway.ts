import { spawn, ChildProcess, spawnSync } from 'child_process';
import path from 'path';
import os from 'os';
import fs from 'fs';
import { app } from 'electron';
import log from 'electron-log';
import http from 'http';

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

    this.status = { state: 'starting', port: 18890 };
    log.info('Starting Gateway...');

    const binaryPath = this.getBinaryPath();
    const configPath = path.join(os.homedir(), '.nanobot', 'config.json');

    if (!fs.existsSync(binaryPath)) {
      const message = `Gateway binary not found: ${binaryPath}. Run "make build" in repository root first.`;
      this.status = { state: 'error', port: 18890, error: message };
      throw new Error(message);
    }

    // Ensure config exists
    if (!fs.existsSync(configPath)) {
      log.warn('Config not found, Gateway may fail to start');
    }

    return new Promise((resolve, reject) => {
      this.process = spawn(binaryPath, ['gateway', '-p', '18890'], {
        stdio: ['ignore', 'pipe', 'pipe'],
        env: {
          ...process.env,
          NANOBOT_ELECTRON: '1'
        }
      });

      let startupTimeout: NodeJS.Timeout;

      // Handle stdout
      this.process.stdout?.on('data', (data: Buffer) => {
        const output = data.toString().trim();
        log.info('[Gateway]', output);

        // Check for successful startup indicators
        if (output.includes('Gateway started') || output.includes('listening on')) {
          this.status = { state: 'running', port: 18890 };
          this.restartAttempts = 0;
          clearTimeout(startupTimeout);
          resolve();
        }
      });

      // Handle stderr
      this.process.stderr?.on('data', (data: Buffer) => {
        log.error('[Gateway]', data.toString().trim());
      });

      // Handle process exit
      this.process.on('exit', (code) => {
        log.warn(`Gateway exited with code ${code}`);
        this.process = null;

        if (this.status.state === 'starting') {
          clearTimeout(startupTimeout);
          reject(new Error(`Gateway failed to start (exit code: ${code})`));
        } else if (this.status.state === 'running') {
          this.status = { state: 'stopped', port: 18890 };
          this.attemptRestart();
        }
      });

      // Handle errors
      this.process.on('error', (error) => {
        log.error('Gateway process error:', error);
        this.status = { state: 'error', port: 18890, error: error.message };
        clearTimeout(startupTimeout);
        reject(error);
      });

      // Timeout for startup
      startupTimeout = setTimeout(() => {
        // Try health check as fallback
        this.healthCheck().then(healthy => {
          if (healthy) {
            this.status = { state: 'running', port: 18890 };
            this.restartAttempts = 0;
            resolve();
          } else {
            reject(new Error('Gateway startup timeout'));
          }
        });
      }, 10000);
    });
  }

  async startFresh(): Promise<void> {
    log.info('Starting Gateway with fresh restart...');
    await this.stop();
    await this.terminateExistingGatewayProcesses();
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
      const req = http.get('http://localhost:18890/api/status', (res) => {
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

  getStatus(): GatewayStatus {
    return { ...this.status };
  }

  private getBinaryPath(): string {
    const platform = os.platform();
    const ext = platform === 'win32' ? '.exe' : '';
    const binaryName = `nanobot-go${ext}`;

    if (app.isPackaged) {
      return path.join(process.resourcesPath, 'bin', binaryName);
    }

    const overrideBinaryPath = process.env.NANOBOT_BINARY_PATH;
    const appPath = app.getAppPath();
    const candidates = [
      overrideBinaryPath,
      path.resolve(appPath, '..', 'build', binaryName),
      path.resolve(appPath, 'build', binaryName),
      path.resolve(process.cwd(), '..', 'build', binaryName),
      path.resolve(process.cwd(), 'build', binaryName)
    ].filter((candidate): candidate is string => Boolean(candidate));

    const existingPath = candidates.find(candidate => fs.existsSync(candidate));
    if (existingPath) {
      return existingPath;
    }

    log.warn('Gateway binary was not found in expected locations:', candidates);
    return candidates[0];
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

  private async terminateExistingGatewayProcesses(): Promise<void> {
    if (process.platform === 'win32') {
      log.info('Skip external gateway cleanup on Windows');
      return;
    }

    const pgrepResult = spawnSync('pgrep', ['-f', 'nanobot-go gateway -p 18890'], {
      encoding: 'utf8'
    });

    if (pgrepResult.status !== 0 || !pgrepResult.stdout.trim()) {
      return;
    }

    const pids = pgrepResult.stdout
      .split('\n')
      .map((pid) => pid.trim())
      .filter(Boolean)
      .map((pid) => Number(pid))
      .filter((pid) => Number.isFinite(pid) && pid > 0 && pid !== process.pid && pid !== this.process?.pid);

    if (pids.length === 0) {
      return;
    }

    log.warn(`Terminating existing gateway processes on startup: ${pids.join(', ')}`);

    for (const pid of pids) {
      try {
        process.kill(pid, 'SIGTERM');
      } catch (error) {
        log.warn(`Failed to SIGTERM process ${pid}:`, error);
      }
    }

    await new Promise((resolve) => setTimeout(resolve, 800));

    for (const pid of pids) {
      try {
        process.kill(pid, 0);
      } catch {
        continue;
      }

      try {
        process.kill(pid, 'SIGKILL');
      } catch (error) {
        log.warn(`Failed to SIGKILL process ${pid}:`, error);
      }
    }
  }
}
