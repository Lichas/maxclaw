import React, { useEffect, useMemo, useState } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { RootState, setActiveTab, setCurrentSessionKey } from '../store';
import { SessionSummary, useGateway } from '../hooks/useGateway';

const menuItems = [
  { id: 'sessions', label: '搜索任务', icon: SearchIcon },
  { id: 'scheduled', label: '定时任务', icon: ClockIcon },
  { id: 'skills', label: '技能', icon: PuzzleIcon },
] as const;

export function Sidebar() {
  const dispatch = useDispatch();
  const { activeTab, sidebarCollapsed, currentSessionKey } = useSelector((state: RootState) => state.ui);
  const { status } = useSelector((state: RootState) => state.gateway);
  const { getSessions } = useGateway();
  const [sessions, setSessions] = useState<SessionSummary[]>([]);

  useEffect(() => {
    let cancelled = false;

    const loadSessions = async () => {
      try {
        const list = await getSessions();
        if (!cancelled) {
          setSessions(list);
        }
      } catch {
        if (!cancelled) {
          setSessions([]);
        }
      }
    };

    void loadSessions();
    const timer = setInterval(() => void loadSessions(), 15000);

    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [getSessions]);

  const sessionItems = useMemo(
    () => sessions.filter((session) => session.key.startsWith('desktop:')).slice(0, 8),
    [sessions]
  );

  const handleNewTask = () => {
    const newSessionKey = `desktop:${Date.now()}`;
    dispatch(setCurrentSessionKey(newSessionKey));
    dispatch(setActiveTab('chat'));
  };

  return (
    <aside
      className={`bg-secondary/90 border-r border-border flex flex-col transition-all duration-200 ${
        sidebarCollapsed ? 'w-16' : 'w-64'
      }`}
    >
      {/* New Chat Button */}
      <div className="p-3">
        <button
          onClick={handleNewTask}
          className="w-full flex items-center justify-center gap-2 bg-primary/15 text-primary border border-primary/30 rounded-lg py-2.5 px-4 hover:bg-primary/20 transition-colors"
        >
          <EditIcon className="w-5 h-5" />
          {!sidebarCollapsed && <span className="font-medium">新建任务</span>}
        </button>
      </div>

      {/* Menu Items */}
      <nav className="flex-1 px-2">
        {menuItems.map((item) => {
          const Icon = item.icon;
          const isActive = activeTab === item.id;

          return (
            <button
              key={item.id}
              onClick={() => dispatch(setActiveTab(item.id))}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg mb-1 transition-colors ${
                isActive
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-foreground/60 hover:bg-background/60 hover:text-foreground'
              }`}
            >
              <Icon className="w-5 h-5 flex-shrink-0" />
              {!sidebarCollapsed && <span className="font-medium">{item.label}</span>}
            </button>
          );
        })}

        {!sidebarCollapsed && (
          <div className="mt-4 px-2">
            <p className="text-xs font-semibold text-foreground/45 tracking-wide mb-2">任务记录</p>
            <div className="space-y-1">
              {sessionItems.length === 0 && (
                <div className="text-xs text-foreground/45 px-2 py-1">暂无桌面任务记录</div>
              )}

              {sessionItems.map((session) => {
                const isCurrent = session.key === currentSessionKey;
                return (
                  <button
                    key={session.key}
                    onClick={() => {
                      dispatch(setCurrentSessionKey(session.key));
                      dispatch(setActiveTab('chat'));
                    }}
                    className={`w-full text-left px-2 py-2 rounded-lg transition-colors ${
                      isCurrent ? 'bg-background text-foreground' : 'hover:bg-background/60'
                    }`}
                  >
                    <p className="text-sm font-medium truncate">
                      {session.lastMessage || session.key.replace(/^desktop:/, '新任务')}
                    </p>
                    <p className="text-xs text-foreground/45 mt-1">
                      {formatRelativeTime(session.lastMessageAt)}
                    </p>
                  </button>
                );
              })}
            </div>
          </div>
        )}
      </nav>

      {/* Gateway Status */}
      <button
        onClick={() => dispatch(setActiveTab('settings'))}
        className={`mx-3 mb-2 mt-1 flex items-center gap-2 px-3 py-2 rounded-lg text-foreground/65 hover:bg-background/60 transition-colors ${
          sidebarCollapsed ? 'justify-center' : ''
        }`}
      >
        <SettingsIcon className="w-4 h-4 flex-shrink-0" />
        {!sidebarCollapsed && <span className="text-sm">设置</span>}
      </button>

      <div className="p-3 border-t border-border/70">
        <div className={`flex items-center gap-2 px-3 py-2 rounded-lg ${sidebarCollapsed ? 'justify-center' : ''}`}>
          <div
            className={`w-2 h-2 rounded-full flex-shrink-0 ${
              status === 'running'
                ? 'bg-green-500'
                : status === 'error'
                ? 'bg-red-500'
                : 'bg-yellow-500'
            }`}
          />
          {!sidebarCollapsed && (
            <span className="text-sm text-foreground/60 capitalize">{status}</span>
          )}
        </div>
      </div>
    </aside>
  );
}

// Icon components
function ChatIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
    </svg>
  );
}

function HistoryIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );
}

function SearchIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-4.35-4.35m1.35-5.15a7 7 0 11-14 0 7 7 0 0114 0z" />
    </svg>
  );
}

function ClockIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );
}

function PuzzleIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 4a2 2 0 114 0v1a1 1 0 001 1h3a1 1 0 011 1v3a1 1 0 01-1 1h-1a2 2 0 100 4h1a1 1 0 011 1v3a1 1 0 01-1 1h-3a1 1 0 01-1-1v-1a2 2 0 10-4 0v1a1 1 0 01-1 1H7a1 1 0 01-1-1v-3a1 1 0 00-1-1H4a2 2 0 110-4h1a1 1 0 001-1V7a1 1 0 011-1h3a1 1 0 001-1V4z" />
    </svg>
  );
}

function SettingsIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  );
}

function EditIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 20h9" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16.5 3.5a2.1 2.1 0 113 3L7 19l-4 1 1-4 12.5-12.5z" />
    </svg>
  );
}

function formatRelativeTime(time?: string): string {
  if (!time) return '刚刚';

  const date = new Date(time);
  if (Number.isNaN(date.getTime())) {
    return '刚刚';
  }

  const diffMs = Date.now() - date.getTime();
  const minutes = Math.max(1, Math.floor(diffMs / 60000));
  if (minutes < 60) return `${minutes}m`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;

  const days = Math.floor(hours / 24);
  return `${days}d`;
}
