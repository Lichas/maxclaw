import React, { Suspense, lazy, useEffect, useMemo, useRef, useState } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { RootState, setCurrentSessionKey } from '../store';
import { GatewayStreamEvent, SkillSummary, useGateway } from '../hooks/useGateway';
import { wsClient } from '../services/websocket';
import { MarkdownRenderer } from '../components/MarkdownRenderer';
import { FileAttachment, UploadedFile } from '../components/FileAttachment';
import { CustomSelect } from '../components/CustomSelect';
import { FilePreviewSidebar } from '../components/FilePreviewSidebar';
import { extractFileReferences, FileReference } from '../utils/fileReferences';

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
  kind?: 'markdown' | 'text' | 'image' | 'pdf' | 'audio' | 'video' | 'office' | 'binary';
  extension?: string;
  fileUrl?: string;
  content?: string;
  error?: string;
}

interface StreamActivity {
  type: 'status' | 'tool_start' | 'tool_result' | 'error';
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

const starterCards = [
  {
    title: '工作汇报',
    description: '季度工作总结与下阶段规划',
    prompt: '帮我整理一份管理层工作汇报，包含进度、问题、数据指标和下季度计划。'
  },
  {
    title: '内容调研',
    description: '行业趋势与竞品分析',
    prompt: '请做一份行业趋势与竞品调研框架，包含指标维度、信息来源和结论结构。'
  },
  {
    title: '教育教学',
    description: '课堂教学设计与知识讲解',
    prompt: '请给我一份 45 分钟课程教学方案，包含目标、流程、互动和作业。'
  },
  {
    title: '人工智能入门',
    description: '面向非技术同学的科普演示',
    prompt: '请生成一份 AI 入门分享大纲，要求通俗易懂并包含可演示案例。'
  }
];

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

function fileReferenceCacheKey(sessionKey: string, pathHint: string): string {
  return `${sessionKey}::${pathHint.trim().toLowerCase()}`;
}

function isBrowserActivitySummary(summary: string): boolean {
  const normalized = summary.trim().toLowerCase();
  return normalized.startsWith('browser ') || normalized.startsWith('browser ->');
}

function extractFirstURL(text: string): string {
  const matched = text.match(/https?:\/\/[^\s)>\]'"`]+/i);
  return matched ? matched[0] : '';
}

export function ChatView() {
  const dispatch = useDispatch();
  const { currentSessionKey, sidebarCollapsed, terminalVisible } = useSelector((state: RootState) => state.ui);
  const isMac = window.electronAPI.platform.isMac;
  const { sendMessage, getSession, getSessions, getSkills, getModels, getConfig, updateConfig, runBrowserAction } =
    useGateway();

  const [messages, setMessages] = useState<Message[]>([]);
  const [sessionTitle, setSessionTitle] = useState('New thread');
  const [inputBySession, setInputBySession] = useState<Record<string, string>>({});
  const [streamingTimeline, setStreamingTimeline] = useState<TimelineEntry[]>([]);
  const [availableSkills, setAvailableSkills] = useState<SkillSummary[]>([]);
  const [selectedSkills, setSelectedSkills] = useState<string[]>([]);
  const [skillsQuery, setSkillsQuery] = useState('');
  const [skillsPickerOpen, setSkillsPickerOpen] = useState(false);
  const [skillsLoadError, setSkillsLoadError] = useState<string | null>(null);
  const [availableModels, setAvailableModels] = useState<Array<{ id: string; name: string; provider: string }>>([]);
  const [currentModel, setCurrentModel] = useState<string>('');
  const [modelsLoading, setModelsLoading] = useState(false);
  const [workspacePath, setWorkspacePath] = useState('');
  const [previewSidebarCollapsed, setPreviewSidebarCollapsed] = useState(true);
  const [previewSidebarWidth, setPreviewSidebarWidth] = useState(460);
  const [selectedFileRef, setSelectedFileRef] = useState<FileReference | null>(null);
  const [previewData, setPreviewData] = useState<PreviewPayload | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
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

  const slashCommands = useMemo(
    () => [
      {
        id: 'new',
        label: '/new',
        description: '新建会话',
        action: () => {
          const newSessionKey = `desktop:${Date.now()}`;
          dispatch(setCurrentSessionKey(newSessionKey));
          setMessages([]);
          setSessionTitle('New thread');
          setPreviewSidebarCollapsed(true);
          resetTypingState();
        }
      },
      {
        id: 'clear',
        label: '/clear',
        description: '清空当前会话消息',
        action: () => {
          setMessages([]);
          setPreviewSidebarCollapsed(true);
          resetTypingState();
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
    [dispatch]
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

    return {
      hasBrowserActivity: collectTexts.length > 0,
      latestURL
    };
  }, [messages, streamingTimeline]);

  const inputRef = useRef<HTMLTextAreaElement>(null);
  const skillsPickerRef = useRef<HTMLDivElement>(null);
  const isComposingRef = useRef(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const typingQueueRef = useRef<string[]>([]);
  const typingTimerRef = useRef<number | null>(null);
  const entrySeqRef = useRef(0);
  const streamingTimelineRef = useRef<TimelineEntry[]>([]);
  const previewRequestRef = useRef(0);
  const currentSessionKeyRef = useRef(currentSessionKey);
  const pendingFileRefChecksRef = useRef<Set<string>>(new Set());

  const input = inputBySession[currentSessionKey] || '';
  const isGenerating = Boolean(generatingSessions[currentSessionKey]);
  const interruptHintVisible = Boolean(interruptHintSessions[currentSessionKey]);
  const browserCopilotOutput = browserCopilotOutputBySession[currentSessionKey] || '';
  const browserCopilotBusy = Boolean(browserCopilotBusyBySession[currentSessionKey]);
  const browserCopilotError = browserCopilotErrorBySession[currentSessionKey] || '';

  const isStarterMode = messages.length === 0 && streamingTimeline.length === 0;

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

  const setInputForSession = (sessionKey: string, value: string) => {
    setInputBySession((prev) => {
      if ((prev[sessionKey] || '') === value) {
        return prev;
      }
      return { ...prev, [sessionKey]: value };
    });
  };

  const setInputForCurrentSession = (value: string) => {
    setInputForSession(currentSessionKey, value);
  };

  const clearInputForSession = (sessionKey: string) => {
    setInputBySession((prev) => {
      if (!(sessionKey in prev)) {
        return prev;
      }
      const next = { ...prev };
      delete next[sessionKey];
      return next;
    });
  };

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
    let cancelled = false;

    const loadSkills = async () => {
      try {
        const skills = await getSkills();
        if (cancelled) {
          return;
        }
        setAvailableSkills(skills);
        setSkillsLoadError(null);
      } catch (err) {
        if (cancelled) {
          return;
        }
        setAvailableSkills([]);
        setSkillsLoadError(err instanceof Error ? err.message : '加载技能失败');
      }
    };

    void loadSkills();
    return () => {
      cancelled = true;
    };
  }, [getSkills]);

  useEffect(() => {
    let cancelled = false;

    const loadModels = async () => {
      try {
        setModelsLoading(true);
        const models = await getModels();
        if (cancelled) return;
        setAvailableModels(models);
        if (models.length === 0) {
          setCurrentModel('');
          return;
        }

        const preferredModel = loadPreferredModel();
        const preferredExists = preferredModel !== '' && models.some((model) => model.id === preferredModel);
        const resolvedModel = preferredExists ? preferredModel : models[0].id;

        setCurrentModel(resolvedModel);
        if (!preferredExists) {
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
  }, [getModels]);

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

  const stopTypingTimer = () => {
    if (typingTimerRef.current !== null) {
      window.clearInterval(typingTimerRef.current);
      typingTimerRef.current = null;
    }
  };

  const resetTypingState = () => {
    typingQueueRef.current = [];
    stopTypingTimer();
    streamingTimelineRef.current = [];
    setStreamingTimeline([]);
  };

  const setStreamingTimelineWithRef = (updater: (prev: TimelineEntry[]) => TimelineEntry[]) => {
    setStreamingTimeline((prev) => {
      const next = updater(prev);
      streamingTimelineRef.current = next;
      return next;
    });
  };

  const ensureTypingTimer = () => {
    if (typingTimerRef.current !== null) {
      return;
    }

    typingTimerRef.current = window.setInterval(() => {
      if (typingQueueRef.current.length === 0) {
        stopTypingTimer();
        return;
      }

      const chunk = typingQueueRef.current.splice(0, 2).join('');
      setStreamingTimelineWithRef((prev) => {
        if (chunk === '') {
          return prev;
        }
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
    }, 18);
  };

  const enqueueTyping = (text: string) => {
    if (!text) {
      return;
    }
    typingQueueRef.current.push(...Array.from(text));
    ensureTypingTimer();
  };

  const waitForTypingDrain = async () => {
    while (typingQueueRef.current.length > 0 || typingTimerRef.current !== null) {
      await new Promise((resolve) => setTimeout(resolve, 20));
    }
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

  const handleInputChange = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
    const value = event.target.value;
    const cursorPosition = event.target.selectionStart || 0;

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
    setInputForCurrentSession(value);
  };

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
        // Enter during generation = interrupt with cancel mode
        handleInterrupt('cancel');
      } else {
        void handleSubmit(event);
      }
    }

    if (event.key === 'Enter' && event.shiftKey && !isComposing) {
      event.preventDefault();
      if (isGenerating) {
        // Shift+Enter during generation = interrupt with append mode
        handleInterrupt('append');
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
      case 'error':
        return {
          type: 'error',
          summary: event.error || '请求失败'
        };
      default:
        return null;
    }
  };

  const appendActivityToTimeline = (activity: StreamActivity) => {
    setStreamingTimelineWithRef((prev) => {
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

  const getActivityLabel = (type: StreamActivity['type']) => {
    if (type === 'status') {
      return 'Thinking';
    }
    if (type === 'error') {
      return 'Error';
    }
    return 'Tool';
  };

  const normalizeStoredTimeline = (
    entries: Array<{
      kind: 'activity' | 'text';
      activity?: {
        type: 'status' | 'tool_start' | 'tool_result' | 'error';
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

  useEffect(() => {
    let cancelled = false;
    setPreviewSidebarCollapsed(true);
    setSelectedFileRef(null);
    setPreviewData(null);
    setPreviewLoading(false);

    const loadSession = async () => {
      try {
        const session = await getSession(currentSessionKey);
        if (cancelled) {
          return;
        }

        const restored = (session.messages || [])
          .filter((message) => message.role === 'user' || message.role === 'assistant')
          .map((message, index) => ({
            id: `${currentSessionKey}-${index}`,
            role: message.role as 'user' | 'assistant',
            content: message.content,
            timestamp: new Date(message.timestamp),
            timeline: normalizeStoredTimeline(message.timeline, `${currentSessionKey}-${index}`)
          }));

        setMessages(restored);
        const fallbackTitle = resolveTitleFromMessages(restored);
        setSessionTitle(fallbackTitle);
        setPreviewSidebarCollapsed(true);

        try {
          const sessions = await getSessions();
          if (cancelled) {
            return;
          }
          const matched = sessions.find((item) => item.key === currentSessionKey);
          if (matched?.lastMessage) {
            setSessionTitle(formatSessionTitle(matched.lastMessage));
          } else {
            setSessionTitle(fallbackTitle);
          }
        } catch {
          if (!cancelled) {
            setSessionTitle(fallbackTitle);
          }
        }

        resetTypingState();
      } catch {
        if (!cancelled) {
          setMessages([]);
          setSessionTitle('New thread');
          setPreviewSidebarCollapsed(true);
          resetTypingState();
        }
      }
    };

    void loadSession();

    return () => {
      cancelled = true;
      resetTypingState();
    };
  }, [currentSessionKey, getSession, getSessions]);

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
    const requestSessionKey = currentSessionKey;
    setPreviewSidebarCollapsed(true);
    setSessionGenerating(requestSessionKey, true);
    setSessionInterruptHint(requestSessionKey, true);

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
    resetTypingState();

    let assistantContent = '';
    const startTime = Date.now();

    try {
      const result = await sendMessage(
        userMessage.content,
        requestSessionKey,
        (delta) => {
          assistantContent += delta;
          if (currentSessionKeyRef.current === requestSessionKey) {
            enqueueTyping(delta);
          }
        },
        (event) => {
          if (currentSessionKeyRef.current !== requestSessionKey) {
            return;
          }
          const activity = toStreamActivity(event);
          if (!activity) {
            return;
          }
          appendActivityToTimeline(activity);
        },
        selectedSkills,
        userMessage.attachments
      );

      if (result.sessionKey && result.sessionKey !== requestSessionKey) {
        dispatch(setCurrentSessionKey(result.sessionKey));
      }

      if (!assistantContent && result.response) {
        assistantContent = result.response;
        if (currentSessionKeyRef.current === requestSessionKey) {
          enqueueTyping(result.response);
        }
      }

      if (currentSessionKeyRef.current === requestSessionKey) {
        await waitForTypingDrain();
      }

      const durationMs = Date.now() - startTime;
      if (currentSessionKeyRef.current === requestSessionKey) {
        setMessages((prev) => [
          ...prev,
          {
            id: `${Date.now()}-assistant`,
            role: 'assistant',
            content: assistantContent,
            timestamp: new Date(),
            timeline: streamingTimelineRef.current.length > 0 ? [...streamingTimelineRef.current] : undefined,
            durationMs
          }
        ]);
        setPreviewSidebarCollapsed(true);
        resetTypingState();
      }
    } catch (err) {
      const errorTimeline = streamingTimelineRef.current.length > 0 ? [...streamingTimelineRef.current] : undefined;
      const durationMs = Date.now() - startTime;
      if (currentSessionKeyRef.current === requestSessionKey) {
        resetTypingState();
        setMessages((prev) => [
          ...prev,
          {
            id: `${Date.now()}-error`,
            role: 'assistant',
            content: err instanceof Error ? `消息发送失败：${err.message}` : '消息发送失败，请检查 Gateway 状态后重试。',
            timestamp: new Date(),
            timeline: errorTimeline,
            durationMs
          }
        ]);
        setPreviewSidebarCollapsed(true);
      }
    } finally {
      setSessionGenerating(requestSessionKey, false);
      setSessionInterruptHint(requestSessionKey, false);
    }
  };

  const applyTemplate = (prompt: string) => {
    setInputForCurrentSession(prompt);
    inputRef.current?.focus();
  };

  const renderTimeline = (items: TimelineEntry[], streaming: boolean) => {
    const openIndex =
      streaming && items.length > 0 && items[items.length - 1].kind === 'activity' ? items.length - 1 : -1;
    const activityItems = items.filter(
      (entry): entry is Extract<TimelineEntry, { kind: 'activity' }> => entry.kind === 'activity'
    );
    const textItems = items.filter(
      (entry): entry is Extract<TimelineEntry, { kind: 'text' }> => entry.kind === 'text' && entry.text.trim() !== ''
    );

    const renderActivityItem = (
      entry: Extract<TimelineEntry, { kind: 'activity' }>,
      defaultOpen?: boolean
    ) => {
      const activityContent = [entry.activity.summary, entry.activity.detail || ''].filter(Boolean).join('\n');
      const fileActions = renderFileActions(activityContent, `${entry.id}-activity-files`);
      return (
        <details key={entry.id} open={defaultOpen} className="rounded-lg border border-border/65 bg-background/90">
          <summary className="cursor-pointer list-none px-3 py-2.5">
            <div className="flex items-center gap-2 text-sm text-foreground/80">
              <ActivityTypeIcon type={entry.activity.type} className="h-4 w-4 flex-shrink-0" />
              <span className="text-[11px] font-semibold uppercase tracking-wide text-foreground/45">
                {getActivityLabel(entry.activity.type)}
              </span>
              <span className="truncate">{entry.activity.summary}</span>
              <ChevronDownIcon className="ml-auto h-3.5 w-3.5 flex-shrink-0 text-foreground/40" />
            </div>
          </summary>
          {entry.activity.detail && (
            <pre className="border-t border-border/60 px-3 py-2 whitespace-pre-wrap break-all font-sans text-foreground/60">
              {entry.activity.detail}
            </pre>
          )}
          {fileActions && (
            <div className={entry.activity.detail ? 'px-3 pb-2' : 'border-t border-border/60 px-3 py-2'}>
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
            <details className="rounded-xl border border-border/70 bg-secondary/35">
              <summary className="cursor-pointer list-none px-3 py-2.5">
                <div className="flex items-center gap-2 text-sm text-foreground/80">
                  <WorkflowIcon className="h-4 w-4 flex-shrink-0" />
                  <span className="font-medium">执行过程（{activityItems.length} 步）</span>
                  <span className="text-xs text-foreground/45">默认折叠，点击展开</span>
                  <ChevronDownIcon className="ml-auto h-3.5 w-3.5 flex-shrink-0 text-foreground/40" />
                </div>
              </summary>
              <div className="space-y-2 border-t border-border/60 px-2 py-2">
                {activityItems.map((entry) => renderActivityItem(entry))}
              </div>
            </details>
          )}

          {textItems.map((entry) => (
            <div key={entry.id} className="text-foreground">
              {renderMarkdownWithActions(entry.text, entry.id)}
            </div>
          ))}
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
  };

  const handleModelChange = async (modelId: string) => {
    if (!modelId || modelId === currentModel) {
      return;
    }

    setCurrentModel(modelId);
    savePreferredModel(modelId);

    try {
      await updateConfig({ model: modelId });
    } catch (err) {
      console.error('Failed to switch model:', err);
    }
  };

  const fallbackReferenceFromPath = (pathHint: string): FileReference | null => {
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
  };

  const referenceFromHref = (href: string): FileReference | null => {
    const parsed = extractFileReferences(`[preview](${href})`);
    if (parsed.length > 0) {
      return parsed[0];
    }
    return fallbackReferenceFromPath(href);
  };

  const previewReference = async (reference: FileReference) => {
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
  };

  const handleFileLinkPreview = (href: string): boolean => {
    const reference = referenceFromHref(href);
    if (!reference) {
      return false;
    }
    void previewReference(reference);
    return true;
  };

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

  const handleBrowserCopilotAction = async (params: Record<string, unknown>) => {
    const requestSessionKey = currentSessionKey;
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

  const renderFileActions = (content: string, keyPrefix: string) => {
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
            className="inline-flex items-center gap-1.5 rounded-lg border border-border/80 bg-secondary/50 px-2 py-1 text-xs text-foreground/80"
          >
            <DocumentIcon className="h-3.5 w-3.5 text-foreground/60" />
            <span className="max-w-[190px] truncate">{reference.displayName}</span>
            <button
              type="button"
              onClick={() => void previewReference(reference)}
              className="rounded border border-border/80 bg-background px-1.5 py-0.5 text-[11px] text-foreground/75 transition-colors hover:bg-secondary"
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
              className="rounded border border-border/80 bg-background px-1.5 py-0.5 text-[11px] text-foreground/65 transition-colors hover:bg-secondary"
            >
              打开目录
            </button>
          </div>
        ))}
      </div>
    );
  };

  const renderMarkdownWithActions = (content: string, keyPrefix: string) => (
    <div className="space-y-1.5">
      <MarkdownRenderer content={content} onFileLinkClick={handleFileLinkPreview} />
      {renderFileActions(content, keyPrefix)}
    </div>
  );

  const browserCopilotURL = browserActivityContext.latestURL || extractFirstURL(browserCopilotOutput);
  const browserCopilotVisible = Boolean(
    browserActivityContext.hasBrowserActivity || browserCopilotOutput || browserCopilotError
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
      <div className="mb-3 rounded-xl border border-border/70 bg-card/70 p-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-foreground/55">Browser Co-Pilot</p>
            <p className="text-sm text-foreground/80">
              手动在真实浏览器完成登录/点击后，可在这里让 Agent 同步当前页面状态继续执行。
            </p>
          </div>
          {browserCopilotURL && (
            <button
              type="button"
              onClick={() => void window.electronAPI.system.openExternal(browserCopilotURL)}
              className="rounded-md border border-border/80 bg-background px-2 py-1 text-xs text-foreground/75 transition-colors hover:bg-secondary"
            >
              打开当前页面
            </button>
          )}
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={() => void handleBrowserCopilotAction({ action: 'screenshot', full_page: false })}
            disabled={browserCopilotBusy}
            className="rounded-md border border-border/80 bg-background px-3 py-1.5 text-xs text-foreground/80 transition-colors hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-60"
          >
            {browserCopilotBusy ? '同步中...' : '同步截图'}
          </button>
          <button
            type="button"
            onClick={() => void handleBrowserCopilotAction({ action: 'snapshot', max_chars: 12000 })}
            disabled={browserCopilotBusy}
            className="rounded-md border border-border/80 bg-background px-3 py-1.5 text-xs text-foreground/80 transition-colors hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-60"
          >
            抓取结构快照
          </button>
          <button
            type="button"
            onClick={() => {
              setInputForCurrentSession('我已在真实浏览器完成手动操作，请基于当前页面状态继续执行任务。');
              inputRef.current?.focus();
            }}
            className="rounded-md border border-primary/40 bg-primary/10 px-3 py-1.5 text-xs text-primary transition-colors hover:bg-primary/15"
          >
            插入继续指令
          </button>
        </div>

        {browserScreenshotInteractive && (
          <p className="mt-2 text-[11px] text-foreground/55">
            当前预览为浏览器截图，可直接在右侧预览图点击位置，系统会把坐标回传给浏览器执行点击并自动刷新截图。
          </p>
        )}

        {browserCopilotError && (
          <p className="mt-2 rounded-md border border-red-300/60 bg-red-50/60 px-2 py-1 text-xs text-red-600">
            {browserCopilotError}
          </p>
        )}

        {browserCopilotOutput && (
          <div className="mt-2 rounded-lg border border-border/70 bg-background/65 px-2 py-2 text-xs text-foreground/75">
            {renderMarkdownWithActions(browserCopilotOutput, `${currentSessionKey}-browser-copilot-output`)}
          </div>
        )}
      </div>
    );
  };

  const renderComposer = (landing: boolean) => (
    <form
      onSubmit={handleSubmit}
      className={`relative rounded-xl border border-primary/40 bg-background shadow-sm ${
        landing ? 'p-4' : 'p-3'
      }`}
    >
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
        placeholder="描述你的任务目标、上下文和输出要求..."
        rows={landing ? 8 : 4}
        className="w-full resize-none border-none bg-transparent px-2 py-1 text-sm leading-6 text-foreground placeholder:text-foreground/35 focus:outline-none"
      />

      {/* @mention dropdown */}
      {mentionOpen && mentionSkills.length > 0 && (
        <div
          ref={mentionRef}
          className="absolute left-4 bottom-24 z-40 w-64 rounded-xl border border-border bg-background p-2 shadow-xl"
        >
          <div className="mb-1 px-2 py-1 text-xs text-foreground/50">选择技能</div>
          <div className="max-h-48 overflow-y-auto">
            {mentionSkills.map((skill, index) => (
              <button
                key={skill.name}
                type="button"
                onClick={() => insertMention(skill.name)}
                className={`w-full rounded-lg px-2 py-2 text-left text-xs transition-colors ${
                  index === mentionIndex ? 'bg-primary/15 text-primary' : 'hover:bg-secondary'
                }`}
              >
                <div className="font-medium">@{skill.displayName || skill.name}</div>
                {skill.description && (
                  <div className="truncate text-foreground/50">{skill.description}</div>
                )}
              </button>
            ))}
          </div>
          <div className="mt-1 border-t border-border/50 px-2 pt-1 text-[10px] text-foreground/40">
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
          <div className="mb-1 px-2 py-1 text-xs text-foreground/50">快捷命令</div>
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
                  index === slashIndex ? 'bg-primary/15 text-primary' : 'hover:bg-secondary'
                }`}
              >
                <div className="font-medium text-sm">{cmd.label}</div>
                <div className="text-xs text-foreground/50">{cmd.description}</div>
              </button>
            ))}
          </div>
          <div className="mt-1 border-t border-border/50 px-2 pt-1 text-[10px] text-foreground/40">
            ↑↓ 选择 · Enter/Tab 确认 · Esc 关闭
          </div>
        </div>
      )}

      <div className="mt-3 flex items-center justify-between gap-3 border-t border-border/70 pt-3">
        <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
          <CustomSelect
            value={currentModel}
            onChange={handleModelChange}
            options={modelOptions}
            placeholder="选择模型..."
            disabled={modelsLoading || isGenerating}
            size="sm"
            className="w-[220px] max-w-full"
            triggerClassName="bg-secondary"
          />
          {modelsLoading && <span className="text-xs text-foreground/50">加载中...</span>}
          <div ref={skillsPickerRef} className="relative flex items-center gap-2 text-xs text-foreground/55">
            <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-2 py-1">
              <FolderIcon className="h-3.5 w-3.5" />
              project
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
              className={`inline-flex items-center gap-1 rounded-md px-2 py-1 transition-colors ${
                selectedSkills.length > 0 || skillsPickerOpen
                  ? 'bg-primary/15 text-primary'
                  : 'bg-secondary text-foreground/70 hover:bg-secondary/80'
              }`}
            >
              <PuzzleIcon className="h-3.5 w-3.5" />
              skills{selectedSkills.length > 0 ? `(${selectedSkills.length})` : ''}
            </button>
            {selectedSkills.length > 0 && (
              <button
                type="button"
                onClick={() => setSelectedSkills([])}
                className="rounded-md border border-border px-1.5 py-1 text-[11px] text-foreground/55 hover:bg-secondary"
              >
                清空
              </button>
            )}

            {skillsPickerOpen && (
              <div className="absolute bottom-10 left-0 z-30 w-80 rounded-xl border border-border bg-background p-3 shadow-xl">
                <input
                  value={skillsQuery}
                  onChange={(event) => setSkillsQuery(event.target.value)}
                  placeholder="搜索技能"
                  className="mb-2 w-full rounded-md border border-border bg-background px-2 py-1.5 text-xs text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
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
                            <span className="block truncate text-foreground/55">{skill.description}</span>
                          )}
                        </span>
                      </label>
                    );
                  })}
                  {filteredSkills.length === 0 && (
                    <div className="px-2 py-1 text-xs text-foreground/45">没有匹配的技能</div>
                  )}
                </div>
                {skillsLoadError && (
                  <p className="mt-2 text-xs text-red-500">技能加载失败: {skillsLoadError}</p>
                )}
                <p className="mt-2 text-[11px] text-foreground/45">
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
              title="Shift+Enter 补充上下文"
              className="inline-flex h-9 items-center gap-1.5 rounded-lg bg-secondary px-3 text-sm font-medium text-foreground transition-colors hover:bg-secondary/80 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <PlusIcon className="h-4 w-4" />
              补充
            </button>
            <button
              type="button"
              onClick={() => handleInterrupt('cancel')}
              title="Enter 打断并重试"
              className="inline-flex h-9 items-center gap-1.5 rounded-lg bg-destructive px-3 text-sm font-medium text-destructive-foreground transition-colors hover:bg-destructive/90"
            >
              <StopIcon className="h-4 w-4" />
              打断
            </button>
          </div>
        ) : (
          <button
            type="submit"
            disabled={!input.trim() || isGenerating}
            className="inline-flex h-10 w-10 items-center justify-center rounded-full bg-primary text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <SendIcon className="h-4 w-4" />
          </button>
        )}
      </div>
      {interruptHintVisible && isGenerating && (
        <div className="mt-2 flex items-center justify-center gap-4 text-xs text-foreground/50">
          <span className="flex items-center gap-1">
            <kbd className="rounded bg-secondary px-1.5 py-0.5 text-[10px]">Enter</kbd>
            打断并重试
          </span>
          <span className="flex items-center gap-1">
            <kbd className="rounded bg-secondary px-1.5 py-0.5 text-[10px]">Shift+Enter</kbd>
            补充上下文
          </span>
        </div>
      )}
    </form>
  );

  const renderThreadHeader = () => (
    <div
      className={`flex h-12 items-center border-b border-border/60 bg-card/95 ${
        isMac && sidebarCollapsed ? 'pl-44 pr-6' : 'px-6'
      }`}
    >
      <div className="min-w-0">
        <h1 className="truncate text-[15px] font-semibold text-foreground">{sessionTitle}</h1>
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
      onToggle={() => setPreviewSidebarCollapsed((prev) => !prev)}
      onResize={setPreviewSidebarWidth}
      onOpenFile={() => {
        void handleOpenSelectedFile();
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

  if (isStarterMode) {
    return (
      <div className="h-full flex flex-col bg-card">
        {renderThreadHeader()}
        <div className="min-h-0 flex flex-1">
          <div className="flex-1 overflow-y-auto px-8 py-10">
            <div className="mx-auto max-w-4xl">
              <div className="mb-8 text-center">
                <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-secondary p-1 shadow-md">
                  <img
                    src="./icon.png"
                    alt="maxclaw"
                    className="h-full w-full rounded-xl object-cover"
                  />
                </div>
                <h1 className="text-4xl font-semibold text-foreground">开始协作</h1>
                <p className="mt-3 text-base text-foreground/55">7x24 小时帮你干活的全场景个人助理 Agent</p>
              </div>

              {renderBrowserCopilotPanel()}
              {renderComposer(true)}

              <section className="mt-10">
                <p className="mb-3 text-sm font-medium text-foreground/65">任务模板</p>
                <div className="grid grid-cols-2 gap-3">
                  {starterCards.map((card) => (
                    <button
                      key={card.title}
                      onClick={() => applyTemplate(card.prompt)}
                      className="rounded-xl border border-border bg-background px-4 py-4 text-left transition-colors hover:border-primary/45 hover:bg-primary/5"
                    >
                      <p className="text-base font-semibold text-foreground">{card.title}</p>
                      <p className="mt-1 text-sm text-foreground/55">{card.description}</p>
                    </button>
                  ))}
                </div>
              </section>
            </div>
          </div>
          {renderPreviewSidebar()}
        </div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col bg-card">
      {renderThreadHeader()}
      <div className="flex-1 flex min-h-0">
        <div className="min-w-0 flex flex-1 flex-col">
          <div className="flex-1 overflow-y-auto p-6 space-y-4">
            {messages.map((message) => (
              <div
                key={message.id}
                className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
              >
                {message.role === 'user' ? (
                  <div className="max-w-3xl space-y-2">
                    {message.attachments && message.attachments.length > 0 && (
                      <div className="flex flex-wrap gap-2">
                        {message.attachments.map((file) => (
                          <div
                            key={file.id}
                            className="flex items-center gap-1.5 rounded-lg bg-secondary px-2.5 py-1.5 text-xs text-foreground"
                          >
                            <DocumentIcon className="h-3.5 w-3.5 text-foreground/60" />
                            <span className="max-w-[150px] truncate">{file.filename}</span>
                          </div>
                        ))}
                      </div>
                    )}
                    <div className="group relative rounded-xl bg-primary px-4 py-3 text-sm leading-6 text-primary-foreground">
                      <pre className="whitespace-pre-wrap break-all font-sans selection:bg-primary-foreground/30">{message.content}</pre>
                      <button
                        type="button"
                        onClick={() => {
                          void navigator.clipboard.writeText(message.content);
                          showToast('已复制到剪贴板');
                        }}
                        className="absolute -top-2 -right-2 flex h-7 w-7 items-center justify-center rounded-full bg-card text-foreground shadow-md opacity-0 transition-opacity group-hover:opacity-100 hover:bg-secondary"
                        title="复制内容"
                      >
                        <CopyIcon className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="w-full px-1 py-1 text-foreground">
                    {message.timeline && message.timeline.length > 0 ? (
                      renderTimeline(message.timeline, false)
                    ) : (
                      renderMarkdownWithActions(message.content, message.id)
                    )}
                    {message.durationMs !== undefined && message.durationMs > 0 && (
                      <div className="mt-2 flex items-center gap-1.5 text-[11px] text-foreground/40">
                        <svg className="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <circle cx="12" cy="12" r="9" strokeWidth={1.5} />
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 7v5l3 3" />
                        </svg>
                        <span>{formatDuration(message.durationMs)}</span>
                      </div>
                    )}
                  </div>
                )}
              </div>
            ))}

            {streamingTimeline.length > 0 && (
              <div className="flex justify-start">
                <div className="w-full px-1 py-1 text-sm leading-7 text-foreground">
                  {renderTimeline(streamingTimeline, true)}
                  <span className="ml-1 inline-block h-4 w-2 animate-pulse bg-primary" />
                </div>
              </div>
            )}

            <div ref={messagesEndRef} />
          </div>

          <div className="p-4 pt-3">
            {renderBrowserCopilotPanel()}
            {renderComposer(false)}
            {terminalVisible && (
              <Suspense
                fallback={
                  <div className="mt-3 rounded-xl border border-border/70 bg-background/70 px-3 py-4 text-xs text-foreground/55">
                    Loading terminal...
                  </div>
                }
              >
                <LazyTerminalPanel key={currentSessionKey} sessionKey={currentSessionKey} />
              </Suspense>
            )}
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
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
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
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.8} d="M12 3v3m0 12v3m9-9h-3M6 12H3m15.36 6.36l-2.12-2.12M7.76 7.76L5.64 5.64m12.72 0l-2.12 2.12M7.76 16.24l-2.12 2.12" />
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

  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <rect x={4} y={5} width={16} height={12} rx={2.5} strokeWidth={1.8} />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.8} d="M8 19h8M9 17l-1.2 2m6.2-2l1.2 2" />
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
