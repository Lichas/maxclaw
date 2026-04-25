import { configureStore, createSlice, PayloadAction } from '@reduxjs/toolkit';

// Detect system language: 'zh' for Chinese, 'en' for others
const detectSystemLanguage = (): 'zh' | 'en' => {
  const systemLang = navigator.language || 'en';
  return systemLang.toLowerCase().startsWith('zh') ? 'zh' : 'en';
};

// Load saved language from localStorage or detect from system
const getInitialLanguage = (): 'zh' | 'en' => {
  if (typeof window === 'undefined') return 'zh';
  const savedLang = localStorage.getItem('nanobot_lang');
  if (savedLang === 'zh' || savedLang === 'en') {
    return savedLang;
  }
  return detectSystemLanguage();
};

type GatewayRuntimeStatus = 'running' | 'stopped' | 'error' | 'starting';

interface GatewayState {
  status: GatewayRuntimeStatus;
  port: number;
  error?: string;
}

type GatewayStatusPayload =
  | GatewayState
  | {
      state: GatewayRuntimeStatus;
      port: number;
      error?: string;
    };

function normalizeGatewayStatus(payload: GatewayStatusPayload): GatewayState {
  return {
    status: 'status' in payload ? payload.status : payload.state,
    port: payload.port,
    error: payload.error
  };
}

interface UIState {
  theme: 'light' | 'dark' | 'system';
  language: 'zh' | 'en';
  renderThinkTags: boolean;
  sidebarCollapsed: boolean;
  terminalVisible: boolean;
  activeTab: 'chat' | 'sessions' | 'scheduled' | 'skills' | 'mcp' | 'settings';
  currentSessionKey: string;
  runningSessionKeys: string[];
}

const gatewaySlice = createSlice({
  name: 'gateway',
  initialState: {
    status: 'stopped',
    port: 18890
  } as GatewayState,
  reducers: {
    setStatus: (_, action: PayloadAction<GatewayStatusPayload>) => {
      return normalizeGatewayStatus(action.payload);
    }
  }
});

const uiSlice = createSlice({
  name: 'ui',
  initialState: {
    theme: 'system',
    language: getInitialLanguage(),
    renderThinkTags: true,
    sidebarCollapsed: false,
    terminalVisible: false,
    activeTab: 'chat',
    currentSessionKey: 'desktop:default',
    runningSessionKeys: []
  } as UIState,
  reducers: {
    setTheme: (state, action: PayloadAction<'light' | 'dark' | 'system'>) => {
      state.theme = action.payload;
    },
    setLanguage: (state, action: PayloadAction<'zh' | 'en'>) => {
      state.language = action.payload;
    },
    setRenderThinkTags: (state, action: PayloadAction<boolean>) => {
      state.renderThinkTags = action.payload;
    },
    toggleSidebar: (state) => {
      state.sidebarCollapsed = !state.sidebarCollapsed;
    },
    toggleTerminal: (state) => {
      state.terminalVisible = !state.terminalVisible;
    },
    setTerminalVisible: (state, action: PayloadAction<boolean>) => {
      state.terminalVisible = action.payload;
    },
    setActiveTab: (state, action: PayloadAction<UIState['activeTab']>) => {
      state.activeTab = action.payload;
    },
    setCurrentSessionKey: (state, action: PayloadAction<string>) => {
      state.currentSessionKey = action.payload;
    },
    setSessionRunning: (state, action: PayloadAction<{ sessionKey: string; running: boolean }>) => {
      const sessionKey = action.payload.sessionKey.trim();
      if (!sessionKey) {
        return;
      }
      const current = new Set(state.runningSessionKeys);
      if (action.payload.running) {
        current.add(sessionKey);
      } else {
        current.delete(sessionKey);
      }
      state.runningSessionKeys = Array.from(current);
    }
  }
});

export const { setStatus } = gatewaySlice.actions;
export const {
  setTheme,
  setLanguage,
  setRenderThinkTags,
  toggleSidebar,
  toggleTerminal,
  setTerminalVisible,
  setActiveTab,
  setCurrentSessionKey,
  setSessionRunning
} = uiSlice.actions;

export const store = configureStore({
  reducer: {
    gateway: gatewaySlice.reducer,
    ui: uiSlice.reducer
  }
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
