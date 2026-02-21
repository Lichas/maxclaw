import React, { useEffect, useRef, useState } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { RootState, setCurrentSessionKey } from '../store';
import { useGateway } from '../hooks/useGateway';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
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
  const [streamingContent, setStreamingContent] = useState('');

  const inputRef = useRef<HTMLTextAreaElement>(null);
  const isComposingRef = useRef(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const isStarterMode = messages.length === 0 && !streamingContent;

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streamingContent]);

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
            timestamp: new Date(message.timestamp)
          }));

        setMessages(restored);
        setStreamingContent('');
      } catch {
        if (!cancelled) {
          setMessages([]);
          setStreamingContent('');
        }
      }
    };

    void loadSession();

    return () => {
      cancelled = true;
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
    setStreamingContent('');

    let assistantContent = '';

    try {
      const result = await sendMessage(userMessage.content, currentSessionKey, (delta) => {
        assistantContent += delta;
        setStreamingContent(assistantContent);
      });

      if (result.sessionKey && result.sessionKey !== currentSessionKey) {
        dispatch(setCurrentSessionKey(result.sessionKey));
      }

      if (!assistantContent && result.response) {
        assistantContent = result.response;
      }

      setMessages((prev) => [
        ...prev,
        {
          id: `${Date.now()}-assistant`,
          role: 'assistant',
          content: assistantContent,
          timestamp: new Date()
        }
      ]);
      setStreamingContent('');
    } catch {
      setStreamingContent('');
      setMessages((prev) => [
        ...prev,
        {
          id: `${Date.now()}-error`,
          role: 'assistant',
          content: '消息发送失败，请检查 Gateway 状态后重试。',
          timestamp: new Date()
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
            <div
              className={`max-w-3xl rounded-2xl px-4 py-3 text-sm leading-6 ${
                message.role === 'user'
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-background text-foreground border border-border'
              }`}
            >
              <pre className="whitespace-pre-wrap font-sans">{message.content}</pre>
            </div>
          </div>
        ))}

        {streamingContent && (
          <div className="flex justify-start">
            <div className="max-w-3xl rounded-2xl border border-border bg-background px-4 py-3 text-sm leading-6 text-foreground">
              <pre className="whitespace-pre-wrap font-sans">{streamingContent}</pre>
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
