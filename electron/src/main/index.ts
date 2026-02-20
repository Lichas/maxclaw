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

const isDev = process.env.NODE_ENV === 'development';

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

  // Create main window
  mainWindow = createWindow();

  // Load content
  if (isDev) {
    await mainWindow.loadURL('http://localhost:5173');
    mainWindow.webContents.openDevTools();
  } else {
    await mainWindow.loadFile(path.join(__dirname, '../renderer/index.html'));
  }

  // Initialize tray
  initializeTray(mainWindow);

  // Setup IPC handlers
  createIPCHandlers(mainWindow, gatewayManager);

  // Handle window closed
  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

// App event handlers
app.whenReady().then(initializeApp);

app.on('window-all-closed', () => {
  // Keep Gateway running in background on macOS
  if (process.platform !== 'darwin') {
    gatewayManager?.stop();
    app.quit();
  }
});

app.on('activate', () => {
  if (mainWindow === null) {
    mainWindow = createWindow();
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
