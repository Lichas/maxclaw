import React, { useState, useEffect } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { RootState, setTheme, setLanguage } from '../store';
import { useTranslation } from '../i18n';
import { ProviderConfig, PRESET_PROVIDERS } from '../types/providers';
import { ProviderEditor } from '../components/ProviderEditor';
import { EmailConfig } from '../components/EmailConfig';
import { IMBotConfig } from '../components/IMBotConfig';
import type { ChannelsConfig, EmailConfig as EmailConfigType } from '../types/channels';

interface Settings {
  theme: 'light' | 'dark' | 'system';
  language: 'zh' | 'en';
  autoLaunch: boolean;
  minimizeToTray: boolean;
  notificationsEnabled: boolean;
}

export function SettingsView() {
  const dispatch = useDispatch();
  const { t } = useTranslation();
  const { theme: storeTheme, language: storeLanguage } = useSelector((state: RootState) => state.ui);

  const [settings, setSettings] = useState<Settings>({
    theme: 'system',
    language: 'zh',
    autoLaunch: false,
    minimizeToTray: true,
    notificationsEnabled: true
  });
  const [gatewayConfig, setGatewayConfig] = useState<any>(null);
  const [providers, setProviders] = useState<ProviderConfig[]>([]);
  const [editingProvider, setEditingProvider] = useState<ProviderConfig | null>(null);
  const [showAddProvider, setShowAddProvider] = useState(false);

  // Channel config states
  const [channels, setChannels] = useState<ChannelsConfig>({
    telegram: { enabled: false, token: '', allowFrom: [] },
    discord: { enabled: false, token: '', allowFrom: [] },
    whatsapp: { enabled: false, bridgeUrl: '', bridgeToken: '', allowFrom: [], allowSelf: false },
    websocket: { enabled: false, host: '0.0.0.0', port: 18791, path: '/ws', allowOrigins: [] },
    slack: { enabled: false, botToken: '', appToken: '', allowFrom: [] },
    email: { enabled: false, consentGranted: false, allowFrom: [], imapPort: 993, smtpPort: 587, pollIntervalSeconds: 30 },
    qq: { enabled: false, wsUrl: '', accessToken: '', allowFrom: [] },
    feishu: { enabled: false, appId: '', appSecret: '', verificationToken: '', listenAddr: '0.0.0.0:18792', webhookPath: '/feishu/events', allowFrom: [] },
  });

  useEffect(() => {
    // Load app settings
    window.electronAPI.config.get().then((config) => {
      setSettings({
        theme: config.theme || 'system',
        language: config.language || 'zh',
        autoLaunch: config.autoLaunch || false,
        minimizeToTray: config.minimizeToTray !== false,
        notificationsEnabled: config.notificationsEnabled !== false
      });
    });

    // Request notification permission on mount
    if (window.electronAPI.system.requestNotificationPermission) {
      window.electronAPI.system.requestNotificationPermission();
    }

    // Load Gateway config
    fetch('http://localhost:18890/api/config')
      .then(res => res.json())
      .then(config => {
        setGatewayConfig(config);
        // Convert gateway providers format to our format
        if (config.providers) {
          const loadedProviders: ProviderConfig[] = [];
          Object.entries(config.providers).forEach(([key, value]: [string, any]) => {
            if (value && (value.apiKey || value.apiBase)) {
              loadedProviders.push({
                id: key,
                name: key.charAt(0).toUpperCase() + key.slice(1),
                type: key === 'anthropic' ? 'anthropic' : 'openai',
                apiKey: value.apiKey || '',
                baseURL: value.apiBase || '',
                apiFormat: key === 'anthropic' ? 'anthropic' : 'openai',
                models: [],
                enabled: true,
              });
            }
          });
          setProviders(loadedProviders);
        }
        // Load channels config
        if (config.channels) {
          setChannels(prev => ({
            ...prev,
            ...config.channels
          }));
        }
      })
      .catch(console.error);
  }, []);

  // Sync with store changes
  useEffect(() => {
    setSettings(prev => ({
      ...prev,
      theme: storeTheme,
      language: storeLanguage
    }));
  }, [storeTheme, storeLanguage]);

  const handleChange = async <K extends keyof Settings>(key: K, value: Settings[K]) => {
    const updated = { ...settings, [key]: value };
    setSettings(updated);
    await window.electronAPI.config.set({ [key]: value });

    // Update store for immediate UI feedback
    if (key === 'theme') {
      dispatch(setTheme(value as 'light' | 'dark' | 'system'));
    } else if (key === 'language') {
      dispatch(setLanguage(value as 'zh' | 'en'));
    }
  };

  const handleRestartGateway = async () => {
    await window.electronAPI.gateway.restart();
  };

  const handleAddProvider = (preset: typeof PRESET_PROVIDERS[0]) => {
    const newProvider: ProviderConfig = {
      ...preset,
      id: `${Date.now()}`,
      apiKey: '',
    };
    setEditingProvider(newProvider);
    setShowAddProvider(false);
  };

  const handleSaveProvider = async (provider: ProviderConfig) => {
    try {
      const existingIndex = providers.findIndex((p) => p.id === provider.id);
      let newProviders;

      if (existingIndex >= 0) {
        newProviders = [...providers];
        newProviders[existingIndex] = provider;
      } else {
        newProviders = [...providers, provider];
      }

      // Convert to gateway config format
      const gatewayProviders: Record<string, { apiKey: string; apiBase?: string }> = {};
      newProviders.forEach((p) => {
        const key = p.name.toLowerCase().replace(/\s+/g, '');
        gatewayProviders[key] = {
          apiKey: p.apiKey,
          apiBase: p.baseURL,
        };
      });

      // Update Gateway config
      const response = await fetch('http://localhost:18890/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ providers: gatewayProviders }),
      });

      if (response.ok) {
        setProviders(newProviders);
        setEditingProvider(null);

        // Restart Gateway to apply changes
        await fetch('http://localhost:18890/api/gateway/restart', {
          method: 'POST',
        });
      }
    } catch (error) {
      console.error('Failed to save provider:', error);
    }
  };

  const handleTestConnection = async (provider: ProviderConfig) => {
    try {
      const startTime = Date.now();
      const response = await fetch('http://localhost:18890/api/providers/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(provider),
      });
      const latency = Date.now() - startTime;

      if (response.ok) {
        return { success: true, latency };
      } else {
        const error = await response.text();
        return { success: false, error };
      }
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : 'Connection failed',
      };
    }
  };

  const handleDeleteProvider = async (id: string) => {
    const newProviders = providers.filter((p) => p.id !== id);
    setProviders(newProviders);

    // Update Gateway config
    const gatewayProviders: Record<string, { apiKey: string; apiBase?: string }> = {};
    newProviders.forEach((p) => {
      const key = p.name.toLowerCase().replace(/\s+/g, '');
      gatewayProviders[key] = {
        apiKey: p.apiKey,
        apiBase: p.baseURL,
      };
    });

    await fetch('http://localhost:18890/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ providers: gatewayProviders }),
    });
  };

  // Channel config handlers
  const handleChannelsChange = async (newChannels: ChannelsConfig) => {
    setChannels(newChannels);
    try {
      await fetch('http://localhost:18890/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ channels: newChannels }),
      });
    } catch (error) {
      console.error('Failed to save channels config:', error);
    }
  };

  const handleTestChannel = async (channel: keyof ChannelsConfig): Promise<{ success: boolean; error?: string }> => {
    try {
      const response = await fetch(`http://localhost:18890/api/channels/${channel}/test`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(channels[channel]),
      });

      if (response.ok) {
        return { success: true };
      } else {
        const error = await response.text();
        return { success: false, error };
      }
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : 'Connection failed',
      };
    }
  };

  const handleTestEmail = async (): Promise<{ success: boolean; latency?: number; error?: string }> => {
    try {
      const startTime = Date.now();
      const response = await fetch('http://localhost:18890/api/channels/email/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(channels.email),
      });
      const latency = Date.now() - startTime;

      if (response.ok) {
        return { success: true, latency };
      } else {
        const error = await response.text();
        return { success: false, error };
      }
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : 'Connection failed',
      };
    }
  };

  return (
    <div className="h-full overflow-y-auto p-6">
      <h1 className="text-2xl font-bold mb-6">{t('settings.title')}</h1>

      {/* Appearance */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-4">{t('settings.appearance')}</h2>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <label className="text-sm font-medium">{t('settings.theme')}</label>
            <select
              value={settings.theme}
              onChange={(e) => handleChange('theme', e.target.value as Settings['theme'])}
              className="bg-secondary rounded-lg px-3 py-2 text-sm border border-border"
            >
              <option value="light">{t('settings.theme.light')}</option>
              <option value="dark">{t('settings.theme.dark')}</option>
              <option value="system">{t('settings.theme.system')}</option>
            </select>
          </div>

          <div className="flex items-center justify-between">
            <label className="text-sm font-medium">{t('settings.language')}</label>
            <select
              value={settings.language}
              onChange={(e) => handleChange('language', e.target.value as Settings['language'])}
              className="bg-secondary rounded-lg px-3 py-2 text-sm border border-border"
            >
              <option value="zh">{t('settings.language.zh')}</option>
              <option value="en">{t('settings.language.en')}</option>
            </select>
          </div>
        </div>
      </section>

      {/* System */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-4">{t('settings.system')}</h2>
        <div className="space-y-4">
          <label className="flex items-center justify-between cursor-pointer">
            <span className="text-sm font-medium">{t('settings.autoLaunch')}</span>
            <input
              type="checkbox"
              checked={settings.autoLaunch}
              onChange={(e) => handleChange('autoLaunch', e.target.checked)}
              className="w-4 h-4"
            />
          </label>

          <label className="flex items-center justify-between cursor-pointer">
            <span className="text-sm font-medium">{t('settings.minimizeToTray')}</span>
            <input
              type="checkbox"
              checked={settings.minimizeToTray}
              onChange={(e) => handleChange('minimizeToTray', e.target.checked)}
              className="w-4 h-4"
            />
          </label>
        </div>
      </section>

      {/* Notifications */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-4">{t('settings.notifications')}</h2>
        <div className="space-y-4">
          <label className="flex items-center justify-between cursor-pointer">
            <span className="text-sm font-medium">{t('settings.notifications.enable')}</span>
            <input
              type="checkbox"
              checked={settings.notificationsEnabled}
              onChange={(e) => handleChange('notificationsEnabled', e.target.checked)}
              className="w-4 h-4"
            />
          </label>
        </div>
      </section>

      {/* Providers */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-4">{t('settings.providers') || 'Model Providers'}</h2>

        {editingProvider ? (
          <ProviderEditor
            provider={editingProvider}
            onSave={handleSaveProvider}
            onTest={handleTestConnection}
            onCancel={() => setEditingProvider(null)}
          />
        ) : showAddProvider ? (
          <div className="rounded-lg border border-border bg-card p-4">
            <h3 className="mb-3 text-sm font-medium">{t('settings.providers.add') || 'Select Provider'}</h3>
            <div className="flex flex-wrap gap-2">
              {PRESET_PROVIDERS.map((preset) => (
                <button
                  key={preset.name}
                  onClick={() => handleAddProvider(preset)}
                  className="rounded-lg border border-border px-3 py-2 text-sm hover:bg-secondary"
                >
                  + {preset.name}
                </button>
              ))}
              <button
                onClick={() =>
                  handleAddProvider({
                    name: 'Custom',
                    type: 'custom',
                    apiFormat: 'openai',
                    models: [],
                    enabled: false,
                  })
                }
                className="rounded-lg border border-dashed border-border px-3 py-2 text-sm hover:bg-secondary"
              >
                + Custom
              </button>
            </div>
            <button
              onClick={() => setShowAddProvider(false)}
              className="mt-3 text-sm text-foreground/60 hover:text-foreground"
            >
              Cancel
            </button>
          </div>
        ) : (
          <div className="space-y-3">
            {providers.length === 0 ? (
              <p className="text-sm text-foreground/50">{t('settings.providers.empty') || 'No providers configured.'}</p>
            ) : (
              providers.map((provider) => (
                <div
                  key={provider.id}
                  className="flex items-center justify-between rounded-lg border border-border bg-card p-3"
                >
                  <div>
                    <h3 className="font-medium">{provider.name}</h3>
                    <p className="text-xs text-foreground/60">
                      {provider.baseURL || 'Default endpoint'}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => setEditingProvider(provider)}
                      className="rounded-lg border border-border px-3 py-1.5 text-sm hover:bg-secondary"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => handleDeleteProvider(provider.id)}
                      className="rounded-lg border border-border px-3 py-1.5 text-sm text-red-500 hover:bg-red-500/10"
                    >
                      Delete
                    </button>
                  </div>
                </div>
              ))
            )}
            <button
              onClick={() => setShowAddProvider(true)}
              className="w-full rounded-lg border border-dashed border-border py-2 text-sm text-foreground/60 hover:bg-secondary hover:text-foreground"
            >
              + {t('settings.providers.add') || 'Add Provider'}
            </button>
          </div>
        )}
      </section>

      {/* Email Configuration */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-4">
          {language === 'zh' ? '邮箱配置' : 'Email Configuration'}
        </h2>
        <EmailConfig
          config={channels.email}
          onChange={(emailConfig) => handleChannelsChange({ ...channels, email: emailConfig })}
          onTest={handleTestEmail}
        />
      </section>

      {/* IM Bot Configuration */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-4">
          {language === 'zh' ? 'IM Bot 配置' : 'IM Bot Configuration'}
        </h2>
        <IMBotConfig
          config={channels}
          onChange={handleChannelsChange}
          onTestChannel={handleTestChannel}
        />
      </section>

      {/* Gateway */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-4">{t('settings.gateway')}</h2>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">{t('settings.gateway.status')}</span>
            <button
              onClick={handleRestartGateway}
              className="px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm hover:bg-primary/90"
            >
              {t('settings.gateway.restart')}
            </button>
          </div>

          {gatewayConfig && (
            <div className="bg-secondary rounded-lg p-4 mt-4">
              <h3 className="text-sm font-medium mb-2">{t('settings.gateway.currentModel')}</h3>
              <code className="text-xs bg-background px-2 py-1 rounded block mb-4">
                {gatewayConfig.agents?.defaults?.model || t('settings.gateway.notConfigured')}
              </code>

              <h3 className="text-sm font-medium mb-2">{t('settings.gateway.workspace')}</h3>
              <code className="text-xs bg-background px-2 py-1 rounded block">
                {gatewayConfig.agents?.defaults?.workspace || t('settings.gateway.notConfigured')}
              </code>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
