import { BrowserWindow, screen, ipcMain } from 'electron';
import path from 'path';
import Store from 'electron-store';

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

  const window = new BrowserWindow({
    width,
    height,
    x: windowX,
    y: windowY,
    minWidth: 900,
    minHeight: 600,
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
  ipcMain.handle('window:minimize', () => window.minimize());
  ipcMain.handle('window:maximize', () => {
    if (window.isMaximized()) {
      window.unmaximize();
      return false;
    } else {
      window.maximize();
      return true;
    }
  });
  ipcMain.handle('window:close', () => window.close());
  ipcMain.handle('window:isMaximized', () => window.isMaximized());

  return window;
}

export function getWindowState(): WindowState {
  return store.get();
}
