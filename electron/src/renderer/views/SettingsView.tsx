import React, { useState, useEffect } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { RootState, setTheme, setLanguage } from '../store';
import { useTranslation } from '../i18n';
import { ProviderConfig, PRESET_PROVIDERS } from '../types/providers';
import { ProviderEditor } from '../components/ProviderEditor';
import { EmailConfig } from '../components/EmailConfig';
import { IMBotConfig } from '../components/IMBotConfig';
import { CustomSelect } from '../components/CustomSelect';
import { useGateway } from '../hooks/useGateway';
import type { ChannelsConfig } from '../types/channels';

interface Settings {
  theme: 'light' | 'dark' | 'system';
  language: 'zh' | 'en';
  autoLaunch: boolean;
  minimizeToTray: boolean;
  notificationsEnabled: boolean;
}

interface ShortcutsState {
  toggleWindow: string;
  newChat: string;
}

type SettingsCategory = 'general' | 'providers' | 'channels' | 'gateway' | 'advanced';

export function SettingsView() {
  const dispatch = useDispatch();
  const { t, language } = useTranslation();
  const { theme: storeTheme, language: storeLanguage } = useSelector((state: RootState) => state.ui);
  const { getWhatsAppStatus } = useGateway();

  const [settings, setSettings] = useState<Settings>({
    theme: 'system',
    language: 'zh',
    autoLaunch: false,
    minimizeToTray: true,
    notificationsEnabled: true
  });
  const [gatewayConfig, setGatewayConfig] = useState<any>(null);
  const [maxToolIterationsInput, setMaxToolIterationsInput] = useState('200');
  const [savingMaxToolIterations, setSavingMaxToolIterations] = useState(false);
  const [executionModeInput, setExecutionModeInput] = useState<'safe' | 'ask' | 'auto'>('ask');
  const [savingExecutionMode, setSavingExecutionMode] = useState(false);
  const [providers, setProviders] = useState<ProviderConfig[]>([]);
  const [editingProvider, setEditingProvider] = useState<ProviderConfig | null>(null);
  const [showAddProvider, setShowAddProvider] = useState(false);
  const [activeCategory, setActiveCategory] = useState<SettingsCategory>('general');

  const [shortcuts, setShortcuts] = useState<ShortcutsState>({
    toggleWindow: 'CommandOrControl+Shift+Space',
    newChat: 'CommandOrControl+N',
  });

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

  // Update states
  const [updateStatus, setUpdateStatus] = useState<'checking' | 'available' | 'downloading' | 'downloaded' | 'none'>('none');
  const [updateInfo, setUpdateInfo] = useState<{ version: string; releaseDate?: string } | null>(null);

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
        const configuredMaxIterations = config.agents?.defaults?.maxToolIterations;
        if (typeof configuredMaxIterations === 'number' && configuredMaxIterations > 0) {
          setMaxToolIterationsInput(String(configuredMaxIterations));
        }
        const configuredExecutionMode = config.agents?.defaults?.executionMode;
        if (configuredExecutionMode === 'safe' || configuredExecutionMode === 'ask' || configuredExecutionMode === 'auto') {
          setExecutionModeInput(configuredExecutionMode);
        }
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

    // Load shortcuts config
    window.electronAPI.shortcuts?.get?.().then((current: Record<string, string>) => {
      setShortcuts(prev => ({ ...prev, ...current }));
    }).catch(console.error);

    // Setup update listeners
    const unsubscribeAvailable = window.electronAPI.update?.onAvailable?.((info: { version: string; releaseDate?: string }) => {
      setUpdateStatus('available');
      setUpdateInfo(info);
    });

    const unsubscribeDownloaded = window.electronAPI.update?.onDownloaded?.(() => {
      setUpdateStatus('downloaded');
    });

    return () => {
      unsubscribeAvailable?.();
      unsubscribeDownloaded?.();
    };
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

  const handleShortcutChange = (key: keyof ShortcutsState, value: string) => {
    const updated = { ...shortcuts, [key]: value };
    setShortcuts(updated);
    window.electronAPI.config.set({ shortcuts: updated });
    window.electronAPI.shortcuts?.update?.(updated);
  };

  const handleRestartGateway = async () => {
    await window.electronAPI.gateway.restart();
  };

  const handleSaveMaxToolIterations = async () => {
    const parsed = Number.parseInt(maxToolIterationsInput.trim(), 10);
    if (!Number.isFinite(parsed) || parsed < 1) {
      alert(t('settings.gateway.maxToolIterations.invalid'));
      return;
    }
    if (!gatewayConfig?.agents?.defaults) {
      alert(t('settings.gateway.notConfigured'));
      return;
    }

    setSavingMaxToolIterations(true);
    try {
      const updatedAgents = {
        ...gatewayConfig.agents,
        defaults: {
          ...gatewayConfig.agents.defaults,
          maxToolIterations: parsed,
        },
      };

      const response = await fetch('http://localhost:18890/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agents: updatedAgents }),
      });
      if (!response.ok) {
        throw new Error(`save failed: ${response.status}`);
      }
      const updatedConfig = await response.json();
      setGatewayConfig(updatedConfig);
      setMaxToolIterationsInput(String(updatedConfig.agents?.defaults?.maxToolIterations ?? parsed));
    } catch (error) {
      console.error('Failed to save max tool iterations:', error);
      alert(t('settings.gateway.maxToolIterations.saveError'));
    } finally {
      setSavingMaxToolIterations(false);
    }
  };

  const handleSaveExecutionMode = async () => {
    if (!gatewayConfig?.agents?.defaults) {
      alert(t('settings.gateway.notConfigured'));
      return;
    }

    setSavingExecutionMode(true);
    try {
      const updatedAgents = {
        ...gatewayConfig.agents,
        defaults: {
          ...gatewayConfig.agents.defaults,
          executionMode: executionModeInput,
        },
      };

      const response = await fetch('http://localhost:18890/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agents: updatedAgents }),
      });
      if (!response.ok) {
        throw new Error(`save failed: ${response.status}`);
      }
      const updatedConfig = await response.json();
      setGatewayConfig(updatedConfig);
      const mode = updatedConfig.agents?.defaults?.executionMode;
      if (mode === 'safe' || mode === 'ask' || mode === 'auto') {
        setExecutionModeInput(mode);
      }
    } catch (error) {
      console.error('Failed to save execution mode:', error);
      alert(t('settings.gateway.executionMode.saveError'));
    } finally {
      setSavingExecutionMode(false);
    }
  };

  const handleExport = async () => {
    const result = await window.electronAPI.data?.export?.();
    if (result?.success) {
      alert(`备份已保存到: ${result.path}`);
    } else if (result?.error) {
      alert(`导出失败: ${result.error}`);
    }
  };

  const handleImport = async () => {
    if (!confirm('导入将覆盖当前配置，确定继续吗？')) return;

    const result = await window.electronAPI.data?.import?.();
    if (result?.success) {
      alert('导入成功，应用将重启');
      window.electronAPI.gateway?.restart?.();
    } else if (result?.error) {
      alert(`导入失败: ${result.error}`);
    }
  };

  const handleCheckUpdate = async () => {
    setUpdateStatus('checking');
    const result = await window.electronAPI.update?.check?.();
    if (!result?.updateInfo) {
      setUpdateStatus('none');
      alert(t('settings.noUpdate') || '当前已是最新版本');
    }
  };

  const handleDownload = async () => {
    setUpdateStatus('downloading');
    await window.electronAPI.update?.download?.();
  };

  const handleInstall = () => {
    window.electronAPI.update?.install?.();
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

  const categoryItems: Array<{
    id: SettingsCategory;
    label: string;
    description: string;
    icon: ({ className }: { className?: string }) => JSX.Element;
  }> = [
    {
      id: 'general',
      label: t('settings.category.general'),
      description: t('settings.category.general.desc'),
      icon: GeneralIcon
    },
    {
      id: 'providers',
      label: t('settings.category.providers'),
      description: t('settings.category.providers.desc'),
      icon: ProvidersIcon
    },
    {
      id: 'channels',
      label: t('settings.category.channels'),
      description: t('settings.category.channels.desc'),
      icon: ChannelsIcon
    },
    {
      id: 'gateway',
      label: t('settings.category.gateway'),
      description: t('settings.category.gateway.desc'),
      icon: GatewayIcon
    },
    {
      id: 'advanced',
      label: t('settings.category.advanced'),
      description: t('settings.category.advanced.desc'),
      icon: AdvancedIcon
    }
  ];

  const activeCategoryMeta = categoryItems.find((item) => item.id === activeCategory) ?? categoryItems[0];

  return (
    <div className="h-full overflow-y-auto bg-background p-6">
      <div className="mx-auto max-w-6xl">
        <h1 className="mb-6 text-2xl font-bold">{t('settings.title')}</h1>

        <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-[220px_minmax(0,1fr)]">
          <aside className="lg:sticky lg:top-0">
            <div className="rounded-2xl border border-border bg-secondary/45 p-2">
              {categoryItems.map((item) => {
                const active = activeCategory === item.id;
                const Icon = item.icon;

                return (
                  <button
                    key={item.id}
                    onClick={() => setActiveCategory(item.id)}
                    className={`mb-1 flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left text-sm transition-all ${
                      active
                        ? 'bg-background text-foreground shadow-sm'
                        : 'text-foreground/70 hover:bg-background/70 hover:text-foreground'
                    }`}
                  >
                    <Icon className="h-4 w-4 flex-shrink-0" />
                    <span className="font-medium">{item.label}</span>
                  </button>
                );
              })}
            </div>
          </aside>

          <main className="min-w-0 space-y-6">
            <header>
              <h2 className="text-2xl font-semibold text-foreground">{activeCategoryMeta.label}</h2>
              <p className="mt-1 text-sm text-foreground/55">{activeCategoryMeta.description}</p>
            </header>

            {activeCategory === 'general' && (
              <div className="space-y-6">
                <section className="rounded-xl border border-border bg-card">
                  <div className="border-b border-border px-4 py-3">
                    <h3 className="text-base font-semibold">{t('settings.appearance')}</h3>
                  </div>
                  <div className="divide-y divide-border">
                    <div className="flex items-center justify-between gap-4 px-4 py-3">
                      <label className="text-sm font-medium">{t('settings.theme')}</label>
                      <CustomSelect
                        value={settings.theme}
                        onChange={(value) => handleChange('theme', value as Settings['theme'])}
                        options={[
                          { value: 'light', label: t('settings.theme.light') },
                          { value: 'dark', label: t('settings.theme.dark') },
                          { value: 'system', label: t('settings.theme.system') }
                        ]}
                        size="md"
                        className="min-w-[140px]"
                        triggerClassName="bg-secondary"
                      />
                    </div>

                    <div className="flex items-center justify-between gap-4 px-4 py-3">
                      <label className="text-sm font-medium">{t('settings.language')}</label>
                      <CustomSelect
                        value={settings.language}
                        onChange={(value) => handleChange('language', value as Settings['language'])}
                        options={[
                          { value: 'zh', label: t('settings.language.zh') },
                          { value: 'en', label: t('settings.language.en') }
                        ]}
                        size="md"
                        className="min-w-[140px]"
                        triggerClassName="bg-secondary"
                      />
                    </div>
                  </div>
                </section>

                <section className="rounded-xl border border-border bg-card">
                  <div className="border-b border-border px-4 py-3">
                    <h3 className="text-base font-semibold">{t('settings.system')}</h3>
                  </div>
                  <div className="divide-y divide-border">
                    <label className="flex cursor-pointer items-center justify-between gap-4 px-4 py-3">
                      <span className="text-sm font-medium">{t('settings.autoLaunch')}</span>
                      <input
                        type="checkbox"
                        checked={settings.autoLaunch}
                        onChange={(e) => handleChange('autoLaunch', e.target.checked)}
                        className="h-4 w-4 rounded border-border"
                      />
                    </label>

                    <label className="flex cursor-pointer items-center justify-between gap-4 px-4 py-3">
                      <span className="text-sm font-medium">{t('settings.minimizeToTray')}</span>
                      <input
                        type="checkbox"
                        checked={settings.minimizeToTray}
                        onChange={(e) => handleChange('minimizeToTray', e.target.checked)}
                        className="h-4 w-4 rounded border-border"
                      />
                    </label>

                    <label className="flex cursor-pointer items-center justify-between gap-4 px-4 py-3">
                      <span className="text-sm font-medium">{t('settings.notifications.enable')}</span>
                      <input
                        type="checkbox"
                        checked={settings.notificationsEnabled}
                        onChange={(e) => handleChange('notificationsEnabled', e.target.checked)}
                        className="h-4 w-4 rounded border-border"
                      />
                    </label>
                  </div>
                </section>

                <section className="rounded-xl border border-border bg-card">
                  <div className="border-b border-border px-4 py-3">
                    <h3 className="text-base font-semibold">{t('settings.shortcuts')}</h3>
                  </div>
                  <div className="divide-y divide-border">
                    <div className="flex items-center justify-between gap-4 px-4 py-3">
                      <label className="text-sm font-medium">{t('settings.shortcuts.toggle')}</label>
                      <input
                        type="text"
                        value={shortcuts.toggleWindow}
                        onChange={(e) => handleShortcutChange('toggleWindow', e.target.value)}
                        className="w-48 rounded-lg border border-border bg-secondary px-3 py-2 font-mono text-sm"
                        placeholder="Cmd+Shift+Space"
                      />
                    </div>
                    <div className="flex items-center justify-between gap-4 px-4 py-3">
                      <label className="text-sm font-medium">{t('settings.shortcuts.newChat')}</label>
                      <input
                        type="text"
                        value={shortcuts.newChat}
                        onChange={(e) => handleShortcutChange('newChat', e.target.value)}
                        className="w-48 rounded-lg border border-border bg-secondary px-3 py-2 font-mono text-sm"
                        placeholder="Cmd+N"
                      />
                    </div>
                  </div>
                  <div className="border-t border-border px-4 py-2">
                    <p className="text-xs text-foreground/50">
                      Use "CommandOrControl" for cross-platform shortcuts
                    </p>
                  </div>
                </section>

                <section className="rounded-xl border border-border bg-card">
                  <div className="border-b border-border px-4 py-3">
                    <h3 className="text-base font-semibold">{t('settings.dataManagement')}</h3>
                  </div>
                  <div className="p-4 space-y-4">
                    <div className="flex gap-4">
                      <button
                        onClick={handleExport}
                        className="px-4 py-2 bg-secondary rounded-lg text-sm font-medium hover:bg-border transition-colors"
                      >
                        {t('settings.export')}
                      </button>
                      <button
                        onClick={handleImport}
                        className="px-4 py-2 bg-secondary rounded-lg text-sm font-medium hover:bg-border transition-colors"
                      >
                        {t('settings.import')}
                      </button>
                    </div>
                    <p className="text-xs text-foreground/50">
                      导出包含配置和会话数据，导入将覆盖当前配置
                    </p>
                  </div>
                </section>

                <section className="rounded-xl border border-border bg-card">
                  <div className="border-b border-border px-4 py-3">
                    <h3 className="text-base font-semibold">{t('settings.updates')}</h3>
                  </div>
                  <div className="p-4 space-y-4">
                    {updateStatus === 'none' && (
                      <button
                        onClick={handleCheckUpdate}
                        className="px-4 py-2 bg-secondary rounded-lg text-sm font-medium hover:bg-border transition-colors"
                      >
                        {t('settings.checkUpdate')}
                      </button>
                    )}
                    {updateStatus === 'checking' && (
                      <p className="text-sm text-foreground/60">{t('settings.checking')}</p>
                    )}
                    {updateStatus === 'available' && updateInfo && (
                      <div className="space-y-3">
                        <p className="text-sm">
                          新版本可用: <span className="font-semibold">{updateInfo.version}</span>
                        </p>
                        <button
                          onClick={handleDownload}
                          className="px-4 py-2 bg-primary text-white rounded-lg text-sm font-medium hover:bg-primary/90 transition-colors"
                        >
                          {t('settings.downloadUpdate')}
                        </button>
                      </div>
                    )}
                    {updateStatus === 'downloading' && (
                      <p className="text-sm text-foreground/60">{t('settings.downloading')}</p>
                    )}
                    {updateStatus === 'downloaded' && (
                      <div className="space-y-3">
                        <p className="text-sm text-green-600">{t('settings.updateReady')}</p>
                        <button
                          onClick={handleInstall}
                          className="px-4 py-2 bg-primary text-white rounded-lg text-sm font-medium hover:bg-primary/90 transition-colors"
                        >
                          {t('settings.installAndRestart')}
                        </button>
                      </div>
                    )}
                  </div>
                </section>
              </div>
            )}

            {activeCategory === 'providers' && (
              <section className="space-y-4">
                <h3 className="text-lg font-semibold">{t('settings.providers')}</h3>

                {editingProvider ? (
                  <ProviderEditor
                    provider={editingProvider}
                    onSave={handleSaveProvider}
                    onTest={handleTestConnection}
                    onCancel={() => setEditingProvider(null)}
                  />
                ) : showAddProvider ? (
                  <div className="rounded-lg border border-border bg-card p-4">
                    <h4 className="mb-3 text-sm font-medium">{t('settings.providers.add') || 'Select Provider'}</h4>
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
                            <h4 className="font-medium">{provider.name}</h4>
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
            )}

            {activeCategory === 'channels' && (
              <div className="space-y-6">
                <section>
                  <h3 className="mb-4 text-lg font-semibold">{t('settings.email')}</h3>
                  <EmailConfig
                    config={channels.email}
                    onChange={(emailConfig) => handleChannelsChange({ ...channels, email: emailConfig })}
                    onTest={handleTestEmail}
                  />
                </section>

                <section>
                  <h3 className="mb-4 text-lg font-semibold">{t('settings.imbot')}</h3>
                  <IMBotConfig
                    config={channels}
                    onChange={handleChannelsChange}
                    onTestChannel={handleTestChannel}
                    getWhatsAppStatus={getWhatsAppStatus}
                  />
                </section>
              </div>
            )}

            {activeCategory === 'gateway' && (
              <section className="rounded-xl border border-border bg-card p-5">
                <h3 className="mb-4 text-lg font-semibold">{t('settings.gateway')}</h3>
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium">{t('settings.gateway.status')}</span>
                    <button
                      onClick={handleRestartGateway}
                      className="rounded-lg bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90"
                    >
                      {t('settings.gateway.restart')}
                    </button>
                  </div>

                  {gatewayConfig && (
                    <div className="mt-4 rounded-lg bg-secondary p-4">
                      <h4 className="mb-2 text-sm font-medium">{t('settings.gateway.currentModel')}</h4>
                      <code className="mb-4 block rounded bg-background px-2 py-1 text-xs">
                        {gatewayConfig.agents?.defaults?.model || t('settings.gateway.notConfigured')}
                      </code>

                      <h4 className="mb-2 text-sm font-medium">{t('settings.gateway.workspace')}</h4>
                      <code className="block rounded bg-background px-2 py-1 text-xs">
                        {gatewayConfig.agents?.defaults?.workspace || t('settings.gateway.notConfigured')}
                      </code>

                      <div className="mt-4">
                        <h4 className="mb-2 text-sm font-medium">{t('settings.gateway.executionMode')}</h4>
                        <div className="flex items-center gap-3">
                          <CustomSelect
                            value={executionModeInput}
                            onChange={(value) => setExecutionModeInput(value as 'safe' | 'ask' | 'auto')}
                            options={[
                              { value: 'safe', label: t('settings.gateway.executionMode.safe') },
                              { value: 'ask', label: t('settings.gateway.executionMode.ask') },
                              { value: 'auto', label: t('settings.gateway.executionMode.auto') }
                            ]}
                            size="md"
                            className="min-w-[180px]"
                            triggerClassName="bg-background"
                          />
                          <button
                            onClick={handleSaveExecutionMode}
                            disabled={savingExecutionMode}
                            className="rounded-lg bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            {savingExecutionMode ? t('settings.gateway.executionMode.saving') : t('common.save')}
                          </button>
                        </div>
                        <p className="mt-2 text-xs text-foreground/60">{t('settings.gateway.executionMode.hint')}</p>
                      </div>

                      <div className="mt-4">
                        <h4 className="mb-2 text-sm font-medium">{t('settings.gateway.maxToolIterations')}</h4>
                        <div className="flex items-center gap-3">
                          <input
                            type="number"
                            min={1}
                            step={1}
                            value={maxToolIterationsInput}
                            onChange={(e) => setMaxToolIterationsInput(e.target.value)}
                            className="w-36 rounded-lg border border-border bg-background px-3 py-2 text-sm"
                          />
                          <button
                            onClick={handleSaveMaxToolIterations}
                            disabled={savingMaxToolIterations}
                            className="rounded-lg bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            {savingMaxToolIterations ? t('settings.gateway.maxToolIterations.saving') : t('common.save')}
                          </button>
                        </div>
                        <p className="mt-2 text-xs text-foreground/60">{t('settings.gateway.maxToolIterations.hint')}</p>
                      </div>
                    </div>
                  )}
                </div>
              </section>
            )}

            {activeCategory === 'advanced' && <AdvancedSettings t={t} language={language} />}
          </main>
        </div>
      </div>
    </div>
  );
}

function GeneralIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8h18M3 16h18M8 3v18M16 3v18" />
    </svg>
  );
}

function ProvidersIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7h16M4 12h16M4 17h10" />
      <circle cx="18" cy="17" r="2" strokeWidth={2} />
    </svg>
  );
}

function ChannelsIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h8M8 14h5M4 6h16a1 1 0 011 1v10a1 1 0 01-1 1H4a1 1 0 01-1-1V7a1 1 0 011-1z" />
    </svg>
  );
}

function GatewayIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 3v4m0 10v4m9-9h-4M7 12H3m14.95-6.95l-2.83 2.83M8.88 15.12l-2.83 2.83m0-12.78l2.83 2.83m9.07 9.07l2.83 2.83" />
      <circle cx="12" cy="12" r="3" strokeWidth={2} />
    </svg>
  );
}

function AdvancedIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  );
}

// Workspace File Editor Component - Edit real USER.md and SOUL.md files
function AdvancedSettings({ t, language }: { t: (key: string) => string; language: string }) {
  const [activeTab, setActiveTab] = useState<'USER' | 'SOUL'>('USER');
  const [userContent, setUserContent] = useState<string>('');
  const [soulContent, setSoulContent] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  useEffect(() => {
    const loadFiles = async () => {
      setLoading(true);
      setError(null);
      try {
        const [userRes, soulRes] = await Promise.all([
          fetch('http://localhost:18890/api/workspace-file/USER.md'),
          fetch('http://localhost:18890/api/workspace-file/SOUL.md')
        ]);
        if (userRes.ok) {
          const data = await userRes.json();
          setUserContent(data.content || '');
        }
        if (soulRes.ok) {
          const data = await soulRes.json();
          setSoulContent(data.content || '');
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载失败');
      } finally {
        setLoading(false);
      }
    };
    void loadFiles();
  }, []);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSuccess(null);
    const filename = activeTab === 'USER' ? 'USER.md' : 'SOUL.md';
    const content = activeTab === 'USER' ? userContent : soulContent;
    try {
      const res = await fetch(`http://localhost:18890/api/workspace-file/${filename}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content })
      });
      if (!res.ok) throw new Error('保存失败');
      setSuccess(`${filename} 保存成功`);
      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center text-foreground/50">
        <div className="flex items-center gap-2">
          <div className="h-5 w-5 animate-spin rounded-full border-2 border-primary border-t-transparent" />
          {t('common.loading')}
        </div>
      </div>
    );
  }

  const currentContent = activeTab === 'USER' ? userContent : soulContent;
  const setCurrentContent = activeTab === 'USER' ? setUserContent : setSoulContent;

  return (
    <div className="flex h-[calc(100vh-200px)] flex-col">
      <div className="mb-4 flex items-center justify-between border-b border-border">
        <div className="flex gap-1">
          <button
            onClick={() => setActiveTab('USER')}
            className={`relative px-4 py-2 text-sm font-medium transition-colors ${
              activeTab === 'USER' ? 'text-primary' : 'text-foreground/60 hover:text-foreground'
            }`}
          >
            <span className="flex items-center gap-2">
              <FileIcon className="h-4 w-4" />
              USER.md
            </span>
            {activeTab === 'USER' && <span className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary" />}
          </button>
          <button
            onClick={() => setActiveTab('SOUL')}
            className={`relative px-4 py-2 text-sm font-medium transition-colors ${
              activeTab === 'SOUL' ? 'text-primary' : 'text-foreground/60 hover:text-foreground'
            }`}
          >
            <span className="flex items-center gap-2">
              <SparklesIcon className="h-4 w-4" />
              SOUL.md
            </span>
            {activeTab === 'SOUL' && <span className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary" />}
          </button>
        </div>
        <button
          onClick={handleSave}
          disabled={saving}
          className="mb-2 flex items-center gap-1.5 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-60"
        >
          {saving ? (
            <><div className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />{t('common.loading')}</>
          ) : (
            <><SaveIcon className="h-4 w-4" />{t('common.save')}</>
          )}
        </button>
      </div>

      {error && <div className="mb-3 rounded-lg border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-700">{error}</div>}
      {success && <div className="mb-3 rounded-lg border border-green-200 bg-green-50 px-4 py-2 text-sm text-green-700">{success}</div>}

      <div className="flex flex-1 flex-col overflow-hidden rounded-xl border border-border bg-card">
        <div className="flex items-center gap-2 border-b border-border bg-secondary/50 px-4 py-2 text-xs text-foreground/60">
          <FolderIcon className="h-3.5 w-3.5" />
          <span>~/.maxclaw/workspace/</span>
          <span className="font-medium text-foreground">{activeTab}.md</span>
          <span className="ml-2 rounded bg-primary/10 px-1.5 py-0.5 text-[10px] text-primary">
            {language === 'zh' ? '就地编辑' : 'Live Edit'}
          </span>
        </div>
        <div className="border-b border-border bg-secondary/30 px-4 py-2 text-xs text-foreground/60">
          {activeTab === 'USER'
            ? (language === 'zh' ? '定义你的个人信息、偏好设置和背景信息。AI 会在对话中参考这些信息来提供个性化回复。' : 'Define your personal information, preferences, and background. The AI references this for personalized responses.')
            : (language === 'zh' ? '定义 AI 助手的行为准则、个性特征和回复风格。这些指令直接影响 AI 的回复方式。' : "Define the AI assistant's behavior guidelines, personality traits, and response style. These directly affect how the AI responds.")}
        </div>
        <textarea
          value={currentContent}
          onChange={(e) => setCurrentContent(e.target.value)}
          className="flex-1 resize-none bg-background px-4 py-3 font-mono text-sm leading-relaxed text-foreground placeholder:text-foreground/30 focus:outline-none"
          placeholder={activeTab === 'USER' ? '# User\n\nYour information here...' : '# SOUL\n\nAI personality here...'}
          spellCheck={false}
        />
        <div className="flex items-center justify-between border-t border-border bg-secondary/30 px-4 py-2 text-xs text-foreground/50">
          <span>{currentContent.length} {language === 'zh' ? '字符' : 'chars'}</span>
          <span>Markdown {language === 'zh' ? '支持' : 'supported'}</span>
        </div>
      </div>
    </div>
  );
}

// Icon Components
function FileIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
    </svg>
  );
}

function SparklesIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z" />
    </svg>
  );
}

function FolderIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
    </svg>
  );
}

function SaveIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4" />
    </svg>
  );
}
