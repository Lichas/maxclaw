import React, { useEffect, useMemo, useState } from 'react';
import { useDispatch } from 'react-redux';
import { setActiveTab, setCurrentSessionKey } from '../store';
import { SessionSummary, useGateway } from '../hooks/useGateway';

const baseChannelOptions = ['desktop', 'telegram', 'webui'] as const;
const channelLabelMap: Record<string, string> = {
  desktop: '桌面',
  telegram: 'Telegram',
  webui: 'WebUI'
};

function extractSessionChannel(sessionKey: string): string {
  const prefix = sessionKey.split(':', 2)[0];
  return prefix ? prefix.toLowerCase() : 'unknown';
}

function getChannelLabel(channel: string): string {
  return channelLabelMap[channel] || channel;
}

function formatRelativeTime(time?: string): string {
  if (!time) return '刚刚';
  const date = new Date(time);
  if (Number.isNaN(date.getTime())) return '刚刚';

  const diffMs = Date.now() - date.getTime();
  const minutes = Math.max(1, Math.floor(diffMs / 60000));
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d`;
}

export function SessionsView() {
  const dispatch = useDispatch();
  const { getSessions, deleteSession, renameSession } = useGateway();
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [loading, setLoading] = useState(true);

  // Search and filter states
  const [searchQuery, setSearchQuery] = useState('');
  const [channelFilter, setChannelFilter] = useState<string>('all');

  // Edit/Delete states
  const [editingSession, setEditingSession] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [openMenuKey, setOpenMenuKey] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadSessions = async () => {
      setLoading(true);
      try {
        const list = await getSessions();
        if (!cancelled) {
          setSessions(list);
        }
      } catch {
        if (!cancelled) {
          setSessions([]);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void loadSessions();
  }, [getSessions]);

  // Build channel options dynamically
  const channelOptions = useMemo(() => {
    const dynamicChannels = sessions
      .map((session) => extractSessionChannel(session.key))
      .filter((channel) => !baseChannelOptions.includes(channel as (typeof baseChannelOptions)[number]))
      .filter((channel, index, arr) => arr.indexOf(channel) === index)
      .sort((a, b) => a.localeCompare(b));

    return ['all', ...baseChannelOptions, ...dynamicChannels];
  }, [sessions]);

  // Filter and search sessions
  const filteredSessions = useMemo(() => {
    return sessions
      .filter((session) => {
        if (channelFilter === 'all') return true;
        return extractSessionChannel(session.key) === channelFilter;
      })
      .filter((session) => {
        if (!searchQuery.trim()) return true;
        const query = searchQuery.toLowerCase();
        return (
          (session.lastMessage?.toLowerCase().includes(query) ?? false) ||
          session.key.toLowerCase().includes(query)
        );
      })
      .sort((a, b) => {
        // Sort by lastMessageAt desc
        const aTime = a.lastMessageAt ? new Date(a.lastMessageAt).getTime() : 0;
        const bTime = b.lastMessageAt ? new Date(b.lastMessageAt).getTime() : 0;
        return bTime - aTime;
      });
  }, [sessions, channelFilter, searchQuery]);

  const handleDelete = async (sessionKey: string) => {
    if (!confirm('确定要删除这个会话吗？此操作不可恢复。')) return;
    try {
      await deleteSession(sessionKey);
      setSessions((prev) => prev.filter((s) => s.key !== sessionKey));
    } catch {
      alert('删除会话失败');
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
      alert('重命名失败');
    }
    setEditingSession(null);
    setEditTitle('');
  };

  const handleOpenSession = (sessionKey: string) => {
    dispatch(setCurrentSessionKey(sessionKey));
    dispatch(setActiveTab('chat'));
  };

  return (
    <div className="h-full overflow-y-auto bg-[#f7f8fb] p-6">
      <div className="mx-auto max-w-5xl">
        {/* Header */}
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-foreground">搜索任务</h1>
          <p className="mt-1 text-sm text-foreground/55">搜索和管理所有历史会话</p>
        </div>

        {/* Search and Filter Bar */}
        <div className="mb-6 flex flex-col sm:flex-row gap-3">
          {/* Search Input */}
          <div className="relative flex-1">
            <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-foreground/40" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="搜索会话内容..."
              className="w-full rounded-lg border border-border bg-background pl-9 pr-9 py-2.5 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery('')}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-foreground/40 hover:text-foreground"
              >
                <CloseIcon className="w-4 h-4" />
              </button>
            )}
          </div>

          {/* Channel Filter */}
          <div className="relative sm:w-48">
            <select
              value={channelFilter}
              onChange={(e) => setChannelFilter(e.target.value)}
              className="w-full appearance-none rounded-lg border border-border bg-background px-3 py-2.5 text-sm font-medium text-foreground/75 focus:border-primary/40 focus:outline-none"
            >
              <option value="all">所有渠道</option>
              {channelOptions
                .filter((c) => c !== 'all')
                .map((channel) => (
                  <option key={channel} value={channel}>
                    {getChannelLabel(channel)}
                  </option>
                ))}
            </select>
            <ChevronDownIcon className="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-foreground/45" />
          </div>
        </div>

        {/* Stats */}
        <div className="mb-4 text-sm text-foreground/50">
          共 {filteredSessions.length} 个会话
          {searchQuery && `（搜索 "${searchQuery}"）`}
          {channelFilter !== 'all' && ` · ${getChannelLabel(channelFilter)}`}
        </div>

        {/* Session List */}
        {loading ? (
          <div className="py-12 text-center text-foreground/50">加载中...</div>
        ) : filteredSessions.length === 0 ? (
          <div className="py-12 text-center">
            <p className="text-foreground/50">{searchQuery ? '未找到匹配的会话' : '暂无会话记录'}</p>
            {searchQuery && (
              <button
                onClick={() => {
                  setSearchQuery('');
                  setChannelFilter('all');
                }}
                className="mt-2 text-sm text-primary hover:underline"
              >
                清除筛选条件
              </button>
            )}
          </div>
        ) : (
          <div className="space-y-2">
            {filteredSessions.map((session) => {
              const isEditing = editingSession === session.key;

              if (isEditing) {
                return (
                  <div key={session.key} className="rounded-xl border border-border bg-background p-4 shadow-sm">
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
                      className="w-full text-base font-medium bg-transparent border-b border-primary/50 focus:outline-none focus:border-primary text-foreground"
                    />
                    <p className="text-xs text-foreground/40 mt-1">按 Enter 确认，Esc 取消</p>
                  </div>
                );
              }

              return (
                <div
                  key={session.key}
                  className="group rounded-xl border border-border bg-background p-4 shadow-sm hover:shadow-md transition-shadow"
                >
                  <div className="flex items-start justify-between gap-3">
                    <button
                      onClick={() => handleOpenSession(session.key)}
                      className="flex-1 text-left min-w-0"
                    >
                      <h3 className="font-semibold text-foreground truncate">
                        {session.lastMessage || session.key.replace(/^desktop:/, '新任务')}
                      </h3>
                      <div className="mt-1 flex items-center gap-3 text-xs text-foreground/50">
                        <span className="inline-flex items-center gap-1">
                          <span
                            className={`w-1.5 h-1.5 rounded-full ${
                              extractSessionChannel(session.key) === 'desktop'
                                ? 'bg-blue-500'
                                : extractSessionChannel(session.key) === 'telegram'
                                ? 'bg-sky-500'
                                : 'bg-purple-500'
                            }`}
                          />
                          {getChannelLabel(extractSessionChannel(session.key))}
                        </span>
                        <span>{session.messageCount || 0} 条消息</span>
                        <span>{formatRelativeTime(session.lastMessageAt)}</span>
                      </div>
                    </button>

                    {/* Actions */}
                    <div className="relative">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setOpenMenuKey(openMenuKey === session.key ? null : session.key);
                        }}
                        className="p-2 rounded-lg hover:bg-secondary text-foreground/50 hover:text-foreground transition-colors"
                      >
                        <DotsIcon className="w-4 h-4" />
                      </button>

                      {openMenuKey === session.key && (
                        <>
                          <div
                            className="fixed inset-0 z-40"
                            onClick={() => setOpenMenuKey(null)}
                          />
                          <div className="absolute right-0 top-full mt-1 w-32 rounded-lg border border-border bg-background shadow-lg z-50 py-1">
                            <button
                              onClick={() => handleStartRename(session)}
                              className="w-full px-3 py-2 text-sm text-left hover:bg-secondary flex items-center gap-2"
                            >
                              <EditIcon className="w-3.5 h-3.5" />
                              重命名
                            </button>
                            <button
                              onClick={() => handleDelete(session.key)}
                              className="w-full px-3 py-2 text-sm text-left hover:bg-red-50 text-red-600 flex items-center gap-2"
                            >
                              <TrashIcon className="w-3.5 h-3.5" />
                              删除
                            </button>
                          </div>
                        </>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

// Icons
function SearchIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
    </svg>
  );
}

function CloseIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
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

function EditIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
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
