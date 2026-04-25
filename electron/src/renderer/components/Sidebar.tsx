import React, { useEffect, useMemo, useState, useRef } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { RootState, setActiveTab, setCurrentSessionKey, toggleSidebar } from '../store';
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

// Interface for cron execution record
interface ExecutionRecord {
  id: string;
  jobId: string;
  jobTitle: string;
  startedAt: string;
  endedAt?: string;
  status: 'running' | 'success' | 'failed';
  output: string;
  error?: string;
  durationMs: number;
}

const menuItems = [
  { id: 'sessions', labelKey: 'nav.sessions', icon: SearchIcon },
  { id: 'scheduled', labelKey: 'nav.scheduled', icon: ClockIcon },
  { id: 'skills', labelKey: 'nav.skills', icon: PuzzleIcon },
  { id: 'mcp', labelKey: 'nav.mcp', icon: ServerIcon },
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
  const [hasFailedCronJobs, setHasFailedCronJobs] = useState(false);

  // Delete/Rename state
  const [editingSession, setEditingSession] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [openMenuKey, setOpenMenuKey] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [sessionToDelete, setSessionToDelete] = useState<string | null>(null);
  const [sessionToDeleteTitle, setSessionToDeleteTitle] = useState('');
  const menuRef = useRef<HTMLDivElement>(null);
  const shouldSyncTaskContext = activeTab === 'chat' || activeTab === 'sessions' || activeTab === 'scheduled';

  const buildDraftSession = (key: string): SessionSummary => ({
    key,
    messageCount: 0,
    title: t('sidebar.newTask'),
    lastMessage: t('sidebar.newTask'),
    lastMessageAt: new Date().toISOString()
  });

  // Check for failed cron jobs
  useEffect(() => {
    const checkFailedJobs = async () => {
      try {
        // Fetch recent execution history
        const response = await fetch('http://127.0.0.1:18890/api/cron/history?limit=100');
        if (!response.ok) return;
        const data = await response.json();
        const records: ExecutionRecord[] = data.records || [];

        // Get all job IDs
        const jobsResponse = await fetch('http://127.0.0.1:18890/api/cron');
        if (!jobsResponse.ok) return;
        const jobsData = await jobsResponse.json();
        const jobIds: string[] = (jobsData.jobs || []).map((j: { id: string }) => j.id);

        // Find the latest execution record for each job
        const latestExecutions = new Map<string, ExecutionRecord>();
        for (const record of records) {
          if (!jobIds.includes(record.jobId)) continue;
          const existing = latestExecutions.get(record.jobId);
          if (!existing || new Date(record.startedAt) > new Date(existing.startedAt)) {
            latestExecutions.set(record.jobId, record);
          }
        }

        // Check if any job's latest execution failed
        let hasFailed = false;
        for (const record of latestExecutions.values()) {
          if (record.status === 'failed') {
            hasFailed = true;
            break;
          }
        }
        setHasFailedCronJobs(hasFailed);
      } catch {
        // Silently ignore errors (gateway might be down)
      }
    };

    void checkFailedJobs();
    const timer = setInterval(() => void checkFailedJobs(), 30000);
    return () => clearInterval(timer);
  }, []);

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
    const title = (session.title || '').trim();
    if (title !== '') {
      return title;
    }
    const fallback = (session.lastMessage || '').trim();
    if (fallback !== '') {
      return fallback;
    }
    return session.key.replace(/^desktop:/, t('sidebar.newTask'));
  };

  const getSessionPreview = (session: SessionSummary): string => {
    const preview = (session.lastMessage || '').trim();
    if (!preview) {
      return '';
    }
    return preview === getSessionDisplayTitle(session) ? '' : preview;
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
    setEditTitle(getSessionDisplayTitle(session));
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
      setSessions((prev) => prev.map((s) => (s.key === editingSession ? { ...s, title: editTitle.trim() } : s)));
      setDraftSessions((prev) => {
        if (!prev[editingSession]) {
          return prev;
        }
        return {
          ...prev,
          [editingSession]: { ...prev[editingSession], title: editTitle.trim() }
        };
      });
    } catch {
      alert(t('common.error'));
    }
    setEditingSession(null);
    setEditTitle('');
  };

  useEffect(() => {
    if (!shouldSyncTaskContext) {
      return;
    }

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
  }, [getSessions, shouldSyncTaskContext]);

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

  // Sync current session with channel filter - when switching channels, select the most recent session of that channel
  useEffect(() => {
    if (!shouldSyncTaskContext) {
      return;
    }

    if (channelOptions.length === 0) {
      return;
    }
    const normalizedFilter = normalizeChannelKey(channelFilter);
    if (!channelOptions.includes(normalizedFilter)) {
      setChannelFilter(channelOptions[0]);
      return;
    }

    // When channel filter changes, check if current session belongs to this channel
    const currentChannel = extractSessionChannel(currentSessionKey);
    if (currentChannel !== normalizedFilter) {
      // Find the most recent session of the selected channel
      const channelSessions = mergedSessions.filter(
        (session) => extractSessionChannel(session.key) === normalizedFilter
      );

      if (channelSessions.length > 0) {
        // Select the most recent session of this channel
        dispatch(setCurrentSessionKey(channelSessions[0].key));
      } else {
        // No sessions for this channel, create a draft session
        const newSessionKey = `${normalizedFilter}:${Date.now()}`;
        setDraftSessions((prev) => ({
          ...prev,
          [newSessionKey]: buildDraftSession(newSessionKey)
        }));
        dispatch(setCurrentSessionKey(newSessionKey));
      }
    }
  }, [channelFilter, channelOptions, mergedSessions, currentSessionKey, dispatch, shouldSyncTaskContext]);

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

  const statusTone =
    status === 'running'
      ? 'border-success/25 bg-success-bg text-success'
      : status === 'starting'
        ? 'border-warning/25 bg-warning-bg text-warning'
        : 'border-danger/25 bg-danger-bg text-danger';
  const statusLabel =
    language === 'zh'
      ? status === 'running'
        ? '在线'
        : status === 'starting'
          ? '启动中'
          : '离线'
      : status === 'running'
        ? 'Online'
        : status === 'starting'
          ? 'Starting'
          : 'Offline';

  if (sidebarCollapsed) {
    return null;
  }

  return (
    <aside
      className={`relative z-10 flex h-full w-[280px] shrink-0 flex-col overflow-hidden border-r border-border bg-background ${isMac ? 'pt-10' : 'pt-3'}`}
    >
      <div className="px-4 pb-3">
        <div className="rounded-xl border border-border bg-secondary px-4 py-4 text-foreground">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-2.5">
              <h2 className="text-[22px] font-semibold tracking-[-0.04em]">MaxClaw</h2>
              <div className={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] font-medium ${statusTone}`}>
                <span className={`status-dot ${status}`} />
                {statusLabel}
              </div>
            </div>
            <button
              onClick={() => dispatch(toggleSidebar())}
              className="flex h-7 w-7 items-center justify-center rounded-md text-muted transition-colors hover:bg-hover hover:text-foreground"
              aria-label="Collapse sidebar"
              title="Collapse sidebar"
            >
              <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <rect x="3" y="4" width="18" height="16" rx="2" strokeWidth={1.5} />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 4v16" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M14 9l-3 3 3 3" />
              </svg>
            </button>
          </div>
          <button
            onClick={handleNewTask}
            className="mt-3 flex w-full items-center justify-center gap-2 rounded-lg border border-border bg-background px-4 py-3 text-sm font-semibold text-foreground transition-all duration-150 hover:bg-hover dark:hover:bg-hover"
          >
            <EditIcon className="h-5 w-5 flex-shrink-0" />
            <span>{t('sidebar.newTask')}</span>
          </button>
        </div>
      </div>

      <nav className="sidebar-scroll min-h-0 flex-1 overflow-y-auto px-3 pb-4 [transform:translateZ(0)]">

        <div className="rounded-lg border border-border bg-secondary p-1">
          {menuItems.map((item) => {
            const Icon = item.icon;
            const isActive = activeTab === item.id;
            const showFailedBadge = item.id === 'scheduled' && hasFailedCronJobs && !isActive;

            return (
              <button
                key={item.id}
                onClick={() => dispatch(setActiveTab(item.id))}
                className={`group mb-1 flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-all duration-200 ${
                  isActive
                    ? 'border-l-2 border-primary bg-hover font-medium text-foreground'
                    : 'text-secondary-foreground hover:bg-hover hover:text-foreground'
                }`}
              >
                <div className="relative">
                  <Icon className="h-5 w-5 flex-shrink-0 transition-transform duration-200 group-hover:scale-110" />
                  {showFailedBadge && (
                    <span
                      className="absolute -right-1 -top-1 h-2.5 w-2.5 rounded-full bg-danger ring-2 ring-background"
                      title={language === 'zh' ? '有任务执行失败' : 'Some tasks have failed'}
                    />
                  )}
                </div>
                <span>{t(item.labelKey)}</span>
              </button>
            );
          })}
        </div>

        <div className="mt-5 px-2">
          <div className="mb-3 flex items-center justify-between gap-3 px-2">
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
              {t('sidebar.history')}
            </p>
            <span className="rounded-full border border-border bg-secondary px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.18em] text-muted">
              {sessionItems.length}
            </span>
          </div>

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
              triggerClassName="rounded-lg border-border bg-secondary dark:bg-secondary"
              menuClassName="border-border bg-background dark:bg-card"
            />
          </div>

          <div className="mt-3 space-y-1.5">
            {sessionItems.length === 0 && (
              <div className="rounded-lg border border-dashed border-border px-3 py-4 text-sm text-muted">
                {t('sidebar.empty')}
              </div>
            )}

            {sessionItems.map((session) => {
              const isCurrent = session.key === currentSessionKey;
              const isEditing = editingSession === session.key;
              const isMenuOpen = openMenuKey === session.key;
              const preview = getSessionPreview(session);

              if (isEditing) {
                return (
                  <div key={session.key} className="rounded-lg border border-border bg-background px-3 py-3 dark:bg-card">
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
                      className="w-full border-0 border-b bg-transparent pb-1 text-sm font-medium focus:outline-none focus:ring-0"
                      style={{
                        borderColor: 'var(--primary)',
                        color: 'var(--foreground)'
                      }}
                    />
                    <p className="mt-1.5 text-xs" style={{ color: 'var(--muted)' }}>
                      Enter to confirm, Esc to cancel
                    </p>
                  </div>
                );
              }

              return (
                <div
                  key={session.key}
                  className={`group relative flex cursor-pointer items-center gap-1 rounded-lg border px-3 py-3 transition-colors duration-150 ${
                    isCurrent
                      ? 'border-border bg-hover text-foreground'
                      : isMenuOpen
                        ? 'border-border bg-secondary'
                        : 'border-transparent hover:border-border hover:bg-secondary'
                  }`}
                >
                  <button
                    onClick={() => {
                      dispatch(setCurrentSessionKey(session.key));
                      dispatch(setActiveTab('chat'));
                    }}
                    className="min-w-0 flex-1 text-left"
                  >
                    <p className={`truncate text-[14px] font-medium leading-5 ${isCurrent ? 'text-foreground' : 'text-foreground'}`}>
                      {getSessionDisplayTitle(session)}
                    </p>
                    {preview && (
                      <p className={`truncate text-[11px] leading-4 ${isCurrent ? 'text-muted' : 'text-muted'}`}>
                        {preview}
                      </p>
                    )}
                    <p className={`mt-1 text-[11px] leading-5 ${isCurrent ? 'text-muted' : 'text-muted'}`}>
                      {getChannelLabel(extractSessionChannel(session.key), language)} · {formatRelativeTime(session.lastMessageAt)}
                    </p>
                  </button>

                  <div className="relative" ref={isMenuOpen ? menuRef : undefined}>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        setOpenMenuKey(isMenuOpen ? null : session.key);
                      }}
                      className={`rounded-xl p-1.5 transition-opacity duration-150 ${
                        isMenuOpen ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'
                      }`}
                      style={{
                        background: isMenuOpen ? 'var(--secondary)' : 'transparent'
                      }}
                    >
                      <DotsIcon className="h-4 w-4" style={{ color: isCurrent ? 'var(--muted)' : 'var(--muted)' }} />
                    </button>

                    {isMenuOpen && (
                      <div
                        className="absolute right-10 top-1/2 z-50 w-36 -translate-y-1/2 rounded-lg border border-border bg-background py-1 shadow-lg dark:bg-card"
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
                            style={{ color: 'var(--danger)' }}
                            onMouseEnter={(e) => {
                              e.currentTarget.style.background = 'var(--danger-bg)';
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
      <div className="mt-auto border-t border-border p-3">
        <button
          onClick={() => dispatch(setActiveTab('settings'))}
          className="group flex w-full items-center gap-2 rounded-lg px-3 py-3 transition-all duration-200 hover:bg-hover"
        >
          <div className="relative">
            <SettingsIcon
              className="w-4 h-4 flex-shrink-0 transition-transform duration-200 group-hover:rotate-45 text-secondary-foreground"
            />
            <div
              className="absolute -bottom-0.5 -right-0.5 w-1.5 h-1.5 rounded-full"
              style={{
                background: status === 'running' ? 'var(--success)' : status === 'error' ? 'var(--danger)' : 'var(--warning)'
              }}
            />
          </div>
          <span className="text-sm text-secondary-foreground">
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
    <img src="./icons/skills-hammer.png" alt="" className={className} />
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

function ServerIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 01-2 2v4a2 2 0 012 2h14a2 2 0 012-2v-4a2 2 0 01-2-2m-2-4h.01M17 16h.01" />
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
