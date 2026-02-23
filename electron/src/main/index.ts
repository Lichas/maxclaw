import { app, BrowserWindow } from 'electron';
import path from 'path';
import { createWindow, applyMacDockIcon } from './window';
import { initializeTray } from './tray';
import { GatewayManager } from './gateway';
import { createIPCHandlers } from './ipc';
import { NotificationManager } from './notifications';
import { ShortcutManager, DEFAULT_SHORTCUTS } from './shortcuts';
import log from 'electron-log';
import Store from 'electron-store';
import { autoUpdater } from 'electron-updater';
import { initializeWindowsIntegration } from './windows-integration';

// Avoid crashing on detached stdio (EPIPE) in desktop runtime.
const swallowBrokenPipe = (error: NodeJS.ErrnoException) => {
  if (error?.code === 'EPIPE') {
    return;
  }
};

process.stdout?.on('error', swallowBrokenPipe);
process.stderr?.on('error', swallowBrokenPipe);

// Keep file logging, but disable console transport to avoid writing to broken pipes.
log.transports.console.level = false;
log.initialize();

let mainWindow: BrowserWindow | null = null;
let gatewayManager: GatewayManager | null = null;
let notificationManager: NotificationManager | null = null;
let shortcutManager: ShortcutManager | null = null;
let openingMainWindow: Promise<void> | null = null;
const store = new Store();

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
    createIPCHandlers(mainWindow, gatewayManager, notificationManager, shortcutManager);
  }

  // Initialize shortcut manager
  if (!shortcutManager) {
    shortcutManager = new ShortcutManager(mainWindow);
    const shortcutsConfig = store.get('shortcuts') as Partial<typeof DEFAULT_SHORTCUTS> || {};
    shortcutManager.register(shortcutsConfig);
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

function setupAutoUpdater(): void {
  // Skip in development
  if (isDev) {
    log.info('Auto-updater disabled in development');
    return;
  }

  autoUpdater.logger = log;
  autoUpdater.autoDownload = false; // Manual download

  autoUpdater.on('checking-for-update', () => {
    log.info('Checking for update...');
  });

  autoUpdater.on('update-available', (info) => {
    log.info('Update available:', info);
    mainWindow?.webContents.send('update:available', info);
  });

  autoUpdater.on('update-not-available', () => {
    log.info('Update not available');
  });

  autoUpdater.on('error', (err) => {
    log.error('Update error:', err);
  });

  autoUpdater.on('download-progress', (progress) => {
    mainWindow?.webContents.send('update:progress', progress);
  });

  autoUpdater.on('update-downloaded', (info) => {
    log.info('Update downloaded:', info);
    mainWindow?.webContents.send('update:downloaded', info);
  });

  // Check on startup (after 5 seconds)
  setTimeout(() => {
    autoUpdater.checkForUpdates().catch(err => {
      log.error('Failed to check for updates:', err);
    });
  }, 5000);

  // Check every hour
  setInterval(() => {
    autoUpdater.checkForUpdates().catch(err => {
      log.error('Failed to check for updates:', err);
    });
  }, 60 * 60 * 1000);
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

  // Initialize Windows-specific integrations
  if (mainWindow) {
    initializeWindowsIntegration(mainWindow);
  }

  // Setup auto-updater
  setupAutoUpdater();
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

// Cleanup on quit
app.on('will-quit', () => {
  shortcutManager?.unregisterAll();
});

// Security: Prevent new window creation
app.on('web-contents-created', (_, contents) => {
  contents.on('new-window', (event, url) => {
    event.preventDefault();
    require('electron').shell.openExternal(url);
  });
});
