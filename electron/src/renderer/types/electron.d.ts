export interface GatewayStatus {
  state: 'running' | 'stopped' | 'error' | 'starting';
  port: number;
  error?: string;
}

export interface AppConfig {
  theme: 'light' | 'dark' | 'system';
  language: 'zh' | 'en';
  autoLaunch: boolean;
  minimizeToTray: boolean;
  shortcuts: Record<string, string>;
}

export interface ElectronAPI {
  window: {
    minimize: () => Promise<void>;
    maximize: () => Promise<boolean>;
    close: () => Promise<void>;
    isMaximized: () => Promise<boolean>;
  };

  gateway: {
    getStatus: () => Promise<GatewayStatus>;
    restart: () => Promise<{ success: boolean; error?: string }>;
    onStatusChange: (callback: (status: GatewayStatus) => void) => () => void;
  };

  config: {
    get: () => Promise<AppConfig>;
    set: (config: Partial<AppConfig>) => Promise<AppConfig>;
    onChange: (callback: (config: AppConfig) => void) => () => void;
  };

  system: {
    showNotification: (title: string, body: string) => Promise<void>;
    openExternal: (url: string) => Promise<void>;
    selectFolder: () => Promise<string | null>;
    selectFile: (filters?: Array<{ name: string; extensions: string[] }>) => Promise<string | null>;
  };

  tray: {
    onNewChat: (callback: () => void) => () => void;
    onOpenSettings: (callback: () => void) => () => void;
    onRestartGateway: (callback: () => void) => () => void;
  };

  terminal: {
    start: () => Promise<{ success: boolean; shell?: string; alreadyRunning?: boolean; error?: string }>;
    input: (value: string) => Promise<{ success: boolean; error?: string }>;
    stop: () => Promise<{ success: boolean }>;
    onData: (callback: (chunk: string) => void) => () => void;
    onExit: (callback: (code: number | null, signal: string | null) => void) => () => void;
  };

  platform: {
    isMac: boolean;
    isWindows: boolean;
    isLinux: boolean;
  };
}

declare global {
  interface Window {
    electronAPI: ElectronAPI;
  }
}

export {};
