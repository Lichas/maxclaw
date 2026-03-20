import { contextBridge, ipcRenderer } from 'electron';

// Gateway API types
interface GatewayStatus {
  state: 'running' | 'stopped' | 'error' | 'starting';
  port: number;
  error?: string;
}

interface AppConfig {
  theme: 'light' | 'dark' | 'system';
  language: 'zh' | 'en';
  renderThinkTags: boolean;
  autoLaunch: boolean;
  minimizeToTray: boolean;
  shortcuts: Record<string, string>;
}

// Expose APIs to renderer process
const electronAPI = {
  // Window controls
  window: {
    minimize: () => ipcRenderer.invoke('window:minimize'),
    maximize: () => ipcRenderer.invoke('window:maximize'),
    close: () => ipcRenderer.invoke('window:close'),
    isMaximized: () => ipcRenderer.invoke('window:isMaximized')
  },

  // Gateway management
  gateway: {
    getStatus: () => ipcRenderer.invoke('gateway:getStatus'),
    restart: () => ipcRenderer.invoke('gateway:restart'),
    onStatusChange: (callback: (status: GatewayStatus) => void) => {
      const listener = (_: unknown, status: GatewayStatus) => callback(status);
      ipcRenderer.on('gateway:status-change', listener);
      return () => ipcRenderer.removeListener('gateway:status-change', listener);
    }
  },

  // App configuration (local settings)
  config: {
    get: () => ipcRenderer.invoke('config:get'),
    set: (config: Partial<AppConfig>) => ipcRenderer.invoke('config:set', config),
    onChange: (callback: (config: AppConfig) => void) => {
      const listener = (_: unknown, config: AppConfig) => callback(config);
      ipcRenderer.on('config:change', listener);
      return () => ipcRenderer.removeListener('config:change', listener);
    }
  },

  // System features
  system: {
    showNotification: (title: string, body: string) =>
      ipcRenderer.invoke('notification:show', { title, body }),
    requestNotificationPermission: () =>
      ipcRenderer.invoke('notification:request-permission'),
    openExternal: (url: string) => ipcRenderer.invoke('system:openExternal', url),
    openPath: (targetPath: string, options?: { workspace?: string; sessionKey?: string }) =>
      ipcRenderer.invoke('system:openPath', targetPath, options),
    openInFolder: (targetPath: string, options?: { workspace?: string; sessionKey?: string }) =>
      ipcRenderer.invoke('system:openInFolder', targetPath, options),
    previewFile: (targetPath: string, options?: { workspace?: string; sessionKey?: string }) =>
      ipcRenderer.invoke('system:previewFile', targetPath, options),
    fileExists: (targetPath: string, options?: { workspace?: string; sessionKey?: string }) =>
      ipcRenderer.invoke('system:fileExists', targetPath, options),
    listDirectory: (dirPath: string, options?: { workspace?: string; sessionKey?: string }) =>
      ipcRenderer.invoke('system:listDirectory', dirPath, options),
    selectFolder: () => ipcRenderer.invoke('system:selectFolder'),
    selectFile: (filters?: Array<{ name: string; extensions: string[] }>) =>
      ipcRenderer.invoke('system:selectFile', filters)
  },

  // Tray events
  tray: {
    onNewChat: (callback: () => void) => {
      ipcRenderer.on('tray:new-chat', callback);
      return () => ipcRenderer.removeListener('tray:new-chat', callback);
    },
    onOpenSettings: (callback: () => void) => {
      ipcRenderer.on('tray:open-settings', callback);
      return () => ipcRenderer.removeListener('tray:open-settings', callback);
    },
    onRestartGateway: (callback: () => void) => {
      ipcRenderer.on('tray:restart-gateway', callback);
      return () => ipcRenderer.removeListener('tray:restart-gateway', callback);
    }
  },

  // Shortcuts
  shortcuts: {
    update: (config: Record<string, string>) => ipcRenderer.invoke('shortcuts:update', config),
    get: () => ipcRenderer.invoke('shortcuts:get'),
    onNewChat: (callback: () => void) => {
      ipcRenderer.on('shortcut:newChat', callback);
      return () => ipcRenderer.removeListener('shortcut:newChat', callback);
    }
  },

  // Data export/import
  data: {
    export: () => ipcRenderer.invoke('data:export'),
    import: () => ipcRenderer.invoke('data:import'),
  },

  // Auto-updater
  update: {
    check: () => ipcRenderer.invoke('update:check'),
    download: () => ipcRenderer.invoke('update:download'),
    install: () => ipcRenderer.invoke('update:install'),
    onAvailable: (callback: (info: unknown) => void) => {
      ipcRenderer.on('update:available', (_, info) => callback(info));
      return () => ipcRenderer.removeAllListeners('update:available');
    },
    onProgress: (callback: (progress: unknown) => void) => {
      ipcRenderer.on('update:progress', (_, progress) => callback(progress));
      return () => ipcRenderer.removeAllListeners('update:progress');
    },
    onDownloaded: (callback: () => void) => {
      ipcRenderer.on('update:downloaded', callback);
      return () => ipcRenderer.removeAllListeners('update:downloaded');
    },
  },

  terminal: {
    start: (sessionKey: string, options?: { cols?: number; rows?: number }) =>
      ipcRenderer.invoke('terminal:start', sessionKey, options),
    input: (sessionKey: string, value: string) =>
      ipcRenderer.invoke('terminal:input', sessionKey, value),
    resize: (sessionKey: string, cols: number, rows: number) =>
      ipcRenderer.invoke('terminal:resize', sessionKey, cols, rows),
    stop: (sessionKey: string) => ipcRenderer.invoke('terminal:stop', sessionKey),
    onData: (callback: (payload: { sessionKey: string; chunk: string }) => void) => {
      const listener = (_: unknown, payload: { sessionKey: string; chunk: string }) =>
        callback(payload);
      ipcRenderer.on('terminal:data', listener);
      return () => ipcRenderer.removeListener('terminal:data', listener);
    },
    onExit: (
      callback: (payload: { sessionKey: string; code: number | null; signal: string | null }) => void
    ) => {
      const listener = (
        _: unknown,
        payload: { sessionKey: string; code: number | null; signal: string | null }
      ) => callback(payload);
      ipcRenderer.on('terminal:exit', listener);
      return () => ipcRenderer.removeListener('terminal:exit', listener);
    }
  },

  // Platform info
  platform: {
    isMac: process.platform === 'darwin',
    isWindows: process.platform === 'win32',
    isLinux: process.platform === 'linux'
  }
};

contextBridge.exposeInMainWorld('electronAPI', electronAPI);

// Type declarations for TypeScript
declare global {
  interface Window {
    electronAPI: typeof electronAPI;
  }
}
