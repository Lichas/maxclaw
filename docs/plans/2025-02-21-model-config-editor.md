# Model Configuration Editor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a comprehensive model provider management UI that allows users to add, edit, test, and configure AI providers (DeepSeek, OpenAI, Anthropic, etc.) with API keys, base URLs, and model lists.

**Architecture:** Frontend provides CRUD interface for provider configuration. Changes are saved to Gateway config via `/api/config` endpoint. Each provider has connection test capability that validates API key by making a test request. Gateway hot-reloads configuration after updates.

**Tech Stack:** React forms, Gateway config API, provider validation endpoints

---

## Prerequisites

- Gateway `/api/config` endpoint supports GET/PUT
- Provider config structure is documented in config schema

---

### Task 1: Create Provider Configuration Types

**Files:**
- Create: `electron/src/renderer/types/providers.ts`

**Step 1: Define provider types**

```typescript
export interface ProviderConfig {
  id: string;
  name: string;
  type: 'openai' | 'anthropic' | 'custom';
  apiKey: string;
  baseURL?: string;
  apiFormat: 'openai' | 'anthropic';
  models: ModelConfig[];
  enabled: boolean;
}

export interface ModelConfig {
  id: string;
  name: string;
  maxTokens?: number;
  enabled: boolean;
}

export const PRESET_PROVIDERS: Omit<ProviderConfig, 'id' | 'apiKey'>[] = [
  {
    name: 'DeepSeek',
    type: 'openai',
    baseURL: 'https://api.deepseek.com/v1',
    apiFormat: 'openai',
    models: [
      { id: 'deepseek-chat', name: 'DeepSeek Chat', enabled: true },
      { id: 'deepseek-coder', name: 'DeepSeek Coder', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'OpenAI',
    type: 'openai',
    baseURL: 'https://api.openai.com/v1',
    apiFormat: 'openai',
    models: [
      { id: 'gpt-4', name: 'GPT-4', enabled: true },
      { id: 'gpt-4-turbo', name: 'GPT-4 Turbo', enabled: true },
      { id: 'gpt-3.5-turbo', name: 'GPT-3.5 Turbo', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Anthropic',
    type: 'anthropic',
    baseURL: 'https://api.anthropic.com',
    apiFormat: 'anthropic',
    models: [
      { id: 'claude-opus-4', name: 'Claude Opus 4', enabled: true },
      { id: 'claude-sonnet-4', name: 'Claude Sonnet 4', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Moonshot',
    type: 'openai',
    baseURL: 'https://api.moonshot.cn/v1',
    apiFormat: 'openai',
    models: [
      { id: 'moonshot-v1-8k', name: 'Moonshot v1 8K', enabled: true },
      { id: 'moonshot-v1-32k', name: 'Moonshot v1 32K', enabled: true },
    ],
    enabled: false,
  },
];
```

**Step 2: Commit**

```bash
git add electron/src/renderer/types/providers.ts
git commit -m "feat(electron): add provider configuration types"
```

---

### Task 2: Create ProviderEditor Component

**Files:**
- Create: `electron/src/renderer/components/ProviderEditor.tsx`

**Step 1: Create provider editing form**

```typescript
import React, { useState } from 'react';
import { ProviderConfig, ModelConfig } from '../types/providers';

interface ProviderEditorProps {
  provider: ProviderConfig;
  onSave: (provider: ProviderConfig) => void;
  onTest: (provider: ProviderConfig) => Promise<{ success: boolean; latency?: number; error?: string }>;
  onCancel: () => void;
}

export function ProviderEditor({ provider, onSave, onTest, onCancel }: ProviderEditorProps) {
  const [config, setConfig] = useState<ProviderConfig>(provider);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string }> | null>(null);

  const handleTest = async () => {
    setTesting(true);
    setTestResult(null);
    const result = await onTest(config);
    setTestResult({
      success: result.success,
      message: result.success
        ? `Connection successful! Latency: ${result.latency}ms`
        : `Connection failed: ${result.error}`,
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

  return (
    <div className="bg-card rounded-lg border border-border p-6">
      <h3 className="text-lg font-semibold mb-4">{config.name} Configuration</h3>

      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium mb-1">API Key</label>
          <input
            type="password"
            value={config.apiKey}
            onChange={(e) => setConfig({ ...config, apiKey: e.target.value })}
            className="w-full rounded-lg border border-border bg-background px-3 py-2"
            placeholder="sk-..."
          />
        </div>

        <div>
          <label className="block text-sm font-medium mb-1">Base URL (optional)</label>
          <input
            type="url"
            value={config.baseURL || ''}
            onChange={(e) => setConfig({ ...config, baseURL: e.target.value })}
            className="w-full rounded-lg border border-border bg-background px-3 py-2"
            placeholder="https://api.example.com/v1"
          />
        </div>

        <div>
          <label className="block text-sm font-medium mb-1">API Format</label>
          <select
            value={config.apiFormat}
            onChange={(e) => setConfig({ ...config, apiFormat: e.target.value as 'openai' | 'anthropic' })}
            className="w-full rounded-lg border border-border bg-background px-3 py-2"
          >
            <option value="openai">OpenAI Compatible</option>
            <option value="anthropic">Anthropic</option>
          </select>
        </div>

        <div>
          <div className="flex items-center justify-between mb-2">
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
                  onChange={(e) => {
                    const newModels = [...config.models];
                    newModels[index] = { ...model, id: e.target.value };
                    setConfig({ ...config, models: newModels });
                  }}
                  placeholder="Model ID"
                  className="flex-1 rounded-lg border border-border bg-background px-3 py-1 text-sm"
                />
                <input
                  type="text"
                  value={model.name}
                  onChange={(e) => {
                    const newModels = [...config.models];
                    newModels[index] = { ...model, name: e.target.value };
                    setConfig({ ...config, models: newModels });
                  }}
                  placeholder="Display Name"
                  className="flex-1 rounded-lg border border-border bg-background px-3 py-1 text-sm"
                />
                <button
                  onClick={() => handleRemoveModel(index)}
                  className="text-red-500 hover:text-red-600 px-2"
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
                ? 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-300'
                : 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'
            }`}
          >
            {testResult.message}
          </div>
        )}

        <div className="flex gap-3 pt-4">
          <button
            onClick={handleTest}
            disabled={testing || !config.apiKey}
            className="px-4 py-2 border border-border rounded-lg hover:bg-secondary disabled:opacity-50"
          >
            {testing ? 'Testing...' : 'Test Connection'}
          </button>
          <button
            onClick={() => onSave(config)}
            className="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90"
          >
            Save
          </button>          <button
            onClick={onCancel}
            className="px-4 py-2 border border-border rounded-lg hover:bg-secondary"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add electron/src/renderer/components/ProviderEditor.tsx
git commit -m "feat(electron): add ProviderEditor component"
```

---

### Task 3: Create Providers Management Page

**Files:**
- Create: `electron/src/renderer/views/ProvidersView.tsx`

**Step 1: Create providers list and management**

```typescript
import React, { useState, useEffect } from 'react';
import { ProviderConfig, PRESET_PROVIDERS } from '../types/providers';
import { ProviderEditor } from '../components/ProviderEditor';

export function ProvidersView() {
  const [providers, setProviders] = useState<ProviderConfig[]>([]);
  const [editingProvider, setEditingProvider] = useState<ProviderConfig | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadProviders();
  }, []);

  const loadProviders = async () => {
    try {
      const response = await fetch('http://localhost:18890/api/config');
      const config = await response.json();
      setProviders(config.providers || []);
    } catch (error) {
      console.error('Failed to load providers:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleAddProvider = (preset: typeof PRESET_PROVIDERS[0]) => {
    const newProvider: ProviderConfig = {
      ...preset,
      id: crypto.randomUUID(),
      apiKey: '',
    };
    setEditingProvider(newProvider);
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

      // Update Gateway config
      const response = await fetch('http://localhost:18890/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ providers: newProviders }),
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
      alert('Failed to save provider');
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

  const handleToggleProvider = async (id: string) => {
    const provider = providers.find((p) => p.id === id);
    if (!provider) return;

    const updated = { ...provider, enabled: !provider.enabled };
    await handleSaveProvider(updated);
  };

  if (loading) {
    return <div className="p-6">Loading...</div>;
  }

  return (
    <div className="h-full overflow-y-auto bg-background p-6">
      <div className="mx-auto max-w-4xl">
        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold">Model Providers</h1>
            <p className="mt-1 text-sm text-foreground/60">Configure AI model providers and API keys</p>
          </div>
        </div>

        {editingProvider ? (
          <ProviderEditor
            provider={editingProvider}
            onSave={handleSaveProvider}
            onTest={handleTestConnection}
            onCancel={() => setEditingProvider(null)}
          />
        ) : (
          <>
            <div className="mb-8">
              <h2 className="text-lg font-semibold mb-3">Add Provider</h2>
              <div className="flex flex-wrap gap-3">
                {PRESET_PROVIDERS.map((preset) => (
                  <button
                    key={preset.name}
                    onClick={() => handleAddProvider(preset)}
                    className="px-4 py-2 border border-border rounded-lg hover:bg-secondary"
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
                  className="px-4 py-2 border border-dashed border-border rounded-lg hover:bg-secondary"
                >
                  + Custom Provider
                </button>
              </div>
            </div>

            <div>
              <h2 className="text-lg font-semibold mb-3">Configured Providers</h2>
              <div className="space-y-3">
                {providers.length === 0 ? (
                  <p className="text-foreground/50">No providers configured yet.</p>
                ) : (
                  providers.map((provider) => (
                    <div
                      key={provider.id}
                      className="flex items-center justify-between p-4 border border-border rounded-lg"
                    >
                      <div>
                        <h3 className="font-medium">{provider.name}</h3>
                        <p className="text-sm text-foreground/60">
                          {provider.models.filter((m) => m.enabled).length} models
                          {provider.baseURL && ` • ${provider.baseURL}`}
                        </p>
                      </div>
                      <div className="flex items-center gap-3">
                        <button
                          onClick={() => handleToggleProvider(provider.id)}
                          className={`relative inline-flex h-6 w-11 items-center rounded-full ${
                            provider.enabled ? 'bg-primary' : 'bg-secondary'
                          }`}
                        >
                          <span
                            className={`inline-block h-4 w-4 transform rounded-full bg-background transition-transform ${
                              provider.enabled ? 'translate-x-6' : 'translate-x-1'
                            }`}
                          />
                        </button>
                        <button
                          onClick={() => setEditingProvider(provider)}
                          className="px-3 py-1 text-sm border border-border rounded-lg hover:bg-secondary"
                        >
                          Edit
                        </button>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
```

**Step 2: Add route**

Update sidebar/menu to include Providers page.

**Step 3: Commit**

```bash
git add electron/src/renderer/views/ProvidersView.tsx
git commit -m "feat(electron): add Providers management view"
```

---

### Task 4: Backend Test Endpoint

**Files:**
- Modify: `internal/webui/server.go`

**Step 1: Add provider test endpoint**

```go
func (s *Server) handleTestProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var provider ProviderConfig
	if err := json.NewDecoder(r.Body).Decode(&provider); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Test the provider connection
	client := &http.Client{Timeout: 10 * time.Second}

	var testURL string
	var headers map[string]string

	switch provider.APIFormat {
	case "anthropic":
		testURL = provider.BaseURL + "/v1/models"
		headers = map[string]string{
			"x-api-key":         provider.APIKey,
			"anthropic-version": "2023-06-01",
		}
	default: // openai
		testURL = provider.BaseURL + "/models"
		headers = map[string]string{
			"Authorization": "Bearer " + provider.APIKey,
		}
	}

	req, _ := http.NewRequest("GET", testURL, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "API returned status "+resp.Status, resp.StatusCode)
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
}
```

**Step 2: Commit**

```bash
git add internal/webui/server.go
git commit -m "feat(api): add provider connection test endpoint"
```

---

### Task 5: Documentation

Update CHANGELOG.md, Electron_STATUS.md, Electron_PRD.md.

---

## Summary

After completing this plan:
- Full provider management UI with CRUD operations
- Connection testing with latency display
- Preset providers for quick setup
- Gateway hot-reload on config changes
