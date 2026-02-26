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

interface GatewayState {
  status: 'running' | 'stopped' | 'error' | 'starting';
  port: number;
  error?: string;
}

interface UIState {
  theme: 'light' | 'dark' | 'system';
  language: 'zh' | 'en';
  sidebarCollapsed: boolean;
  terminalVisible: boolean;
  activeTab: 'chat' | 'sessions' | 'scheduled' | 'skills' | 'mcp' | 'settings';
  currentSessionKey: string;
}

const gatewaySlice = createSlice({
  name: 'gateway',
  initialState: {
    status: 'stopped',
    port: 18890
  } as GatewayState,
  reducers: {
    setStatus: (state, action: PayloadAction<GatewayState>) => {
      return action.payload;
    }
  }
});

const uiSlice = createSlice({
  name: 'ui',
  initialState: {
    theme: 'system',
    language: getInitialLanguage(),
    sidebarCollapsed: false,
    terminalVisible: false,
    activeTab: 'chat',
    currentSessionKey: 'desktop:default'
  } as UIState,
  reducers: {
    setTheme: (state, action: PayloadAction<'light' | 'dark' | 'system'>) => {
      state.theme = action.payload;
    },
    setLanguage: (state, action: PayloadAction<'zh' | 'en'>) => {
      state.language = action.payload;
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
    }
  }
});

export const { setStatus } = gatewaySlice.actions;
export const { setTheme, setLanguage, toggleSidebar, toggleTerminal, setTerminalVisible, setActiveTab, setCurrentSessionKey } = uiSlice.actions;

export const store = configureStore({
  reducer: {
    gateway: gatewaySlice.reducer,
    ui: uiSlice.reducer
  }
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
