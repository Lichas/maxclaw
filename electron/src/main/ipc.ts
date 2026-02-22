import { ipcMain, BrowserWindow, dialog, shell, app } from 'electron';
import { spawn as spawnPty, type IPty } from 'node-pty';
import { createRequire } from 'node:module';
import fs from 'fs';
import path from 'path';
import Store from 'electron-store';
import AutoLaunch from 'auto-launch';
import log from 'electron-log';
import { GatewayManager } from './gateway';
import { NotificationManager } from './notifications';

interface AppConfig {
  theme: 'light' | 'dark' | 'system';
  language: 'zh' | 'en';
  autoLaunch: boolean;
  minimizeToTray: boolean;
  shortcuts: Record<string, string>;
}

const configStore = new Store<AppConfig>({
  name: 'app-config',
  defaults: {
    theme: 'system',
    language: 'zh',
    autoLaunch: false,
    minimizeToTray: true,
    shortcuts: {
      toggleWindow: 'CommandOrControl+Shift+N',
      newChat: 'CommandOrControl+N'
    }
  }
});

const autoLauncher = new AutoLaunch({
  name: 'Maxclaw',
  path: app.getPath('exe')
});
const nodeRequire = createRequire(import.meta.url);

let handlersRegistered = false;
let currentMainWindow: BrowserWindow | null = null;
let gatewayStatusTimer: NodeJS.Timeout | null = null;
const terminalProcesses = new Map<string, IPty>();

function sendTerminalData(sessionKey: string, chunk: string): void {
  if (currentMainWindow && !currentMainWindow.isDestroyed()) {
    currentMainWindow.webContents.send('terminal:data', { sessionKey, chunk });
  }
}

function sendTerminalExit(sessionKey: string, code: number | null, signal: string | null): void {
  if (currentMainWindow && !currentMainWindow.isDestroyed()) {
    currentMainWindow.webContents.send('terminal:exit', { sessionKey, code, signal });
  }
}

function parseShellExecutable(raw: string | undefined): string | undefined {
  if (!raw) {
    return undefined;
  }

  const value = raw.trim();
  if (value === '') {
    return undefined;
  }

  const [command] = value.split(/\s+/);
  return command || undefined;
}

function resolveShellCandidates(): Array<{ command: string; args: string[] }> {
  if (process.platform === 'win32') {
    return [
      { command: 'powershell.exe', args: ['-NoLogo'] },
      { command: 'cmd.exe', args: [] }
    ];
  }

  const candidates: Array<{ command: string; args: string[] }> = [];
  const appendCandidate = (command: string | undefined) => {
    if (!command) {
      return;
    }
    const exists = command.startsWith('/') ? fs.existsSync(command) : true;
    if (!exists) {
      return;
    }
    if (!candidates.some((candidate) => candidate.command === command)) {
      candidates.push({ command, args: ['-i'] });
    }
  };

  appendCandidate(parseShellExecutable(process.env.SHELL));
  appendCandidate('/bin/zsh');
  appendCandidate('/bin/bash');
  appendCandidate('/bin/sh');

  return candidates;
}

function ensurePtySpawnHelperExecutable(): void {
  if (process.platform === 'win32') {
    return;
  }

  const helperCandidates = new Set<string>();

  try {
    const packageJson = nodeRequire.resolve('node-pty/package.json');
    const packageDir = path.dirname(packageJson);
    helperCandidates.add(path.join(packageDir, 'prebuilds', `${process.platform}-${process.arch}`, 'spawn-helper'));
  } catch (error) {
    log.warn('Failed to ensure node-pty spawn-helper executable:', error);
  }

  helperCandidates.add(
    path.join(process.cwd(), 'node_modules', 'node-pty', 'prebuilds', `${process.platform}-${process.arch}`, 'spawn-helper')
  );
  helperCandidates.add(
    path.join(
      process.resourcesPath,
      'app.asar.unpacked',
      'node_modules',
      'node-pty',
      'prebuilds',
      `${process.platform}-${process.arch}`,
      'spawn-helper'
    )
  );

  for (const helperPath of helperCandidates) {
    try {
      if (!fs.existsSync(helperPath)) {
        continue;
      }

      const stats = fs.statSync(helperPath);
      if ((stats.mode & 0o111) === 0) {
        fs.chmodSync(helperPath, 0o755);
        log.info('Updated node-pty spawn-helper permission:', helperPath);
      }
    } catch (error) {
      log.warn('Failed to update node-pty spawn-helper permission:', helperPath, error);
    }
  }
}

function buildPtyEnv(): Record<string, string> {
  const env: Record<string, string> = {};
  for (const [key, value] of Object.entries(process.env)) {
    if (typeof value === 'string') {
      env[key] = value;
    }
  }

  if (!env.TERM) {
    env.TERM = 'xterm-256color';
  }
  if (!env.LANG) {
    env.LANG = 'en_US.UTF-8';
  }

  return env;
}

function resolveTerminalCwd(): string {
  const home = process.env.HOME || app.getPath('home');
  if (home && fs.existsSync(home)) {
    return home;
  }
  return process.cwd();
}

function normalizeSessionKey(value: unknown): string {
  if (typeof value !== 'string') {
    return 'default';
  }
  const normalized = value.trim();
  return normalized === '' ? 'default' : normalized;
}

export function createIPCHandlers(
  mainWindow: BrowserWindow,
  gatewayManager: GatewayManager,
  notificationManager?: NotificationManager
): void {
  currentMainWindow = mainWindow;
  mainWindow.once('closed', () => {
    if (currentMainWindow === mainWindow) {
      currentMainWindow = null;
    }
    for (const [, terminalProcess] of terminalProcesses) {
      try {
        terminalProcess.kill();
      } catch (error) {
        log.warn('Failed to stop terminal process on window close:', error);
      }
    }
    terminalProcesses.clear();
  });

  if (handlersRegistered) {
    return;
  }
  handlersRegistered = true;

  // Gateway IPC
  ipcMain.handle('gateway:getStatus', () => gatewayManager.getStatus());

  ipcMain.handle('gateway:restart', async () => {
    try {
      await gatewayManager.restart();
      return { success: true };
    } catch (error) {
      log.error('Failed to restart gateway:', error);
      return { success: false, error: String(error) };
    }
  });

  // Config IPC
  ipcMain.handle('config:get', () => configStore.get());

  ipcMain.handle('config:set', (_, config: Partial<AppConfig>) => {
    const current = configStore.get();
    const updated = { ...current, ...config };
    configStore.set(updated);

    // Handle auto-launch
    if (config.autoLaunch !== undefined) {
      if (config.autoLaunch) {
        autoLauncher.enable();
      } else {
        autoLauncher.disable();
      }
    }

    // Notify renderer of config change
    if (currentMainWindow && !currentMainWindow.isDestroyed()) {
      currentMainWindow.webContents.send('config:change', updated);
    }

    return updated;
  });

  // System IPC
  ipcMain.handle('notification:show', (_, payload) => {
    if (notificationManager) {
      notificationManager.showNotification(payload);
    }
  });

  ipcMain.handle('notification:request-permission', async () => {
    if (notificationManager) {
      return await notificationManager.requestPermission();
    }
    return false;
  });

  ipcMain.handle('system:openExternal', (_, url: string) => {
    shell.openExternal(url);
  });

  ipcMain.handle('system:selectFolder', async () => {
    const targetWindow = currentMainWindow && !currentMainWindow.isDestroyed() ? currentMainWindow : undefined;
    const result = await dialog.showOpenDialog(targetWindow, {
      properties: ['openDirectory'],
      title: 'Select Folder'
    });

    if (result.canceled || result.filePaths.length === 0) {
      return null;
    }

    return result.filePaths[0];
  });

  ipcMain.handle('system:selectFile', async (_, filters) => {
    const targetWindow = currentMainWindow && !currentMainWindow.isDestroyed() ? currentMainWindow : undefined;
    const result = await dialog.showOpenDialog(targetWindow, {
      properties: ['openFile'],
      filters: filters || [{ name: 'All Files', extensions: ['*'] }],
      title: 'Select File'
    });

    if (result.canceled || result.filePaths.length === 0) {
      return null;
    }

    return result.filePaths[0];
  });

  // Terminal IPC
  ipcMain.handle(
    'terminal:start',
    async (
      _,
      sessionKeyOrOptions?: string | { cols?: number; rows?: number },
      maybeOptions?: { cols?: number; rows?: number }
    ) => {
      const key = normalizeSessionKey(typeof sessionKeyOrOptions === 'string' ? sessionKeyOrOptions : undefined);
      const options = typeof sessionKeyOrOptions === 'string' ? maybeOptions : sessionKeyOrOptions;
      const existing = terminalProcesses.get(key);
      if (existing) {
        if (options?.cols && options?.rows) {
          existing.resize(options.cols, options.rows);
        }
        return { success: true, alreadyRunning: true };
      }

      ensurePtySpawnHelperExecutable();

      const shellCandidates = resolveShellCandidates();
      const cols = options?.cols && options.cols > 0 ? options.cols : 120;
      const rows = options?.rows && options.rows > 0 ? options.rows : 28;
      const cwd = resolveTerminalCwd();
      const env = buildPtyEnv();
      let usedShell = '';
      let lastError = '';
      let ptyProcess: IPty | null = null;

      for (const shellCandidate of shellCandidates) {
        try {
          ptyProcess = spawnPty(shellCandidate.command, shellCandidate.args, {
            name: 'xterm-256color',
            cols,
            rows,
            cwd,
            env
          });
          usedShell = shellCandidate.command;
          break;
        } catch (error) {
          const message = error instanceof Error ? error.message : String(error);
          lastError = message;
          log.warn(`Failed to start terminal shell candidate ${shellCandidate.command}:`, message);
        }
      }

      if (!ptyProcess) {
        const fallbackError = lastError || 'no available shell candidates';
        log.error('Failed to start terminal shell:', fallbackError);
        return { success: false, error: fallbackError };
      }

      terminalProcesses.set(key, ptyProcess);

      ptyProcess.onData((data: string) => {
        sendTerminalData(key, data);
      });

      ptyProcess.onExit(({ exitCode, signal }) => {
        sendTerminalExit(key, exitCode, signal !== undefined && signal !== null ? String(signal) : null);
        terminalProcesses.delete(key);
      });

      sendTerminalData(key, `\r\n[terminal] started with ${usedShell}\r\n`);
      return { success: true, shell: usedShell };
    }
  );

  ipcMain.handle('terminal:input', async (_, sessionKeyOrValue: string, maybeValue?: string) => {
    const key = maybeValue === undefined ? 'default' : normalizeSessionKey(sessionKeyOrValue);
    const value = maybeValue === undefined ? sessionKeyOrValue : maybeValue;
    const terminalProcess = terminalProcesses.get(key);
    if (!terminalProcess) {
      return { success: false, error: 'terminal not running' };
    }
    if (typeof value !== 'string') {
      return { success: false, error: 'invalid terminal input' };
    }

    terminalProcess.write(value);
    return { success: true };
  });

  ipcMain.handle('terminal:resize', async (_, sessionKeyOrCols: string | number, maybeCols: number, maybeRows?: number) => {
    const key = typeof sessionKeyOrCols === 'string' ? normalizeSessionKey(sessionKeyOrCols) : 'default';
    const cols = typeof sessionKeyOrCols === 'string' ? maybeCols : sessionKeyOrCols;
    const rows = typeof sessionKeyOrCols === 'string' ? maybeRows : maybeCols;
    const terminalProcess = terminalProcesses.get(key);
    if (!terminalProcess) {
      return { success: false, error: 'terminal not running' };
    }
    if (typeof cols === 'number' && typeof rows === 'number' && cols > 0 && rows > 0) {
      terminalProcess.resize(cols, rows);
      return { success: true };
    }
    return { success: false, error: 'invalid cols/rows' };
  });

  ipcMain.handle('terminal:stop', async (_, sessionKey?: string) => {
    const key = normalizeSessionKey(sessionKey);
    const terminalProcess = terminalProcesses.get(key);
    if (terminalProcess) {
      terminalProcess.kill();
      terminalProcesses.delete(key);
    }
    return { success: true };
  });

  // Gateway status polling - notify renderer
  gatewayStatusTimer = setInterval(() => {
    const status = gatewayManager.getStatus();
    if (currentMainWindow && !currentMainWindow.isDestroyed()) {
      currentMainWindow.webContents.send('gateway:status-change', status);
    }
  }, 5000);

  // Notification polling from Gateway
  if (notificationManager) {
    const notificationTimer = setInterval(async () => {
      try {
        const response = await fetch('http://localhost:18890/api/notifications/pending');
        if (!response.ok) return;

        const notifications = await response.json();
        for (const notif of notifications) {
          notificationManager.showNotification({
            title: notif.title,
            body: notif.body,
            data: notif.data
          });

          // Mark as delivered
          await fetch(`http://localhost:18890/api/notifications/${notif.id}/delivered`, {
            method: 'POST'
          });
        }
      } catch (error) {
        // Gateway might not support notifications yet
        log.debug('Notification check failed:', error);
      }
    }, 5000);

    notificationTimer.unref();
  }

  // Keep Node process from being blocked by this timer on shutdown.
  gatewayStatusTimer.unref();
}
