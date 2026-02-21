import React, { useEffect, useMemo, useState, useRef } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { RootState, setActiveTab, setCurrentSessionKey } from '../store';
import { SessionSummary, useGateway } from '../hooks/useGateway';
import { useTranslation } from '../i18n';

const menuItems = [
  { id: 'sessions', labelKey: 'nav.sessions', icon: SearchIcon },
  { id: 'scheduled', labelKey: 'nav.scheduled', icon: ClockIcon },
  { id: 'skills', labelKey: 'nav.skills', icon: PuzzleIcon },
] as const;

const baseChannelOptions = ['desktop', 'telegram', 'webui'] as const;

function extractSessionChannel(sessionKey: string): string {
  const prefix = sessionKey.split(':', 2)[0];
  if (!prefix) {
    return 'unknown';
  }
  return prefix.toLowerCase();
}


export function Sidebar() {
  const dispatch = useDispatch();
  const { t } = useTranslation();
  const { activeTab, sidebarCollapsed, currentSessionKey } = useSelector((state: RootState) => state.ui);
  const { status } = useSelector((state: RootState) => state.gateway);
  const { getSessions, deleteSession, renameSession } = useGateway();
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [channelFilter, setChannelFilter] = useState<string>('desktop');

  // Delete/Rename state
  const [editingSession, setEditingSession] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [openMenuKey, setOpenMenuKey] = useState<string | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const buildDraftSession = (key: string): SessionSummary => ({
    key,
    messageCount: 0,
    lastMessage: t('sidebar.newTask'),
    lastMessageAt: new Date().toISOString()
  });

  // Close menu when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setOpenMenuKey(null);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleDelete = async (sessionKey: string) => {
    if (!confirm(t('sidebar.confirmDelete'))) return;
    try {
      await deleteSession(sessionKey);
      setSessions((prev) => prev.filter((s) => s.key !== sessionKey));
      if (currentSessionKey === sessionKey) {
        dispatch(setCurrentSessionKey(''));
      }
    } catch {
      alert(t('common.error'));
    }
    setOpenMenuKey(null);
  };

  const handleStartRename = (session: SessionSummary) => {
    setEditingSession(session.key);
    setEditTitle(session.lastMessage || session.key);
    setOpenMenuKey(null);
  };

  const handleRename = async () => {
    if (!editingSession || !editTitle.trim()) {
      setEditingSession(null);
      return;
    }
    try {
      await renameSession(editingSession, editTitle.trim());
      setSessions((prev) =>
        prev.map((s) =>
          s.key === editingSession ? { ...s, lastMessage: editTitle.trim() } : s
        )
      );
    } catch {
      alert(t('common.error'));
    }
    setEditingSession(null);
    setEditTitle('');
  };

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

  const mergedSessions = useMemo(() => {
    const currentChannel = extractSessionChannel(currentSessionKey);
    if (!sessions.some((session) => session.key === currentSessionKey) && currentChannel === 'desktop') {
      return [buildDraftSession(currentSessionKey), ...sessions];
    }
    return sessions;
  }, [sessions, currentSessionKey]);

  const channelOptions = useMemo(() => {
    const dynamicChannels = mergedSessions
      .map((session) => extractSessionChannel(session.key))
      .filter((channel) => !baseChannelOptions.includes(channel as (typeof baseChannelOptions)[number]))
      .filter((channel, index, arr) => arr.indexOf(channel) === index)
      .sort((a, b) => a.localeCompare(b));

    return [...baseChannelOptions, ...dynamicChannels];
  }, [mergedSessions]);

  const sessionItems = useMemo(
    () =>
      mergedSessions
        .filter((session) => extractSessionChannel(session.key) === channelFilter)
        .slice(0, 20),
    [mergedSessions, channelFilter]
  );

  const handleNewTask = () => {
    const newSessionKey = `desktop:${Date.now()}`;
    setSessions((prev) => [buildDraftSession(newSessionKey), ...prev.filter((session) => session.key !== newSessionKey)]);
    setChannelFilter('desktop');
    dispatch(setCurrentSessionKey(newSessionKey));
    dispatch(setActiveTab('chat'));
  };

  return (
    <aside
      className={`bg-secondary border-r border-border flex flex-col transition-all duration-200 ${
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
          {!sidebarCollapsed && <span className="font-medium">{t('sidebar.newTask')}</span>}
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
              {!sidebarCollapsed && <span className="font-medium">{t(item.labelKey)}</span>}
            </button>
          );
        })}

        {!sidebarCollapsed && (
          <div className="mt-4 px-2">
            <p className="text-xs font-semibold text-foreground/45 tracking-wide mb-2">{t('nav.sessions')}</p>
            <div className="mb-2 relative">
              <select
                value={channelFilter}
                onChange={(event) => setChannelFilter(event.target.value)}
                className="w-full appearance-none rounded-lg border border-border bg-background px-3 py-2 text-sm font-medium text-foreground/75 focus:border-primary/40 focus:outline-none"
              >
                {channelOptions.map((channel) => (
                  <option key={channel} value={channel}>
                    {channel}
                  </option>
                ))}
              </select>
              <ChevronDownIcon className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-foreground/45" />
            </div>

            <div className="space-y-1 mt-2">
              {sessionItems.length === 0 && (
                <div className="text-sm text-foreground/45 px-2 py-1">
                  {t('skills.empty')}
                </div>
              )}

              {sessionItems.map((session) => {
                const isCurrent = session.key === currentSessionKey;
                const isEditing = editingSession === session.key;
                const isMenuOpen = openMenuKey === session.key;

                if (isEditing) {
                  return (
                    <div key={session.key} className="px-2 py-2 rounded-lg bg-background">
                      <input
                        type="text"
                        value={editTitle}
                        onChange={(e) => setEditTitle(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') handleRename();
                          if (e.key === 'Escape') setEditingSession(null);
                        }}
                        onBlur={handleRename}
                        autoFocus
                        className="w-full text-sm font-medium bg-transparent border-b border-primary/50 focus:outline-none focus:border-primary text-foreground"
                      />
                      <p className="text-xs text-foreground/40 mt-1">
                        Enter to confirm, Esc to cancel
                      </p>
                    </div>
                  );
                }

                return (
                  <div
                    key={session.key}
                    className={`group relative flex items-center gap-1 px-2 py-2 rounded-lg transition-colors ${
                      isCurrent ? 'bg-background text-foreground' : 'hover:bg-background/60'
                    }`}
                  >
                    <button
                      onClick={() => {
                        dispatch(setCurrentSessionKey(session.key));
                        dispatch(setActiveTab('chat'));
                      }}
                      className="flex-1 text-left min-w-0"
                    >
                      <p className="text-sm font-medium leading-5 truncate">
                        {session.lastMessage || session.key.replace(/^desktop:/, '新任务')}
                      </p>
                      <p className="text-sm text-foreground/50 leading-5 mt-0.5">
                        {formatRelativeTime(session.lastMessageAt)}
                      </p>
                    </button>

                    {/* Menu Button */}
                    <div className="relative" ref={isMenuOpen ? menuRef : undefined}>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setOpenMenuKey(isMenuOpen ? null : session.key);
                        }}
                        className="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-foreground/10 transition-opacity"
                      >
                        <DotsIcon className="w-4 h-4 text-foreground/50" />
                      </button>

                      {/* Dropdown Menu */}
                      {isMenuOpen && (
                        <div className="absolute right-0 top-full mt-1 w-32 rounded-lg border border-border bg-background shadow-lg z-50 py-1">
                          <button
                            onClick={() => handleStartRename(session)}
                            className="w-full px-3 py-2 text-sm text-left hover:bg-secondary flex items-center gap-2"
                          >
                            <EditIcon className="w-3.5 h-3.5" />
                            {t('sidebar.rename')}
                          </button>
                          <button
                            onClick={() => handleDelete(session.key)}
                            className="w-full px-3 py-2 text-sm text-left hover:bg-red-50 text-red-600 flex items-center gap-2"
                          >
                            <TrashIcon className="w-3.5 h-3.5" />
                            {t('sidebar.delete')}
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </nav>

      {/* Settings Button with Gateway Status */}
      <div className="p-3 border-t border-border/70">
        <button
          onClick={() => dispatch(setActiveTab('settings'))}
          className={`w-full flex items-center gap-2 px-3 py-2 rounded-lg text-foreground/65 hover:bg-background/60 transition-colors ${
            sidebarCollapsed ? 'justify-center' : ''
          }`}
        >
          <div className="relative">
            <SettingsIcon className="w-4 h-4 flex-shrink-0" />
            <div
              className={`absolute -bottom-0.5 -right-0.5 w-1.5 h-1.5 rounded-full ${
                status === 'running'
                  ? 'bg-green-500'
                  : status === 'error'
                  ? 'bg-red-500'
                  : 'bg-yellow-500'
              }`}
            />
          </div>
          {!sidebarCollapsed && <span className="text-sm">{t('nav.settings')}</span>}
        </button>
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

function ChevronDownIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
    </svg>
  );
}

function DotsIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="currentColor" viewBox="0 0 20 20">
      <path d="M6 10a2 2 0 11-4 0 2 2 0 014 0zM12 10a2 2 0 11-4 0 2 2 0 014 0zM16 12a2 2 0 100-4 2 2 0 000 4z" />
    </svg>
  );
}

function TrashIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
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
