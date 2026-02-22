import { BrowserWindow, screen, ipcMain, app, nativeImage } from 'electron';
import path from 'path';
import Store from 'electron-store';
import fs from 'fs';
import log from 'electron-log';

interface WindowState {
  width: number;
  height: number;
  x: number;
  y: number;
  maximized: boolean;
}

const store = new Store<WindowState>({
  name: 'window-state',
  defaults: {
    width: 1400,
    height: 900,
    x: 0,
    y: 0,
    maximized: false
  }
});

function getIconCandidates(): string[] {
  const appPath = app.getAppPath();
  const cwd = process.cwd();
  return [
    path.join(appPath, 'assets/icon.png'),
    path.join(appPath, 'assets/icon.icns'),
    path.join(appPath, 'dist/main/icon.png'),
    path.join(appPath, 'public/icon.png'),
    path.join(appPath, '../icon.png'),
    path.join(cwd, 'assets/icon.png'),
    path.join(cwd, 'public/icon.png'),
    path.join(cwd, 'icon.png'),
    path.join(process.resourcesPath, 'assets/icon.png'),
    path.join(process.resourcesPath, 'assets/icon.icns')
  ];
}

function resolveIconPath(): string | null {
  for (const candidate of getIconCandidates()) {
    if (!fs.existsSync(candidate)) {
      continue;
    }
    const icon = nativeImage.createFromPath(candidate);
    if (!icon.isEmpty()) {
      return candidate;
    }
  }
  return null;
}

export function applyMacDockIcon(): void {
  if (process.platform !== 'darwin' || !app.dock) {
    return;
  }

  const iconPath = resolveIconPath();
  if (!iconPath) {
    log.warn('[icon] Dock icon not applied: no valid icon file found');
    return;
  }

  app.dock.setIcon(nativeImage.createFromPath(iconPath));
  log.info('[icon] Dock icon applied:', iconPath);
}

export function createWindow(): BrowserWindow {
  const { width, height, x, y, maximized } = store.get();

  // Ensure window is on visible screen
  const displays = screen.getAllDisplays();
  const isVisible = displays.some(display => {
    const { bounds } = display;
    return x >= bounds.x && x < bounds.x + bounds.width &&
           y >= bounds.y && y < bounds.y + bounds.height;
  });

  const windowX = isVisible ? x : undefined;
  const windowY = isVisible ? y : undefined;

  // Re-apply Dock icon when creating window, keeping dev mode behavior stable.
  applyMacDockIcon();

  const window = new BrowserWindow({
    width,
    height,
    x: windowX,
    y: windowY,
    minWidth: 900,
    minHeight: 600,
    title: 'nanobot-go',
    titleBarStyle: process.platform === 'darwin' ? 'hiddenInset' : 'default',
    trafficLightPosition: { x: 16, y: 16 },
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true
    },
    show: false // Show after ready-to-show
  });

  if (maximized) {
    window.maximize();
  }

  // Save window state on close
  window.on('close', () => {
    const bounds = window.getBounds();
    store.set({
      width: bounds.width,
      height: bounds.height,
      x: bounds.x,
      y: bounds.y,
      maximized: window.isMaximized()
    });
  });

  // Show window when ready
  window.once('ready-to-show', () => {
    window.show();
    if (maximized) {
      window.maximize();
    }
  });

  // Handle window control IPC
  ipcMain.removeHandler('window:minimize');
  ipcMain.handle('window:minimize', () => window.minimize());
  ipcMain.removeHandler('window:maximize');
  ipcMain.handle('window:maximize', () => {
    if (window.isMaximized()) {
      window.unmaximize();
      return false;
    } else {
      window.maximize();
      return true;
    }
  });
  ipcMain.removeHandler('window:close');
  ipcMain.handle('window:close', () => window.close());
  ipcMain.removeHandler('window:isMaximized');
  ipcMain.handle('window:isMaximized', () => window.isMaximized());

  return window;
}

export function getWindowState(): WindowState {
  return store.get();
}
