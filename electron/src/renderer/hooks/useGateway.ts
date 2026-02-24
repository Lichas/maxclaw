import { useState, useCallback } from 'react';

interface SendMessageResult {
  response: string;
  sessionKey: string;
}

export interface BrowserActionResult {
  ok: boolean;
  sessionKey: string;
  result: string;
}

interface MessageAttachmentPayload {
  id: string;
  filename: string;
  size: number;
  url: string;
  path?: string;
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

interface GatewayConfigResponse {
  agents?: {
    defaults?: {
      model?: string;
      workspace?: string;
    };
  };
  providers?: Record<
    string,
    {
      apiKey?: string;
      apiBase?: string;
    }
  >;
}

const PROVIDER_DEFAULT_MODELS: Record<string, string[]> = {
  openrouter: ['openrouter/auto', 'openrouter/anthropic/claude-sonnet-4.5'],
  anthropic: ['anthropic/claude-opus-4-1', 'anthropic/claude-sonnet-4'],
  openai: ['openai/gpt-5.1', 'openai/gpt-5-mini'],
  deepseek: ['deepseek-chat', 'deepseek-reasoner'],
  zhipu: ['glm-5', 'glm-4.7'],
  groq: ['groq/llama-3.3-70b-versatile', 'groq/mistral-saba-24b'],
  gemini: ['gemini/gemini-2.5-pro', 'gemini/gemini-2.5-flash'],
  dashscope: ['qwen-max-latest', 'qwen-plus-latest'],
  moonshot: ['kimi-k2-0905-preview', 'kimi-k2-turbo-preview'],
  minimax: ['MiniMax-M2.5', 'MiniMax-M2.5-highspeed']
};

const PROVIDER_KEYWORDS: Array<{ provider: string; keywords: string[] }> = [
  { provider: 'openrouter', keywords: ['openrouter'] },
  { provider: 'deepseek', keywords: ['deepseek'] },
  { provider: 'zhipu', keywords: ['zhipu', 'glm', 'zai'] },
  { provider: 'anthropic', keywords: ['anthropic', 'claude'] },
  { provider: 'openai', keywords: ['openai', 'gpt'] },
  { provider: 'gemini', keywords: ['gemini'] },
  { provider: 'dashscope', keywords: ['dashscope', 'qwen'] },
  { provider: 'groq', keywords: ['groq'] },
  { provider: 'moonshot', keywords: ['moonshot', 'kimi'] },
  { provider: 'minimax', keywords: ['minimax'] },
  { provider: 'vllm', keywords: ['vllm'] }
];

function inferProviderFromModel(modelId: string, configuredProviderKeys: string[]): string {
  const normalized = modelId.toLowerCase();
  const prefix = normalized.includes('/') ? normalized.split('/')[0] : '';
  if (prefix && configuredProviderKeys.includes(prefix)) {
    return prefix;
  }

  for (const entry of PROVIDER_KEYWORDS) {
    if (entry.keywords.some((keyword) => normalized.includes(keyword))) {
      return entry.provider;
    }
  }

  return configuredProviderKeys[0] || prefix || 'custom';
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
    selectedSkills?: string[],
    attachments?: MessageAttachmentPayload[]
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
          attachments: (attachments || []).map((item) => ({
            id: item.id,
            filename: item.filename,
            size: item.size,
            url: item.url,
            path: item.path || ''
          })),
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
    const config = await getConfig() as GatewayConfigResponse;
    const providers = config.providers || {};
    const models: Array<{ id: string; name: string; provider: string }> = [];
    const seen = new Set<string>();
    const configuredProviderKeys = Object.entries(providers)
      .filter(([, providerConfig]) => Boolean(providerConfig?.apiKey) || Boolean(providerConfig?.apiBase))
      .map(([providerKey]) => providerKey);

    const addModel = (modelId: string, provider: string) => {
      if (!modelId || seen.has(modelId)) {
        return;
      }

      seen.add(modelId);
      models.push({
        id: modelId,
        name: modelId.split('/').pop() || modelId,
        provider
      });
    };

    for (const providerKey of configuredProviderKeys) {
      const candidates = PROVIDER_DEFAULT_MODELS[providerKey] || [];
      for (const modelId of candidates) {
        addModel(modelId, providerKey);
      }
    }

    const currentModel = config.agents?.defaults?.model || '';
    if (currentModel) {
      addModel(currentModel, inferProviderFromModel(currentModel, configuredProviderKeys));
    }

    return models;
  }, [getConfig]);

  const updateConfig = useCallback(async (updates: { model?: string }) => {
    const payload: Record<string, unknown> = {};

    if (updates.model) {
      const config = await getConfig() as { agents?: Record<string, unknown> };
      const agents = (config.agents || {}) as Record<string, unknown>;
      const defaults = (agents.defaults as Record<string, unknown> | undefined) || {};

      payload.agents = {
        ...agents,
        defaults: {
          ...defaults,
          model: updates.model
        }
      };
    }

    if (Object.keys(payload).length === 0) {
      return getConfig();
    }

    const response = await fetch('http://localhost:18890/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (!response.ok) throw new Error('Failed to update config');
    return response.json();
  }, [getConfig]);

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

  const getWhatsAppStatus = useCallback(async () => {
    const response = await fetch('http://localhost:18890/api/channels/whatsapp/status');
    if (!response.ok) throw new Error('Failed to fetch WhatsApp status');
    return response.json() as Promise<{
      enabled: boolean;
      connected: boolean;
      status: string;
      qr?: string;
      qrAt?: string;
    }>;
  }, []);

  const runBrowserAction = useCallback(async (
    sessionKey: string,
    params: Record<string, unknown>
  ) => {
    const response = await fetch('http://localhost:18890/api/browser/action', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        sessionKey,
        channel: 'desktop',
        chatId: sessionKey,
        params
      })
    });
    if (!response.ok) {
      const message = await response.text();
      throw new Error(message || 'Failed to execute browser action');
    }
    return response.json() as Promise<BrowserActionResult>;
  }, []);

  return {
    sendMessage,
    getSessions,
    getSession,
    getConfig,
    getSkills,
    getModels,
    updateConfig,
    deleteSession,
    renameSession,
    getWhatsAppStatus,
    runBrowserAction,
    isLoading,
    error
  };
}
