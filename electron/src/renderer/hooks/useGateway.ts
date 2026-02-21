import { useState, useCallback } from 'react';

interface SendMessageResult {
  response: string;
  sessionKey: string;
}

export interface SessionSummary {
  key: string;
  messageCount: number;
  lastMessageAt?: string;
  lastMessage?: string;
}

export interface SessionDetail {
  key: string;
  messages: Array<{
    role: string;
    content: string;
    timestamp: string;
  }>;
}

export function useGateway() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const sendMessage = useCallback(async (
    content: string,
    sessionKey: string,
    onDelta: (delta: string) => void
  ): Promise<SendMessageResult> => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await fetch('http://localhost:18890/api/message', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          content,
          sessionKey,
          channel: 'desktop',
          chatId: sessionKey
        })
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const contentType = response.headers.get('content-type') || '';
      if (contentType.includes('application/json')) {
        const data = await response.json() as { response?: string; sessionKey?: string };
        const fullResponse = data.response || '';
        if (fullResponse) {
          onDelta(fullResponse);
        }
        return {
          response: fullResponse,
          sessionKey: data.sessionKey || sessionKey
        };
      }

      const reader = response.body?.getReader();
      if (!reader) {
        throw new Error('No response body');
      }

      const decoder = new TextDecoder();
      let buffer = '';
      let fullResponse = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6);
            if (data === '[DONE]') {
              continue;
            }

            try {
              const parsed = JSON.parse(data) as { delta?: string; response?: string };
              if (parsed.delta) {
                fullResponse += parsed.delta;
                onDelta(parsed.delta);
              } else if (parsed.response) {
                fullResponse += parsed.response;
                onDelta(parsed.response);
              }
            } catch {
              // Handle plain text deltas
              fullResponse += data;
              onDelta(data);
            }
          } else if (line.trim() !== '') {
            fullResponse += line;
            onDelta(line);
          }
        }
      }

      return {
        response: fullResponse,
        sessionKey
      };
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const getSessions = useCallback(async () => {
    const response = await fetch('http://localhost:18890/api/sessions');
    if (!response.ok) throw new Error('Failed to fetch sessions');
    const data = await response.json() as { sessions?: SessionSummary[] };
    return data.sessions || [];
  }, []);

  const getSession = useCallback(async (sessionKey: string) => {
    const response = await fetch(`http://localhost:18890/api/sessions/${encodeURIComponent(sessionKey)}`);
    if (!response.ok) throw new Error('Failed to fetch session');
    return response.json() as Promise<SessionDetail>;
  }, []);

  const getConfig = useCallback(async () => {
    const response = await fetch('http://localhost:18890/api/config');
    if (!response.ok) throw new Error('Failed to fetch config');
    return response.json();
  }, []);

  return { sendMessage, getSessions, getSession, getConfig, isLoading, error };
}
