import { useState, useCallback } from 'react';

interface SendMessageResult {
  response: string;
  sessionKey: string;
}

export interface SkillSummary {
  name: string;
  displayName: string;
  description?: string;
}

export interface GatewayStreamEvent {
  type?: string;
  iteration?: number;
  message?: string;
  delta?: string;
  toolId?: string;
  toolName?: string;
  toolArgs?: string;
  summary?: string;
  toolResult?: string;
  response?: string;
  error?: string;
  sessionKey?: string;
  done?: boolean;
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
    timeline?: Array<{
      kind: 'activity' | 'text';
      activity?: {
        type: 'status' | 'tool_start' | 'tool_result' | 'error';
        summary: string;
        detail?: string;
      };
      text?: string;
    }>;
  }>;
}

export function useGateway() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const parseStreamChunk = (
    raw: string,
    onDelta: (delta: string) => void,
    onEvent: ((event: GatewayStreamEvent) => void) | undefined,
    state: {
      sawDelta: boolean;
      fullResponse: string;
      resolvedSessionKey: string;
    }
  ) => {
    if (!raw || raw === '[DONE]') {
      return;
    }

    let parsed: GatewayStreamEvent;
    try {
      parsed = JSON.parse(raw) as GatewayStreamEvent;
    } catch {
      state.fullResponse += raw;
      onDelta(raw);
      return;
    }

    if (parsed.sessionKey) {
      state.resolvedSessionKey = parsed.sessionKey;
    }

    if (parsed.type) {
      onEvent?.(parsed);
    }

    if (parsed.type === 'error' || (parsed.error && !parsed.type)) {
      throw new Error(parsed.error || 'Gateway stream error');
    }

    if (parsed.delta) {
      state.sawDelta = true;
      state.fullResponse += parsed.delta;
      onDelta(parsed.delta);
    }

    if (parsed.response) {
      if (!state.sawDelta) {
        state.fullResponse += parsed.response;
        onDelta(parsed.response);
      } else if (parsed.response.length >= state.fullResponse.length) {
        state.fullResponse = parsed.response;
      }
    }
  };

  const sendMessage = useCallback(async (
    content: string,
    sessionKey: string,
    onDelta: (delta: string) => void,
    onEvent?: (event: GatewayStreamEvent) => void,
    selectedSkills?: string[]
  ): Promise<SendMessageResult> => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await fetch('http://localhost:18890/api/message?stream=1', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'text/event-stream, application/json'
        },
        body: JSON.stringify({
          content,
          sessionKey,
          channel: 'desktop',
          chatId: sessionKey,
          selectedSkills: (selectedSkills || []).filter(Boolean),
          stream: true
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
      const state = {
        fullResponse: '',
        sawDelta: false,
        resolvedSessionKey: sessionKey
      };

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data:')) {
            const payload = line.replace(/^data:\s?/, '');
            parseStreamChunk(payload, onDelta, onEvent, state);
          }
        }
      }

      if (buffer.trim().startsWith('data:')) {
        const payload = buffer.trim().replace(/^data:\s?/, '');
        parseStreamChunk(payload, onDelta, onEvent, state);
      }

      return {
        response: state.fullResponse,
        sessionKey: state.resolvedSessionKey
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

  const getSkills = useCallback(async () => {
    const response = await fetch('http://localhost:18890/api/skills');
    if (!response.ok) throw new Error('Failed to fetch skills');
    const data = await response.json() as { skills?: SkillSummary[] };
    return data.skills || [];
  }, []);

  const getModels = useCallback(async () => {
    const config = await getConfig();
    const providers = config.providers || {};
    const models: Array<{ id: string; name: string; provider: string }> = [];

    for (const [providerKey, providerConfig] of Object.entries(providers)) {
      const pc = providerConfig as { enabled?: boolean; models?: string[] };
      if (pc.enabled !== false && Array.isArray(pc.models)) {
        for (const modelId of pc.models) {
          models.push({
            id: modelId,
            name: modelId.split('/').pop() || modelId,
            provider: providerKey
          });
        }
      }
    }

    return models;
  }, [getConfig]);

  const updateConfig = useCallback(async (updates: { model?: string }) => {
    const response = await fetch('http://localhost:18890/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(updates)
    });
    if (!response.ok) throw new Error('Failed to update config');
    return response.json();
  }, []);

  const deleteSession = useCallback(async (sessionKey: string) => {
    const response = await fetch(`http://localhost:18890/api/sessions/${encodeURIComponent(sessionKey)}`, {
      method: 'DELETE'
    });
    if (!response.ok) throw new Error('Failed to delete session');
    return response.json();
  }, []);

  const renameSession = useCallback(async (sessionKey: string, newTitle: string) => {
    const response = await fetch(`http://localhost:18890/api/sessions/${encodeURIComponent(sessionKey)}/rename`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: newTitle })
    });
    if (!response.ok) throw new Error('Failed to rename session');
    return response.json();
  }, []);

  return { sendMessage, getSessions, getSession, getConfig, getSkills, getModels, updateConfig, deleteSession, renameSession, isLoading, error };
}
