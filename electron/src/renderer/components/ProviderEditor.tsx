import React, { useState } from 'react';
import { ProviderConfig, ModelConfig } from '../types/providers';
import { useTranslation } from '../i18n';
import { CustomSelect } from './CustomSelect';

interface ProviderEditorProps {
  provider: ProviderConfig;
  onSave: (provider: ProviderConfig) => void;
  onTest: (provider: ProviderConfig) => Promise<{ success: boolean; latency?: number; error?: string }>;
  onFetchModels?: (provider: ProviderConfig) => Promise<{ success: boolean; models?: Array<{ id: string; name: string }>; error?: string }>;
  onCancel: () => void;
}

export function ProviderEditor({ provider, onSave, onTest, onFetchModels, onCancel }: ProviderEditorProps) {
  const { t } = useTranslation();
  const [config, setConfig] = useState<ProviderConfig>(provider);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [fetching, setFetching] = useState(false);
  const [fetchResult, setFetchResult] = useState<{ success: boolean; message: string } | null>(null);

  const handleTest = async () => {
    if (!config.apiKey) return;
    setTesting(true);
    setTestResult(null);
    const result = await onTest(config);
    setTestResult({
      success: result.success,
      message: result.success
        ? t('settings.providerEditor.testSuccess', { latency: result.latency || 0 })
        : t('settings.providerEditor.testFailed', { error: result.error || '' }),
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

  const handleModelChange = (index: number, field: keyof ModelConfig, value: string | boolean | number | undefined) => {
    const newModels = [...config.models];
    newModels[index] = { ...newModels[index], [field]: value };
    setConfig({ ...config, models: newModels });
  };

  return (
    <div className="rounded-lg border border-border bg-card p-6">
      <h3 className="mb-4 text-lg font-semibold">{t('settings.providerEditor.title', { name: config.name })}</h3>

      <div className="space-y-4">
        <div>
          <label className="mb-1 block text-sm font-medium">{t('settings.providerEditor.apiKey')}</label>
          <input
            type="password"
            value={config.apiKey}
            onChange={(e) => setConfig({ ...config, apiKey: e.target.value })}
            className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
            placeholder={t('settings.providerEditor.apiKeyPlaceholder')}
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium">{t('settings.providerEditor.baseUrl')}</label>
          <input
            type="url"
            value={config.baseURL || ''}
            onChange={(e) => setConfig({ ...config, baseURL: e.target.value })}
            className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
            placeholder={t('settings.providerEditor.baseUrlPlaceholder')}
          />
        </div>

        <div>
          <label className="mb-1 block text-sm font-medium">{t('settings.providerEditor.apiFormat')}</label>
          <CustomSelect
            value={config.apiFormat}
            onChange={(value) => setConfig({ ...config, apiFormat: value as 'openai' | 'anthropic' })}
            options={[
              { value: 'openai', label: t('settings.providerEditor.apiFormatOpenAI') },
              { value: 'anthropic', label: t('settings.providerEditor.apiFormatAnthropic') }
            ]}
            size="md"
          />
        </div>

        <div>
          <div className="mb-2 flex items-center justify-between">
            <label className="text-sm font-medium">{t('settings.providerEditor.models')}</label>
            <div className="flex items-center gap-3">
              {onFetchModels && (
                <button
                  onClick={async () => {
                    if (!config.apiKey) return;
                    setFetching(true);
                    setFetchResult(null);
                    const result = await onFetchModels(config);
                    if (result.success && result.models && result.models.length > 0) {
                      const newModels = result.models.map((m) => ({
                        id: m.id,
                        name: m.name || m.id,
                        enabled: true,
                        supportsImageInput: false,
                      }));
                      setConfig((prev) => ({ ...prev, models: newModels }));
                      setFetchResult({ success: true, message: t('settings.providerEditor.fetchSuccess', { count: result.models.length }) });
                    } else {
                      setFetchResult({ success: false, message: t('settings.providerEditor.fetchFailed', { error: result.error || t('settings.providerEditor.noModelsFound') }) });
                    }
                    setFetching(false);
                  }}
                  disabled={fetching || !config.apiKey}
                  className="text-sm text-primary hover:underline disabled:opacity-50"
                >
                  {fetching ? t('settings.providerEditor.fetching') : t('settings.providerEditor.fetchModels')}
                </button>
              )}
              <button
                onClick={handleAddModel}
                className="text-sm text-primary hover:underline"
              >
                + {t('settings.providerEditor.addModel')}
              </button>
            </div>
          </div>
          <div className="space-y-2">
            {config.models.map((model, index) => (
              <div key={index} className="flex items-center gap-2">
                <input
                  type="text"
                  value={model.id}
                  onChange={(e) => handleModelChange(index, 'id', e.target.value)}
                  placeholder={t('settings.providerEditor.modelIdPlaceholder')}
                  className="flex-1 rounded-lg border border-border bg-background px-3 py-1.5 text-sm"
                />
                <input
                  type="text"
                  value={model.name}
                  onChange={(e) => handleModelChange(index, 'name', e.target.value)}
                  placeholder={t('settings.providerEditor.modelNamePlaceholder')}
                  className="flex-1 rounded-lg border border-border bg-background px-3 py-1.5 text-sm"
                />
                <label className="flex items-center gap-1.5 text-sm">
                  <input
                    type="checkbox"
                    checked={model.enabled}
                    onChange={(e) => handleModelChange(index, 'enabled', e.target.checked)}
                    className="h-4 w-4"
                  />
                  {t('settings.providerEditor.enable')}
                </label>
                <label className="flex items-center gap-1.5 whitespace-nowrap text-sm">
                  <input
                    type="checkbox"
                    checked={model.supportsImageInput === true}
                    onChange={(e) => handleModelChange(index, 'supportsImageInput', e.target.checked)}
                    className="h-4 w-4"
                  />
                  {t('settings.providerEditor.multimodal')}
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

        {fetchResult && (
          <div
            className={`rounded-lg p-3 text-sm ${
              fetchResult.success
                ? 'bg-green-500/10 text-green-600 dark:text-green-400'
                : 'bg-red-500/10 text-red-600 dark:text-red-400'
            }`}
          >
            {fetchResult.message}
          </div>
        )}

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
            {testing ? t('settings.providerEditor.testing') : t('settings.providerEditor.testConnection')}
          </button>
          <button
            onClick={() => onSave(config)}
            disabled={!config.apiKey || config.models.length === 0}
            className="rounded-lg bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
          >
            {t('common.save')}
          </button>
          <button
            onClick={onCancel}
            className="rounded-lg border border-border px-4 py-2 text-sm hover:bg-secondary"
          >
            {t('common.cancel')}
          </button>
        </div>
      </div>
    </div>
  );
}
