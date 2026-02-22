import React, { useState, useEffect } from 'react';
import { useTranslation } from '../i18n';
import type { EmailConfig as EmailConfigType } from '../types/channels';
import { EMAIL_PROVIDER_PRESETS } from '../types/channels';

interface EmailConfigProps {
  config: EmailConfigType;
  onChange: (config: EmailConfigType) => void;
  onTest?: () => Promise<{ success: boolean; latency?: number; error?: string }>;
}

export function EmailConfig({ config, onChange, onTest }: EmailConfigProps) {
  const { t, language } = useTranslation();
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [selectedProvider, setSelectedProvider] = useState('custom');

  // Auto-detect provider based on IMAP host
  useEffect(() => {
    if (config.imapHost) {
      const preset = EMAIL_PROVIDER_PRESETS.find(
        p => p.imapHost === config.imapHost && p.key !== 'custom'
      );
      if (preset) {
        setSelectedProvider(preset.key);
      }
    }
  }, [config.imapHost]);

  const handleProviderChange = (providerKey: string) => {
    setSelectedProvider(providerKey);
    const preset = EMAIL_PROVIDER_PRESETS.find(p => p.key === providerKey);
    if (preset && preset.key !== 'custom') {
      onChange({
        ...config,
        imapHost: preset.imapHost,
        imapPort: preset.imapPort,
        imapUseSSL: preset.imapUseSSL,
        smtpHost: preset.smtpHost,
        smtpPort: preset.smtpPort,
        smtpUseTLS: preset.smtpUseTLS,
        smtpUseSSL: preset.smtpUseSSL,
      });
    }
  };

  const handleTest = async () => {
    if (!onTest) return;
    setTesting(true);
    setTestResult(null);
    try {
      const result = await onTest();
      setTestResult({
        success: result.success,
        message: result.success
          ? `连接成功 (${result.latency}ms)`
          : result.error || '连接失败',
      });
    } catch (error) {
      setTestResult({
        success: false,
        message: error instanceof Error ? error.message : '测试失败',
      });
    } finally {
      setTesting(false);
    }
  };

  const updateField = <K extends keyof EmailConfigType>(field: K, value: EmailConfigType[K]) => {
    onChange({ ...config, [field]: value });
  };

  const renderLabel = (zh: string, en: string) => (language === 'zh' ? zh : en);

  return (
    <div className="space-y-6">
      {/* Enable Toggle */}
      <label className="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          checked={config.enabled}
          onChange={(e) => updateField('enabled', e.target.checked)}
          className="w-4 h-4 rounded border-border bg-background"
        />
        <span className="font-medium">
          {renderLabel('启用邮件收发', 'Enable Email Integration')}
        </span>
      </label>

      {config.enabled && (
        <>
          {/* Provider Selection */}
          <div>
            <label className="block text-sm font-medium mb-2">
              {renderLabel('邮箱服务商', 'Email Provider')}
            </label>
            <select
              value={selectedProvider}
              onChange={(e) => handleProviderChange(e.target.value)}
              className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
            >
              {EMAIL_PROVIDER_PRESETS.map((preset) => (
                <option key={preset.key} value={preset.key}>
                  {preset.name}
                </option>
              ))}
            </select>
          </div>

          {/* Consent Checkbox */}
          <div className="bg-yellow-500/10 border border-yellow-500/20 rounded-lg p-4">
            <label className="flex items-start gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={config.consentGranted}
                onChange={(e) => updateField('consentGranted', e.target.checked)}
                className="w-4 h-4 mt-0.5 rounded border-border bg-background"
              />
              <div className="text-sm">
                <p className="font-medium mb-1">
                  {renderLabel('隐私与安全声明', 'Privacy & Security Notice')}
                </p>
                <p className="text-foreground/60">
                  {renderLabel(
                    '启用邮件功能后，Bot 将能够读取您的邮件内容并发送回复。您的邮箱凭证将被安全存储在本地配置文件中。请确保您信任此 Bot 后再启用此功能。',
                    'When enabled, the bot will be able to read your emails and send replies. Your email credentials will be stored securely in the local config file. Please ensure you trust this bot before enabling.'
                  )}
                </p>
              </div>
            </label>
          </div>

          {/* IMAP Settings */}
          <div className="border border-border rounded-lg p-4 space-y-4">
            <h4 className="font-medium text-sm uppercase tracking-wide text-foreground/60">
              {renderLabel('IMAP 收件设置', 'IMAP Incoming Settings')}
            </h4>

            <div className="grid grid-cols-2 gap-4">
              <div className="col-span-2">
                <label className="block text-sm font-medium mb-1">
                  {renderLabel('IMAP 服务器', 'IMAP Server')}
                </label>
                <input
                  type="text"
                  value={config.imapHost || ''}
                  onChange={(e) => updateField('imapHost', e.target.value)}
                  placeholder="imap.gmail.com"
                  className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-1">
                  {renderLabel('端口', 'Port')}
                </label>
                <input
                  type="number"
                  value={config.imapPort || 993}
                  onChange={(e) => updateField('imapPort', parseInt(e.target.value))}
                  className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-1">
                  {renderLabel('邮箱地址', 'Email Address')}
                </label>
                <input
                  type="email"
                  value={config.imapUsername || ''}
                  onChange={(e) => {
                    updateField('imapUsername', e.target.value);
                    if (!config.smtpUsername) {
                      updateField('smtpUsername', e.target.value);
                    }
                    if (!config.fromAddress) {
                      updateField('fromAddress', e.target.value);
                    }
                  }}
                  placeholder="user@gmail.com"
                  className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
                />
              </div>

              <div className="col-span-2">
                <label className="block text-sm font-medium mb-1">
                  {renderLabel('密码 / 授权码', 'Password / App Password')}
                </label>
                <input
                  type="password"
                  value={config.imapPassword || ''}
                  onChange={(e) => {
                    updateField('imapPassword', e.target.value);
                    if (!config.smtpPassword) {
                      updateField('smtpPassword', e.target.value);
                    }
                  }}
                  placeholder={renderLabel('对于 Gmail，请使用应用专用密码', 'For Gmail, use an App Password')}
                  className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
                />
              </div>

              <div className="col-span-2 flex gap-4">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={config.imapUseSSL !== false}
                    onChange={(e) => updateField('imapUseSSL', e.target.checked)}
                    className="w-4 h-4 rounded border-border bg-background"
                  />
                  <span className="text-sm">{renderLabel('使用 SSL', 'Use SSL')}</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={config.markSeen !== false}
                    onChange={(e) => updateField('markSeen', e.target.checked)}
                    className="w-4 h-4 rounded border-border bg-background"
                  />
                  <span className="text-sm">{renderLabel('读取后标记为已读', 'Mark as read after processing')}</span>
                </label>
              </div>
            </div>
          </div>

          {/* SMTP Settings */}
          <div className="border border-border rounded-lg p-4 space-y-4">
            <h4 className="font-medium text-sm uppercase tracking-wide text-foreground/60">
              {renderLabel('SMTP 发件设置', 'SMTP Outgoing Settings')}
            </h4>

            <div className="grid grid-cols-2 gap-4">
              <div className="col-span-2">
                <label className="block text-sm font-medium mb-1">
                  {renderLabel('SMTP 服务器', 'SMTP Server')}
                </label>
                <input
                  type="text"
                  value={config.smtpHost || ''}
                  onChange={(e) => updateField('smtpHost', e.target.value)}
                  placeholder="smtp.gmail.com"
                  className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-1">
                  {renderLabel('端口', 'Port')}
                </label>
                <input
                  type="number"
                  value={config.smtpPort || 587}
                  onChange={(e) => updateField('smtpPort', parseInt(e.target.value))}
                  className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-1">
                  {renderLabel('发件人地址', 'From Address')}
                </label>
                <input
                  type="email"
                  value={config.fromAddress || ''}
                  onChange={(e) => updateField('fromAddress', e.target.value)}
                  placeholder="bot@example.com"
                  className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
                />
              </div>

              <div className="col-span-2 flex gap-4">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={config.smtpUseTLS !== false}
                    onChange={(e) => updateField('smtpUseTLS', e.target.checked)}
                    className="w-4 h-4 rounded border-border bg-background"
                  />
                  <span className="text-sm">{renderLabel('使用 TLS', 'Use TLS')}</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={config.smtpUseSSL || false}
                    onChange={(e) => updateField('smtpUseSSL', e.target.checked)}
                    className="w-4 h-4 rounded border-border bg-background"
                  />
                  <span className="text-sm">{renderLabel('使用 SSL', 'Use SSL')}</span>
                </label>

                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={config.autoReplyEnabled !== false}
                    onChange={(e) => updateField('autoReplyEnabled', e.target.checked)}
                    className="w-4 h-4 rounded border-border bg-background"
                  />
                  <span className="text-sm">{renderLabel('自动回复', 'Auto Reply')}</span>
                </label>
              </div>
            </div>
          </div>

          {/* Polling Settings */}
          <div>
            <label className="block text-sm font-medium mb-1">
              {renderLabel('检查频率（秒）', 'Poll Interval (seconds)')}
            </label>
            <input
              type="number"
              min={10}
              max={3600}
              value={config.pollIntervalSeconds || 30}
              onChange={(e) => updateField('pollIntervalSeconds', parseInt(e.target.value))}
              className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
            />
            <p className="text-xs text-foreground/50 mt-1">
              {renderLabel('建议 30-60 秒，过快可能导致被封禁', 'Recommended 30-60 seconds, too fast may get blocked')}
            </p>
          </div>

          {/* Allowed Senders */}
          <div>
            <label className="block text-sm font-medium mb-1">
              {renderLabel('允许的发件人（可选）', 'Allowed Senders (optional)')}
            </label>
            <textarea
              value={(config.allowFrom || []).join('\n')}
              onChange={(e) => updateField('allowFrom', e.target.value.split('\n').filter(s => s.trim()))}
              placeholder={renderLabel('user1@example.com\nuser2@example.com', 'user1@example.com\nuser2@example.com')}
              rows={3}
              className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
            />
            <p className="text-xs text-foreground/50 mt-1">
              {renderLabel('每行一个邮箱地址，留空表示允许所有人', 'One email per line, leave empty to allow all')}
            </p>
          </div>

          {/* Test Connection */}
          {onTest && (
            <div className="flex items-center gap-4">
              <button
                onClick={handleTest}
                disabled={testing || !config.imapHost || !config.imapUsername || !config.imapPassword}
                className="px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm font-medium hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                {testing
                  ? renderLabel('测试中...', 'Testing...')
                  : renderLabel('测试连接', 'Test Connection')}
              </button>

              {testResult && (
                <span
                  className={`text-sm ${testResult.success ? 'text-green-500' : 'text-red-500'}`}
                >
                  {testResult.message}
                </span>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}
