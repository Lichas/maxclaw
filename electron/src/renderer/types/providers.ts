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
    name: 'OpenRouter',
    type: 'openai',
    baseURL: 'https://openrouter.ai/api/v1',
    apiFormat: 'openai',
    models: [
      { id: 'openrouter/auto', name: 'OpenRouter Auto', enabled: true },
      { id: 'anthropic/claude-sonnet-4.5', name: 'Claude Sonnet 4.5', enabled: true },
    ],
    enabled: false,
  },
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
      { id: 'gpt-5.1', name: 'GPT-5.1', enabled: true },
      { id: 'gpt-5-mini', name: 'GPT-5 mini', enabled: true },
      { id: 'gpt-4.1-mini', name: 'GPT-4.1 mini', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Anthropic',
    type: 'anthropic',
    baseURL: 'https://api.anthropic.com',
    apiFormat: 'anthropic',
    models: [
      { id: 'claude-opus-4-1', name: 'Claude Opus 4.1', enabled: true },
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
      { id: 'glm-5', name: 'GLM-5', enabled: true },
      { id: 'glm-4.7', name: 'GLM-4.7', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Moonshot',
    type: 'openai',
    baseURL: 'https://api.moonshot.cn/v1',
    apiFormat: 'openai',
    models: [
      { id: 'kimi-k2-0905-preview', name: 'Kimi K2 0905 Preview', enabled: true },
      { id: 'kimi-k2-turbo-preview', name: 'Kimi K2 Turbo Preview', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Groq',
    type: 'openai',
    baseURL: 'https://api.groq.com/openai/v1',
    apiFormat: 'openai',
    models: [
      { id: 'llama-3.3-70b-versatile', name: 'Llama 3.3 70B', enabled: true },
      { id: 'mistral-saba-24b', name: 'Mistral Saba 24B', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Gemini',
    type: 'openai',
    baseURL: 'https://generativelanguage.googleapis.com/v1beta',
    apiFormat: 'openai',
    models: [
      { id: 'gemini-2.5-pro', name: 'Gemini 2.5 Pro', enabled: true },
      { id: 'gemini-2.5-flash', name: 'Gemini 2.5 Flash', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'DashScope',
    type: 'openai',
    baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    apiFormat: 'openai',
    models: [
      { id: 'qwen-max-latest', name: 'Qwen Max Latest', enabled: true },
      { id: 'qwen-plus-latest', name: 'Qwen Plus Latest', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'MiniMax',
    type: 'openai',
    baseURL: 'https://api.minimax.io/v1',
    apiFormat: 'openai',
    models: [
      { id: 'MiniMax-M2.1', name: 'MiniMax M2.1', enabled: true },
      { id: 'MiniMax-M2.1-lightning', name: 'MiniMax M2.1 Lightning', enabled: true },
    ],
    enabled: false,
  },
];
