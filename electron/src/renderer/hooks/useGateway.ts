import { useState, useCallback } from 'react';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
}

interface StreamResponse {
  content: string;
  done: boolean;
}

export function useGateway() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const sendMessage = useCallback(async (
    content: string,
    sessionKey: string,
    onDelta: (delta: string) => void
  ): Promise<void> => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await fetch('http://localhost:18890/api/message', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          content,
          session_key: sessionKey,
          channel: 'desktop',
          chat_id: sessionKey
        })
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const reader = response.body?.getReader();
      if (!reader) {
        throw new Error('No response body');
      }

      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6);
            if (data === '[DONE]') continue;

            try {
              const parsed = JSON.parse(data);
              if (parsed.delta) {
                onDelta(parsed.delta);
              }
            } catch {
              // Handle plain text deltas
              onDelta(data);
            }
          }
        }
      }
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
    return response.json();
  }, []);

  const getConfig = useCallback(async () => {
    const response = await fetch('http://localhost:18890/api/config');
    if (!response.ok) throw new Error('Failed to fetch config');
    return response.json();
  }, []);

  return { sendMessage, getSessions, getConfig, isLoading, error };
}
