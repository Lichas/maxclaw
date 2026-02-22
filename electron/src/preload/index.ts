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
      ipcRenderer.on('gateway:status-change', (_, status) => callback(status));
      return () => ipcRenderer.removeAllListeners('gateway:status-change');
    }
  },

  // App configuration (local settings)
  config: {
    get: () => ipcRenderer.invoke('config:get'),
    set: (config: Partial<AppConfig>) => ipcRenderer.invoke('config:set', config),
    onChange: (callback: (config: AppConfig) => void) => {
      ipcRenderer.on('config:change', (_, config) => callback(config));
      return () => ipcRenderer.removeAllListeners('config:change');
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
    selectFolder: () => ipcRenderer.invoke('system:selectFolder'),
    selectFile: (filters?: Array<{ name: string; extensions: string[] }>) =>
      ipcRenderer.invoke('system:selectFile', filters)
  },

  // Tray events
  tray: {
    onNewChat: (callback: () => void) => {
      ipcRenderer.on('tray:new-chat', callback);
      return () => ipcRenderer.removeAllListeners('tray:new-chat');
    },
    onOpenSettings: (callback: () => void) => {
      ipcRenderer.on('tray:open-settings', callback);
      return () => ipcRenderer.removeAllListeners('tray:open-settings');
    },
    onRestartGateway: (callback: () => void) => {
      ipcRenderer.on('tray:restart-gateway', callback);
      return () => ipcRenderer.removeAllListeners('tray:restart-gateway');
    }
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
