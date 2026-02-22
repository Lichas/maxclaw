import React, { useState } from 'react';
import { useTranslation } from '../i18n';
import type { ChannelsConfig } from '../types/channels';
import { CHANNEL_DEFINITIONS } from '../types/channels';

interface IMBotConfigProps {
  config: ChannelsConfig;
  onChange: (config: ChannelsConfig) => void;
  onTestChannel?: (channel: keyof ChannelsConfig) => Promise<{ success: boolean; error?: string }>;
}

type ChannelKey = keyof ChannelsConfig;

export function IMBotConfig({ config, onChange, onTestChannel }: IMBotConfigProps) {
  const { t, language } = useTranslation();
  const [activeChannel, setActiveChannel] = useState<ChannelKey>('telegram');
  const [testing, setTesting] = useState<ChannelKey | null>(null);
  const [testResults, setTestResults] = useState<Record<ChannelKey, { success: boolean; message: string } | null>>({
    telegram: null,
    discord: null,
    whatsapp: null,
    websocket: null,
    slack: null,
    email: null,
    qq: null,
    feishu: null,
  });

  const renderLabel = (zh: string, en: string) => (language === 'zh' ? zh : en);

  const updateChannel = <K extends ChannelKey>(
    channel: K,
    updates: Partial<ChannelsConfig[K]>
  ) => {
    onChange({
      ...config,
      [channel]: { ...config[channel], ...updates },
    });
  };

  const handleTest = async (channel: ChannelKey) => {
    if (!onTestChannel) return;
    setTesting(channel);
    setTestResults((prev) => ({ ...prev, [channel]: null }));
    try {
      const result = await onTestChannel(channel);
      setTestResults((prev) => ({
        ...prev,
        [channel]: {
          success: result.success,
          message: result.success
            ? renderLabel('连接成功', 'Connection successful')
            : result.error || renderLabel('连接失败', 'Connection failed'),
        },
      }));
    } catch (error) {
      setTestResults((prev) => ({
        ...prev,
        [channel]: {
          success: false,
          message: error instanceof Error ? error.message : 'Test failed',
        },
      }));
    } finally {
      setTesting(null);
    }
  };

  const renderField = (
    channel: ChannelKey,
    field: (typeof CHANNEL_DEFINITIONS)[number]['fields'][number],
    value: unknown
  ) => {
    const label = language === 'zh' ? field.labelZh : field.label;
    const placeholder = language === 'zh' ? field.placeholderZh : field.placeholder;
    const hint = language === 'zh' ? field.hintZh : field.hint;

    const inputClass =
      'w-full bg-background border border-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary';

    switch (field.type) {
      case 'text':
      case 'password':
        return (
          <div key={field.key}>
            <label className="block text-sm font-medium mb-1">
              {label}
              {field.required && <span className="text-red-500 ml-1">*</span>}
            </label>
            <input
              type={field.type}
              value={(value as string) || ''}
              onChange={(e) =>
                updateChannel(channel, { [field.key]: e.target.value } as Partial<ChannelsConfig[typeof channel]>)
              }
              placeholder={placeholder}
              className={inputClass}
            />
            {hint && <p className="text-xs text-foreground/50 mt-1">{hint}</p>}
          </div>
        );

      case 'number':
        return (
          <div key={field.key}>
            <label className="block text-sm font-medium mb-1">
              {label}
              {field.required && <span className="text-red-500 ml-1">*</span>}
            </label>
            <input
              type="number"
              value={(value as number) || 0}
              onChange={(e) =>
                updateChannel(channel, { [field.key]: parseInt(e.target.value) } as Partial<ChannelsConfig[typeof channel]>)
              }
              placeholder={placeholder}
              className={inputClass}
            />
            {hint && <p className="text-xs text-foreground/50 mt-1">{hint}</p>}
          </div>
        );

      case 'checkbox':
        return (
          <div key={field.key}>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={(value as boolean) || false}
                onChange={(e) =>
                  updateChannel(channel, { [field.key]: e.target.checked } as Partial<ChannelsConfig[typeof channel]>)
                }
                className="w-4 h-4 rounded border-border bg-background"
              />
              <span className="text-sm font-medium">{label}</span>
            </label>
            {hint && <p className="text-xs text-foreground/50 mt-1 ml-6">{hint}</p>}
          </div>
        );

      case 'list':
        return (
          <div key={field.key}>
            <label className="block text-sm font-medium mb-1">{label}</label>
            <textarea
              value={Array.isArray(value) ? value.join('\n') : ''}
              onChange={(e) =>
                updateChannel(channel, {
                  [field.key]: e.target.value.split('\n').filter((s) => s.trim()),
                } as Partial<ChannelsConfig[typeof channel]>)
              }
              placeholder={placeholder}
              rows={3}
              className={inputClass}
            />
            {hint && <p className="text-xs text-foreground/50 mt-1">{hint}</p>}
          </div>
        );

      case 'textarea':
        return (
          <div key={field.key}>
            <label className="block text-sm font-medium mb-1">
              {label}
              {field.required && <span className="text-red-500 ml-1">*</span>}
            </label>
            <textarea
              value={(value as string) || ''}
              onChange={(e) =>
                updateChannel(channel, { [field.key]: e.target.value } as Partial<ChannelsConfig[typeof channel]>)
              }
              placeholder={placeholder}
              rows={4}
              className={inputClass}
            />
            {hint && <p className="text-xs text-foreground/50 mt-1">{hint}</p>}
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <div className="space-y-6">
      {/* Channel Selector Tabs */}
      <div className="border-b border-border">
        <div className="flex gap-1 overflow-x-auto">
          {CHANNEL_DEFINITIONS.map((def) => {
            const isActive = activeChannel === def.key;
            const channelConfig = config[def.key];
            const isEnabled = channelConfig?.enabled;

            return (
              <button
                key={def.key}
                onClick={() => setActiveChannel(def.key)}
                className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors whitespace-nowrap ${
                  isActive
                    ? 'border-primary text-primary'
                    : 'border-transparent text-foreground/60 hover:text-foreground'
                }`}
              >
                <span className="flex items-center gap-2">
                  {isEnabled && (
                    <span className="w-2 h-2 rounded-full bg-green-500" title="Enabled" />
                  )}
                  {language === 'zh' ? def.nameZh : def.name}
                </span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Active Channel Config */}
      {CHANNEL_DEFINITIONS.map((def) => {
        if (def.key !== activeChannel) return null;

        const channelConfig = config[def.key];

        return (
          <div key={def.key} className="space-y-6">
            {/* Header */}
            <div className="flex items-start justify-between">
              <div>
                <h3 className="text-lg font-semibold flex items-center gap-2">
                  {language === 'zh' ? def.nameZh : def.name}
                  {channelConfig.enabled && (
                    <span className="px-2 py-0.5 bg-green-500/10 text-green-500 text-xs rounded-full">
                      {renderLabel('已启用', 'Enabled')}
                    </span>
                  )}
                </h3>
                <p className="text-sm text-foreground/60 mt-1">
                  {language === 'zh' ? def.descriptionZh : def.description}
                </p>
              </div>
              <a
                href={def.docsUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm text-primary hover:underline"
              >
                {renderLabel('查看文档 →', 'View Docs →')}
              </a>
            </div>

            {/* Enable Toggle */}
            <label className="flex items-center gap-3 p-4 bg-secondary rounded-lg cursor-pointer">
              <input
                type="checkbox"
                checked={channelConfig.enabled}
                onChange={(e) => updateChannel(def.key, { enabled: e.target.checked } as Partial<ChannelsConfig[typeof def.key]>)}
                className="w-4 h-4 rounded border-border bg-background"
              />
              <span className="font-medium">
                {renderLabel('启用此频道', 'Enable this channel')}
              </span>
            </label>

            {channelConfig.enabled && (
              <div className="space-y-6">
                {/* Fields */}
                <div className="grid grid-cols-2 gap-4">
                  {def.fields.map((field) =>
                    renderField(def.key, field, channelConfig[field.key as keyof typeof channelConfig])
                  )}
                </div>

                {/* Actions */}
                <div className="flex items-center gap-4 pt-4 border-t border-border">
                  {onTestChannel && (
                    <>
                      <button
                        onClick={() => handleTest(def.key)}
                        disabled={testing === def.key}
                        className="px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm font-medium hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                      >
                        {testing === def.key
                          ? renderLabel('测试中...', 'Testing...')
                          : renderLabel('测试连接', 'Test Connection')}
                      </button>

                      {testResults[def.key] && (
                        <span
                          className={`text-sm ${
                            testResults[def.key]!.success ? 'text-green-500' : 'text-red-500'
                          }`}
                        >
                          {testResults[def.key]!.message}
                        </span>
                      )}
                    </>
                  )}
                </div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
