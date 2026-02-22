// Channel configuration types - mirrors internal/config/schema.go

export interface TelegramConfig {
  enabled: boolean;
  token: string;
  allowFrom: string[];
  proxy?: string;
}

export interface DiscordConfig {
  enabled: boolean;
  token: string;
  allowFrom: string[];
}

export interface WhatsAppConfig {
  enabled: boolean;
  bridgeUrl?: string;
  bridgeToken?: string;
  allowFrom: string[];
  allowSelf?: boolean;
}

export interface WebSocketConfig {
  enabled: boolean;
  host?: string;
  port?: number;
  path?: string;
  allowOrigins?: string[];
}

export interface SlackConfig {
  enabled: boolean;
  botToken?: string;
  appToken?: string;
  allowFrom: string[];
}

export interface EmailConfig {
  enabled: boolean;
  consentGranted: boolean;
  imapHost?: string;
  imapPort?: number;
  imapUsername?: string;
  imapPassword?: string;
  imapMailbox?: string;
  imapUseSSL?: boolean;
  smtpHost?: string;
  smtpPort?: number;
  smtpUsername?: string;
  smtpPassword?: string;
  smtpUseTLS?: boolean;
  smtpUseSSL?: boolean;
  fromAddress?: string;
  autoReplyEnabled?: boolean;
  pollIntervalSeconds?: number;
  markSeen?: boolean;
  allowFrom: string[];
}

export interface QQConfig {
  enabled: boolean;
  wsUrl?: string;
  accessToken?: string;
  allowFrom: string[];
}

export interface FeishuConfig {
  enabled: boolean;
  appId?: string;
  appSecret?: string;
  verificationToken?: string;
  listenAddr?: string;
  webhookPath?: string;
  allowFrom: string[];
}

export interface ChannelsConfig {
  telegram: TelegramConfig;
  discord: DiscordConfig;
  whatsapp: WhatsAppConfig;
  websocket: WebSocketConfig;
  slack: SlackConfig;
  email: EmailConfig;
  qq: QQConfig;
  feishu: FeishuConfig;
}

// Channel metadata for UI display
export interface ChannelInfo {
  key: keyof ChannelsConfig;
  name: string;
  nameZh: string;
  description: string;
  descriptionZh: string;
  icon: string;
  docsUrl: string;
  fields: ChannelField[];
}

export interface ChannelField {
  key: string;
  label: string;
  labelZh: string;
  type: 'text' | 'password' | 'number' | 'checkbox' | 'textarea' | 'list';
  placeholder?: string;
  placeholderZh?: string;
  required?: boolean;
  secret?: boolean;
  hint?: string;
  hintZh?: string;
}

// Email provider presets
export interface EmailProviderPreset {
  key: string;
  name: string;
  imapHost: string;
  imapPort: number;
  imapUseSSL: boolean;
  smtpHost: string;
  smtpPort: number;
  smtpUseTLS: boolean;
  smtpUseSSL: boolean;
}

export const EMAIL_PROVIDER_PRESETS: EmailProviderPreset[] = [
  {
    key: 'gmail',
    name: 'Gmail',
    imapHost: 'imap.gmail.com',
    imapPort: 993,
    imapUseSSL: true,
    smtpHost: 'smtp.gmail.com',
    smtpPort: 587,
    smtpUseTLS: true,
    smtpUseSSL: false,
  },
  {
    key: 'outlook',
    name: 'Outlook/Hotmail',
    imapHost: 'outlook.office365.com',
    imapPort: 993,
    imapUseSSL: true,
    smtpHost: 'smtp.office365.com',
    smtpPort: 587,
    smtpUseTLS: true,
    smtpUseSSL: false,
  },
  {
    key: 'qq',
    name: 'QQ邮箱',
    imapHost: 'imap.qq.com',
    imapPort: 993,
    imapUseSSL: true,
    smtpHost: 'smtp.qq.com',
    smtpPort: 587,
    smtpUseTLS: true,
    smtpUseSSL: false,
  },
  {
    key: '163',
    name: '163网易邮箱',
    imapHost: 'imap.163.com',
    imapPort: 993,
    imapUseSSL: true,
    smtpHost: 'smtp.163.com',
    smtpPort: 465,
    smtpUseTLS: false,
    smtpUseSSL: true,
  },
  {
    key: 'custom',
    name: '自定义',
    imapHost: '',
    imapPort: 993,
    imapUseSSL: true,
    smtpHost: '',
    smtpPort: 587,
    smtpUseTLS: true,
    smtpUseSSL: false,
  },
];

// Channel definitions for UI
export const CHANNEL_DEFINITIONS: ChannelInfo[] = [
  {
    key: 'telegram',
    name: 'Telegram',
    nameZh: 'Telegram',
    description: 'Telegram Bot integration via Bot API',
    descriptionZh: '通过 Bot API 集成 Telegram',
    icon: 'telegram',
    docsUrl: 'https://core.telegram.org/bots#how-do-i-create-a-bot',
    fields: [
      {
        key: 'token',
        label: 'Bot Token',
        labelZh: 'Bot Token',
        type: 'password',
        placeholder: '123456789:ABCdefGHIjklMNOpqrsTUVwxyz',
        placeholderZh: '从 @BotFather 获取',
        required: true,
        secret: true,
        hint: 'Get from @BotFather on Telegram',
        hintZh: '在 Telegram 中向 @BotFather 获取',
      },
      {
        key: 'allowFrom',
        label: 'Allowed Users',
        labelZh: '允许的用户',
        type: 'list',
        placeholder: 'username1, username2',
        placeholderZh: '用户名1, 用户名2（留空表示允许所有）',
        hint: 'Leave empty to allow all users',
        hintZh: '留空表示允许所有用户',
      },
      {
        key: 'proxy',
        label: 'Proxy URL (optional)',
        labelZh: '代理地址（可选）',
        type: 'text',
        placeholder: 'socks5://127.0.0.1:1080',
        placeholderZh: 'socks5://127.0.0.1:1080',
      },
    ],
  },
  {
    key: 'discord',
    name: 'Discord',
    nameZh: 'Discord',
    description: 'Discord bot integration',
    descriptionZh: 'Discord 机器人集成',
    icon: 'discord',
    docsUrl: 'https://discord.com/developers/applications',
    fields: [
      {
        key: 'token',
        label: 'Bot Token',
        labelZh: 'Bot Token',
        type: 'password',
        placeholder: 'MTAxMD...',
        placeholderZh: '从 Discord Developer Portal 获取',
        required: true,
        secret: true,
        hint: 'Get from Discord Developer Portal',
        hintZh: '从 Discord Developer Portal 获取',
      },
      {
        key: 'allowFrom',
        label: 'Allowed Users',
        labelZh: '允许的用户',
        type: 'list',
        placeholder: 'userId1, userId2',
        placeholderZh: '用户ID1, 用户ID2',
        hint: 'Discord user IDs (leave empty for all)',
        hintZh: 'Discord 用户 ID（留空表示允许所有）',
      },
    ],
  },
  {
    key: 'whatsapp',
    name: 'WhatsApp',
    nameZh: 'WhatsApp',
    description: 'WhatsApp via bridge (whatsmeow)',
    descriptionZh: '通过 Bridge（whatsmeow）连接 WhatsApp',
    icon: 'whatsapp',
    docsUrl: 'https://github.com/tulir/whatsmeow',
    fields: [
      {
        key: 'bridgeUrl',
        label: 'Bridge URL',
        labelZh: 'Bridge 地址',
        type: 'text',
        placeholder: 'ws://localhost:3001',
        placeholderZh: 'ws://localhost:3001',
        hint: 'WebSocket URL of the whatsmeow bridge',
        hintZh: 'whatsmeow bridge 的 WebSocket 地址',
      },
      {
        key: 'bridgeToken',
        label: 'Bridge Token',
        labelZh: 'Bridge Token',
        type: 'password',
        secret: true,
        placeholder: 'optional authentication token',
        placeholderZh: '可选的认证令牌',
      },
      {
        key: 'allowFrom',
        label: 'Allowed Phone Numbers',
        labelZh: '允许的电话号码',
        type: 'list',
        placeholder: '+86138..., +86139...',
        placeholderZh: '+86138..., +86139...',
      },
      {
        key: 'allowSelf',
        label: 'Allow Self Messages',
        labelZh: '允许自己的消息',
        type: 'checkbox',
        hint: 'Respond to messages sent by yourself',
        hintZh: '回复自己发送的消息',
      },
    ],
  },
  {
    key: 'slack',
    name: 'Slack',
    nameZh: 'Slack',
    description: 'Slack Socket Mode integration',
    descriptionZh: 'Slack Socket Mode 集成',
    icon: 'slack',
    docsUrl: 'https://api.slack.com/apps',
    fields: [
      {
        key: 'botToken',
        label: 'Bot Token',
        labelZh: 'Bot Token',
        type: 'password',
        placeholder: 'xoxb-...',
        placeholderZh: 'xoxb-...',
        required: true,
        secret: true,
      },
      {
        key: 'appToken',
        label: 'App Token',
        labelZh: 'App Token',
        type: 'password',
        placeholder: 'xapp-...',
        placeholderZh: 'xapp-...',
        required: true,
        secret: true,
        hint: 'Required for Socket Mode',
        hintZh: 'Socket Mode 需要',
      },
      {
        key: 'allowFrom',
        label: 'Allowed Users',
        labelZh: '允许的用户',
        type: 'list',
        placeholder: 'U123..., U456...',
        placeholderZh: '用户ID1, 用户ID2',
      },
    ],
  },
  {
    key: 'feishu',
    name: 'Feishu / Lark',
    nameZh: '飞书 / Lark',
    description: 'Feishu/Lark bot integration',
    descriptionZh: '飞书/Lark 机器人集成',
    icon: 'feishu',
    docsUrl: 'https://open.feishu.cn/app',
    fields: [
      {
        key: 'appId',
        label: 'App ID',
        labelZh: 'App ID',
        type: 'text',
        placeholder: 'cli_...',
        placeholderZh: 'cli_...',
        required: true,
      },
      {
        key: 'appSecret',
        label: 'App Secret',
        labelZh: 'App Secret',
        type: 'password',
        required: true,
        secret: true,
      },
      {
        key: 'verificationToken',
        label: 'Verification Token',
        labelZh: 'Verification Token',
        type: 'password',
        secret: true,
      },
      {
        key: 'listenAddr',
        label: 'Listen Address',
        labelZh: '监听地址',
        type: 'text',
        placeholder: '0.0.0.0:18792',
        placeholderZh: '0.0.0.0:18792',
      },
      {
        key: 'webhookPath',
        label: 'Webhook Path',
        labelZh: 'Webhook 路径',
        type: 'text',
        placeholder: '/feishu/events',
        placeholderZh: '/feishu/events',
      },
    ],
  },
  {
    key: 'qq',
    name: 'QQ (OneBot)',
    nameZh: 'QQ（OneBot）',
    description: 'QQ via OneBot WebSocket',
    descriptionZh: '通过 OneBot WebSocket 连接 QQ',
    icon: 'qq',
    docsUrl: 'https://github.com/Mrs4s/go-cqhttp',
    fields: [
      {
        key: 'wsUrl',
        label: 'WebSocket URL',
        labelZh: 'WebSocket 地址',
        type: 'text',
        placeholder: 'ws://localhost:3002',
        placeholderZh: 'ws://localhost:3002',
        hint: 'go-cqhttp or Lagrange.Core WebSocket address',
        hintZh: 'go-cqhttp 或 Lagrange.Core 的 WebSocket 地址',
      },
      {
        key: 'accessToken',
        label: 'Access Token',
        labelZh: 'Access Token',
        type: 'password',
        secret: true,
      },
      {
        key: 'allowFrom',
        label: 'Allowed QQ Numbers',
        labelZh: '允许的 QQ 号',
        type: 'list',
        placeholder: '123456789, 987654321',
        placeholderZh: 'QQ号1, QQ号2',
      },
    ],
  },
];
