import { useEffect, useMemo, useRef, useState } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import {
  CheckCircledIcon,
  EnterFullScreenIcon,
  ExitFullScreenIcon,
  ReloadIcon,
} from '@radix-ui/react-icons';
import QRCode from 'qrcode';
import Editor from 'react-simple-code-editor';
import * as Prism from 'prismjs';
import 'prismjs/components/prism-json';

type Status = {
  workspace: string;
  model: string;
  channels: string[];
  cron?: {
    totalJobs?: number;
    enabledJobs?: number;
  };
  restrictToWorkspace?: boolean;
  whatsapp?: {
    enabled: boolean;
    connected: boolean;
    status?: string;
    qr?: string;
    qrAt?: string;
  };
  telegram?: {
    enabled: boolean;
    status?: string;
    username?: string;
    name?: string;
    link?: string;
    error?: string;
  };
};

type SessionSummary = {
  key: string;
  messageCount: number;
  lastMessageAt?: string;
  lastMessage?: string;
};

type SessionMessage = {
  role: string;
  content: string;
  timestamp: string;
};

type SessionDetail = {
  key: string;
  messages: SessionMessage[];
};

async function fetchJSON<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  return res.json() as Promise<T>;
}

type Lang = 'en' | 'zh';

const translations = {
  en: {
    heroBadge: 'nanobot control',
    heroTitle: 'Command the swarm.',
    heroSubtitle:
      'Real-time status, sessions, and configuration in one place. Built for operators who want clarity and speed.',
    refresh: 'Refresh',
    statusTab: 'Status',
    chatTab: 'Chat',
    sessionsTab: 'Sessions',
    settingsTab: 'Settings',
    workspace: 'Workspace',
    model: 'Model',
    channels: 'Channels',
    cron: 'Cron',
    restrict: 'restrict',
    active: 'active',
    enabled: 'enabled',
    total: 'total',
    connected: 'connected',
    waitingQr: 'waiting for QR',
    disabled: 'disabled',
    qrWaiting: 'QR appears here when bridge is ready',
    telegramQrWaiting: 'QR appears after token is verified',
    sessionLabel: 'Session',
    noMessages: 'No messages yet.',
    sendPlaceholder: 'Send a message to nanobot...',
    send: 'Send',
    noSessions: 'No sessions found.',
    messages: 'messages',
    workspaceHint: 'Changes require gateway restart.',
    save: 'Save',
    gatewayTitle: 'Gateway',
    gatewayHint: 'Restart gateway service from the UI after config changes.',
    restartGateway: 'Restart Gateway',
    configTitle: 'Config JSON',
    configHint: 'Edit full config. Restart gateway to apply changes.',
    saveConfig: 'Save Config',
    fullscreen: 'Fullscreen',
    exitFullscreen: 'Exit Fullscreen',
    restartSent: 'Gateway restart triggered. Service should be back in a few seconds.',
    fullscreenUnavailable: 'Fullscreen is unavailable in this browser.',
    telegramTitle: 'Telegram',
    telegramHint: 'Paste BotFather token. Restart gateway to apply changes.',
    telegramPlaceholder: '123456:AAE...',
  },
  zh: {
    heroBadge: 'nanobot 控制台',
    heroTitle: '指挥你的智能体群。',
    heroSubtitle: '状态、会话与配置一屏掌控。面向效率与清晰度的操作台。',
    refresh: '刷新',
    statusTab: '状态',
    chatTab: '对话',
    sessionsTab: '会话',
    settingsTab: '设置',
    workspace: '工作区',
    model: '模型',
    channels: '渠道',
    cron: '定时任务',
    restrict: '限制',
    active: '当前',
    enabled: '启用',
    total: '总数',
    connected: '已连接',
    waitingQr: '等待扫码',
    disabled: '未启用',
    qrWaiting: 'Bridge 就绪后二维码会显示在这里',
    telegramQrWaiting: 'Token 验证后显示二维码',
    sessionLabel: '会话',
    noMessages: '暂无消息。',
    sendPlaceholder: '发送消息给 nanobot...',
    send: '发送',
    noSessions: '暂无会话记录。',
    messages: '条消息',
    workspaceHint: '修改后需重启 gateway 生效。',
    save: '保存',
    gatewayTitle: '网关',
    gatewayHint: '配置修改后可直接在 UI 重启 gateway 服务。',
    restartGateway: '重启 Gateway',
    configTitle: '配置 JSON',
    configHint: '编辑完整配置。重启 gateway 后生效。',
    saveConfig: '保存配置',
    fullscreen: '全屏',
    exitFullscreen: '退出全屏',
    restartSent: '已触发 Gateway 重启，服务会在几秒内恢复。',
    fullscreenUnavailable: '当前浏览器暂不支持全屏。',
    telegramTitle: 'Telegram',
    telegramHint: '粘贴 BotFather Token。重启 gateway 后生效。',
    telegramPlaceholder: '123456:AAE...',
  },
} as const;

const getInitialLang = (): Lang => {
  if (typeof window === 'undefined') return 'en';
  const saved = window.localStorage.getItem('nanobot_lang');
  if (saved === 'en' || saved === 'zh') return saved;
  return window.navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en';
};

export default function App() {
  const [lang, setLang] = useState<Lang>(getInitialLang);
  const copy = translations[lang];
  const [status, setStatus] = useState<Status | null>(null);
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [selectedSession, setSelectedSession] = useState('webui:default');
  const [sessionDetail, setSessionDetail] = useState<SessionDetail | null>(null);
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [configText, setConfigText] = useState('');
  const [workspaceInput, setWorkspaceInput] = useState('');
  const [qrDataUrl, setQrDataUrl] = useState<string>('');
  const [telegramQrDataUrl, setTelegramQrDataUrl] = useState<string>('');
  const [telegramToken, setTelegramToken] = useState('');
  const [configFullscreen, setConfigFullscreen] = useState(false);
  const configEditorRef = useRef<HTMLDivElement | null>(null);

  const sessionOptions = useMemo(() => {
    if (sessions.length === 0) {
      return [{ key: 'webui:default', label: 'webui:default' }];
    }
    return sessions.map((s) => ({ key: s.key, label: s.key }));
  }, [sessions]);

  const loadAll = async () => {
    const [statusRes, sessionsRes, configRes] = await Promise.all([
      fetchJSON<Status>('/api/status'),
      fetchJSON<{ sessions: SessionSummary[] }>('/api/sessions'),
      fetchJSON<Record<string, unknown>>('/api/config'),
    ]);
    setStatus(statusRes);
    setSessions(sessionsRes.sessions || []);
    const jsonPretty = JSON.stringify(configRes, null, 2);
    setConfigText(jsonPretty);
    const ws = (configRes as any)?.agents?.defaults?.workspace;
    if (typeof ws === 'string') {
      setWorkspaceInput(ws);
    }
    const tgToken = (configRes as any)?.channels?.telegram?.token;
    if (typeof tgToken === 'string') {
      setTelegramToken(tgToken);
    }
  };

  useEffect(() => {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem('nanobot_lang', lang);
    }
  }, [lang]);

  useEffect(() => {
    if (typeof document === 'undefined') return;
    const onFullscreenChange = () => {
      setConfigFullscreen(document.fullscreenElement === configEditorRef.current);
    };
    document.addEventListener('fullscreenchange', onFullscreenChange);
    return () => document.removeEventListener('fullscreenchange', onFullscreenChange);
  }, []);

  useEffect(() => {
    loadAll().catch((err) => setNotice((err as Error).message));
    const timer = setInterval(() => {
      fetchJSON<Status>('/api/status')
        .then((data) => setStatus(data))
        .catch(() => undefined);
      refreshSessions().catch(() => undefined);
    }, 5000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!selectedSession) return;
    fetchJSON<SessionDetail>(`/api/sessions/${encodeURIComponent(selectedSession)}`)
      .then((data) => setSessionDetail(data))
      .catch((err) => setNotice((err as Error).message));
  }, [selectedSession]);

  useEffect(() => {
    const qr = status?.whatsapp?.qr;
    if (!qr) {
      setQrDataUrl('');
      return;
    }
    QRCode.toDataURL(qr, { margin: 2, width: 220 })
      .then((url) => setQrDataUrl(url))
      .catch(() => setQrDataUrl(''));
  }, [status?.whatsapp?.qr]);

  useEffect(() => {
    const link = status?.telegram?.link;
    if (!link) {
      setTelegramQrDataUrl('');
      return;
    }
    QRCode.toDataURL(link, { margin: 2, width: 220 })
      .then((url) => setTelegramQrDataUrl(url))
      .catch(() => setTelegramQrDataUrl(''));
  }, [status?.telegram?.link]);

  const refreshSessions = async () => {
    try {
      const data = await fetchJSON<{ sessions: SessionSummary[] }>('/api/sessions');
      setSessions(data.sessions || []);
    } catch (err) {
      setNotice((err as Error).message);
    }
  };

  const sendMessage = async () => {
    if (!message.trim()) return;
    setLoading(true);
    try {
      const res = await fetchJSON<{ response: string; sessionKey: string }>('/api/message', {
        method: 'POST',
        body: JSON.stringify({
          sessionKey: selectedSession,
          content: message.trim(),
          channel: 'webui',
          chatId: selectedSession,
        }),
      });
      setMessage('');
      setNotice(
        lang === 'zh'
          ? `已收到回复（${res.sessionKey}）`
          : `Response received (${res.sessionKey})`,
      );
      await refreshSessions();
      const detail = await fetchJSON<SessionDetail>(`/api/sessions/${encodeURIComponent(res.sessionKey)}`);
      setSessionDetail(detail);
    } catch (err) {
      setNotice((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const saveConfig = async () => {
    setLoading(true);
    try {
      const parsed = JSON.parse(configText);
      const saved = await fetchJSON<Record<string, unknown>>('/api/config', {
        method: 'PUT',
        body: JSON.stringify(parsed),
      });
      setConfigText(JSON.stringify(saved, null, 2));
      setNotice(
        lang === 'zh'
          ? '配置已保存。请重启 gateway 生效。'
          : 'Config saved. Restart gateway to apply changes.',
      );
    } catch (err) {
      setNotice((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const saveWorkspace = async () => {
    setLoading(true);
    try {
      const parsed = JSON.parse(configText || '{}');
      if (!parsed.agents) parsed.agents = {};
      if (!parsed.agents.defaults) parsed.agents.defaults = {};
      parsed.agents.defaults.workspace = workspaceInput;
      const saved = await fetchJSON<Record<string, unknown>>('/api/config', {
        method: 'PUT',
        body: JSON.stringify(parsed),
      });
      setConfigText(JSON.stringify(saved, null, 2));
      setNotice(
        lang === 'zh'
          ? '工作区已更新。请重启 gateway 生效。'
          : 'Workspace updated. Restart gateway to apply changes.',
      );
    } catch (err) {
      setNotice((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const saveTelegram = async () => {
    if (!telegramToken.trim()) {
      setNotice(lang === 'zh' ? '需要填写 Telegram Token。' : 'Telegram token is required.');
      return;
    }
    setLoading(true);
    try {
      const parsed = JSON.parse(configText || '{}');
      if (!parsed.channels) parsed.channels = {};
      if (!parsed.channels.telegram) parsed.channels.telegram = {};
      parsed.channels.telegram.enabled = true;
      parsed.channels.telegram.token = telegramToken.trim();
      const saved = await fetchJSON<Record<string, unknown>>('/api/config', {
        method: 'PUT',
        body: JSON.stringify(parsed),
      });
      setConfigText(JSON.stringify(saved, null, 2));
      setNotice(
        lang === 'zh'
          ? 'Telegram Token 已保存。请重启 gateway 生效。'
          : 'Telegram token saved. Restart gateway to apply changes.',
      );
    } catch (err) {
      setNotice((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const restartGateway = async () => {
    setLoading(true);
    try {
      await fetchJSON<{ ok: boolean }>('/api/gateway/restart', {
        method: 'POST',
        body: '{}',
      });
      setNotice(copy.restartSent);
    } catch (err) {
      setNotice((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const toggleConfigFullscreen = async () => {
    const target = configEditorRef.current;
    if (!target || typeof document === 'undefined') return;
    try {
      if (document.fullscreenElement === target) {
        await document.exitFullscreen();
        return;
      }
      await target.requestFullscreen();
    } catch {
      setNotice(copy.fullscreenUnavailable);
    }
  };

  return (
    <div className="app">
      <header className="hero">
        <div className="hero-badge">{copy.heroBadge}</div>
        <h1>{copy.heroTitle}</h1>
        <p>{copy.heroSubtitle}</p>
        <div className="hero-actions">
          <button className="primary" onClick={refreshSessions} disabled={loading}>
            <ReloadIcon /> {copy.refresh}
          </button>
          <button
            className="secondary"
            onClick={() => setLang(lang === 'en' ? 'zh' : 'en')}
          >
            {lang === 'en' ? '中文' : 'EN'}
          </button>
          {notice && (
            <span className="notice">
              <CheckCircledIcon /> {notice}
            </span>
          )}
        </div>
      </header>

      <Tabs.Root className="tabs" defaultValue="status">
        <Tabs.List className="tab-list">
          <Tabs.Trigger value="status">{copy.statusTab}</Tabs.Trigger>
          <Tabs.Trigger value="chat">{copy.chatTab}</Tabs.Trigger>
          <Tabs.Trigger value="sessions">{copy.sessionsTab}</Tabs.Trigger>
          <Tabs.Trigger value="settings">{copy.settingsTab}</Tabs.Trigger>
        </Tabs.List>

        <Tabs.Content value="status" className="tab-content">
          <div className="grid">
            <div className="card">
              <h3>{copy.workspace}</h3>
              <p>{status?.workspace || '—'}</p>
              <span className="label">
                {copy.restrict}: {status?.restrictToWorkspace ? 'on' : 'off'}
              </span>
            </div>
            <div className="card">
              <h3>{copy.model}</h3>
              <p>{status?.model || '—'}</p>
              <span className="label">{copy.active}</span>
            </div>
            <div className="card">
              <h3>{copy.channels}</h3>
              <p>
                {status?.channels?.length
                  ? status.channels.join(', ')
                  : lang === 'zh'
                    ? '无'
                    : 'none'}
              </p>
              <span className="label">{copy.enabled}</span>
            </div>
            <div className="card">
              <h3>{copy.cron}</h3>
              <p>
                {status?.cron?.enabledJobs ?? 0} {copy.enabled}
              </p>
              <span className="label">
                {status?.cron?.totalJobs ?? 0} {copy.total}
              </span>
            </div>
            <div className="card whatsapp-card">
              <h3>WhatsApp</h3>
              <p>
                {status?.whatsapp?.enabled
                  ? status?.whatsapp?.connected
                    ? copy.connected
                    : copy.waitingQr
                  : copy.disabled}
              </p>
              {qrDataUrl ? (
                <img className="qr" src={qrDataUrl} alt="WhatsApp QR" />
              ) : (
                <span className="label">{copy.qrWaiting}</span>
              )}
            </div>
            <div className="card whatsapp-card">
              <h3>{copy.telegramTitle}</h3>
              <p>
                {status?.telegram?.enabled
                  ? status?.telegram?.status === 'ready'
                    ? `@${status?.telegram?.username || 'bot'}`
                    : status?.telegram?.status || copy.enabled
                  : copy.disabled}
              </p>
              {telegramQrDataUrl ? (
                <img className="qr" src={telegramQrDataUrl} alt="Telegram QR" />
              ) : status?.telegram?.error ? (
                <span className="label">{status.telegram.error}</span>
              ) : (
                <span className="label">{copy.telegramQrWaiting}</span>
              )}
            </div>
          </div>
        </Tabs.Content>

        <Tabs.Content value="chat" className="tab-content">
            <div className="chat-panel">
            <div className="chat-header">
              <label>
                {copy.sessionLabel}
                <select
                  value={selectedSession}
                  onChange={(e) => setSelectedSession(e.target.value)}
                >
                  {sessionOptions.map((opt) => (
                    <option key={opt.key} value={opt.key}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <div className="chat-history">
              {sessionDetail?.messages?.length ? (
                sessionDetail.messages.map((msg, idx) => (
                  <div key={idx} className={`chat-line ${msg.role}`}>
                    <span className="role">{msg.role}</span>
                    <span className="content">{msg.content}</span>
                    <span className="time">{new Date(msg.timestamp).toLocaleString()}</span>
                  </div>
                ))
              ) : (
                <div className="empty">{copy.noMessages}</div>
              )}
            </div>
            <div className="chat-input">
              <textarea
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                placeholder={copy.sendPlaceholder}
              />
              <button className="primary" onClick={sendMessage} disabled={loading}>
                {copy.send}
              </button>
            </div>
          </div>
        </Tabs.Content>

        <Tabs.Content value="sessions" className="tab-content">
          <div className="session-list">
            {sessions.length === 0 && <div className="empty">{copy.noSessions}</div>}
            {sessions.map((s) => (
              <button
                key={s.key}
                className={`session-card ${s.key === selectedSession ? 'active' : ''}`}
                onClick={() => setSelectedSession(s.key)}
              >
                <h4>{s.key}</h4>
                <p>{s.lastMessage || '—'}</p>
                <span className="label">
                  {s.messageCount} {copy.messages}
                </span>
              </button>
            ))}
          </div>
        </Tabs.Content>

        <Tabs.Content value="settings" className="tab-content">
          <div className="settings">
            <div className="card">
              <h3>{copy.workspace}</h3>
              <p>{copy.workspaceHint}</p>
              <div className="inline">
                <input
                  value={workspaceInput}
                  onChange={(e) => setWorkspaceInput(e.target.value)}
                  placeholder="/absolute/path/to/workspace"
                />
                <button className="primary" onClick={saveWorkspace} disabled={loading}>
                  {copy.save}
                </button>
              </div>
            </div>

            <div className="card">
              <h3>{copy.telegramTitle}</h3>
              <p>{copy.telegramHint}</p>
              <div className="inline">
                <input
                  value={telegramToken}
                  onChange={(e) => setTelegramToken(e.target.value)}
                  placeholder={copy.telegramPlaceholder}
                />
                <button className="primary" onClick={saveTelegram} disabled={loading}>
                  {copy.save}
                </button>
              </div>
            </div>

            <div className="card">
              <h3>{copy.gatewayTitle}</h3>
              <p>{copy.gatewayHint}</p>
              <div className="actions actions-left">
                <button className="secondary" onClick={restartGateway} disabled={loading}>
                  {copy.restartGateway}
                </button>
              </div>
            </div>

            <div className="card">
              <h3>{copy.configTitle}</h3>
              <p>{copy.configHint}</p>
              <div
                ref={configEditorRef}
                className={`config-editor-shell ${configFullscreen ? 'is-fullscreen' : ''}`}
              >
                <div className="config-toolbar">
                  <span className="label">JSON</span>
                  <button className="secondary small" onClick={toggleConfigFullscreen}>
                    {configFullscreen ? <ExitFullScreenIcon /> : <EnterFullScreenIcon />}
                    {configFullscreen ? copy.exitFullscreen : copy.fullscreen}
                  </button>
                </div>
                <Editor
                  value={configText}
                  onValueChange={setConfigText}
                  highlight={(code) => Prism.highlight(code, Prism.languages.json, 'json')}
                  padding={12}
                  textareaClassName="config-editor-textarea"
                  preClassName="config-editor-preview"
                  className="config-editor"
                />
              </div>
              <div className="actions">
                <button className="primary" onClick={saveConfig} disabled={loading}>
                  {copy.saveConfig}
                </button>
              </div>
            </div>
          </div>
        </Tabs.Content>
      </Tabs.Root>
    </div>
  );
}
