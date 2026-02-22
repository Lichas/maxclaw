import { app, BrowserWindow } from 'electron';
import path from 'path';
import { createWindow, applyMacDockIcon } from './window';
import { initializeTray } from './tray';
import { GatewayManager } from './gateway';
import { createIPCHandlers } from './ipc';
import { NotificationManager } from './notifications';
import log from 'electron-log';

log.initialize();

let mainWindow: BrowserWindow | null = null;
let gatewayManager: GatewayManager | null = null;
let notificationManager: NotificationManager | null = null;
let openingMainWindow: Promise<void> | null = null;

const isDev = !app.isPackaged;
const loopbackNoProxyHosts = ['localhost', '127.0.0.1', '::1'];

function ensureLoopbackNoProxy(): void {
  const currentRaw = process.env.NO_PROXY || process.env.no_proxy || '';
  const parts = currentRaw
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
  const lowerSet = new Set(parts.map((item) => item.toLowerCase()));

  let changed = false;
  for (const host of loopbackNoProxyHosts) {
    if (!lowerSet.has(host)) {
      parts.push(host);
      lowerSet.add(host);
      changed = true;
    }
  }

  if (!changed) {
    return;
  }

  const merged = parts.join(',');
  process.env.NO_PROXY = merged;
  process.env.no_proxy = merged;
  log.info('Updated NO_PROXY for local gateway access:', merged);
}

async function openMainWindow(): Promise<void> {
  if (mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.show();
    return;
  }

  mainWindow = createWindow();
  mainWindow.on('closed', () => {
    mainWindow = null;
  });

  const rendererDevURL = process.env.ELECTRON_RENDERER_URL || process.env.VITE_DEV_SERVER_URL;
  if (rendererDevURL) {
    await mainWindow.loadURL(rendererDevURL);
  } else {
    await mainWindow.loadFile(path.join(__dirname, '../renderer/index.html'));
  }

  if (isDev && !mainWindow.webContents.isDevToolsOpened()) {
    mainWindow.webContents.openDevTools({ mode: 'detach' });
  }

  if (gatewayManager) {
    notificationManager = new NotificationManager(mainWindow);
    createIPCHandlers(mainWindow, gatewayManager, notificationManager);
  }
}

async function ensureMainWindow(): Promise<void> {
  if (mainWindow && !mainWindow.isDestroyed()) {
    return;
  }
  if (openingMainWindow) {
    return openingMainWindow;
  }

  openingMainWindow = openMainWindow().finally(() => {
    openingMainWindow = null;
  });
  return openingMainWindow;
}

async function initializeApp(): Promise<void> {
  log.info('Initializing Maxclaw Desktop App');

  // Initialize Gateway Manager
  gatewayManager = new GatewayManager();

  // Start Gateway before creating window
  try {
    await gatewayManager.startFresh();
    log.info('Gateway started successfully');
  } catch (error) {
    log.error('Failed to start Gateway:', error);
    // Continue anyway - will show error in UI
  }

  await ensureMainWindow();

  // Initialize tray
  initializeTray(mainWindow);
}

// App event handlers
app.whenReady().then(() => {
  ensureLoopbackNoProxy();
  applyMacDockIcon();
  void initializeApp().catch((error) => {
    log.error('Failed to initialize app:', error);
  });
});

app.on('window-all-closed', () => {
  // Keep Gateway running in background on macOS
  if (process.platform !== 'darwin') {
    gatewayManager?.stop();
    app.quit();
  }
});

app.on('activate', () => {
  if (mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.show();
    return;
  }

  void ensureMainWindow().catch((error) => {
    log.error('Failed to reopen main window:', error);
  });
});

app.on('before-quit', async () => {
  await gatewayManager?.stop();
});

// Security: Prevent new window creation
app.on('web-contents-created', (_, contents) => {
  contents.on('new-window', (event, url) => {
    event.preventDefault();
    require('electron').shell.openExternal(url);
  });
});
