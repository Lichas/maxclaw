import { ipcMain, BrowserWindow, dialog, shell, app } from 'electron';
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
  name: 'Nanobot',
  path: app.getPath('exe')
});

let handlersRegistered = false;
let currentMainWindow: BrowserWindow | null = null;
let gatewayStatusTimer: NodeJS.Timeout | null = null;

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
