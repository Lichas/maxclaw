export interface ModelConfig {
  id: string;
  name: string;
  maxTokens?: number;
  enabled: boolean;
}

export interface ProviderConfig {
  id: string;
  name: string;
  type: 'openai' | 'anthropic' | 'custom';
  apiKey: string;
  baseURL?: string;
  apiFormat: 'openai' | 'anthropic';
  models: ModelConfig[];
  enabled: boolean;
}

export const PRESET_PROVIDERS: Omit<ProviderConfig, 'id' | 'apiKey'>[] = [
  {
    name: 'DeepSeek',
    type: 'openai',
    baseURL: 'https://api.deepseek.com/v1',
    apiFormat: 'openai',
    models: [
      { id: 'deepseek-chat', name: 'DeepSeek Chat', enabled: true },
      { id: 'deepseek-coder', name: 'DeepSeek Coder', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'OpenAI',
    type: 'openai',
    baseURL: 'https://api.openai.com/v1',
    apiFormat: 'openai',
    models: [
      { id: 'gpt-4', name: 'GPT-4', enabled: true },
      { id: 'gpt-4-turbo', name: 'GPT-4 Turbo', enabled: true },
      { id: 'gpt-3.5-turbo', name: 'GPT-3.5 Turbo', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Anthropic',
    type: 'anthropic',
    baseURL: 'https://api.anthropic.com',
    apiFormat: 'anthropic',
    models: [
      { id: 'claude-opus-4.5', name: 'Claude Opus 4.5', enabled: true },
      { id: 'claude-sonnet-4', name: 'Claude Sonnet 4', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Zhipu',
    type: 'openai',
    baseURL: 'https://open.bigmodel.cn/api/coding/paas/v4',
    apiFormat: 'openai',
    models: [
      { id: 'glm-4.7', name: 'GLM-4.7', enabled: true },
      { id: 'glm-5', name: 'GLM-5', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Moonshot',
    type: 'openai',
    baseURL: 'https://api.moonshot.cn/v1',
    apiFormat: 'openai',
    models: [
      { id: 'moonshot-v1-8k', name: 'Moonshot v1 8K', enabled: true },
      { id: 'moonshot-v1-32k', name: 'Moonshot v1 32K', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Groq',
    type: 'openai',
    baseURL: 'https://api.groq.com/openai/v1',
    apiFormat: 'openai',
    models: [
      { id: 'llama-3.1-70b-versatile', name: 'Llama 3.1 70B', enabled: true },
      { id: 'mixtral-8x7b-32768', name: 'Mixtral 8x7B', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Gemini',
    type: 'openai',
    baseURL: 'https://generativelanguage.googleapis.com/v1beta',
    apiFormat: 'openai',
    models: [
      { id: 'gemini-pro', name: 'Gemini Pro', enabled: true },
      { id: 'gemini-pro-vision', name: 'Gemini Pro Vision', enabled: true },
    ],
    enabled: false,
  },
];
