export type UILanguage = 'zh' | 'en';

const CHANNEL_ALIASES: Record<string, string> = {
  web: 'webui',
  'web-ui': 'webui',
  website: 'webui',
  lark: 'feishu',
  onebot: 'qq',
  mail: 'email',
  ws: 'websocket'
};

const CHANNEL_LABELS: Record<string, { zh: string; en: string }> = {
  desktop: { zh: '桌面', en: 'Desktop' },
  webui: { zh: 'Web UI', en: 'Web UI' },
  telegram: { zh: 'Telegram', en: 'Telegram' },
  discord: { zh: 'Discord', en: 'Discord' },
  whatsapp: { zh: 'WhatsApp', en: 'WhatsApp' },
  slack: { zh: 'Slack', en: 'Slack' },
  feishu: { zh: '飞书', en: 'Feishu' },
  qq: { zh: 'QQ', en: 'QQ' },
  email: { zh: '邮箱', en: 'Email' },
  websocket: { zh: 'WebSocket', en: 'WebSocket' },
  unknown: { zh: '其他', en: 'Other' }
};

export const DEFAULT_CHANNEL_ORDER = [
  'desktop',
  'webui',
  'telegram',
  'discord',
  'whatsapp',
  'slack',
  'feishu',
  'qq',
  'email',
  'websocket'
] as const;

export function normalizeChannelKey(channel: string): string {
  const raw = channel.trim().toLowerCase();
  if (raw === '') {
    return 'unknown';
  }
  return CHANNEL_ALIASES[raw] || raw;
}

export function extractSessionChannel(sessionKey: string): string {
  const prefix = sessionKey.split(':', 2)[0] || '';
  return normalizeChannelKey(prefix);
}

export function getChannelLabel(channel: string, language: UILanguage): string {
  const normalized = normalizeChannelKey(channel);
  const labels = CHANNEL_LABELS[normalized];
  if (labels) {
    return labels[language];
  }
  return normalized;
}
