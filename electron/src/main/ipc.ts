import { ipcMain, BrowserWindow, dialog, shell, app } from 'electron';
import { spawn as spawnPty, type IPty } from 'node-pty';
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

let handlersRegistered = false;
let currentMainWindow: BrowserWindow | null = null;
let gatewayStatusTimer: NodeJS.Timeout | null = null;
let terminalProcess: IPty | null = null;

function sendTerminalData(chunk: string): void {
  if (currentMainWindow && !currentMainWindow.isDestroyed()) {
    currentMainWindow.webContents.send('terminal:data', chunk);
  }
}

function sendTerminalExit(code: number | null, signal: string | null): void {
  if (currentMainWindow && !currentMainWindow.isDestroyed()) {
    currentMainWindow.webContents.send('terminal:exit', { code, signal });
  }
}

function resolveShell(): { command: string; args: string[] } {
  if (process.platform === 'win32') {
    return { command: 'powershell.exe', args: ['-NoLogo'] };
  }

  const shellPath = process.env.SHELL || '/bin/zsh';
  return { command: shellPath, args: ['-i'] };
}

export function createIPCHandlers(
  mainWindow: BrowserWindow,
  gatewayManager: GatewayManager,
  notificationManager?: NotificationManager
): void {
  currentMainWindow = mainWindow;

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
  ipcMain.handle('terminal:start', async (_, options?: { cols?: number; rows?: number }) => {
    if (terminalProcess) {
      if (options?.cols && options?.rows) {
        terminalProcess.resize(options.cols, options.rows);
      }
      return { success: true, alreadyRunning: true };
    }

    const shell = resolveShell();
    const cols = options?.cols && options.cols > 0 ? options.cols : 120;
    const rows = options?.rows && options.rows > 0 ? options.rows : 28;

    try {
      terminalProcess = spawnPty(shell.command, shell.args, {
        name: 'xterm-256color',
        cols,
        rows,
        cwd: process.cwd(),
        env: {
          ...process.env,
          TERM: process.env.TERM || 'xterm-256color'
        } as Record<string, string>
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      log.error('Failed to start terminal shell:', error);
      return { success: false, error: message };
    }

    terminalProcess.onData((data: string) => {
      sendTerminalData(data);
    });

    terminalProcess.onExit(({ exitCode, signal }) => {
      sendTerminalExit(exitCode, signal !== undefined && signal !== null ? String(signal) : null);
      terminalProcess = null;
    });

    sendTerminalData(`\r\n[terminal] started with ${shell.command}\r\n`);
    return { success: true, shell: shell.command };
  });

  ipcMain.handle('terminal:input', async (_, value: string) => {
    if (!terminalProcess) {
      return { success: false, error: 'terminal not running' };
    }

    terminalProcess.write(value);
    return { success: true };
  });

  ipcMain.handle('terminal:resize', async (_, cols: number, rows: number) => {
    if (!terminalProcess) {
      return { success: false, error: 'terminal not running' };
    }
    if (cols > 0 && rows > 0) {
      terminalProcess.resize(cols, rows);
      return { success: true };
    }
    return { success: false, error: 'invalid cols/rows' };
  });

  ipcMain.handle('terminal:stop', async () => {
    if (terminalProcess) {
      terminalProcess.kill();
      terminalProcess = null;
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
