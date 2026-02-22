import React, { useState } from 'react';
import { ProviderConfig, ModelConfig } from '../types/providers';
import { useTranslation } from '../i18n';
import { CustomSelect } from './CustomSelect';

interface ProviderEditorProps {
  provider: ProviderConfig;
  onSave: (provider: ProviderConfig) => void;
  onTest: (provider: ProviderConfig) => Promise<{ success: boolean; latency?: number; error?: string }>;
  onCancel: () => void;
}

export function ProviderEditor({ provider, onSave, onTest, onCancel }: ProviderEditorProps) {
  const { t } = useTranslation();
  const [config, setConfig] = useState<ProviderConfig>(provider);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);

  const handleTest = async () => {
    if (!config.apiKey) return;
    setTesting(true);
    setTestResult(null);
    const result = await onTest(config);
    setTestResult({
      success: result.success,
      message: result.success
        ? `✓ Connection successful! Latency: ${result.latency}ms`
        : `✗ Connection failed: ${result.error}`,
    });
    setTesting(false);
  };

  const handleAddModel = () => {
    const newModel: ModelConfig = {
      id: '',
      name: '',
      enabled: true,
    };
    setConfig({ ...config, models: [...config.models, newModel] });
  };

  const handleRemoveModel = (index: number) => {
    setConfig({
      ...config,
      models: config.models.filter((_, i) => i !== index),
    });
  };

  const handleModelChange = (index: number, field: keyof ModelConfig, value: string | boolean) => {
    const newModels = [...config.models];
    newModels[index] = { ...newModels[index], [field]: value };
    setConfig({ ...config, models: newModels });
  };

  return (
    <div className="rounded-lg border border-border bg-card p-6">
      <h3 className="mb-4 text-lg font-semibold">{config.name} Configuration</h3>

      <div className="space-y-4">
        <div>
          <label className="mb-1 block text-sm font-medium">API Key</label>
          <input
            type="password"
            value={config.apiKey}
            onChange={(e) => setConfig({ ...config, apiKey: e.target.value })}
            className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
            placeholder="sk-..."
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium">Base URL (optional)</label>
          <input
            type="url"
            value={config.baseURL || ''}
            onChange={(e) => setConfig({ ...config, baseURL: e.target.value })}
            className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
            placeholder="https://api.example.com/v1"
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium">API Format</label>
          <CustomSelect
            value={config.apiFormat}
            onChange={(value) => setConfig({ ...config, apiFormat: value as 'openai' | 'anthropic' })}
            options={[
              { value: 'openai', label: 'OpenAI Compatible' },
              { value: 'anthropic', label: 'Anthropic' }
            ]}
            size="md"
          />
        </div>

        <div>
          <div className="mb-2 flex items-center justify-between">
            <label className="text-sm font-medium">Models</label>
            <button
              onClick={handleAddModel}
              className="text-sm text-primary hover:underline"
            >
              + Add Model
            </button>
          </div>
          <div className="space-y-2">
            {config.models.map((model, index) => (
              <div key={index} className="flex items-center gap-2">
                <input
                  type="text"
                  value={model.id}
                  onChange={(e) => handleModelChange(index, 'id', e.target.value)}
                  placeholder="Model ID"
                  className="flex-1 rounded-lg border border-border bg-background px-3 py-1.5 text-sm"
                />
                <input
                  type="text"
                  value={model.name}
                  onChange={(e) => handleModelChange(index, 'name', e.target.value)}
                  placeholder="Display Name"
                  className="flex-1 rounded-lg border border-border bg-background px-3 py-1.5 text-sm"
                />
                <label className="flex items-center gap-1.5 text-sm">
                  <input
                    type="checkbox"
                    checked={model.enabled}
                    onChange={(e) => handleModelChange(index, 'enabled', e.target.checked)}
                    className="h-4 w-4"
                  />
                  Enable
                </label>
                <button
                  onClick={() => handleRemoveModel(index)}
                  className="px-2 text-red-500 hover:text-red-600"
                >
                  ×
                </button>
              </div>
            ))}
          </div>
        </div>

        {testResult && (
          <div
            className={`rounded-lg p-3 text-sm ${
              testResult.success
                ? 'bg-green-500/10 text-green-600 dark:text-green-400'
                : 'bg-red-500/10 text-red-600 dark:text-red-400'
            }`}
          >
            {testResult.message}
          </div>
        )}

        <div className="flex gap-3 pt-4">
          <button
            onClick={handleTest}
            disabled={testing || !config.apiKey}
            className="rounded-lg border border-border px-4 py-2 text-sm hover:bg-secondary disabled:opacity-50"
          >
            {testing ? 'Testing...' : 'Test Connection'}
          </button>
          <button
            onClick={() => onSave(config)}
            disabled={!config.apiKey || config.models.length === 0}
            className="rounded-lg bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
          >
            Save
          </button>
          <button
            onClick={onCancel}
            className="rounded-lg border border-border px-4 py-2 text-sm hover:bg-secondary"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
