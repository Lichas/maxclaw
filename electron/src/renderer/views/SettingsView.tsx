import React, { useState, useEffect } from 'react';

interface Settings {
  theme: 'light' | 'dark' | 'system';
  language: 'zh' | 'en';
  autoLaunch: boolean;
  minimizeToTray: boolean;
}

export function SettingsView() {
  const [settings, setSettings] = useState<Settings>({
    theme: 'system',
    language: 'zh',
    autoLaunch: false,
    minimizeToTray: true
  });
  const [gatewayConfig, setGatewayConfig] = useState<any>(null);

  useEffect(() => {
    // Load app settings
    window.electronAPI.config.get().then(setSettings);

    // Load Gateway config
    fetch('http://localhost:18890/api/config')
      .then(res => res.json())
      .then(setGatewayConfig)
      .catch(console.error);
  }, []);

  const handleChange = async <K extends keyof Settings>(key: K, value: Settings[K]) => {
    const updated = { ...settings, [key]: value };
    setSettings(updated);
    await window.electronAPI.config.set({ [key]: value });
  };

  const handleRestartGateway = async () => {
    await window.electronAPI.gateway.restart();
  };

  return (
    <div className="h-full overflow-y-auto p-6">
      <h1 className="text-2xl font-bold mb-6">Settings</h1>

      {/* Appearance */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-4">Appearance</h2>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <label className="text-sm font-medium">Theme</label>
            <select
              value={settings.theme}
              onChange={(e) => handleChange('theme', e.target.value as Settings['theme'])}
              className="bg-secondary rounded-lg px-3 py-2 text-sm border border-border"
            >
              <option value="light">Light</option>
              <option value="dark">Dark</option>
              <option value="system">System</option>
            </select>
          </div>

          <div className="flex items-center justify-between">
            <label className="text-sm font-medium">Language</label>
            <select
              value={settings.language}
              onChange={(e) => handleChange('language', e.target.value as Settings['language'])}
              className="bg-secondary rounded-lg px-3 py-2 text-sm border border-border"
            >
              <option value="zh">中文</option>
              <option value="en">English</option>
            </select>
          </div>
        </div>
      </section>

      {/* System */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-4">System</h2>
        <div className="space-y-4">
          <label className="flex items-center justify-between cursor-pointer">
            <span className="text-sm font-medium">Auto Launch</span>
            <input
              type="checkbox"
              checked={settings.autoLaunch}
              onChange={(e) => handleChange('autoLaunch', e.target.checked)}
              className="w-4 h-4"
            />
          </label>

          <label className="flex items-center justify-between cursor-pointer">
            <span className="text-sm font-medium">Minimize to Tray</span>
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
        <h2 className="text-lg font-semibold mb-4">Gateway</h2>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">Gateway Status</span>
            <button
              onClick={handleRestartGateway}
              className="px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm hover:bg-primary/90"
            >
              Restart Gateway
            </button>
          </div>

          {gatewayConfig && (
            <div className="bg-secondary rounded-lg p-4 mt-4">
              <h3 className="text-sm font-medium mb-2">Current Model</h3>
              <code className="text-xs bg-background px-2 py-1 rounded block mb-4">
                {gatewayConfig.agents?.defaults?.model || 'Not configured'}
              </code>

              <h3 className="text-sm font-medium mb-2">Workspace</h3>
              <code className="text-xs bg-background px-2 py-1 rounded block">
                {gatewayConfig.agents?.defaults?.workspace || 'Not configured'}
              </code>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
