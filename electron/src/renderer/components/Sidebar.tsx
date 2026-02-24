import React, { useEffect, useMemo, useState, useRef } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { RootState, setActiveTab, setCurrentSessionKey } from '../store';
import { SessionSummary, useGateway } from '../hooks/useGateway';
import { useTranslation } from '../i18n';
import { CustomSelect } from './CustomSelect';
import { ConfirmDialog } from './ConfirmDialog';
import {
  DEFAULT_CHANNEL_ORDER,
  extractSessionChannel,
  getChannelLabel,
  normalizeChannelKey
} from '../utils/sessionChannels';

const menuItems = [
  { id: 'sessions', labelKey: 'nav.sessions', icon: SearchIcon },
  { id: 'scheduled', labelKey: 'nav.scheduled', icon: ClockIcon },
  { id: 'skills', labelKey: 'nav.skills', icon: PuzzleIcon },
] as const;

export function Sidebar() {
  const dispatch = useDispatch();
  const { t, language } = useTranslation();
  const { activeTab, sidebarCollapsed, currentSessionKey } = useSelector((state: RootState) => state.ui);
  const { status } = useSelector((state: RootState) => state.gateway);
  const { getSessions, deleteSession, renameSession } = useGateway();
  const isMac = window.electronAPI.platform.isMac;
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [draftSessions, setDraftSessions] = useState<Record<string, SessionSummary>>({});
  const [channelFilter, setChannelFilter] = useState<string>('desktop');

  // Delete/Rename state
  const [editingSession, setEditingSession] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [openMenuKey, setOpenMenuKey] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [sessionToDelete, setSessionToDelete] = useState<string | null>(null);
  const [sessionToDeleteTitle, setSessionToDeleteTitle] = useState('');
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

  const getSessionDisplayTitle = (session: SessionSummary): string => {
    const title = (session.lastMessage || '').trim();
    if (title !== '') {
      return title;
    }
    return session.key.replace(/^desktop:/, t('sidebar.newTask'));
  };

  const handleDelete = (session: SessionSummary) => {
    setSessionToDelete(session.key);
    setSessionToDeleteTitle(getSessionDisplayTitle(session));
    setDeleteDialogOpen(true);
    setOpenMenuKey(null);
  };

  const confirmDelete = async () => {
    if (!sessionToDelete) return;
    const isDraftOnly = Boolean(draftSessions[sessionToDelete]) && !sessions.some((s) => s.key === sessionToDelete);
    try {
      if (!isDraftOnly) {
        await deleteSession(sessionToDelete);
      }
      setSessions((prev) => prev.filter((s) => s.key !== sessionToDelete));
      setDraftSessions((prev) => {
        if (!prev[sessionToDelete]) {
          return prev;
        }
        const next = { ...prev };
        delete next[sessionToDelete];
        return next;
      });
      if (currentSessionKey === sessionToDelete) {
        dispatch(setCurrentSessionKey(''));
      }
    } catch {
      alert(t('common.error'));
    }
    setDeleteDialogOpen(false);
    setSessionToDelete(null);
    setSessionToDeleteTitle('');
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
    const isDraftOnly = Boolean(draftSessions[editingSession]) && !sessions.some((s) => s.key === editingSession);
    try {
      if (!isDraftOnly) {
        await renameSession(editingSession, editTitle.trim());
      }
      setSessions((prev) => prev.map((s) => (s.key === editingSession ? { ...s, lastMessage: editTitle.trim() } : s)));
      setDraftSessions((prev) => {
        if (!prev[editingSession]) {
          return prev;
        }
        return {
          ...prev,
          [editingSession]: { ...prev[editingSession], lastMessage: editTitle.trim() }
        };
      });
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
          setDraftSessions((prev) => {
            if (Object.keys(prev).length === 0) {
              return prev;
            }
            const next = { ...prev };
            let changed = false;
            for (const item of list) {
              if (next[item.key]) {
                delete next[item.key];
                changed = true;
              }
            }
            return changed ? next : prev;
          });
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
    const mergedMap = new Map<string, SessionSummary>();
    Object.values(draftSessions).forEach((session) => {
      mergedMap.set(session.key, session);
    });
    sessions.forEach((session) => {
      mergedMap.set(session.key, session);
    });

    const currentChannel = extractSessionChannel(currentSessionKey);
    if (!mergedMap.has(currentSessionKey) && currentChannel === 'desktop') {
      mergedMap.set(currentSessionKey, buildDraftSession(currentSessionKey));
    }

    return Array.from(mergedMap.values()).sort((a, b) => {
      const ta = a.lastMessageAt ? Date.parse(a.lastMessageAt) : 0;
      const tb = b.lastMessageAt ? Date.parse(b.lastMessageAt) : 0;
      return tb - ta;
    });
  }, [sessions, draftSessions, currentSessionKey]);

  const channelOptions = useMemo(() => {
    const defaultOptions = [...DEFAULT_CHANNEL_ORDER];
    const defaultSet = new Set<string>(defaultOptions);
    const dynamicChannels = mergedSessions
      .map((session) => extractSessionChannel(session.key))
      .filter((channel) => !defaultSet.has(channel))
      .filter((channel, index, arr) => arr.indexOf(channel) === index)
      .sort((a, b) => a.localeCompare(b));

    return [...defaultOptions, ...dynamicChannels];
  }, [mergedSessions]);

  const sessionItems = useMemo(
    () =>
      mergedSessions
        .filter((session) => extractSessionChannel(session.key) === normalizeChannelKey(channelFilter))
        .slice(0, 20),
    [mergedSessions, channelFilter]
  );

  useEffect(() => {
    if (channelOptions.length === 0) {
      return;
    }
    const normalizedFilter = normalizeChannelKey(channelFilter);
    if (!channelOptions.includes(normalizedFilter)) {
      setChannelFilter(channelOptions[0]);
    }
  }, [channelOptions, channelFilter]);

  const handleNewTask = () => {
    const newSessionKey = `desktop:${Date.now()}`;
    setDraftSessions((prev) => {
      const next = { ...prev };
      if (
        currentSessionKey &&
        extractSessionChannel(currentSessionKey) === 'desktop' &&
        !sessions.some((session) => session.key === currentSessionKey) &&
        !next[currentSessionKey]
      ) {
        next[currentSessionKey] = buildDraftSession(currentSessionKey);
      }
      next[newSessionKey] = buildDraftSession(newSessionKey);
      return next;
    });
    setChannelFilter('desktop');
    dispatch(setCurrentSessionKey(newSessionKey));
    dispatch(setActiveTab('chat'));
  };

  if (sidebarCollapsed) {
    return null;
  }

  return (
    <aside
      className={`flex h-full w-72 flex-col ${isMac ? 'pt-10' : 'pt-2'}`}
      style={{ background: 'var(--secondary)' }}
    >
      {/* New Chat Button */}
      <div className="p-3">
        <button
          onClick={handleNewTask}
          className="w-full flex items-center justify-center gap-2 bg-primary/15 text-primary border border-primary/30 rounded-lg py-2.5 px-4 hover:bg-primary/20 transition-colors"
        >
          <EditIcon className="w-5 h-5 flex-shrink-0" />
          <span className="font-medium">{t('sidebar.newTask')}</span>
        </button>
      </div>

      {/* Menu Items */}
      <nav className="sidebar-scroll flex-1 px-2 overflow-y-auto pb-4">
        {menuItems.map((item) => {
          const Icon = item.icon;
          const isActive = activeTab === item.id;

          return (
            <button
              key={item.id}
              onClick={() => dispatch(setActiveTab(item.id))}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-xl mb-1 transition-all duration-200 group ${
                isActive
                  ? 'font-medium'
                  : 'hover:scale-[1.02]'
              }`}
              style={{
                background: isActive ? 'var(--active)' : 'transparent',
                color: isActive ? 'var(--foreground)' : 'var(--secondary-foreground)',
              }}
              onMouseEnter={(e) => {
                if (!isActive) {
                  e.currentTarget.style.background = 'var(--hover)';
                  e.currentTarget.style.color = 'var(--foreground)';
                }
              }}
              onMouseLeave={(e) => {
                if (!isActive) {
                  e.currentTarget.style.background = 'transparent';
                  e.currentTarget.style.color = 'var(--secondary-foreground)';
                }
              }}
            >
              <Icon className="w-5 h-5 flex-shrink-0 transition-transform duration-200 group-hover:scale-110" />
              <span>{t(item.labelKey)}</span>
            </button>
          );
        })}

        <div className="mt-4 px-2">
            <p className="text-xs font-medium mb-3 px-2" style={{ color: 'var(--muted)' }}>
              {t('sidebar.history')}
            </p>

            {/* Channel Filter Dropdown */}
            <div className="relative mb-3">
              <CustomSelect
                value={channelFilter}
                onChange={(value) => {
                  setChannelFilter(normalizeChannelKey(value));
                  setOpenMenuKey(null);
                }}
                options={channelOptions.map((channel) => ({
                  value: channel,
                  label: getChannelLabel(channel, language)
                }))}
                size="md"
              />
            </div>

            {/* Session List */}
            <div className="space-y-1 mt-2">
              {sessionItems.length === 0 && (
                <div className="text-sm px-3 py-2 rounded-xl" style={{ color: 'var(--muted)' }}>
                  {t('skills.empty')}
                </div>
              )}

              {sessionItems.map((session) => {
                const isCurrent = session.key === currentSessionKey;
                const isEditing = editingSession === session.key;
                const isMenuOpen = openMenuKey === session.key;

                if (isEditing) {
                  return (
                    <div
                      key={session.key}
                      className="px-3 py-2.5 rounded-xl"
                      style={{ background: 'var(--card)' }}
                    >
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
                        className="w-full text-sm font-medium bg-transparent border-0 border-b focus:outline-none focus:ring-0 pb-1"
                        style={{
                          borderColor: 'var(--primary)',
                          color: 'var(--foreground)'
                        }}
                      />
                      <p className="text-xs mt-1.5" style={{ color: 'var(--muted)' }}>
                        Enter to confirm, Esc to cancel
                      </p>
                    </div>
                  );
                }

                return (
                  <div
                    key={session.key}
                    className={`group relative flex cursor-pointer items-center gap-1 rounded-xl px-3 py-2.5 transition-colors duration-150 ${
                      !isCurrent ? 'hover:bg-[var(--hover)]' : ''
                    }`}
                    style={{
                      background: isCurrent ? 'var(--card)' : isMenuOpen ? 'var(--hover)' : 'transparent',
                      boxShadow: isCurrent ? '0 1px 3px rgba(0,0,0,0.05)' : 'none'
                    }}
                  >
                    <button
                      onClick={() => {
                        dispatch(setCurrentSessionKey(session.key));
                        dispatch(setActiveTab('chat'));
                      }}
                      className="flex-1 text-left min-w-0"
                    >
                      <p className="text-sm font-medium leading-5 truncate" style={{ color: 'var(--foreground)' }}>
                        {session.lastMessage || session.key.replace(/^desktop:/, '新任务')}
                      </p>
                      <p className="text-xs leading-5 mt-0.5" style={{ color: 'var(--muted)' }}>
                        {getChannelLabel(extractSessionChannel(session.key), language)} · {formatRelativeTime(session.lastMessageAt)}
                      </p>
                    </button>

                    {/* Menu Button */}
                    <div className="relative" ref={isMenuOpen ? menuRef : undefined}>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setOpenMenuKey(isMenuOpen ? null : session.key);
                        }}
                        className={`rounded-lg p-1.5 transition-opacity duration-150 ${
                          isMenuOpen ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'
                        }`}
                        style={{
                          background: isMenuOpen ? 'var(--hover)' : 'transparent'
                        }}
                      >
                        <DotsIcon className="w-4 h-4" style={{ color: 'var(--muted)' }} />
                      </button>

                      {/* Dropdown Menu */}
                      {isMenuOpen && (
                        <div
                          className="absolute right-10 top-1/2 z-50 w-36 -translate-y-1/2 rounded-xl border border-border/75 py-1"
                          style={{
                            background: 'var(--card)',
                            boxShadow: '0 4px 20px rgba(0,0,0,0.1)'
                          }}
                          onClick={(e) => e.stopPropagation()}
                        >
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              handleStartRename(session);
                            }}
                            className="mx-1 flex w-[calc(100%-8px)] items-center gap-2 rounded-lg px-3 py-2 text-left text-sm transition-colors duration-150"
                            style={{ color: 'var(--foreground)' }}
                            onMouseEnter={(e) => {
                              e.currentTarget.style.background = 'var(--hover)';
                            }}
                            onMouseLeave={(e) => {
                              e.currentTarget.style.background = 'transparent';
                            }}
                          >
                            <EditIcon className="w-3.5 h-3.5" />
                            {t('sidebar.rename')}
                          </button>
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              handleDelete(session);
                            }}
                            className="mx-1 flex w-[calc(100%-8px)] items-center gap-2 rounded-lg px-3 py-2 text-left text-sm transition-colors duration-150"
                            style={{ color: '#e74c3c' }}
                            onMouseEnter={(e) => {
                              e.currentTarget.style.background = 'rgba(231, 76, 60, 0.1)';
                            }}
                            onMouseLeave={(e) => {
                              e.currentTarget.style.background = 'transparent';
                            }}
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
      </nav>

      {/* Settings Button with Gateway Status */}
      <div
        className="sticky bottom-0 p-3"
        style={{
          background: 'var(--secondary)',
        }}
      >
        <button
          onClick={() => dispatch(setActiveTab('settings'))}
          className="w-full flex items-center gap-2 px-3 py-2.5 rounded-xl transition-all duration-200 hover:scale-[1.02] group"
          style={{
            background: 'transparent'
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.background = 'var(--hover)';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.background = 'transparent';
          }}
        >
          <div className="relative">
            <SettingsIcon
              className="w-4 h-4 flex-shrink-0 transition-transform duration-200 group-hover:rotate-45"
              style={{ color: 'var(--secondary-foreground)' }}
            />
            <div
              className="absolute -bottom-0.5 -right-0.5 w-1.5 h-1.5 rounded-full"
              style={{
                background: status === 'running' ? '#84a98c' : status === 'error' ? '#e74c3c' : '#f39c12'
              }}
            />
          </div>
          <span className="text-sm" style={{ color: 'var(--secondary-foreground)' }}>
            {t('nav.settings')}
          </span>
        </button>
      </div>

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        isOpen={deleteDialogOpen}
        title={t('sidebar.delete')}
        message={
          sessionToDeleteTitle
            ? language === 'zh'
              ? `确定要删除任务「${sessionToDeleteTitle}」吗？此操作不可恢复。`
              : `Delete task "${sessionToDeleteTitle}"? This action cannot be undone.`
            : t('sidebar.confirmDelete')
        }
        confirmText={t('common.delete')}
        cancelText={t('common.cancel')}
        onConfirm={confirmDelete}
        onCancel={() => {
          setDeleteDialogOpen(false);
          setSessionToDelete(null);
          setSessionToDeleteTitle('');
        }}
        variant="danger"
      />
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
