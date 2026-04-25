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
  toggleSidebar
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
  const { activeTab, theme, sidebarCollapsed } = useSelector((state: RootState) => state.ui);
  const isMac = window.electronAPI.platform.isMac;
  const controlAnchorStyle = isMac
    ? { left: '92px', top: '10px' }
    : { left: '12px', top: '10px' };
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

  // Apply theme to document via data-theme attribute
  useEffect(() => {
    if (theme === 'system') {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      document.documentElement.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
    } else {
      document.documentElement.setAttribute('data-theme', theme);
    }
  }, [theme]);

  const handleNewTask = () => {
    const newSessionKey = `desktop:${Date.now()}`;
    dispatch(setCurrentSessionKey(newSessionKey));
    dispatch(setActiveTab('chat'));
  };

  return (
    <div className="desktop-shell h-screen overflow-hidden text-foreground">
      <div className="relative flex h-full w-full gap-0 overflow-hidden bg-background">
        <div className={`absolute z-10 draggable ${isMac ? 'h-14' : 'h-12'}`} style={dragStripStyle} />
        <div className="absolute z-40 flex items-center gap-2 no-drag" style={controlAnchorStyle}>
          <button
            onClick={() => dispatch(toggleSidebar())}
            className="flex h-9 w-9 items-center justify-center rounded-lg border border-border bg-background text-muted transition-colors hover:bg-hover hover:text-foreground"
            aria-label="Toggle sidebar"
            title="Toggle sidebar"
          >
            <SidebarToggleIcon collapsed={sidebarCollapsed} className="h-4 w-4" />
          </button>
          {sidebarCollapsed && (
            <button
              onClick={handleNewTask}
              className="flex h-9 w-9 items-center justify-center rounded-lg border border-border bg-background text-muted transition-colors hover:bg-hover hover:text-foreground"
              aria-label="New task"
              title="New task"
            >
              <PencilIcon className="h-4 w-4" />
            </button>
          )}
        </div>
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

function SidebarToggleIcon({ className, collapsed }: { className?: string; collapsed: boolean }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <rect x={3} y={4} width={18} height={16} rx={2.5} strokeWidth={1.7} />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.7} d="M9 4v16" />
      {collapsed ? (
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.7} d="M13 9l3 3-3 3" />
      ) : (
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.7} d="M15 9l-3 3 3 3" />
      )}
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
