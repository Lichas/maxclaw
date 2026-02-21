import { app, BrowserWindow } from 'electron';
import path from 'path';
import { createWindow } from './window';
import { initializeTray } from './tray';
import { GatewayManager } from './gateway';
import { createIPCHandlers } from './ipc';
import log from 'electron-log';

log.initialize();

let mainWindow: BrowserWindow | null = null;
let gatewayManager: GatewayManager | null = null;

const isDev = !app.isPackaged;

async function openMainWindow(): Promise<void> {
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

  if (isDev) {
    mainWindow.webContents.openDevTools({ mode: 'detach' });
  }

  if (gatewayManager) {
    createIPCHandlers(mainWindow, gatewayManager);
  }
}

async function initializeApp(): Promise<void> {
  log.info('Initializing Nanobot Desktop App');

  // Initialize Gateway Manager
  gatewayManager = new GatewayManager();

  // Start Gateway before creating window
  try {
    await gatewayManager.start();
    log.info('Gateway started successfully');
  } catch (error) {
    log.error('Failed to start Gateway:', error);
    // Continue anyway - will show error in UI
  }

  await openMainWindow();

  // Initialize tray
  initializeTray(mainWindow);
}

// App event handlers
app.whenReady().then(() => {
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
  if (mainWindow === null) {
    void openMainWindow().catch((error) => {
      log.error('Failed to reopen main window:', error);
    });
  } else {
    mainWindow.show();
  }
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
