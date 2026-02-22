import React, { useEffect } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { RootState, setStatus, setActiveTab, setTheme, setLanguage, setCurrentSessionKey, toggleSidebar } from './store';
import { Sidebar } from './components/Sidebar';
import { ChatView } from './views/ChatView';
import { SessionsView } from './views/SessionsView';
import { ScheduledTasksView } from './views/ScheduledTasksView';
import { SkillsView } from './views/SkillsView';
import { SettingsView } from './views/SettingsView';
import { wsClient } from './services/websocket';

function App() {
  const dispatch = useDispatch();
  const { activeTab, theme, sidebarCollapsed } = useSelector((state: RootState) => state.ui);
  const isMac = window.electronAPI.platform.isMac;

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

  const handleNewTask = () => {
    const newSessionKey = `desktop:${Date.now()}`;
    dispatch(setCurrentSessionKey(newSessionKey));
    dispatch(setActiveTab('chat'));
  };

  return (
    <div className="h-screen bg-background text-foreground">
      <div className={`relative flex h-full overflow-hidden gap-3 p-3 ${isMac ? 'pt-12' : 'pt-4'}`}>
        <div className={`absolute z-40 flex items-center gap-2 ${isMac ? 'left-20 top-3' : 'left-3 top-3'}`}>
          <button
            onClick={() => dispatch(toggleSidebar())}
            className="flex h-9 w-9 items-center justify-center rounded-lg border border-border bg-card text-foreground/80 shadow-sm transition-colors hover:bg-secondary"
            aria-label="Toggle sidebar"
            title="Toggle sidebar"
          >
            <SidebarToggleIcon className="h-4 w-4" />
          </button>
          {sidebarCollapsed && (
            <button
              onClick={handleNewTask}
              className="flex h-9 w-9 items-center justify-center rounded-lg border border-border bg-card text-foreground/80 shadow-sm transition-colors hover:bg-secondary"
              aria-label="New task"
              title="New task"
            >
              <PencilIcon className="h-4 w-4" />
            </button>
          )}
        </div>
        <Sidebar />
        <main className="flex-1 overflow-hidden rounded-2xl border border-border bg-card shadow-sm">
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

function SidebarToggleIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
    </svg>
  );
}

function PencilIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 20h9" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16.5 3.5a2.1 2.1 0 113 3L7 19l-4 1 1-4 12.5-12.5z" />
    </svg>
  );
}
