import React, { useState, useRef, useEffect } from 'react';
import { useGateway } from '../hooks/useGateway';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
}

export function ChatView() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [streamingContent, setStreamingContent] = useState('');
  const [sessionKey, setSessionKey] = useState('desktop:default');
  const isComposingRef = useRef(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const { sendMessage, isLoading } = useGateway();

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streamingContent]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isLoading) return;

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: input,
      timestamp: new Date()
    };

    setMessages(prev => [...prev, userMessage]);
    setInput('');
    setStreamingContent('');

    let assistantContent = '';

    try {
      const result = await sendMessage(userMessage.content, sessionKey, (delta) => {
        assistantContent += delta;
        setStreamingContent(assistantContent);
      });
      setSessionKey(result.sessionKey || sessionKey);

      if (!assistantContent && result.response) {
        assistantContent = result.response;
      }

      // Add complete assistant message
      const assistantMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: assistantContent,
        timestamp: new Date()
      };

      setMessages(prev => [...prev, assistantMessage]);
      setStreamingContent('');
    } catch (error) {
      console.error('Failed to send message:', error);
      setStreamingContent('');
      setMessages(prev => [...prev, {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: 'Message failed. Please check gateway status and try again.',
        timestamp: new Date()
      }]);
    }
  };

  return (
    <div className="h-full flex flex-col">
      {/* Messages Area */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.map((message) => (
          <div
            key={message.id}
            className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
          >
            <div
              className={`max-w-3xl rounded-2xl px-4 py-3 ${
                message.role === 'user'
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-secondary text-foreground'
              }`}
            >
              <pre className="whitespace-pre-wrap font-sans text-sm">
                {message.content}
              </pre>
            </div>
          </div>
        ))}

        {/* Streaming message */}
        {streamingContent && (
          <div className="flex justify-start">
            <div className="max-w-3xl rounded-2xl px-4 py-3 bg-secondary text-foreground">
              <pre className="whitespace-pre-wrap font-sans text-sm">
                {streamingContent}
              </pre>
              <span className="inline-block w-2 h-4 bg-primary ml-1 animate-pulse" />
            </div>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input Area */}
      <div className="border-t border-border p-4">
        <form onSubmit={handleSubmit} className="relative">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onCompositionStart={() => {
              isComposingRef.current = true;
            }}
            onCompositionEnd={() => {
              isComposingRef.current = false;
            }}
            onKeyDown={(e) => {
              const nativeEvent = e.nativeEvent as KeyboardEvent & { isComposing?: boolean; keyCode?: number };
              const isComposing = isComposingRef.current || nativeEvent.isComposing === true || nativeEvent.keyCode === 229;
              if (e.key === 'Enter' && !e.shiftKey && !isComposing) {
                e.preventDefault();
                handleSubmit(e);
              }
            }}
            placeholder="Type a message..."
            rows={3}
            className="w-full bg-secondary rounded-xl px-4 py-3 pr-12 resize-none focus:outline-none focus:ring-2 focus:ring-primary text-foreground placeholder:text-foreground/40"
          />
          <button
            type="submit"
            disabled={!input.trim() || isLoading}
            className="absolute right-3 bottom-3 p-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <SendIcon className="w-4 h-4" />
          </button>
        </form>
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
