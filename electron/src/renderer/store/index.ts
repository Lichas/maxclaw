import { configureStore, createSlice, PayloadAction } from '@reduxjs/toolkit';

interface GatewayState {
  status: 'running' | 'stopped' | 'error' | 'starting';
  port: number;
  error?: string;
}

interface UIState {
  theme: 'light' | 'dark' | 'system';
  sidebarCollapsed: boolean;
  activeTab: 'chat' | 'sessions' | 'scheduled' | 'skills' | 'settings';
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
    sidebarCollapsed: false,
    activeTab: 'chat',
    currentSessionKey: 'desktop:default'
  } as UIState,
  reducers: {
    setTheme: (state, action: PayloadAction<'light' | 'dark' | 'system'>) => {
      state.theme = action.payload;
    },
    toggleSidebar: (state) => {
      state.sidebarCollapsed = !state.sidebarCollapsed;
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
export const { setTheme, toggleSidebar, setActiveTab, setCurrentSessionKey } = uiSlice.actions;

export const store = configureStore({
  reducer: {
    gateway: gatewaySlice.reducer,
    ui: uiSlice.reducer
  }
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
