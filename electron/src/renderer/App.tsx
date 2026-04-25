import React, { useEffect } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import {
  RootState,
  setStatus,
  setActiveTab,
  setTheme,
  setLanguage,
  setRenderThinkTags,
  setCurrentSessionKey,
  setSessionRunning
} from './store';
import { Sidebar } from './components/Sidebar';
import { ChatView } from './views/ChatView';
import { SessionsView } from './views/SessionsView';
import { ScheduledTasksView } from './views/ScheduledTasksView';
import { SkillsView } from './views/SkillsView';
import { MCPView } from './views/MCPView';
import { SettingsView } from './views/SettingsView';
import { wsClient } from './services/websocket';

function App() {
  const dispatch = useDispatch();
  const { activeTab, theme } = useSelector((state: RootState) => state.ui);
  const isMac = window.electronAPI.platform.isMac;
  const dragStripStyle = isMac
    ? { left: '176px', right: '0px', top: '0px' }
    : { left: '120px', right: '0px', top: '0px' };

  useEffect(() => {
    dispatch(setCurrentSessionKey(`desktop:${Date.now()}`));

    // Load app settings from electron store
    window.electronAPI.config.get().then((config) => {
      if (config.theme) {
        dispatch(setTheme(config.theme));
      }
      // Only override system-detected language if user has explicitly set it
      if (config.language) {
        dispatch(setLanguage(config.language));
      }
      if (typeof config.renderThinkTags === 'boolean') {
        dispatch(setRenderThinkTags(config.renderThinkTags));
      }
      // Note: if config.language is not set, the system-detected language from store/index.ts is used
    });

    // Initialize Gateway status
    window.electronAPI.gateway.getStatus().then(status => {
      dispatch(setStatus(status));
    });

    // Listen for status changes
    const unsubscribe = window.electronAPI.gateway.onStatusChange((status) => {
      dispatch(setStatus(status));
    });

    // Listen for config changes
    const unsubscribeConfig = window.electronAPI.config.onChange((config) => {
      if (config.theme) {
        dispatch(setTheme(config.theme));
      }
      if (config.language) {
        dispatch(setLanguage(config.language));
      }
      if (typeof config.renderThinkTags === 'boolean') {
        dispatch(setRenderThinkTags(config.renderThinkTags));
      }
    });

    // Connect WebSocket for real-time updates
    wsClient.connect();

    // Subscribe to WebSocket messages
    const unsubscribeWS = wsClient.on('message', (data) => {
      console.log('Received WebSocket message:', data);
      // Could trigger session refresh here if needed
    });

    const unsubscribeChat = wsClient.on('chat', (payload: { sessionKey?: string }) => {
      const sessionKey = payload?.sessionKey?.trim();
      if (!sessionKey) {
        return;
      }
      dispatch(setSessionRunning({ sessionKey, running: false }));
    });

    // Listen for tray events
    const unsubscribeNewChat = window.electronAPI.tray.onNewChat(() => {
      dispatch(setActiveTab('chat'));
    });

    const unsubscribeSettings = window.electronAPI.tray.onOpenSettings(() => {
      dispatch(setActiveTab('settings'));
    });

    return () => {
      unsubscribe();
      unsubscribeConfig();
      unsubscribeNewChat();
      unsubscribeSettings();
      unsubscribeWS();
      unsubscribeChat();
      wsClient.disconnect();
    };
  }, [dispatch]);

  // Apply theme to document via data-theme attribute
  useEffect(() => {
    if (theme === 'system') {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      document.documentElement.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
    } else {
      document.documentElement.setAttribute('data-theme', theme);
    }
  }, [theme]);

  return (
    <div className="desktop-shell h-screen overflow-hidden text-foreground">
      <div className="relative flex h-full w-full gap-0 overflow-hidden bg-background">
        <div className={`absolute z-10 draggable ${isMac ? 'h-14' : 'h-12'}`} style={dragStripStyle} />
        <Sidebar />
        <main className="flex-1 overflow-hidden bg-transparent border-l border-border">
          {activeTab === 'chat' && <ChatView />}
          {activeTab === 'sessions' && <SessionsView />}
          {activeTab === 'scheduled' && <ScheduledTasksView />}
          {activeTab === 'skills' && <SkillsView />}
          {activeTab === 'mcp' && <MCPView />}
          {activeTab === 'settings' && <SettingsView />}
        </main>
      </div>
    </div>
  );
}

export default App;
