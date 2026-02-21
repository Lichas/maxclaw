import React, { useEffect, useRef, useState } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { RootState, setCurrentSessionKey } from '../store';
import { GatewayStreamEvent, useGateway } from '../hooks/useGateway';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
  timeline?: TimelineEntry[];
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

function isIterationStatus(summary: string): boolean {
  return iterationStatusPattern.test(summary.trim());
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

export function ChatView() {
  const dispatch = useDispatch();
  const { currentSessionKey } = useSelector((state: RootState) => state.ui);
  const { sendMessage, getSession, isLoading } = useGateway();

  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [streamingTimeline, setStreamingTimeline] = useState<TimelineEntry[]>([]);

  const inputRef = useRef<HTMLTextAreaElement>(null);
  const isComposingRef = useRef(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const typingQueueRef = useRef<string[]>([]);
  const typingTimerRef = useRef<number | null>(null);
  const entrySeqRef = useRef(0);
  const streamingTimelineRef = useRef<TimelineEntry[]>([]);

  const isStarterMode = messages.length === 0 && streamingTimeline.length === 0;

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streamingTimeline]);

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
        if (entry.activity.type === 'status' && isIterationStatus(entry.activity.summary)) {
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

  useEffect(() => {
    let cancelled = false;

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
        resetTypingState();
      } catch {
        if (!cancelled) {
          setMessages([]);
          resetTypingState();
        }
      }
    };

    void loadSession();

    return () => {
      cancelled = true;
      resetTypingState();
    };
  }, [currentSessionKey, getSession]);

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!input.trim() || isLoading) {
      return;
    }

    const userMessage: Message = {
      id: `${Date.now()}`,
      role: 'user',
      content: input.trim(),
      timestamp: new Date()
    };

    setMessages((prev) => [...prev, userMessage]);
    setInput('');
    resetTypingState();

    let assistantContent = '';

    try {
      const result = await sendMessage(
        userMessage.content,
        currentSessionKey,
        (delta) => {
          assistantContent += delta;
          enqueueTyping(delta);
        },
        (event) => {
          const activity = toStreamActivity(event);
          if (!activity) {
            return;
          }
          appendActivityToTimeline(activity);
        }
      );

      if (result.sessionKey && result.sessionKey !== currentSessionKey) {
        dispatch(setCurrentSessionKey(result.sessionKey));
      }

      if (!assistantContent && result.response) {
        assistantContent = result.response;
        enqueueTyping(result.response);
      }

      await waitForTypingDrain();

      setMessages((prev) => [
        ...prev,
        {
          id: `${Date.now()}-assistant`,
          role: 'assistant',
          content: assistantContent,
          timestamp: new Date(),
          timeline: streamingTimelineRef.current.length > 0 ? [...streamingTimelineRef.current] : undefined
        }
      ]);
      resetTypingState();
    } catch (err) {
      const errorTimeline = streamingTimelineRef.current.length > 0 ? [...streamingTimelineRef.current] : undefined;
      resetTypingState();
      setMessages((prev) => [
        ...prev,
        {
          id: `${Date.now()}-error`,
          role: 'assistant',
          content: err instanceof Error ? `消息发送失败：${err.message}` : '消息发送失败，请检查 Gateway 状态后重试。',
          timestamp: new Date(),
          timeline: errorTimeline
        }
      ]);
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    const nativeEvent = event.nativeEvent as KeyboardEvent & { isComposing?: boolean; keyCode?: number };
    const isComposing = isComposingRef.current || nativeEvent.isComposing === true || nativeEvent.keyCode === 229;

    if (event.key === 'Enter' && !event.shiftKey && !isComposing) {
      event.preventDefault();
      void handleSubmit(event);
    }
  };

  const applyTemplate = (prompt: string) => {
    setInput(prompt);
    inputRef.current?.focus();
  };

  const renderTimeline = (items: TimelineEntry[], streaming: boolean) => {
    const openIndex =
      streaming && items.length > 0 && items[items.length - 1].kind === 'activity' ? items.length - 1 : -1;

    return (
      <div className="space-y-3">
        <div className="space-y-2">
          {items.map((entry, index) =>
            entry.kind === 'activity' ? (
              <div key={entry.id} className="rounded-lg border border-border/70 bg-background">
                <details open={index === openIndex ? true : undefined}>
                  <summary className="cursor-pointer list-none px-3 py-2 font-medium text-foreground/75">
                    {getActivityLabel(entry.activity.type)}: {entry.activity.summary}
                  </summary>
                  {entry.activity.detail && (
                    <pre className="border-t border-border/70 px-3 py-2 whitespace-pre-wrap break-all font-sans text-foreground/60">
                      {entry.activity.detail}
                    </pre>
                  )}
                </details>
              </div>
            ) : (
              <pre key={entry.id} className="whitespace-pre-wrap break-all font-sans text-sm leading-7 text-foreground">
                {entry.text}
              </pre>
            )
          )}
        </div>
      </div>
    );
  };

  const renderComposer = (landing: boolean) => (
    <form
      onSubmit={handleSubmit}
      className={`relative rounded-2xl border border-primary/40 bg-background shadow-sm ${
        landing ? 'p-4' : 'p-3'
      }`}
    >
      <textarea
        ref={inputRef}
        value={input}
        onChange={(event) => setInput(event.target.value)}
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

      <div className="mt-3 flex items-center justify-between gap-3 border-t border-border/70 pt-3">
        <div className="flex items-center gap-2 text-xs text-foreground/55">
          <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-2 py-1">
            <FolderIcon className="h-3.5 w-3.5" />
            project
          </span>
          <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-2 py-1">
            <PaperClipIcon className="h-3.5 w-3.5" />
            附件
          </span>
          <span className="inline-flex items-center gap-1 rounded-md bg-primary/15 px-2 py-1 text-primary">
            <PuzzleIcon className="h-3.5 w-3.5" />
            pptx
          </span>
        </div>

        <button
          type="submit"
          disabled={!input.trim() || isLoading}
          className="inline-flex h-10 w-10 items-center justify-center rounded-full bg-primary text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <SendIcon className="h-4 w-4" />
        </button>
      </div>
    </form>
  );

  if (isStarterMode) {
    return (
      <div className="h-full overflow-y-auto bg-[#f7f8fb] px-8 py-10">
        <div className="mx-auto max-w-4xl">
          <div className="mb-8 text-center">
            <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-[#ff6a4d] to-[#d92d20] text-2xl text-white shadow-md">
              🦞
            </div>
            <h1 className="text-4xl font-semibold text-foreground">开始协作</h1>
            <p className="mt-3 text-base text-foreground/55">7x24 小时帮你干活的全场景个人助理 Agent</p>
          </div>

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
    );
  }

  return (
    <div className="h-full flex flex-col bg-[#f7f8fb]">
      <div className="flex-1 overflow-y-auto p-6 space-y-4">
        {messages.map((message) => (
          <div
            key={message.id}
            className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
          >
            {message.role === 'user' ? (
              <div className="max-w-3xl rounded-2xl bg-primary px-4 py-3 text-sm leading-6 text-primary-foreground">
                <pre className="whitespace-pre-wrap break-all font-sans">{message.content}</pre>
              </div>
            ) : (
              <div className="w-full px-1 py-1 text-sm leading-7 text-foreground">
                {message.timeline && message.timeline.length > 0 ? (
                  renderTimeline(message.timeline, false)
                ) : (
                  <pre className="whitespace-pre-wrap break-all font-sans">{message.content}</pre>
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

      <div className="border-t border-border bg-background p-4">{renderComposer(false)}</div>
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

function PaperClipIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828L18 9.828a4 4 0 00-5.657-5.657L5.757 10.757a6 6 0 108.486 8.486L20 13.486" />
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
