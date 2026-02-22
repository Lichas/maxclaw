import { Tray, Menu, BrowserWindow, nativeImage, app } from 'electron';
import path from 'path';
import log from 'electron-log';
import fs from 'fs';

let tray: Tray | null = null;

export function initializeTray(mainWindow: BrowserWindow): void {
  try {
    const trayIcon = createTrayIcon();
    if (!trayIcon) {
      log.warn('Could not create tray icon, skipping tray initialization');
      return;
    }

    tray = new Tray(trayIcon);
    tray.setToolTip('Maxclaw AI Assistant');

    updateTrayMenu(mainWindow);

    tray.on('click', () => {
      showWindow(mainWindow);
    });

    tray.on('double-click', () => {
      showWindow(mainWindow);
    });

    log.info('System tray initialized successfully');
  } catch (error) {
    log.error('Failed to initialize tray:', error);
  }
}

function createTrayIcon(): nativeImage | null {
  const iconPaths = [
    path.join(__dirname, '../../assets/tray-icon.png'),
    path.join(__dirname, '../../assets/icon.png'),
    path.join(process.resourcesPath, 'assets/tray-icon.png'),
    path.join(process.resourcesPath, 'assets/icon.png'),
  ];

  for (const iconPath of iconPaths) {
    try {
      if (fs.existsSync(iconPath)) {
        const icon = nativeImage.createFromPath(iconPath);
        const size = process.platform === 'darwin' ? 16 : 16;
        let trayIcon = icon.resize({ width: size, height: size });

        if (process.platform === 'darwin') {
          trayIcon.setTemplateImage(true);
        }

        log.info(`Created tray icon from: ${iconPath}`);
        return trayIcon;
      }
    } catch (err) {
      log.debug(`Failed to create icon from ${iconPath}:`, err);
    }
  }

  log.error('No valid tray icon found in any location');
  return null;
}

function updateTrayMenu(mainWindow: BrowserWindow): void {
  const contextMenu = Menu.buildFromTemplate([
    {
      label: 'Open Maxclaw',
      click: () => showWindow(mainWindow)
    },
    { type: 'separator' },
    {
      label: 'New Chat',
      click: () => {
        showWindow(mainWindow);
        mainWindow.webContents.send('tray:new-chat');
      }
    },
    {
      label: 'Settings',
      click: () => {
        showWindow(mainWindow);
        mainWindow.webContents.send('tray:open-settings');
      }
    },
    { type: 'separator' },
    {
      label: 'Gateway Status',
      submenu: [
        {
          label: 'Restart Gateway',
          click: () => {
            mainWindow.webContents.send('tray:restart-gateway');
          }
        },
        {
          label: 'View Logs',
          click: () => {
            showWindow(mainWindow);
            mainWindow.webContents.send('tray:view-logs');
          }
        }
      ]
    },
    { type: 'separator' },
    {
      label: 'Quit',
      click: () => {
        app.quit();
      }
    }
  ]);

  tray?.setContextMenu(contextMenu);
}

function showWindow(window: BrowserWindow): void {
  if (window.isMinimized()) {
    window.restore();
  }
  window.show();
  window.focus();
}

export function destroyTray(): void {
  if (tray) {
    tray.destroy();
    tray = null;
  }
}

export function updateTrayTooltip(status: string): void {
  if (tray) {
    tray.setToolTip(`Maxclaw AI Assistant\n${status}`);
  }
}
