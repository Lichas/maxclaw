import { Tray, Menu, BrowserWindow, nativeImage, app } from 'electron';
import path from 'path';
import log from 'electron-log';

let tray: Tray | null = null;

export function initializeTray(mainWindow: BrowserWindow): void {
  try {
    // Create tray icon (use nativeImage for proper scaling)
    const iconPath = path.join(__dirname, '../../assets/tray-icon.png');
    let trayIcon: nativeImage;

    try {
      const icon = nativeImage.createFromPath(iconPath);
      // Resize for appropriate platforms
      trayIcon = icon.resize({ width: 16, height: 16 });
      trayIcon.setTemplateImage(true); // macOS template image
    } catch {
      // Fallback if icon doesn't exist
      trayIcon = nativeImage.createEmpty();
    }

    tray = new Tray(trayIcon);
    tray.setToolTip('Nanobot AI Assistant');

    updateTrayMenu(mainWindow);

    // Handle tray click (show window)
    tray.on('click', () => {
      showWindow(mainWindow);
    });

    // Handle double-click
    tray.on('double-click', () => {
      showWindow(mainWindow);
    });

    log.info('System tray initialized');
  } catch (error) {
    log.error('Failed to initialize tray:', error);
  }
}

function updateTrayMenu(mainWindow: BrowserWindow): void {
  const contextMenu = Menu.buildFromTemplate([
    {
      label: 'Open Nanobot',
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
    tray.setToolTip(`Nanobot AI Assistant\n${status}`);
  }
}
