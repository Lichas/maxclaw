import React, { Suspense, lazy, memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { RootState, setCurrentSessionKey, setSessionRunning, toggleTerminal } from '../store';
import { GatewayStreamEvent, SkillSummary, useGateway } from '../hooks/useGateway';
import { wsClient } from '../services/websocket';
import { MarkdownRenderer } from '../components/MarkdownRenderer';
import { FileAttachment, UploadedFile } from '../components/FileAttachment';
import { CustomSelect } from '../components/CustomSelect';
import { FilePreviewSidebar } from '../components/FilePreviewSidebar';
import { FileTreeSidebar } from '../components/FileTreeSidebar';
import { extractFileReferences, FileReference } from '../utils/fileReferences';
import { useTranslation } from '../i18n';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
  timeline?: TimelineEntry[];
  attachments?: UploadedFile[];
  durationMs?: number;
}

interface PreviewPayload {
  success: boolean;
  resolvedPath?: string;
  kind?: 'markdown' | 'text' | 'html' | 'image' | 'pdf' | 'audio' | 'video' | 'office' | 'binary';
  extension?: string;
  fileUrl?: string;
  content?: string;
  error?: string;
}

interface StreamActivity {
  type: 'status' | 'tool_start' | 'tool_result' | 'skill_start' | 'skill_result' | 'error';
  summary: string;
  detail?: string;
}

type TimelineEntry =
  | {
      id: string;
      kind: 'activity';
      activity: StreamActivity;
    }
  | {
      id: string;
      kind: 'text';
      text: string;
    };

const iterationStatusPattern = /^Iteration\s+\d+$/i;
const MODEL_PREFERENCE_KEY = 'nanobot.chat.preferredModel';

function isIterationStatus(summary: string): boolean {
  return iterationStatusPattern.test(summary.trim());
}

function shouldHideStatusInHistory(summary: string): boolean {
  const normalized = summary.trim().toLowerCase();
  return (
    normalized.startsWith('using model:') ||
    normalized === 'preparing final response' ||
    normalized === 'executing tools'
  );
}

function loadPreferredModel(): string {
  try {
    return window.localStorage.getItem(MODEL_PREFERENCE_KEY) || '';
  } catch {
    return '';
  }
}

function savePreferredModel(modelId: string): void {
  try {
    window.localStorage.setItem(MODEL_PREFERENCE_KEY, modelId);
  } catch {
    // Ignore persistence failures (e.g. storage unavailable).
  }
}

const LazyTerminalPanel = lazy(() =>
  import('../components/TerminalPanel').then((module) => ({ default: module.TerminalPanel }))
);

function formatSessionTitle(text?: string): string {
  if (!text) {
    return 'New thread';
  }

  const firstLine = text
    .split('\n')
    .map((line) => line.trim())
    .find((line) => line.length > 0);

  if (!firstLine) {
    return 'New thread';
  }

  const collapsed = firstLine.replace(/\s+/g, ' ');
  return collapsed.length > 72 ? `${collapsed.slice(0, 72)}...` : collapsed;
}

function formatDuration(ms: number): string {
  if (ms < 1000) {
    return `${ms}ms`;
  }
  if (ms < 60000) {
    return `${(ms / 1000).toFixed(1)}s`;
  }
  const minutes = Math.floor(ms / 60000);
  const seconds = ((ms % 60000) / 1000).toFixed(1);
  return `${minutes}m ${seconds}s`;
}

function pad2(value: number): string {
  return String(value).padStart(2, '0');
}

function formatMessageTimestamp(timestamp: Date): string {
  if (Number.isNaN(timestamp.getTime())) {
    return '';
  }

  const now = new Date();
  const isToday =
    timestamp.getFullYear() === now.getFullYear() &&
    timestamp.getMonth() === now.getMonth() &&
    timestamp.getDate() === now.getDate();

  const timeLabel = `${pad2(timestamp.getHours())}:${pad2(timestamp.getMinutes())}`;
  if (isToday) {
    return timeLabel;
  }

  return `${timestamp.getFullYear()}-${pad2(timestamp.getMonth() + 1)}-${pad2(timestamp.getDate())} ${timeLabel}`;
}

function formatWorkspaceLabel(path: string): string {
  const trimmed = path.trim();
  if (!trimmed) {
    return '';
  }

  const normalized = trimmed.replace(/[\\/]+$/, '');
  const segments = normalized.split(/[/\\]/).filter(Boolean);
  return segments[segments.length - 1] || normalized;
}

function copyMessageContent(content: string, language: 'zh' | 'en', showToast: (message: string) => void): void {
  void navigator.clipboard.writeText(content);
  showToast(language === 'zh' ? '已复制到剪贴板' : 'Copied to clipboard');
}

function fileReferenceCacheKey(sessionKey: string, pathHint: string): string {
  return `${sessionKey}::${pathHint.trim().toLowerCase()}`;
}

function isBrowserActivitySummary(summary: string): boolean {
  const normalized = summary.trim().toLowerCase();
  return normalized.startsWith('browser ') || normalized.startsWith('browser ->');
}

function isLoginInterventionText(text: string): boolean {
  const normalized = text.trim().toLowerCase();
  if (!normalized) {
    return false;
  }
  const keywords = [
    'login',
    'sign in',
    'signin',
    'passport',
    'oauth',
    'sso',
    'captcha',
    'verification',
    'security check',
    'access denied',
    'enable javascript',
    '请登录',
    '需要登录',
    '验证码',
    '启用javascript',
    '启用 javascript'
  ];
  return keywords.some((keyword) => normalized.includes(keyword));
}

function extractFirstURL(text: string): string {
  const matched = text.match(/https?:\/\/[^\s)>\]'"`]+/i);
  return matched ? matched[0] : '';
}

function resolveSessionTitleFromSummary(
  summary: { title?: string; lastMessage?: string } | undefined,
  fallbackTitle: string
): string {
  const preferredTitle = (summary?.title || '').trim();
  if (preferredTitle !== '') {
    return formatSessionTitle(preferredTitle);
  }
  const fallbackMessage = (summary?.lastMessage || '').trim();
  if (fallbackMessage !== '') {
    return formatSessionTitle(fallbackMessage);
  }
  return fallbackTitle;
}

function extractModelFromSessionDetail(session: {
  messages?: Array<{
    timeline?: Array<{
      kind?: string;
      activity?: {
        type?: string;
        summary?: string;
        detail?: string;
      };
    }>;
  }>;
}): string {
  const messages = session.messages || [];
  const patternList = [
    /using model:\s*([^\n]+)/i,
    /使用模型[:：]\s*([^\n]+)/i
  ];

  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const timeline = messages[i].timeline || [];
    for (let j = timeline.length - 1; j >= 0; j -= 1) {
      const entry = timeline[j];
      if (entry.kind !== 'activity') {
        continue;
      }
      const content = [entry.activity?.summary || '', entry.activity?.detail || '']
        .filter(Boolean)
        .join('\n');
      if (!content) {
        continue;
      }
      for (const pattern of patternList) {
        const matched = content.match(pattern);
        if (matched && matched[1]) {
          return matched[1].trim();
        }
      }
    }
  }

  return '';
}

const thinkTagPattern = /<think>([\s\S]*?)<\/think>/gi;

type ThinkSegment = {
  kind: 'text' | 'think';
  content: string;
};

function splitThinkSegments(content: string): ThinkSegment[] {
  if (!content.includes('<think>')) {
    return [{ kind: 'text', content }];
  }

  const segments: ThinkSegment[] = [];
  let cursor = 0;
  let matched: RegExpExecArray | null;

  thinkTagPattern.lastIndex = 0;
  while ((matched = thinkTagPattern.exec(content)) !== null) {
    const start = matched.index;
    const end = thinkTagPattern.lastIndex;
    if (start > cursor) {
      segments.push({ kind: 'text', content: content.slice(cursor, start) });
    }
    segments.push({ kind: 'think', content: (matched[1] || '').trim() });
    cursor = end;
  }

  if (cursor < content.length) {
    segments.push({ kind: 'text', content: content.slice(cursor) });
  }

  if (segments.length === 0) {
    return [{ kind: 'text', content }];
  }

  return segments;
}

function stripThinkTags(content: string): string {
  thinkTagPattern.lastIndex = 0;
  return content.replace(thinkTagPattern, (_, body: string) => body || '');
}

// Memoized message item component to prevent re-rendering on input
interface MemoizedMessageItemProps {
  message: Message;
  renderTimeline: (items: TimelineEntry[], streaming: boolean) => React.ReactNode;
  renderMarkdownWithActions: (content: string, keyPrefix: string) => React.ReactNode;
  formatMessageTimestamp: (date: Date) => string;
  formatDuration: (ms: number) => string;
  showToast: (message: string) => void;
  language: string;
}

const MemoizedMessageItem = memo(function MemoizedMessageItem({
  message,
  renderTimeline,
  renderMarkdownWithActions,
  formatMessageTimestamp,
  formatDuration,
  showToast,
  language
}: MemoizedMessageItemProps) {
  if (message.role === 'user') {
    return (
      <div className={`flex justify-end`}>
        <div className="max-w-3xl space-y-2">
          {message.attachments && message.attachments.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {message.attachments.map((file) => (
                <div
                  key={file.id}
                  className="flex items-center gap-1.5 rounded-full border border-border bg-secondary px-3 py-1.5 text-xs text-foreground"
                >
                  <DocumentIcon className="h-3.5 w-3.5 text-muted" />
                  <span className="max-w-[150px] truncate">{file.filename}</span>
                </div>
              ))}
            </div>
          )}
          <div className="group relative rounded-[28px] bg-[#d6f3e3] px-7 py-4 text-sm leading-7 text-[#12392e] shadow-[inset_0_0_0_1px_rgba(18,57,46,0.04)]">
            <pre className="whitespace-pre-wrap break-all font-sans selection:bg-[#12392e]/15">{message.content}</pre>
          </div>
          <div className="flex items-center justify-end gap-3 px-1 text-[11px] uppercase tracking-[0.12em] text-muted">
            <span>{formatMessageTimestamp(message.timestamp)}</span>
            <button
              type="button"
              onClick={() => copyMessageContent(message.content, language, showToast)}
              className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-muted transition-colors hover:bg-secondary hover:text-foreground"
              title={language === 'zh' ? '复制内容' : 'Copy content'}
            >
              <CopyIcon className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className={`flex justify-start`}>
      <div className="w-full space-y-3 text-foreground">
        <div className="px-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
          MaxClaw:
        </div>
        {message.timeline && message.timeline.length > 0 && (
          <div>
            {renderTimeline(message.timeline, false)}
          </div>
        )}
        <div className="rounded-xl border border-border bg-card px-5 py-4">
          {renderMarkdownWithActions(message.content, message.id)}
          <div className="mt-4 flex items-center gap-3 text-[11px] uppercase tracking-[0.12em] text-muted">
            <span>{formatMessageTimestamp(message.timestamp)}</span>
            {message.durationMs !== undefined && message.durationMs > 0 && (
              <>
                <span aria-hidden="true">·</span>
                <span className="inline-flex items-center gap-1.5">
                  <svg className="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <circle cx="12" cy="12" r="9" strokeWidth={1.5} />
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 7v5l3 3" />
                  </svg>
                  <span>{formatDuration(message.durationMs)}</span>
                </span>
              </>
            )}
            <button
              type="button"
              onClick={() => copyMessageContent(message.content, language, showToast)}
              className="ml-auto inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-muted transition-colors hover:bg-secondary hover:text-foreground"
              title={language === 'zh' ? '复制内容' : 'Copy content'}
            >
              <CopyIcon className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
});

export function ChatView() {
  const dispatch = useDispatch();
  const { t, language } = useTranslation();
  const { currentSessionKey, sidebarCollapsed, terminalVisible, renderThinkTags } = useSelector((state: RootState) => state.ui);
  const gatewayStatus = useSelector((state: RootState) => state.gateway.status);
  const isMac = window.electronAPI.platform.isMac;
  const { sendMessage, getSession, getSessions, getSkills, getModels, getConfig, updateConfig, runBrowserAction } =
    useGateway();

  const [messages, setMessages] = useState<Message[]>([]);
  const [sessionTitle, setSessionTitle] = useState('New thread');
  const [inputBySession, setInputBySession] = useState<Record<string, string>>({});
  const [streamingTimelineBySession, setStreamingTimelineBySession] = useState<Record<string, TimelineEntry[]>>({});
  const [availableSkills, setAvailableSkills] = useState<SkillSummary[]>([]);
  const [selectedSkills, setSelectedSkills] = useState<string[]>([]);
  const [skillsQuery, setSkillsQuery] = useState('');
  const [skillsPickerOpen, setSkillsPickerOpen] = useState(false);
  const [skillsLoadError, setSkillsLoadError] = useState<string | null>(null);
  const [headerSkillsOpen, setHeaderSkillsOpen] = useState(false);
  const [availableModels, setAvailableModels] = useState<Array<{ id: string; name: string; provider: string }>>([]);
  const [currentModel, setCurrentModel] = useState<string>('');
  const [sessionModelBySession, setSessionModelBySession] = useState<Record<string, string>>({});
  const [modelsLoading, setModelsLoading] = useState(false);
  const [workspacePath, setWorkspacePath] = useState('');
  const [previewSidebarCollapsed, setPreviewSidebarCollapsed] = useState(true);
  const [previewSidebarWidth, setPreviewSidebarWidth] = useState(460);
  const [selectedFileRef, setSelectedFileRef] = useState<FileReference | null>(null);
  const [previewData, setPreviewData] = useState<PreviewPayload | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewModeBySession, setPreviewModeBySession] = useState<Record<string, 'tree' | 'file' | 'browser'>>({});
  const [existingFileRefs, setExistingFileRefs] = useState<Record<string, boolean>>({});
  const [browserCopilotOutputBySession, setBrowserCopilotOutputBySession] = useState<Record<string, string>>({});
  const [browserCopilotBusyBySession, setBrowserCopilotBusyBySession] = useState<Record<string, boolean>>({});
  const [browserCopilotErrorBySession, setBrowserCopilotErrorBySession] = useState<Record<string, string>>({});

  // File attachments state
  const [attachedFiles, setAttachedFiles] = useState<UploadedFile[]>([]);

  // Interrupt state scoped by session.
  const [generatingSessions, setGeneratingSessions] = useState<Record<string, boolean>>({});
  const [interruptHintSessions, setInterruptHintSessions] = useState<Record<string, boolean>>({});

  // @mention skills state
  const [mentionOpen, setMentionOpen] = useState(false);
  const [mentionQuery, setMentionQuery] = useState('');
  const [mentionIndex, setMentionIndex] = useState(0);
  const mentionRef = useRef<HTMLDivElement>(null);

  // Slash commands state
  const [slashOpen, setSlashOpen] = useState(false);
  const [slashQuery, setSlashQuery] = useState('');
  const [slashIndex, setSlashIndex] = useState(0);
  const slashRef = useRef<HTMLDivElement>(null);
  const activeSessionModel = sessionModelBySession[currentSessionKey] || '';

  const starterCards = useMemo(
    () => [
      {
        title: t('chat.starter.card.report.title'),
        description: t('chat.starter.card.report.description'),
        prompt: t('chat.starter.card.report.prompt')
      },
      {
        title: t('chat.starter.card.meeting.title'),
        description: t('chat.starter.card.meeting.description'),
        prompt: t('chat.starter.card.meeting.prompt')
      },
      {
        title: t('chat.starter.card.bugfix.title'),
        description: t('chat.starter.card.bugfix.description'),
        prompt: t('chat.starter.card.bugfix.prompt')
      },
      {
        title: t('chat.starter.card.feature.title'),
        description: t('chat.starter.card.feature.description'),
        prompt: t('chat.starter.card.feature.prompt')
      },
      {
        title: t('chat.starter.card.plan.title'),
        description: t('chat.starter.card.plan.description'),
        prompt: t('chat.starter.card.plan.prompt')
      },
      {
        title: t('chat.starter.card.research.title'),
        description: t('chat.starter.card.research.description'),
        prompt: t('chat.starter.card.research.prompt')
      }
    ],
    [t]
  );

  const slashCommands = useMemo(
    () => [
      {
        id: 'new',
        label: '/new',
        description: '新建会话',
        action: () => {
          const newSessionKey = `desktop:${Date.now()}`;
          resetStreamingState(currentSessionKey);
          dispatch(setCurrentSessionKey(newSessionKey));
          setMessages([]);
          setSessionTitle('New thread');
          setPreviewSidebarCollapsed(true);
        }
      },
      {
        id: 'clear',
        label: '/clear',
        description: '清空当前会话消息',
        action: () => {
          setMessages([]);
          setPreviewSidebarCollapsed(true);
          resetStreamingState(currentSessionKey);
        }
      },
      {
        id: 'help',
        label: '/help',
        description: '显示帮助信息',
        action: () => {
          const helpMessage: Message = {
            id: `${Date.now()}-help`,
            role: 'assistant',
            content:
              '**可用命令：**\n\n' +
              '- `/new` - 创建新会话\n' +
              '- `/clear` - 清空当前会话\n' +
              '- `/help` - 显示帮助信息\n\n' +
              '**快捷操作：**\n' +
              '- `@技能名` - 在消息中引用技能\n' +
              '- `Shift+Enter` - 换行\n' +
              '- `Enter` - 发送消息',
            timestamp: new Date()
          };
          setMessages((prev) => [...prev, helpMessage]);
        }
      }
    ],
    [dispatch, currentSessionKey]
  );

  const filteredSlashCommands = useMemo(() => {
    const query = slashQuery.toLowerCase();
    return slashCommands.filter(
      (cmd) => cmd.label.toLowerCase().includes(query) || cmd.description.toLowerCase().includes(query)
    );
  }, [slashCommands, slashQuery]);

  const modelOptions = useMemo(
    () => {
      if (availableModels.length === 0) {
        return [{ value: '__no_model__', label: '未检测到可用模型', disabled: true }];
      }

      return availableModels.map((model) => ({
        value: model.id,
        label: `${model.provider} / ${model.name}`
      }));
    },
    [availableModels]
  );

  const loadSkills = useCallback(
    async (cancelledRef?: { current: boolean }) => {
      try {
        const skills = await getSkills();
        if (cancelledRef?.current) {
          return;
        }
        setAvailableSkills(skills.filter((skill) => skill.enabled !== false));
        setSkillsLoadError(null);
      } catch (err) {
        if (cancelledRef?.current) {
          return;
        }
        setAvailableSkills([]);
        setSkillsLoadError(err instanceof Error ? err.message : '加载技能失败');
      }
    },
    [getSkills]
  );

  const input = inputBySession[currentSessionKey] || '';
  const isGenerating = Boolean(generatingSessions[currentSessionKey]);
  const interruptHintVisible = Boolean(interruptHintSessions[currentSessionKey]);
  const browserCopilotOutput = browserCopilotOutputBySession[currentSessionKey] || '';
  const browserCopilotBusy = Boolean(browserCopilotBusyBySession[currentSessionKey]);
  const browserCopilotError = browserCopilotErrorBySession[currentSessionKey] || '';
  const streamingTimeline = streamingTimelineBySession[currentSessionKey] || [];

  const browserActivityContext = useMemo(() => {
    const collectTexts: string[] = [];
    const pushActivity = (activity?: StreamActivity) => {
      if (!activity || !isBrowserActivitySummary(activity.summary)) {
        return;
      }
      collectTexts.push(activity.summary);
      if (activity.detail) {
        collectTexts.push(activity.detail);
      }
    };

    messages.forEach((message) => {
      (message.timeline || []).forEach((entry) => {
        if (entry.kind === 'activity') {
          pushActivity(entry.activity);
        }
      });
    });
    streamingTimeline.forEach((entry) => {
      if (entry.kind === 'activity') {
        pushActivity(entry.activity);
      }
    });

    let latestURL = '';
    for (let index = collectTexts.length - 1; index >= 0; index -= 1) {
      const found = extractFirstURL(collectTexts[index]);
      if (found) {
        latestURL = found;
        break;
      }
    }

    const needsManualIntervention = collectTexts.some((text) => isLoginInterventionText(text));

    return {
      hasBrowserActivity: collectTexts.length > 0,
      latestURL,
      needsManualIntervention
    };
  }, [messages, streamingTimeline]);

  const inputRef = useRef<HTMLTextAreaElement>(null);
  const skillsPickerRef = useRef<HTMLDivElement>(null);
  const headerSkillsRef = useRef<HTMLDivElement>(null);
  const isComposingRef = useRef(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const entrySeqRef = useRef(0);
  const streamingTimelineBySessionRef = useRef<Record<string, TimelineEntry[]>>({});
  const previewRequestRef = useRef(0);
  const currentSessionKeyRef = useRef(currentSessionKey);
  const pendingFileRefChecksRef = useRef<Set<string>>(new Set());

  const isStarterMode = messages.length === 0 && streamingTimeline.length === 0;
  const workspaceLabel = useMemo(
    () => formatWorkspaceLabel(workspacePath) || (language === 'zh' ? '未配置工作空间' : 'No workspace'),
    [workspacePath, language]
  );
  const gatewayStatusMeta = useMemo(() => {
    if (gatewayStatus === 'running') {
      return {
        label: language === 'zh' ? 'Gateway 在线' : 'Gateway online',
        tone: 'border-success/20 bg-success-bg text-success'
      };
    }
    if (gatewayStatus === 'starting') {
      return {
        label: language === 'zh' ? 'Gateway 启动中' : 'Gateway starting',
        tone: 'border-warning/20 bg-warning-bg text-warning'
      };
    }
    return {
      label: language === 'zh' ? 'Gateway 离线' : 'Gateway offline',
      tone: 'border-danger/20 bg-danger-bg text-danger'
    };
  }, [gatewayStatus, language]);
  const selectedSkillSummaries = useMemo(
    () =>
      selectedSkills.map((skillName) => {
        const matched = availableSkills.find((skill) => skill.name === skillName);
        return {
          name: skillName,
          displayName: matched?.displayName || skillName,
          description: matched?.description || ''
        };
      }),
    [selectedSkills, availableSkills]
  );

  const setBrowserCopilotOutput = (sessionKey: string, value: string) => {
    setBrowserCopilotOutputBySession((prev) => {
      if ((prev[sessionKey] || '') === value) {
        return prev;
      }
      return { ...prev, [sessionKey]: value };
    });
  };

  const setBrowserCopilotBusy = (sessionKey: string, value: boolean) => {
    setBrowserCopilotBusyBySession((prev) => {
      if (value) {
        if (prev[sessionKey]) {
          return prev;
        }
        return { ...prev, [sessionKey]: true };
      }
      if (!prev[sessionKey]) {
        return prev;
      }
      const next = { ...prev };
      delete next[sessionKey];
      return next;
    });
  };

  const setBrowserCopilotError = (sessionKey: string, value: string) => {
    const trimmed = value.trim();
    setBrowserCopilotErrorBySession((prev) => {
      if (!trimmed) {
        if (!prev[sessionKey]) {
          return prev;
        }
        const next = { ...prev };
        delete next[sessionKey];
        return next;
      }
      if (prev[sessionKey] === trimmed) {
        return prev;
      }
      return { ...prev, [sessionKey]: trimmed };
    });
  };

  const setPreviewModeForSession = useCallback((sessionKey: string, mode: 'tree' | 'file' | 'browser') => {
    setPreviewModeBySession((prev) => {
      if (prev[sessionKey] === mode) {
        return prev;
      }
      return { ...prev, [sessionKey]: mode };
    });
  }, []);

  const setInputForSession = useCallback((sessionKey: string, value: string) => {
    setInputBySession((prev) => {
      if ((prev[sessionKey] || '') === value) {
        return prev;
      }
      return { ...prev, [sessionKey]: value };
    });
  }, []);

  const setInputForCurrentSession = useCallback((value: string) => {
    setInputForSession(currentSessionKey, value);
  }, [currentSessionKey, setInputForSession]);

  const clearInputForSession = useCallback((sessionKey: string) => {
    setInputBySession((prev) => {
      if (!(sessionKey in prev)) {
        return prev;
      }
      const next = { ...prev };
      delete next[sessionKey];
      return next;
    });
  }, []);

  const setSessionGenerating = (sessionKey: string, value: boolean) => {
    setGeneratingSessions((prev) => {
      if (value) {
        if (prev[sessionKey]) {
          return prev;
        }
        return { ...prev, [sessionKey]: true };
      }
      if (!prev[sessionKey]) {
        return prev;
      }
      const next = { ...prev };
      delete next[sessionKey];
      return next;
    });
  };

  const setSessionInterruptHint = (sessionKey: string, value: boolean) => {
    setInterruptHintSessions((prev) => {
      if (value) {
        if (prev[sessionKey]) {
          return prev;
        }
        return { ...prev, [sessionKey]: true };
      }
      if (!prev[sessionKey]) {
        return prev;
      }
      const next = { ...prev };
      delete next[sessionKey];
      return next;
    });
  };

  useEffect(() => {
    currentSessionKeyRef.current = currentSessionKey;
  }, [currentSessionKey]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streamingTimeline]);

  useEffect(() => {
    const cancelledRef = { current: false };
    void loadSkills(cancelledRef);
    return () => {
      cancelledRef.current = true;
    };
  }, [loadSkills]);

  useEffect(() => {
    if (!skillsPickerOpen) {
      return;
    }
    if (availableSkills.length > 0 && !skillsLoadError) {
      return;
    }
    void loadSkills();
  }, [skillsPickerOpen, availableSkills.length, skillsLoadError, loadSkills]);

  useEffect(() => {
    let cancelled = false;
    const sessionModel = activeSessionModel;

    const loadModels = async () => {
      try {
        setModelsLoading(true);
        const models = await getModels();
        const config = await getConfig() as { agents?: { defaults?: { model?: string } } };
        if (cancelled) return;
        setAvailableModels(models);
        if (models.length === 0) {
          setCurrentModel('');
          return;
        }

        const configuredModel = (config.agents?.defaults?.model || '').trim();
        const preferredModel = loadPreferredModel();
        const configuredExists = configuredModel !== '' && models.some((model) => model.id === configuredModel);
        const preferredExists = preferredModel !== '' && models.some((model) => model.id === preferredModel);
        const resolvedModel = configuredExists ? configuredModel : preferredExists ? preferredModel : models[0].id;
        const sessionModelExists = sessionModel !== '' && models.some((model) => model.id === sessionModel);

        setCurrentModel(sessionModelExists ? sessionModel : resolvedModel);
        if (preferredModel !== resolvedModel) {
          savePreferredModel(resolvedModel);
        }
      } catch {
        if (!cancelled) {
          setAvailableModels([]);
          setCurrentModel('');
        }
      } finally {
        if (!cancelled) setModelsLoading(false);
      }
    };

    void loadModels();
    return () => {
      cancelled = true;
    };
  }, [getConfig, getModels, activeSessionModel, currentSessionKey]);

  useEffect(() => {
    let cancelled = false;

    const loadWorkspace = async () => {
      try {
        const config = await getConfig() as { agents?: { defaults?: { workspace?: string } } };
        if (cancelled) {
          return;
        }
        setWorkspacePath(config.agents?.defaults?.workspace || '');
      } catch {
        if (!cancelled) {
          setWorkspacePath('');
        }
      }
    };

    void loadWorkspace();
    return () => {
      cancelled = true;
    };
  }, [getConfig]);

  useEffect(() => {
    const refsToCheck = new Map<string, FileReference>();
    const addReferences = (content: string) => {
      const refs = extractFileReferences(content);
      refs.forEach((reference) => {
        const key = fileReferenceCacheKey(currentSessionKey, reference.pathHint);
        refsToCheck.set(key, reference);
      });
    };

    addReferences(browserCopilotOutput);

    messages.forEach((message) => {
      addReferences(message.content);
      (message.timeline || []).forEach((entry) => {
        if (entry.kind === 'text') {
          addReferences(entry.text || '');
          return;
        }
        const activityText = [entry.activity.summary, entry.activity.detail || ''].filter(Boolean).join('\n');
        addReferences(activityText);
      });
    });

    streamingTimeline.forEach((entry) => {
      if (entry.kind === 'text') {
        addReferences(entry.text || '');
        return;
      }
      const activityText = [entry.activity.summary, entry.activity.detail || ''].filter(Boolean).join('\n');
      addReferences(activityText);
    });

    refsToCheck.forEach((reference, key) => {
      if (existingFileRefs[key] === true || pendingFileRefChecksRef.current.has(key)) {
        return;
      }
      pendingFileRefChecksRef.current.add(key);
      void window.electronAPI.system
        .fileExists(reference.pathHint, {
          workspace: workspacePath,
          sessionKey: currentSessionKey
        })
        .then((result) => {
          const exists = Boolean(result.exists && (result.isFile ?? true));
          setExistingFileRefs((prev) => {
            if (prev[key] === exists) {
              return prev;
            }
            return { ...prev, [key]: exists };
          });
        })
        .catch(() => {
          setExistingFileRefs((prev) => {
            if (prev[key] === false) {
              return prev;
            }
            return { ...prev, [key]: false };
          });
        })
        .finally(() => {
          pendingFileRefChecksRef.current.delete(key);
        });
    });
  }, [messages, streamingTimeline, currentSessionKey, workspacePath, existingFileRefs, browserCopilotOutput]);

  useEffect(() => {
    if (!skillsPickerOpen) {
      return;
    }

    const handleClickOutside = (event: MouseEvent) => {
      if (!skillsPickerRef.current) {
        return;
      }
      if (!skillsPickerRef.current.contains(event.target as Node)) {
        setSkillsPickerOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [skillsPickerOpen]);

  useEffect(() => {
    if (!headerSkillsOpen) {
      return;
    }

    const handleClickOutside = (event: MouseEvent) => {
      if (!headerSkillsRef.current) {
        return;
      }
      if (!headerSkillsRef.current.contains(event.target as Node)) {
        setHeaderSkillsOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [headerSkillsOpen]);

  const resetStreamingState = (sessionKey: string) => {
    setStreamingTimelineBySession((prev) => {
      if (!prev[sessionKey] || prev[sessionKey].length === 0) {
        return prev;
      }
      const next = { ...prev };
      delete next[sessionKey];
      streamingTimelineBySessionRef.current = next;
      return next;
    });
  };

  const setStreamingTimelineForSession = (
    sessionKey: string,
    updater: (prev: TimelineEntry[]) => TimelineEntry[]
  ) => {
    setStreamingTimelineBySession((prev) => {
      const current = prev[sessionKey] || [];
      const nextTimeline = updater(current);
      if (nextTimeline === current) {
        return prev;
      }
      const next =
        nextTimeline.length > 0
          ? { ...prev, [sessionKey]: nextTimeline }
          : (() => {
              if (!(sessionKey in prev)) {
                return prev;
              }
              const trimmed = { ...prev };
              delete trimmed[sessionKey];
              return trimmed;
            })();
      if (next === prev) {
        return prev;
      }
      streamingTimelineBySessionRef.current = next;
      return next;
    });
  };

  const appendTextToTimeline = (sessionKey: string, chunk: string) => {
    if (!chunk) {
      return;
    }
    setStreamingTimelineForSession(sessionKey, (prev) => {
      const last = prev[prev.length - 1];
      if (last && last.kind === 'text') {
        return [...prev.slice(0, -1), { ...last, text: last.text + chunk }];
      }
      return [
        ...prev,
        {
          id: nextEntryID('text'),
          kind: 'text',
          text: chunk
        }
      ];
    });
  };

  const nextEntryID = (prefix: string) => {
    entrySeqRef.current += 1;
    return `${prefix}-${entrySeqRef.current}`;
  };

  const filteredSkills = useMemo(() => {
    const query = skillsQuery.trim().toLowerCase();
    if (query === '') {
      return availableSkills;
    }
    return availableSkills.filter((skill) =>
      [skill.displayName, skill.name, skill.description || '']
        .join(' ')
        .toLowerCase()
        .includes(query)
    );
  }, [availableSkills, skillsQuery]);

  const mentionSkills = useMemo(() => {
    const query = mentionQuery.toLowerCase();
    return availableSkills
      .filter((skill) =>
        [skill.displayName, skill.name, skill.description || '']
          .join(' ')
          .toLowerCase()
          .includes(query)
      )
      .slice(0, 8);
  }, [availableSkills, mentionQuery]);

  const insertMention = (skillName: string) => {
    const beforeCursor = input.slice(0, input.lastIndexOf('@' + mentionQuery));
    const afterCursor = input.slice(input.lastIndexOf('@' + mentionQuery) + 1 + mentionQuery.length);
    setInputForCurrentSession(beforeCursor + '@' + skillName + ' ' + afterCursor);
    setMentionOpen(false);
    setMentionQuery('');
    setMentionIndex(0);
    inputRef.current?.focus();
  };

  const handleInputChange = useCallback((event: React.ChangeEvent<HTMLTextAreaElement>) => {
    const value = event.target.value;
    const cursorPosition = event.target.selectionStart || 0;
    setInputForCurrentSession(value);

    // Check for @mention
    const textBeforeCursor = value.slice(0, cursorPosition);
    const lastAtIndex = textBeforeCursor.lastIndexOf('@');
    const lastSlashIndex = textBeforeCursor.lastIndexOf('/');

    // Handle @mention
    if (lastAtIndex !== -1 && (lastSlashIndex === -1 || lastAtIndex > lastSlashIndex)) {
      const textAfterAt = textBeforeCursor.slice(lastAtIndex + 1);
      const hasSpaceAfterAt = textAfterAt.includes(' ');
      const isNewAt = textAfterAt === '' || (!hasSpaceAfterAt && textAfterAt.length < 20);

      if (isNewAt && (lastAtIndex === 0 || /\s/.test(textBeforeCursor[lastAtIndex - 1]))) {
        setMentionQuery(textAfterAt);
        setMentionOpen(true);
        setMentionIndex(0);
        setSlashOpen(false);
        return;
      }
    }

    // Handle /slash commands (only at start)
    if (lastSlashIndex !== -1 && lastSlashIndex === 0 && textBeforeCursor.length > 0) {
      const textAfterSlash = textBeforeCursor.slice(1);
      const hasSpace = textAfterSlash.includes(' ');

      if (!hasSpace && textAfterSlash.length < 20) {
        setSlashQuery(textAfterSlash);
        setSlashOpen(true);
        setSlashIndex(0);
        setMentionOpen(false);
        return;
      }
    }

    setMentionOpen(false);
    setSlashOpen(false);
  }, [setInputForCurrentSession]);

  const handleKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    // Handle mention navigation
    if (mentionOpen && mentionSkills.length > 0) {
      if (event.key === 'ArrowDown') {
        event.preventDefault();
        setMentionIndex((prev) => (prev + 1) % mentionSkills.length);
        return;
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault();
        setMentionIndex((prev) => (prev - 1 + mentionSkills.length) % mentionSkills.length);
        return;
      }
      if (event.key === 'Enter' || event.key === 'Tab') {
        event.preventDefault();
        insertMention(mentionSkills[mentionIndex].name);
        return;
      }
      if (event.key === 'Escape') {
        setMentionOpen(false);
        return;
      }
    }

    // Handle slash command navigation
    if (slashOpen && filteredSlashCommands.length > 0) {
      if (event.key === 'ArrowDown') {
        event.preventDefault();
        setSlashIndex((prev) => (prev + 1) % filteredSlashCommands.length);
        return;
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault();
        setSlashIndex((prev) => (prev - 1 + filteredSlashCommands.length) % filteredSlashCommands.length);
        return;
      }
      if (event.key === 'Enter' || event.key === 'Tab') {
        event.preventDefault();
        const cmd = filteredSlashCommands[slashIndex];
        if (cmd) {
          cmd.action();
          clearInputForSession(currentSessionKey);
        }
        setSlashOpen(false);
        return;
      }
      if (event.key === 'Escape') {
        setSlashOpen(false);
        return;
      }
    }

    const nativeEvent = event.nativeEvent as KeyboardEvent & { isComposing?: boolean; keyCode?: number };
    const isComposing = isComposingRef.current || nativeEvent.isComposing === true || nativeEvent.keyCode === 229;

    if (event.key === 'Enter' && !event.shiftKey && !isComposing) {
      event.preventDefault();
      if (isGenerating) {
        // Enter during generation = append more context
        handleInterrupt('append');
      } else {
        void handleSubmit(event);
      }
    }

    if (event.key === 'Enter' && event.shiftKey && !isComposing) {
      event.preventDefault();
      if (isGenerating) {
        // Shift+Enter during generation = interrupt and retry
        handleInterrupt('cancel');
      }
      // If not generating, Shift+Enter adds newline (default behavior)
    }
  };

  const toggleSkill = (name: string) => {
    setSelectedSkills((prev) =>
      prev.includes(name) ? prev.filter((skillName) => skillName !== name) : [...prev, name]
    );
  };

  const toStreamActivity = (event: GatewayStreamEvent): StreamActivity | null => {
    const trimDetail = (value?: string, max = 360) => {
      if (!value) {
        return undefined;
      }
      if (value.length <= max) {
        return value;
      }
      return `${value.slice(0, max)}...`;
    };

    switch (event.type) {
      case 'status': {
        const summary = event.message || event.summary;
        if (!summary) {
          return null;
        }
        if (isIterationStatus(summary)) {
          return null;
        }
        return {
          type: 'status',
          summary
        };
      }
      case 'tool_start': {
        const summary = event.summary || `${event.toolName || 'Tool'} started`;
        return {
          type: 'tool_start',
          summary,
          detail: trimDetail(event.toolArgs)
        };
      }
      case 'tool_result': {
        const summary = event.summary || `${event.toolName || 'Tool'} completed`;
        return {
          type: 'tool_result',
          summary,
          detail: trimDetail(event.toolResult)
        };
      }
      case 'skill_start': {
        const skillName = event.skillName || 'Skill';
        const summary = event.summary || `${skillName} started`;
        return {
          type: 'skill_start',
          summary,
          detail: trimDetail(event.skillDetail)
        };
      }
      case 'skill_result': {
        const skillName = event.skillName || 'Skill';
        const summary = event.summary || `${skillName} completed`;
        return {
          type: 'skill_result',
          summary,
          detail: trimDetail(event.skillDetail)
        };
      }
      case 'error':
        return {
          type: 'error',
          summary: event.error || '请求失败',
          detail: event.error || ''
        };
      default:
        return null;
    }
  };

  const appendActivityToTimeline = (sessionKey: string, activity: StreamActivity) => {
    setStreamingTimelineForSession(sessionKey, (prev) => {
      const last = prev[prev.length - 1];
      if (
        last &&
        last.kind === 'activity' &&
        last.activity.type === activity.type &&
        last.activity.summary === activity.summary &&
        last.activity.detail === activity.detail
      ) {
        return prev;
      }

      return [
        ...prev,
        {
          id: nextEntryID('activity'),
          kind: 'activity',
          activity
        }
      ];
    });
  };

  const getActivityLabel = useCallback((type: StreamActivity['type']) => {
    if (type === 'status') {
      return t('chat.timeline.label.thinking');
    }
    if (type === 'error') {
      return t('chat.timeline.label.error');
    }
    if (type === 'skill_start' || type === 'skill_result') {
      return t('chat.timeline.label.skill');
    }
    return t('chat.timeline.label.tool');
  }, [t]);

  const normalizeStoredTimeline = (
    entries: Array<{
      kind: 'activity' | 'text';
      activity?: {
        type: 'status' | 'tool_start' | 'tool_result' | 'skill_start' | 'skill_result' | 'error';
        summary: string;
        detail?: string;
      };
      text?: string;
    }> | undefined,
    prefix: string
  ): TimelineEntry[] | undefined => {
    if (!entries || entries.length === 0) {
      return undefined;
    }

    const normalized: TimelineEntry[] = [];
    entries.forEach((entry, index) => {
      if (entry.kind === 'activity' && entry.activity && entry.activity.summary) {
        if (
          entry.activity.type === 'status' &&
          (isIterationStatus(entry.activity.summary) || shouldHideStatusInHistory(entry.activity.summary))
        ) {
          return;
        }
        normalized.push({
          id: `${prefix}-activity-${index}`,
          kind: 'activity',
          activity: {
            type: entry.activity.type,
            summary: entry.activity.summary,
            detail: entry.activity.detail
          }
        });
        return;
      }

      if (entry.kind === 'text' && entry.text) {
        const last = normalized[normalized.length - 1];
        if (last && last.kind === 'text') {
          normalized[normalized.length - 1] = { ...last, text: last.text + entry.text };
        } else {
          normalized.push({
            id: `${prefix}-text-${index}`,
            kind: 'text',
            text: entry.text
          });
        }
      }
    });

    if (normalized.length === 0) {
      return undefined;
    }

    return normalized;
  };

  const resolveTitleFromMessages = (restored: Message[]): string => {
    const firstUserMessage = restored.find((message) => message.role === 'user' && message.content.trim().length > 0);
    if (firstUserMessage) {
      return formatSessionTitle(firstUserMessage.content);
    }

    const firstMessage = restored.find((message) => message.content.trim().length > 0);
    return formatSessionTitle(firstMessage?.content);
  };

  const reloadSession = useCallback(async (sessionKey: string) => {
    if (!sessionKey.trim()) {
      setMessages([]);
      setSessionTitle('New thread');
      setPreviewSidebarCollapsed(true);
      setSelectedFileRef(null);
      setPreviewData(null);
      setPreviewLoading(false);
      return;
    }

    setPreviewSidebarCollapsed(true);
    setSelectedFileRef(null);
    setPreviewData(null);
    setPreviewLoading(false);

    const session = await getSession(sessionKey);

    const sessionModel = extractModelFromSessionDetail(session);
    if (sessionModel) {
      setSessionModelBySession((prev) => {
        if (prev[sessionKey] === sessionModel) {
          return prev;
        }
        return { ...prev, [sessionKey]: sessionModel };
      });
      setCurrentModel(sessionModel);
    }

    const restored = (session.messages || [])
      .filter((message) => message.role === 'user' || message.role === 'assistant')
      .map((message, index) => ({
        id: `${sessionKey}-${index}`,
        role: message.role as 'user' | 'assistant',
        content: message.content,
        timestamp: new Date(message.timestamp),
        timeline: normalizeStoredTimeline(message.timeline, `${sessionKey}-${index}`)
      }));

    setMessages(restored);
    const fallbackTitle = resolveTitleFromMessages(restored);
    setSessionTitle(fallbackTitle);
    setPreviewSidebarCollapsed(true);

    try {
      const sessions = await getSessions();
      const matched = sessions.find((item) => item.key === sessionKey);
      if (matched) {
        setSessionTitle(resolveSessionTitleFromSummary(matched, fallbackTitle));
      } else {
        setSessionTitle(fallbackTitle);
      }
    } catch {
      setSessionTitle(fallbackTitle);
    }
  }, [getSession, getSessions]);

  useEffect(() => {
    let cancelled = false;
    setPreviewSidebarCollapsed(true);
    setSelectedFileRef(null);
    setPreviewData(null);
    setPreviewLoading(false);

    const loadSession = async () => {
      try {
        await reloadSession(currentSessionKey);
        if (cancelled) {
          return;
        }
      } catch {
        if (!cancelled) {
          setMessages([]);
          setSessionTitle('New thread');
          setPreviewSidebarCollapsed(true);
        }
      }
    };

    void loadSession();

    return () => {
      cancelled = true;
    };
  }, [currentSessionKey, reloadSession]);

  useEffect(() => {
    const unsubscribe = wsClient.on('chat', (payload: { sessionKey?: string }) => {
      const sessionKey = payload?.sessionKey || '';
      if (!sessionKey || sessionKey !== currentSessionKeyRef.current) {
        return;
      }
      void reloadSession(sessionKey);
    });

    return () => {
      unsubscribe();
    };
  }, [reloadSession]);

  const handleInterrupt = (mode: 'cancel' | 'append') => {
    if (!isGenerating || !currentSessionKey) {
      return;
    }

    const content = input.trim();
    const success = wsClient.sendInterrupt(currentSessionKey, mode, content);

    if (success) {
      // Clear input if content was sent
      if (content) {
        clearInputForSession(currentSessionKey);
      }
      // Show feedback toast
      showToast(mode === 'cancel' ? '已发送打断请求' : '已补充上下文');
      // Reset interrupt hint
      setSessionInterruptHint(currentSessionKey, false);
    }
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!input.trim() || isGenerating) {
      return;
    }
    const requestSessionKey = currentSessionKey.trim() || `desktop:${Date.now()}`;
    if (!currentSessionKey.trim()) {
      dispatch(setCurrentSessionKey(requestSessionKey));
    }
    let waitingForBackgroundResult = false;
    setPreviewSidebarCollapsed(true);
    setSessionGenerating(requestSessionKey, true);
    setSessionInterruptHint(requestSessionKey, true);
    dispatch(setSessionRunning({ sessionKey: requestSessionKey, running: true }));

    const userMessage: Message = {
      id: `${Date.now()}`,
      role: 'user',
      content: input.trim(),
      timestamp: new Date(),
      attachments: attachedFiles.length > 0 ? [...attachedFiles] : undefined
    };
    const shouldUpdateTitle = messages.length === 0;

    setMessages((prev) => [...prev, userMessage]);
    if (shouldUpdateTitle) {
      setSessionTitle(formatSessionTitle(userMessage.content));
    }
    clearInputForSession(requestSessionKey);
    setAttachedFiles([]);
    setSkillsPickerOpen(false);
    resetStreamingState(requestSessionKey);

    let assistantContent = '';
    const startTime = Date.now();

    try {
      const result = await sendMessage(
        userMessage.content,
        requestSessionKey,
        (delta) => {
          assistantContent += delta;
          appendTextToTimeline(requestSessionKey, delta);
        },
        (event) => {
          if (event.type === 'tool_result' && event.toolName === 'spawn') {
            waitingForBackgroundResult = true;
          }
          const activity = toStreamActivity(event);
          if (!activity) {
            return;
          }
          appendActivityToTimeline(requestSessionKey, activity);
        },
        selectedSkills,
        userMessage.attachments
      );

      if (result.sessionKey && result.sessionKey !== requestSessionKey) {
        dispatch(setCurrentSessionKey(result.sessionKey));
      }

      if (!assistantContent && result.response) {
        assistantContent = result.response;
        appendTextToTimeline(requestSessionKey, result.response);
      }

      const durationMs = Date.now() - startTime;
      const completedTimeline = streamingTimelineBySessionRef.current[requestSessionKey] || [];
      if (currentSessionKeyRef.current === requestSessionKey) {
        setMessages((prev) => [
          ...prev,
          {
            id: `${Date.now()}-assistant`,
            role: 'assistant',
            content: assistantContent,
            timestamp: new Date(),
            timeline: completedTimeline.length > 0 ? [...completedTimeline] : undefined,
            durationMs
          }
        ]);
        setPreviewSidebarCollapsed(true);
      }
      resetStreamingState(requestSessionKey);
    } catch (err) {
      const errorTimeline = streamingTimelineBySessionRef.current[requestSessionKey] || [];
      const durationMs = Date.now() - startTime;
      resetStreamingState(requestSessionKey);
      if (currentSessionKeyRef.current === requestSessionKey) {
        setMessages((prev) => [
          ...prev,
          {
            id: `${Date.now()}-error`,
            role: 'assistant',
            content: err instanceof Error ? `消息发送失败：${err.message}` : '消息发送失败，请检查 Gateway 状态后重试。',
            timestamp: new Date(),
            timeline: errorTimeline.length > 0 ? [...errorTimeline] : undefined,
            durationMs
          }
        ]);
        setPreviewSidebarCollapsed(true);
      }
    } finally {
      setSessionGenerating(requestSessionKey, false);
      setSessionInterruptHint(requestSessionKey, false);
      if (!waitingForBackgroundResult) {
        dispatch(setSessionRunning({ sessionKey: requestSessionKey, running: false }));
      }
    }
  };

  const applyTemplate = (prompt: string) => {
    setInputForCurrentSession(prompt);
    inputRef.current?.focus();
  };

  const handleModelChange = async (modelId: string) => {
    if (!modelId || modelId === currentModel) {
      return;
    }

    setCurrentModel(modelId);
    setSessionModelBySession((prev) => ({ ...prev, [currentSessionKey]: modelId }));
    savePreferredModel(modelId);

    try {
      await updateConfig({ model: modelId });
    } catch (err) {
      console.error('Failed to switch model:', err);
    }
  };

  const fallbackReferenceFromPath = useCallback((pathHint: string): FileReference | null => {
    const trimmed = pathHint.trim();
    if (!trimmed || /^https?:\/\//i.test(trimmed) || /^mailto:/i.test(trimmed)) {
      return null;
    }
    const cleaned = trimmed.replace(/^file:\/\//i, '');
    const normalized = cleaned.split('?')[0].split('#')[0];
    const slashIndex = Math.max(normalized.lastIndexOf('/'), normalized.lastIndexOf('\\'));
    const filename = slashIndex >= 0 ? normalized.slice(slashIndex + 1) : normalized;
    const dotIndex = filename.lastIndexOf('.');
    if (dotIndex <= 0) {
      return null;
    }

    return {
      id: cleaned.toLowerCase(),
      pathHint: cleaned,
      displayName: filename,
      extension: filename.slice(dotIndex).toLowerCase(),
      kind: 'binary'
    };
  }, []);

  const referenceFromHref = useCallback((href: string): FileReference | null => {
    const parsed = extractFileReferences(`[preview](${href})`);
    if (parsed.length > 0) {
      return parsed[0];
    }
    return fallbackReferenceFromPath(href);
  }, [fallbackReferenceFromPath]);

  const previewReference = useCallback(async (reference: FileReference) => {
    setPreviewModeForSession(currentSessionKey, 'file');
    setPreviewSidebarCollapsed(false);
    setSelectedFileRef(reference);
    setPreviewLoading(true);
    setPreviewData(null);

    previewRequestRef.current += 1;
    const requestID = previewRequestRef.current;
    try {
      const result = await window.electronAPI.system.previewFile(reference.pathHint, {
        workspace: workspacePath,
        sessionKey: currentSessionKey
      });
      if (requestID !== previewRequestRef.current) {
        return;
      }
      setPreviewData(result as PreviewPayload);
    } catch (error) {
      if (requestID !== previewRequestRef.current) {
        return;
      }
      setPreviewData({
        success: false,
        error: error instanceof Error ? error.message : String(error)
      });
    } finally {
      if (requestID === previewRequestRef.current) {
        setPreviewLoading(false);
      }
    }
  }, [currentSessionKey, setPreviewModeForSession, workspacePath]);

  const handleFileLinkPreview = useCallback((href: string): boolean => {
    const reference = referenceFromHref(href);
    if (!reference) {
      return false;
    }
    void previewReference(reference);
    return true;
  }, [referenceFromHref, previewReference]);

  const handleOpenSelectedFile = async () => {
    if (!selectedFileRef) {
      return;
    }
    const result = await window.electronAPI.system.openInFolder(selectedFileRef.pathHint, {
      workspace: workspacePath,
      sessionKey: currentSessionKey
    });
    if (!result.success) {
      setPreviewData({
        success: false,
        error: result.error || '打开所在目录失败'
      });
    }
  };

  const handleOpenFilePath = async () => {
    if (!selectedFileRef) {
      return;
    }
    const result = await window.electronAPI.system.openPath(selectedFileRef.pathHint, {
      workspace: workspacePath,
      sessionKey: currentSessionKey
    });
    if (!result.success) {
      setPreviewData({
        success: false,
        error: result.error || '打开文件失败'
      });
    }
  };

  const handleBrowserCopilotAction = async (params: Record<string, unknown>) => {
    const requestSessionKey = currentSessionKey;
    setPreviewModeForSession(requestSessionKey, 'browser');
    setPreviewSidebarCollapsed(false);
    setBrowserCopilotBusy(requestSessionKey, true);
    setBrowserCopilotError(requestSessionKey, '');
    try {
      const result = await runBrowserAction(requestSessionKey, params);
      const resultText = (result.result || '').trim();
      setBrowserCopilotOutput(requestSessionKey, resultText);

      const references = extractFileReferences(resultText);
      if (references.length > 0) {
        await previewReference(references[0]);
      }
    } catch (error) {
      setBrowserCopilotError(requestSessionKey, error instanceof Error ? error.message : String(error));
    } finally {
      setBrowserCopilotBusy(requestSessionKey, false);
    }
  };

  const handleBrowserImageClick = async ({ x, y }: { x: number; y: number }) => {
    await handleBrowserCopilotAction({
      action: 'act',
      act: 'click_xy',
      x,
      y,
      wait_ms: 900
    });
    await handleBrowserCopilotAction({
      action: 'screenshot',
      full_page: false
    });
  };

  const renderFileActions = useCallback((content: string, keyPrefix: string) => {
    const references = extractFileReferences(content).filter(
      (reference) => existingFileRefs[fileReferenceCacheKey(currentSessionKey, reference.pathHint)] === true
    );
    if (references.length === 0) {
      return null;
    }

    return (
      <div className="mt-2 flex flex-wrap gap-2">
        {references.map((reference, index) => (
          <div
            key={`${keyPrefix}-${reference.id}-${index}`}
            className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-secondary px-2 py-1 text-xs text-secondary-foreground"
          >
            <DocumentIcon className="h-3.5 w-3.5 text-muted" />
            <span className="max-w-[190px] truncate">{reference.displayName}</span>
            <button
              type="button"
              onClick={() => void previewReference(reference)}
              className="rounded border border-border bg-background px-1.5 py-0.5 text-[11px] text-muted transition-colors hover:bg-secondary"
            >
              预览
            </button>
            <button
              type="button"
              onClick={() =>
                void window.electronAPI.system.openInFolder(reference.pathHint, {
                  workspace: workspacePath,
                  sessionKey: currentSessionKey
                })
              }
              className="rounded border border-border bg-background px-1.5 py-0.5 text-[11px] text-muted transition-colors hover:bg-secondary"
            >
              打开目录
            </button>
          </div>
        ))}
      </div>
    );
  }, [currentSessionKey, existingFileRefs, previewReference, workspacePath]);

  const renderMarkdownWithActions = useCallback((content: string, keyPrefix: string) => {
    if (!renderThinkTags || !content.includes('<think>')) {
      return (
        <div className="space-y-1.5">
          <MarkdownRenderer content={content} onFileLinkClick={handleFileLinkPreview} />
          {renderFileActions(content, keyPrefix)}
        </div>
      );
    }

    const segments = splitThinkSegments(content);
    const thinkTitle = language === 'zh' ? '思考' : 'Thinking';
    const contentForFileActions = stripThinkTags(content);

    return (
      <div className="space-y-1.5">
        {segments.map((segment, index) => {
          if (!segment.content.trim()) {
            return null;
          }

          if (segment.kind === 'think') {
            return (
              <details
                key={`${keyPrefix}-think-${index}`}
                className="overflow-hidden rounded-lg border border-border bg-secondary"
              >
                <summary className="cursor-pointer list-none px-3 py-2 text-sm font-medium text-secondary-foreground">
                  {thinkTitle}
                </summary>
                <div className="border-t border-border/60 px-3 py-2">
                  <MarkdownRenderer content={segment.content} onFileLinkClick={handleFileLinkPreview} />
                </div>
              </details>
            );
          }

          return (
            <MarkdownRenderer
              key={`${keyPrefix}-text-${index}`}
              content={segment.content}
              onFileLinkClick={handleFileLinkPreview}
            />
          );
        })}
        {renderFileActions(contentForFileActions, keyPrefix)}
      </div>
    );
  }, [handleFileLinkPreview, language, renderFileActions, renderThinkTags]);

  const renderTimeline = useCallback((items: TimelineEntry[], streaming: boolean) => {
    const openIndex =
      streaming && items.length > 0 && items[items.length - 1].kind === 'activity' ? items.length - 1 : -1;
    const activityItems = items.filter(
      (entry): entry is Extract<TimelineEntry, { kind: 'activity' }> => entry.kind === 'activity'
    );

    const renderActivityItem = (
      entry: Extract<TimelineEntry, { kind: 'activity' }>,
      defaultOpen?: boolean
    ) => {
      const activityContent = [entry.activity.summary, entry.activity.detail || ''].filter(Boolean).join('\n');
      const fileActions = renderFileActions(activityContent, `${entry.id}-activity-files`);
      return (
        <details
          key={entry.id}
          open={defaultOpen}
          className="overflow-hidden rounded-lg border border-border bg-secondary"
        >
          <summary className="cursor-pointer list-none px-4 py-3">
            <div className="flex items-center gap-2 text-sm text-secondary-foreground">
              <ActivityTypeIcon type={entry.activity.type} className="h-4 w-4 flex-shrink-0" />
              <span className="text-[11px] font-semibold uppercase tracking-[0.2em] text-muted">
                {getActivityLabel(entry.activity.type)}
              </span>
              <span className="truncate">{entry.activity.summary}</span>
              <ChevronDownIcon className="ml-auto h-3.5 w-3.5 flex-shrink-0 text-muted" />
            </div>
          </summary>
          {entry.activity.detail && (
            <pre className="whitespace-pre-wrap break-all border-t border-border px-4 py-3 font-sans text-sm text-muted">
              {entry.activity.detail}
            </pre>
          )}
          {fileActions && (
            <div className={entry.activity.detail ? 'px-4 pb-3' : 'border-t border-border/60 px-4 py-3'}>
              {fileActions}
            </div>
          )}
        </details>
      );
    };

    if (!streaming) {
      return (
        <div className="space-y-3">
          {activityItems.length > 0 && (
            <details className="overflow-hidden rounded-lg border border-border bg-secondary">
              <summary className="cursor-pointer list-none px-4 py-3">
                <div className="flex items-center gap-2 text-sm text-secondary-foreground">
                  <WorkflowIcon className="h-4 w-4 flex-shrink-0" />
                  <span className="font-medium">执行过程（{activityItems.length} 步）</span>
                  <span className="text-xs text-muted">默认折叠，点击展开</span>
                  <ChevronDownIcon className="ml-auto h-3.5 w-3.5 flex-shrink-0 text-muted" />
                </div>
              </summary>
              <div className="space-y-2 border-t border-border px-3 py-3">
                {activityItems.map((entry) => renderActivityItem(entry))}
              </div>
            </details>
          )}
        </div>
      );
    }

    return (
      <div className="space-y-3">
        <div className="space-y-2">
          {items.map((entry, index) =>
            entry.kind === 'activity' ? renderActivityItem(entry, index === openIndex) : (
              <div key={entry.id} className="text-foreground">
                {renderMarkdownWithActions(entry.text, entry.id)}
              </div>
            )
          )}
        </div>
      </div>
    );
  }, [getActivityLabel, renderFileActions, renderMarkdownWithActions]);

  const browserCopilotURL = browserActivityContext.latestURL || extractFirstURL(browserCopilotOutput);
  const browserCopilotVisible = Boolean(
    browserActivityContext.hasBrowserActivity || browserCopilotOutput || browserCopilotError
  );
  const storedPreviewMode = previewModeBySession[currentSessionKey];
  const previewSidebarMode =
    storedPreviewMode === 'browser' && !browserCopilotVisible
      ? (selectedFileRef ? 'file' : 'tree')
      : storedPreviewMode || (browserCopilotVisible ? 'browser' : selectedFileRef ? 'file' : 'tree');
  const browserCopilotNeedsManualIntervention = Boolean(
    browserActivityContext.needsManualIntervention ||
      isLoginInterventionText(browserCopilotOutput) ||
      isLoginInterventionText(browserCopilotError)
  );
  const browserScreenshotInteractive = Boolean(
    selectedFileRef &&
      previewData?.success &&
      previewData.kind === 'image' &&
      (((previewData.resolvedPath || '').includes('/screenshots/') ||
        (previewData.resolvedPath || '').includes('\\screenshots\\')) ||
        selectedFileRef.displayName.toLowerCase().startsWith('browser-'))
  );

  const renderBrowserCopilotPanel = () => {
    if (!browserCopilotVisible) {
      return null;
    }

    return (
      <div className="rounded-xl border border-border bg-card p-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-muted">Browser Co-Pilot</p>
            <p className="text-sm text-secondary-foreground">
              {browserCopilotNeedsManualIntervention
                ? '检测到登录/验证拦截，需要你在真实浏览器介入后再继续同步。'
                : '默认自动执行；仅在检测到登录/验证时才需要你手动介入。'}
            </p>
          </div>
          {browserCopilotURL && (
            <button
              type="button"
              onClick={() =>
                void handleBrowserCopilotAction({
                  action: 'open',
                  url: browserCopilotURL
                })
              }
              disabled={browserCopilotBusy || isGenerating}
              className="rounded-md border border-border bg-background px-2 py-1 text-xs text-muted transition-colors hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-60"
            >
              用当前Profile打开页面
            </button>
          )}
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={() => void handleBrowserCopilotAction({ action: 'screenshot', full_page: false })}
            disabled={browserCopilotBusy}
            className="rounded-md border border-border bg-background px-3 py-1.5 text-xs text-secondary-foreground transition-colors hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-60"
          >
            {browserCopilotBusy ? '同步中...' : '同步截图'}
          </button>
          <button
            type="button"
            onClick={() => void handleBrowserCopilotAction({ action: 'snapshot', max_chars: 12000 })}
            disabled={browserCopilotBusy}
            className="rounded-md border border-border bg-background px-3 py-1.5 text-xs text-secondary-foreground transition-colors hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-60"
          >
            抓取结构快照
          </button>
          <button
            type="button"
            onClick={() => {
              setInputForCurrentSession('我已在真实浏览器完成手动操作，请基于当前页面状态继续执行任务。');
              inputRef.current?.focus();
            }}
            className="rounded-md border border-primary/40 bg-accent px-3 py-1.5 text-xs text-primary transition-colors hover:bg-accent"
          >
            插入继续指令
          </button>
        </div>

        {browserScreenshotInteractive && (
          <p className="mt-2 text-[11px] text-muted">
            当前预览为浏览器截图，可直接在右侧预览图点击位置，系统会把坐标回传给浏览器执行点击并自动刷新截图。
          </p>
        )}

        {browserCopilotError && (
          <p className="mt-2 rounded-md border border-danger/25 bg-danger-bg px-2 py-1 text-xs text-danger">
            {browserCopilotError}
          </p>
        )}

        {browserCopilotOutput && (
          <div className="mt-2 rounded-lg border border-border bg-secondary px-2 py-2 text-xs text-muted">
            {renderMarkdownWithActions(browserCopilotOutput, `${currentSessionKey}-browser-copilot-output`)}
          </div>
        )}
      </div>
    );
  };

  const renderComposer = (landing: boolean) => (
    <form
      onSubmit={handleSubmit}
      className={`relative overflow-visible rounded-xl border border-border bg-card ${
        landing ? 'p-6' : 'p-5'
      }`}
    >
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted">
            {landing ? (language === 'zh' ? 'Task Brief' : 'Task Brief') : language === 'zh' ? 'Thread Composer' : 'Thread Composer'}
          </p>
          <p className="mt-2 text-sm text-muted">
            {landing
              ? language === 'zh'
                ? '把目标、限制和验收条件一次说清楚，剩下的交给 MaxClaw。'
                : 'Describe the goal, constraints, and acceptance bar in one pass.'
              : language === 'zh'
                ? '继续补充上下文，或者直接交给 MaxClaw 执行下一步。'
                : 'Add context or hand off the next execution step.'}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {!landing && (
            <>
              <span className="inline-flex items-center gap-2 rounded-full border border-border bg-secondary px-3 py-1.5 text-[11px] font-medium tracking-[0.14em] text-muted">
                <FolderIcon className="h-3.5 w-3.5" />
                {workspaceLabel}
              </span>
              <span className="inline-flex items-center rounded-full border border-border bg-secondary px-3 py-1.5 text-[11px] font-medium tracking-[0.14em] text-muted">
                {currentModel || (language === 'zh' ? '自动模型' : 'Auto model')}
              </span>
            </>
          )}
          {selectedSkills.length > 0 && (
            <span className="inline-flex items-center rounded-full border border-primary/20 bg-accent px-3 py-1.5 text-[11px] font-medium tracking-[0.14em] text-primary">
              {selectedSkills.length} {language === 'zh' ? '个技能已激活' : 'skills active'}
            </span>
          )}
        </div>
      </div>
      <div className="rounded-lg border border-border bg-secondary px-3 py-3">
      <textarea
        ref={inputRef}
        value={input}
        onChange={handleInputChange}
        onCompositionStart={() => {
          isComposingRef.current = true;
        }}
        onCompositionEnd={() => {
          isComposingRef.current = false;
        }}
        onKeyDown={handleKeyDown}
        placeholder={language === 'zh' ? '描述你的任务目标、上下文和输出要求...' : 'Describe the goal, context, and expected output...'}
        rows={landing ? 6 : 4}
        className={`w-full resize-none border-none bg-transparent px-2 py-1 leading-7 text-foreground placeholder:text-muted focus:outline-none ${
          landing ? 'text-[15px]' : 'text-sm'
        }`}
      />
      </div>

      {/* @mention dropdown */}
      {mentionOpen && mentionSkills.length > 0 && (
        <div
          ref={mentionRef}
          className="absolute left-4 bottom-24 z-40 w-64 rounded-xl border border-border bg-background p-2 shadow-xl"
        >
          <div className="mb-1 px-2 py-1 text-xs text-muted">选择技能</div>
          <div className="max-h-48 overflow-y-auto">
            {mentionSkills.map((skill, index) => (
              <button
                key={skill.name}
                type="button"
                onClick={() => insertMention(skill.name)}
                className={`w-full rounded-lg px-2 py-2 text-left text-xs transition-colors ${
                  index === mentionIndex ? 'bg-accent text-primary' : 'hover:bg-secondary'
                }`}
              >
                <div className="font-medium">@{skill.displayName || skill.name}</div>
                {skill.description && (
                  <div className="truncate text-muted">{skill.description}</div>
                )}
              </button>
            ))}
          </div>
          <div className="mt-1 border-t border-border px-2 pt-1 text-[10px] text-muted">
            ↑↓ 选择 · Enter/Tab 确认 · Esc 关闭
          </div>
        </div>
      )}

      {/* /slash command dropdown */}
      {slashOpen && filteredSlashCommands.length > 0 && (
        <div
          ref={slashRef}
          className="absolute left-4 bottom-24 z-40 w-56 rounded-xl border border-border bg-background p-2 shadow-xl"
        >
          <div className="mb-1 px-2 py-1 text-xs text-muted">快捷命令</div>
          <div className="max-h-48 overflow-y-auto">
            {filteredSlashCommands.map((cmd, index) => (
              <button
                key={cmd.id}
                type="button"
                onClick={() => {
                  cmd.action();
                  clearInputForSession(currentSessionKey);
                  setSlashOpen(false);
                }}
                className={`w-full rounded-lg px-2 py-2 text-left transition-colors ${
                  index === slashIndex ? 'bg-accent text-primary' : 'hover:bg-secondary'
                }`}
              >
                <div className="font-medium text-sm">{cmd.label}</div>
                <div className="text-xs text-muted">{cmd.description}</div>
              </button>
            ))}
          </div>
          <div className="mt-1 border-t border-border px-2 pt-1 text-[10px] text-muted">
            ↑↓ 选择 · Enter/Tab 确认 · Esc 关闭
          </div>
        </div>
      )}

      <div className="mt-5 flex items-end justify-between gap-3 border-t border-border pt-4">
        <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
          <CustomSelect
            value={currentModel}
            onChange={handleModelChange}
            options={modelOptions}
            placeholder="选择模型..."
            disabled={modelsLoading || isGenerating}
            size="sm"
            className="w-[220px] max-w-full"
            triggerClassName="rounded-full border-border bg-secondary"
            menuClassName="border-border bg-background"
          />
          {modelsLoading && <span className="text-xs text-muted">加载中...</span>}
          <div ref={skillsPickerRef} className="relative flex items-center gap-2 text-xs text-muted">
            <span className="inline-flex items-center gap-1 rounded-full border border-border bg-secondary px-2.5 py-1.5">
              <FolderIcon className="h-3.5 w-3.5" />
              {workspaceLabel}
            </span>
            <FileAttachment
              attachedFiles={attachedFiles}
              onFilesUploaded={(files) => setAttachedFiles((prev) => [...prev, ...files])}
              onRemoveFile={(id) => setAttachedFiles((prev) => prev.filter((f) => f.id !== id))}
              disabled={isGenerating}
            />
            <button
              type="button"
              onClick={() => setSkillsPickerOpen((prev) => !prev)}
              className={`inline-flex items-center gap-1 rounded-xl px-2.5 py-1.5 transition-colors ${
                selectedSkills.length > 0 || skillsPickerOpen
                  ? 'bg-accent text-primary shadow-sm'
                  : 'border border-border bg-secondary text-muted hover:bg-secondary'
              }`}
            >
              <PuzzleIcon className="h-3.5 w-3.5" />
              skills{selectedSkills.length > 0 ? `(${selectedSkills.length})` : ''}
            </button>
            {selectedSkills.length > 0 && (
              <button
                type="button"
                onClick={() => setSelectedSkills([])}
                className="rounded-md border border-border px-1.5 py-1 text-[11px] text-muted hover:bg-secondary"
              >
                清空
              </button>
            )}

            {skillsPickerOpen && (
              <div className="absolute bottom-10 left-0 z-30 w-80 rounded-lg border border-border bg-background p-3 shadow-lg">
                <input
                  value={skillsQuery}
                  onChange={(event) => setSkillsQuery(event.target.value)}
                  placeholder="搜索技能"
                  className="mb-2 w-full rounded-md border border-border bg-background px-2 py-1.5 text-xs text-foreground placeholder:text-muted focus:border-primary/40 focus:outline-none"
                />
                <div className="max-h-56 space-y-1 overflow-y-auto pr-1">
                  {filteredSkills.map((skill) => {
                    const checked = selectedSkills.includes(skill.name);
                    return (
                      <label
                        key={skill.name}
                        className="flex cursor-pointer items-start gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-secondary/70"
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => toggleSkill(skill.name)}
                          className="mt-0.5 h-3.5 w-3.5 rounded border-border text-primary focus:ring-primary/30"
                        />
                        <span className="min-w-0">
                          <span className="block truncate font-medium text-foreground">{skill.displayName || skill.name}</span>
                          {skill.description && (
                            <span className="block truncate text-muted">{skill.description}</span>
                          )}
                        </span>
                      </label>
                    );
                  })}
                  {filteredSkills.length === 0 && (
                    <div className="px-2 py-1 text-xs text-muted">没有匹配的技能</div>
                  )}
                </div>
                {skillsLoadError && (
                  <p className="mt-2 text-xs text-danger">技能加载失败: {skillsLoadError}</p>
                )}
                <p className="mt-2 text-[11px] text-muted">
                  已选择 {selectedSkills.length} 个技能。未选择时按系统默认策略加载。
                </p>
              </div>
            )}
          </div>
        </div>

        {isGenerating ? (
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => handleInterrupt('append')}
              disabled={!input.trim()}
              title="Enter 补充上下文"
              className="inline-flex h-10 items-center gap-1.5 rounded-full border border-border bg-secondary px-4 text-sm font-medium text-foreground transition-colors hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-50"
            >
              <PlusIcon className="h-4 w-4" />
              补充
            </button>
            <button
              type="button"
              onClick={() => handleInterrupt('cancel')}
              title="Shift+Enter 打断并重试"
              className="inline-flex h-10 items-center gap-1.5 rounded-full bg-destructive px-4 text-sm font-medium text-destructive-foreground transition-colors hover:bg-destructive/90"
            >
              <StopIcon className="h-4 w-4" />
              打断
            </button>
          </div>
        ) : (
          <button
            type="submit"
            disabled={!input.trim() || isGenerating}
            className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-primary text-primary-foreground transition-colors duration-150 hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <SendIcon className="h-4 w-4" />
          </button>
        )}
      </div>
      {interruptHintVisible && isGenerating && (
        <div className="mt-2 flex items-center justify-center gap-4 text-xs text-muted">
          <span className="flex items-center gap-1">
            <kbd className="rounded bg-secondary px-1.5 py-0.5 text-[10px]">Enter</kbd>
            补充上下文
          </span>
          <span className="flex items-center gap-1">
            <kbd className="rounded bg-secondary px-1.5 py-0.5 text-[10px]">Shift+Enter</kbd>
            打断并重试
          </span>
        </div>
      )}
    </form>
  );

  const renderThreadHeader = () => (
    <div
      className={`relative z-20 border-b border-border bg-background ${
        isMac && sidebarCollapsed ? 'pl-44 pr-7' : 'px-7'
      }`}
    >
      <div className="flex min-h-[88px] flex-wrap items-center justify-between gap-4 py-5">
        <div className="min-w-0">
          <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-muted">
            {isStarterMode ? 'Control Room' : 'Active Thread'}
          </p>
          <h1 className="mt-2 truncate text-[20px] font-semibold tracking-[-0.04em] text-foreground">{sessionTitle}</h1>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <span className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-[11px] font-medium tracking-[0.14em] ${gatewayStatusMeta.tone}`}>
              <span className={`status-dot ${gatewayStatus}`} />
              {gatewayStatusMeta.label}
            </span>
            <span className="inline-flex items-center rounded-full border border-border bg-secondary px-3 py-1.5 text-[11px] font-medium tracking-[0.14em] text-muted">
              {workspaceLabel}
            </span>
            <span className="inline-flex items-center rounded-full border border-border bg-secondary px-3 py-1.5 text-[11px] font-medium tracking-[0.14em] text-muted">
              {currentModel || (language === 'zh' ? '自动模型' : 'Auto model')}
            </span>
            <span className="inline-flex items-center rounded-full border border-border bg-secondary px-3 py-1.5 text-[11px] font-medium tracking-[0.14em] text-muted">
              {messages.length} {language === 'zh' ? '条消息' : 'messages'}
            </span>
            {selectedSkills.length > 0 && (
              <div ref={headerSkillsRef} className="relative">
                <button
                  type="button"
                  onClick={() => setHeaderSkillsOpen((prev) => !prev)}
                  className="inline-flex items-center gap-2 rounded-full border border-primary/20 bg-accent px-3 py-1.5 text-[11px] font-medium tracking-[0.14em] text-primary transition-colors hover:bg-accent"
                >
                  <span>{language === 'zh' ? `技能 ${selectedSkills.length}` : `Skills ${selectedSkills.length}`}</span>
                  <ChevronDownIcon className={`h-3.5 w-3.5 transition-transform ${headerSkillsOpen ? 'rotate-180' : ''}`} />
                </button>
                {headerSkillsOpen && (
                  <div className="absolute left-0 top-[calc(100%+0.45rem)] z-40 w-72 overflow-hidden rounded-lg border border-border bg-background p-2 shadow-md">
                    <div className="px-2 pb-2 pt-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                      {language === 'zh' ? '本次任务技能' : 'Task skills'}
                    </div>
                    <div className="space-y-1">
                      {selectedSkillSummaries.map((skill) => (
                        <div
                          key={skill.name}
                          className="rounded-lg border border-border bg-secondary px-3 py-2"
                        >
                          <div className="text-sm font-medium text-foreground">{skill.displayName}</div>
                          {skill.description && (
                            <div className="mt-0.5 line-clamp-2 text-xs leading-5 text-muted">
                              {skill.description}
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
        {!isStarterMode && (
          <div className="relative z-30 ml-auto flex items-center gap-2 no-drag">
            <button
              type="button"
              onClick={() => {
                if (!previewSidebarCollapsed && previewSidebarMode !== 'browser') {
                  setPreviewSidebarCollapsed(true);
                } else {
                  setPreviewModeForSession(currentSessionKey, 'tree');
                  setPreviewSidebarCollapsed(false);
                }
              }}
              className={`flex h-10 items-center gap-2 rounded-full border px-4 text-xs font-medium uppercase tracking-[0.18em] transition-colors ${
                !previewSidebarCollapsed && previewSidebarMode !== 'browser'
                  ? 'border-primary/30 bg-accent text-primary'
                  : 'border-border bg-secondary text-muted hover:bg-secondary hover:text-foreground'
              }`}
              aria-label="Toggle file preview"
              title="Toggle file preview"
            >
              <FolderIcon className="h-3.5 w-3.5" />
              <span>{language === 'zh' ? '文件' : 'Files'}</span>
            </button>
            {browserCopilotVisible && (
              <button
                type="button"
                onClick={() => {
                  if (!previewSidebarCollapsed && previewSidebarMode === 'browser') {
                    setPreviewSidebarCollapsed(true);
                  } else {
                    setPreviewModeForSession(currentSessionKey, 'browser');
                    setPreviewSidebarCollapsed(false);
                  }
                }}
                className={`flex h-10 items-center gap-2 rounded-full border px-4 text-xs font-medium uppercase tracking-[0.18em] transition-colors ${
                  !previewSidebarCollapsed && previewSidebarMode === 'browser'
                    ? 'border-primary/30 bg-accent text-primary'
                    : 'border-border bg-secondary text-muted hover:bg-secondary hover:text-foreground'
                }`}
                aria-label="Toggle browser copilot"
                title="Toggle browser copilot"
              >
                <BrowserIcon className="h-3.5 w-3.5" />
                <span>Browser</span>
              </button>
            )}
            <button
              type="button"
              onClick={() => dispatch(toggleTerminal())}
              className={`flex h-10 items-center gap-2 rounded-full border px-4 text-xs font-medium uppercase tracking-[0.18em] transition-colors ${
                terminalVisible
                  ? 'border-primary/30 bg-accent text-primary'
                  : 'border-border bg-secondary text-muted hover:bg-secondary hover:text-foreground'
              }`}
              aria-label="Toggle terminal"
              title="Toggle terminal"
            >
              <TerminalIcon className="h-3.5 w-3.5" />
              <span>Terminal</span>
            </button>
          </div>
        )}
      </div>
    </div>
  );

  const renderPreviewSidebar = () => (
    <FilePreviewSidebar
      collapsed={previewSidebarCollapsed}
      width={previewSidebarWidth}
      selected={selectedFileRef}
      preview={previewData}
      loading={previewLoading}
      mode={previewSidebarMode}
      browserAvailable={browserCopilotVisible}
      browserPanel={renderBrowserCopilotPanel()}
      treePanel={(
        <FileTreeSidebar
          sessionKey={currentSessionKey}
          workspacePath={workspacePath}
          onSelectFile={(path) => {
            // 创建 FileReference 并预览
            const fileRef: FileReference = {
              id: path.toLowerCase(),
              pathHint: path,
              displayName: path.split(/[/\\]/).pop() || path,
              extension: path.split('.').pop() || '',
              kind: 'binary'
            };
            void previewReference(fileRef);
          }}
          selectedPath={selectedFileRef?.pathHint}
          onOpenDirectory={(dirPath) => {
            void window.electronAPI.system.openInFolder(dirPath, {
              workspace: workspacePath,
              sessionKey: currentSessionKey
            });
          }}
        />
      )}
      onModeChange={(mode) => {
        setPreviewModeForSession(currentSessionKey, mode);
      }}
      onToggle={() => setPreviewSidebarCollapsed((prev) => !prev)}
      onResize={setPreviewSidebarWidth}
      onOpenFile={() => {
        void handleOpenSelectedFile();
      }}
      onOpenPath={() => {
        void handleOpenFilePath();
      }}
      imageAssist={{
        enabled: browserScreenshotInteractive && !browserCopilotBusy,
        busy: browserCopilotBusy,
        hint: browserCopilotBusy ? '正在执行 browser 点击...' : '点击截图即可回传坐标到浏览器执行点击。',
        onImageClick: ({ x, y }) => {
          void handleBrowserImageClick({ x, y });
        }
      }}
    />
  );

  // Memoized message list to prevent re-rendering on input
  const renderedMessages = useMemo(() => {
    return messages.map((message) => (
      <MemoizedMessageItem
        key={message.id}
        message={message}
        renderTimeline={renderTimeline}
        renderMarkdownWithActions={renderMarkdownWithActions}
        formatMessageTimestamp={formatMessageTimestamp}
        formatDuration={formatDuration}
        showToast={showToast}
        language={language}
      />
    ));
  }, [
    messages,
    renderTimeline,
    renderMarkdownWithActions,
    formatMessageTimestamp,
    formatDuration,
    showToast,
    language
  ]);

  if (isStarterMode) {
    return (
      <div className="flex h-full flex-col bg-transparent">
        {renderThreadHeader()}
        <div className="flex min-h-0 flex-1">
          <div className="flex-1 overflow-y-auto px-8 py-8 md:px-10 md:py-10">
            <div className="mx-auto max-w-4xl">
              <section className="rounded-xl border border-border bg-card p-5 md:p-6">
                {renderComposer(true)}
              </section>

              <section className="mt-6">
                <div className="mb-4 flex items-center justify-between">
                  <p className="text-xs font-semibold uppercase tracking-[0.22em] text-muted">{t('chat.starter.templates')}</p>
                  <div className="hidden h-px flex-1 bg-border md:ml-4 md:block" />
                </div>
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                  {starterCards.map((card) => (
                    <button
                      key={card.title}
                      onClick={() => applyTemplate(card.prompt)}
                      className="group rounded-lg border border-border bg-card px-5 py-5 text-left transition-all duration-150 hover:border-primary/30 hover:bg-secondary"
                    >
                      <p className="text-[18px] font-semibold tracking-[-0.03em] text-foreground">{card.title}</p>
                      <p className="mt-2 text-sm leading-6 text-muted">{card.description}</p>
                      <p className="mt-5 text-[11px] font-semibold uppercase tracking-[0.2em] text-primary">
                        {language === 'zh' ? '使用模板' : 'Use template'}
                      </p>
                    </button>
                  ))}
                </div>
              </section>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col bg-transparent">
      {renderThreadHeader()}
      <div className="flex min-h-0 flex-1">
        <div className="min-w-0 flex flex-1 flex-col">
          <div className="flex-1 overflow-y-auto px-6 py-6">
            <div className="mx-auto flex w-full max-w-[980px] flex-col gap-4">
              {renderedMessages}

              {streamingTimeline.length > 0 && (
                <div className="flex justify-start">
                  <div className="w-full space-y-3 text-sm leading-7 text-foreground">
                    <div className="px-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                      MaxClaw:
                    </div>
                    <div className="rounded-xl border border-border bg-card px-5 py-4">
                      <div className="mb-4 text-[11px] uppercase tracking-[0.14em] text-muted">
                        {language === 'zh' ? '执行中' : 'Running'}
                      </div>
                      {renderTimeline(streamingTimeline, true)}
                      <span className="ml-1 mt-4 inline-block h-4 w-2 animate-pulse bg-primary" />
                    </div>
                  </div>
                </div>
              )}

              <div ref={messagesEndRef} />
            </div>
          </div>

          <div className="border-t border-border px-6 py-5">
            <div className="mx-auto max-w-[980px]">
              {renderComposer(false)}
              {terminalVisible && (
                <Suspense
                  fallback={
                    <div className="mt-3 rounded-lg border border-border bg-secondary px-4 py-4 text-xs text-muted">
                      Loading terminal...
                    </div>
                  }
                >
                  <LazyTerminalPanel key={currentSessionKey} sessionKey={currentSessionKey} />
                </Suspense>
              )}
            </div>
          </div>
        </div>
        {renderPreviewSidebar()}
      </div>
    </div>
  );
}

function SendIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
    </svg>
  );
}

function FolderIcon({ className }: { className?: string }) {
  return (
    <img src="./icons/workspace-folder.png" alt="" className={className} />
  );
}

function TerminalIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <rect x={3} y={5} width={18} height={14} rx={2.5} strokeWidth={1.8} />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.8} d="M7 9l3 3-3 3m5 0h5" />
    </svg>
  );
}

function BrowserIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <rect x={4} y={6} width={16} height={12} rx={2.5} strokeWidth={1.7} />
      <circle cx={9} cy={12} r={1.2} strokeWidth={1.4} />
      <circle cx={15} cy={12} r={1.2} strokeWidth={1.4} />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.7} d="M12 6V3m-3 15h6" />
    </svg>
  );
}

function DocumentIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
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

function WorkflowIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <circle cx="6" cy="6" r="2.5" strokeWidth={1.8} />
      <circle cx="18" cy="6" r="2.5" strokeWidth={1.8} />
      <circle cx="12" cy="18" r="2.5" strokeWidth={1.8} />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.8} d="M8.2 7.2l2.6 7.6m4.4-7.6l-2.6 7.6M8.3 6h7.4" />
    </svg>
  );
}

function ActivityTypeIcon({ className, type }: { className?: string; type: StreamActivity['type'] }) {
  if (type === 'status') {
    return (
      <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <ellipse cx="12" cy="12" rx="7.2" ry="3.1" strokeWidth={1.55} />
        <ellipse cx="12" cy="12" rx="7.2" ry="3.1" strokeWidth={1.55} transform="rotate(60 12 12)" />
        <ellipse cx="12" cy="12" rx="7.2" ry="3.1" strokeWidth={1.55} transform="rotate(-60 12 12)" />
        <circle cx="12" cy="12" r="1.25" fill="currentColor" stroke="none" />
        <circle cx="18.1" cy="12" r="1.05" fill="currentColor" stroke="none" />
        <circle cx="8.95" cy="17.2" r="1.05" fill="currentColor" stroke="none" />
        <circle cx="9.1" cy="6.7" r="1.05" fill="currentColor" stroke="none" />
      </svg>
    );
  }

  if (type === 'error') {
    return (
      <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="9" strokeWidth={1.8} />
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.8} d="M12 8v5m0 3h.01" />
      </svg>
    );
  }

  if (type === 'skill_start' || type === 'skill_result') {
    return <PuzzleIcon className={className} />;
  }

  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={1.8}
        d="M14.8 6.2a3.1 3.1 0 01-4.2 4.2l-5 5a1.4 1.4 0 101.98 1.98l5-5a3.1 3.1 0 004.2-4.2l-1.66 1.67-1.82-.35-.35-1.82 1.85-1.48z"
      />
    </svg>
  );
}

function ChevronDownIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 9l6 6 6-6" />
    </svg>
  );
}

function CopyIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <rect x="9" y="9" width="13" height="13" rx="2" ry="2" strokeWidth={2} />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
    </svg>
  );
}

function PlusIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
    </svg>
  );
}

function StopIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="currentColor" viewBox="0 0 24 24">
      <rect x="6" y="6" width="12" height="12" rx="2" />
    </svg>
  );
}

function showToast(message: string) {
  const toast = document.createElement('div');
  toast.className = 'fixed bottom-4 left-1/2 z-50 -translate-x-1/2 rounded-lg bg-foreground px-4 py-2 text-sm text-background shadow-lg';
  toast.textContent = message;
  document.body.appendChild(toast);
  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transition = 'opacity 0.3s';
    setTimeout(() => toast.remove(), 300);
  }, 2000);
}
