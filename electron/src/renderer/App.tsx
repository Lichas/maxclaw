import React, { useEffect } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { RootState, setStatus, setActiveTab, setTheme, setLanguage, setCurrentSessionKey } from './store';
import { TitleBar } from './components/TitleBar';
import { Sidebar } from './components/Sidebar';
import { ChatView } from './views/ChatView';
import { SessionsView } from './views/SessionsView';
import { ScheduledTasksView } from './views/ScheduledTasksView';
import { SkillsView } from './views/SkillsView';
import { SettingsView } from './views/SettingsView';
import { wsClient } from './services/websocket';

function App() {
  const dispatch = useDispatch();
  const { activeTab, theme, language } = useSelector((state: RootState) => state.ui);

  useEffect(() => {
    dispatch(setCurrentSessionKey(`desktop:${Date.now()}`));

    // Load app settings from electron store
    window.electronAPI.config.get().then((config) => {
      if (config.theme) {
        dispatch(setTheme(config.theme));
      }
      if (config.language) {
        dispatch(setLanguage(config.language));
      }
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
    });

    // Connect WebSocket for real-time updates
    wsClient.connect();

    // Subscribe to WebSocket messages
    const unsubscribeWS = wsClient.on('message', (data) => {
      console.log('Received WebSocket message:', data);
      // Could trigger session refresh here if needed
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
      wsClient.disconnect();
    };
  }, [dispatch]);

  // Apply theme to document
  useEffect(() => {
    document.documentElement.classList.remove('light', 'dark');
    if (theme === 'system') {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      document.documentElement.classList.add(prefersDark ? 'dark' : 'light');
    } else {
      document.documentElement.classList.add(theme);
    }
  }, [theme]);

  return (
    <div className="h-screen flex flex-col bg-background text-foreground">
      <TitleBar />
      <div className="flex-1 flex overflow-hidden">
        <Sidebar />
        <main className="flex-1 overflow-hidden">
          {activeTab === 'chat' && <ChatView />}
          {activeTab === 'sessions' && <SessionsView />}
          {activeTab === 'scheduled' && <ScheduledTasksView />}
          {activeTab === 'skills' && <SkillsView />}
          {activeTab === 'settings' && <SettingsView />}
        </main>
      </div>
    </div>
  );
}

export default App;
