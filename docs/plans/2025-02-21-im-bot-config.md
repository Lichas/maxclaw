# IM Bot Configuration Panel Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create configuration UI for IM bot integrations (Telegram, Discord, WhatsApp, 飞书, 钉钉, etc.) allowing users to enable/disable channels and configure API tokens.

**Architecture:** Frontend provides configuration forms for each supported IM platform. Configurations are saved to Gateway via `/api/config`. Gateway validates tokens and shows connection status. Real-time status updates via polling or WebSocket.

**Tech Stack:** React forms, Gateway config API, platform-specific auth flows

---

### Task 1: Create Channel Config Types

**Files:**
- Create: `electron/src/renderer/types/channels.ts`

```typescript
export interface ChannelConfig {
  id: string;
  type: 'telegram' | 'discord' | 'whatsapp' | 'lark' | 'dingtalk' | 'slack';
  name: string;
  enabled: boolean;
  config: Record<string, string>;
  status: 'connected' | 'disconnected' | 'error';
  errorMessage?: string;
}

export const CHANNEL_PRESETS = {
  telegram: {
    name: 'Telegram',
    icon: '📱',
    fields: [
      { key: 'token', label: 'Bot Token', type: 'password' },
      { key: 'webhook', label: 'Webhook URL (optional)', type: 'text' },
    ],
  },
  discord: {
    name: 'Discord',
    icon: '💬',
    fields: [
      { key: 'token', label: 'Bot Token', type: 'password' },
      { key: 'clientId', label: 'Client ID', type: 'text' },
    ],
  },
  whatsapp: {
    name: 'WhatsApp',
    icon: '📲',
    fields: [
      { key: 'session', label: 'Session Name', type: 'text' },
    ],
    qrCode: true,
  },
  lark: {
    name: '飞书',
    icon: '📋',
    fields: [
      { key: 'appId', label: 'App ID', type: 'text' },
      { key: 'appSecret', label: 'App Secret', type: 'password' },
    ],
  },
  dingtalk: {
    name: '钉钉',
    icon: '🔔',
    fields: [
      { key: 'appKey', label: 'App Key', type: 'text' },
      { key: 'appSecret', label: 'App Secret', type: 'password' },
    ],
  },
};
```

---

### Task 2: Create Channel Config UI

**Files:**
- Create: `electron/src/renderer/views/ChannelsView.tsx`

Key features:
- List of supported channels with enable/disable toggles
- Configuration forms for each channel
- Connection status indicators
- QR code display for WhatsApp
- Test connection buttons

---

### Task 3: Backend Channel Status API

**Files:**
- Modify: `internal/webui/server.go`

Add endpoint: `GET /api/channels/status` returning connection status for all configured channels.

---

### Task 4: Integration and Testing

Test each channel type:
1. Enable channel in UI
2. Enter credentials
3. Verify connection status updates
4. Test message sending/receiving

Update documentation.
