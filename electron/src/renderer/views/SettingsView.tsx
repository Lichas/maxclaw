import React, { useState, useEffect } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { RootState, setTheme, setLanguage } from '../store';
import { useTranslation } from '../i18n';

interface Settings {
  theme: 'light' | 'dark' | 'system';
  language: 'zh' | 'en';
  autoLaunch: boolean;
  minimizeToTray: boolean;
}

export function SettingsView() {
  const dispatch = useDispatch();
  const { t } = useTranslation();
  const { theme: storeTheme, language: storeLanguage } = useSelector((state: RootState) => state.ui);

  const [settings, setSettings] = useState<Settings>({
    theme: 'system',
    language: 'zh',
    autoLaunch: false,
    minimizeToTray: true
  });
  const [gatewayConfig, setGatewayConfig] = useState<any>(null);

  useEffect(() => {
    // Load app settings
    window.electronAPI.config.get().then((config) => {
      setSettings({
        theme: config.theme || 'system',
        language: config.language || 'zh',
        autoLaunch: config.autoLaunch || false,
        minimizeToTray: config.minimizeToTray !== false
      });
    });

    // Load Gateway config
    fetch('http://localhost:18890/api/config')
      .then(res => res.json())
      .then(setGatewayConfig)
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
