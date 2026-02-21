import { Notification, BrowserWindow } from 'electron';
import log from 'electron-log';

export interface NotificationPayload {
  title: string;
  body: string;
  icon?: string;
  data?: {
    sessionKey?: string;
    taskId?: string;
    type: 'message' | 'task_complete' | 'scheduled_task';
  };
}

export class NotificationManager {
  private mainWindow: BrowserWindow;

  constructor(mainWindow: BrowserWindow) {
    this.mainWindow = mainWindow;
  }

  showNotification(payload: NotificationPayload): void {
    if (!Notification.isSupported()) {
      log.warn('Notifications not supported on this platform');
      return;
    }

    const notification = new Notification({
      title: payload.title,
      body: payload.body,
      icon: payload.icon || undefined,
      silent: false
    });

    notification.on('click', () => {
      this.handleNotificationClick(payload);
    });

    notification.show();
    log.info(`Notification shown: ${payload.title}`);
  }

  private handleNotificationClick(payload: NotificationPayload): void {
    // Bring window to front
    if (this.mainWindow.isMinimized()) {
      this.mainWindow.restore();
    }
    this.mainWindow.show();
    this.mainWindow.focus();

    // Send event to renderer
    this.mainWindow.webContents.send('notification:clicked', payload.data);
  }

  // Check and request notification permissions
  async requestPermission(): Promise<boolean> {
    if (process.platform === 'darwin') {
      // macOS uses native notification permissions
      return Notification.isSupported();
    }
    return true;
  }
}
