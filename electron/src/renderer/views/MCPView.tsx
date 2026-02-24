import React, { useEffect, useState, useCallback } from 'react';
import { useTranslation } from '../i18n';

interface MCPServer {
  name: string;
  type: 'stdio' | 'sse';
  endpoint: string;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  headers?: Record<string, string>;
  enabled: boolean;
  description?: string;
}

type TestStatus = 'idle' | 'testing' | 'success' | 'error';

interface TestResult {
  status: TestStatus;
  message?: string;
  tools?: string[];
  count?: number;
}

export function MCPView() {
  const { t } = useTranslation();
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingServer, setEditingServer] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<string, TestResult>>({});

  const [formData, setFormData] = useState({
    name: '',
    type: 'stdio' as 'stdio' | 'sse',
    command: '',
    args: '',
    env: '',
    url: '',
    headers: '',
    description: ''
  });

  const fetchServers = useCallback(async () => {
    try {
      setLoading(true);
      const response = await fetch('http://localhost:18890/api/mcp');
      if (!response.ok) throw new Error('Failed to fetch MCP servers');
      const data = await response.json();
      setServers(data.servers || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load MCP servers');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchServers();
  }, [fetchServers]);

  const resetForm = () => {
    setFormData({
      name: '',
      type: 'stdio',
      command: '',
      args: '',
      env: '',
      url: '',
      headers: '',
      description: ''
    });
    setEditingServer(null);
  };

  const openAddModal = () => {
    resetForm();
    setIsModalOpen(true);
  };

  const openEditModal = (server: MCPServer) => {
    setEditingServer(server.name);
    setFormData({
      name: server.name,
      type: server.type,
      command: server.command || '',
      args: server.args?.join(' ') || '',
      env: server.env ? Object.entries(server.env).map(([k, v]) => `${k}=${v}`).join('\n') : '',
      url: server.url || '',
      headers: server.headers ? Object.entries(server.headers).map(([k, v]) => `${k}: ${v}`).join('\n') : '',
      description: server.description || ''
    });
    setIsModalOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const payload: Record<string, unknown> = {
        name: formData.name,
        type: formData.type,
        description: formData.description
      };

      if (formData.type === 'stdio') {
        payload.command = formData.command;
        payload.args = formData.args.split(' ').filter(Boolean);
        if (formData.env) {
          const envObj: Record<string, string> = {};
          formData.env.split('\n').forEach(line => {
            const [key, ...valueParts] = line.split('=');
            if (key && valueParts.length > 0) {
              envObj[key.trim()] = valueParts.join('=').trim();
            }
          });
          payload.env = envObj;
        }
      } else {
        payload.url = formData.url;
        if (formData.headers) {
          const headersObj: Record<string, string> = {};
          formData.headers.split('\n').forEach(line => {
            const [key, ...valueParts] = line.split(':');
            if (key && valueParts.length > 0) {
              headersObj[key.trim()] = valueParts.join(':').trim();
            }
          });
          payload.headers = headersObj;
        }
      }

      const url = editingServer
        ? `http://localhost:18890/api/mcp/${encodeURIComponent(editingServer)}`
        : 'http://localhost:18890/api/mcp';

      const response = await fetch(url, {
        method: editingServer ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || 'Failed to save MCP server');
      }

      setIsModalOpen(false);
      resetForm();
      void fetchServers();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.error'));
    }
  };

  const handleDelete = async (name: string) => {
    if (!confirm(t('mcp.confirmDelete', { name }))) return;

    try {
      const response = await fetch(`http://localhost:18890/api/mcp/${encodeURIComponent(name)}`, {
        method: 'DELETE'
      });

      if (!response.ok) throw new Error('Failed to delete MCP server');
      void fetchServers();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common.error'));
    }
  };

  const handleTest = async (name: string) => {
    setTestResults(prev => ({
      ...prev,
      [name]: { status: 'testing', message: t('mcp.testing') }
    }));

    try {
      const response = await fetch(`http://localhost:18890/api/mcp/${encodeURIComponent(name)}/test`, {
        method: 'POST'
      });

      const data = await response.json();

      if (data.ok) {
        setTestResults(prev => ({
          ...prev,
          [name]: {
            status: 'success',
            message: data.message,
            tools: data.tools,
            count: data.count
          }
        }));
      } else {
        setTestResults(prev => ({
          ...prev,
          [name]: {
            status: 'error',
            message: data.error || t('mcp.testFailed')
          }
        }));
      }
    } catch (err) {
      setTestResults(prev => ({
        ...prev,
        [name]: {
          status: 'error',
          message: err instanceof Error ? err.message : t('common.error')
        }
      }));
    }
  };

  const getServerIcon = (type: string) => {
    return type === 'sse' ? '🔌' : '⚙️';
  };

  const getEndpointDisplay = (server: MCPServer) => {
    if (server.type === 'sse') {
      return server.url || server.endpoint;
    }
    return server.command || server.endpoint;
  };

  return (
    <div className="h-full overflow-y-auto bg-background p-6">
      <div className="mx-auto max-w-5xl">
        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-foreground">{t('mcp.title')}</h1>
            <p className="mt-1 text-sm text-foreground/55">{t('mcp.subtitle')}</p>
          </div>
          <button
            onClick={openAddModal}
            className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
          >
            + {t('mcp.add')}
          </button>
        </div>

        {error && (
          <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
            {error}
          </div>
        )}

        {isModalOpen && (
          <div className="mb-6 rounded-xl border border-border bg-background p-5 shadow-sm">
            <h3 className="mb-4 text-base font-semibold">
              {editingServer ? t('mcp.edit.title') : t('mcp.add.title')}
            </h3>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="mb-1.5 block text-xs font-medium text-foreground/70">
                    {t('mcp.form.name')} *
                  </label>
                  <input
                    type="text"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    placeholder="my-mcp-server"
                    disabled={!!editingServer}
                    className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none disabled:bg-secondary/50"
                    required
                  />
                </div>
                <div>
                  <label className="mb-1.5 block text-xs font-medium text-foreground/70">
                    {t('mcp.form.type')}
                  </label>
                  <select
                    value={formData.type}
                    onChange={(e) => setFormData({ ...formData, type: e.target.value as 'stdio' | 'sse' })}
                    className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary/40 focus:outline-none"
                  >
                    <option value="stdio">STDIO (Command)</option>
                    <option value="sse">SSE (HTTP Stream)</option>
                  </select>
                </div>
              </div>

              {formData.type === 'stdio' ? (
                <>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-foreground/70">
                      {t('mcp.form.command')} *
                    </label>
                    <input
                      type="text"
                      value={formData.command}
                      onChange={(e) => setFormData({ ...formData, command: e.target.value })}
                      placeholder="npx -y @modelcontextprotocol/server-filesystem"
                      className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                      required={formData.type === 'stdio'}
                    />
                  </div>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-foreground/70">
                      {t('mcp.form.args')}
                    </label>
                    <input
                      type="text"
                      value={formData.args}
                      onChange={(e) => setFormData({ ...formData, args: e.target.value })}
                      placeholder="/path/to/directory"
                      className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                    />
                    <p className="mt-1 text-xs text-foreground/50">{t('mcp.form.argsHint')}</p>
                  </div>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-foreground/70">
                      {t('mcp.form.env')}
                    </label>
                    <textarea
                      value={formData.env}
                      onChange={(e) => setFormData({ ...formData, env: e.target.value })}
                      placeholder="KEY=value&#10;ANOTHER_KEY=another_value"
                      rows={3}
                      className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                    />
                    <p className="mt-1 text-xs text-foreground/50">{t('mcp.form.envHint')}</p>
                  </div>
                </>
              ) : (
                <>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-foreground/70">
                      {t('mcp.form.url')} *
                    </label>
                    <input
                      type="text"
                      value={formData.url}
                      onChange={(e) => setFormData({ ...formData, url: e.target.value })}
                      placeholder="http://localhost:3000/sse"
                      className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                      required={formData.type === 'sse'}
                    />
                  </div>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-foreground/70">
                      {t('mcp.form.headers')}
                    </label>
                    <textarea
                      value={formData.headers}
                      onChange={(e) => setFormData({ ...formData, headers: e.target.value })}
                      placeholder="Authorization: Bearer token&#10;X-Custom-Header: value"
                      rows={3}
                      className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                    />
                    <p className="mt-1 text-xs text-foreground/50">{t('mcp.form.headersHint')}</p>
                  </div>
                </>
              )}

              <div>
                <label className="mb-1.5 block text-xs font-medium text-foreground/70">
                  {t('mcp.form.description')}
                </label>
                <input
                  type="text"
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  placeholder={t('mcp.form.descriptionPlaceholder')}
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                />
              </div>

              <div className="flex justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => {
                    setIsModalOpen(false);
                    resetForm();
                  }}
                  className="rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-secondary"
                >
                  {t('common.cancel')}
                </button>
                <button
                  type="submit"
                  className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
                >
                  {editingServer ? t('common.save') : t('common.add')}
                </button>
              </div>
            </form>
          </div>
        )}

        {loading && servers.length === 0 ? (
          <div className="py-12 text-center text-foreground/50">{t('common.loading')}</div>
        ) : servers.length === 0 ? (
          <div className="py-12 text-center">
            <p className="text-foreground/50">{t('mcp.empty')}</p>
            <p className="mt-1 text-sm text-foreground/40">{t('mcp.empty.hint')}</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4">
            {servers.map((server) => (
              <div
                key={server.name}
                className="rounded-xl border border-border bg-background p-5 shadow-sm transition-all hover:border-border/80"
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <span className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-xl">
                      {getServerIcon(server.type)}
                    </span>
                    <div>
                      <h3 className="font-semibold text-foreground">{server.name}</h3>
                      <div className="flex items-center gap-2 mt-0.5">
                        <span className="inline-flex items-center rounded-full bg-secondary px-2 py-0.5 text-xs font-medium text-foreground/70">
                          {server.type.toUpperCase()}
                        </span>
                        <p className="text-xs text-foreground/50 font-mono truncate max-w-md">
                          {getEndpointDisplay(server)}
                        </p>
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleTest(server.name)}
                      disabled={testResults[server.name]?.status === 'testing'}
                      className={`rounded-lg p-2 transition-colors disabled:opacity-50 ${
                        testResults[server.name]?.status === 'success'
                          ? 'text-green-500 bg-green-50 dark:bg-green-900/20'
                          : testResults[server.name]?.status === 'error'
                          ? 'text-red-500 bg-red-50 dark:bg-red-900/20'
                          : 'text-foreground/60 hover:bg-secondary hover:text-foreground'
                      }`}
                      title={t('common.test')}
                    >
                      {testResults[server.name]?.status === 'testing' ? (
                        <SpinnerIcon className="w-4 h-4 animate-spin" />
                      ) : (
                        <TestIcon className="w-4 h-4" />
                      )}
                    </button>
                    <button
                      onClick={() => openEditModal(server)}
                      className="rounded-lg p-2 text-foreground/60 hover:bg-secondary hover:text-foreground transition-colors"
                      title={t('common.edit')}
                    >
                      <EditIcon className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(server.name)}
                      className="rounded-lg p-2 text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                      title={t('common.delete')}
                    >
                      <TrashIcon className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {server.description && (
                  <p className="mt-3 text-sm text-foreground/70">{server.description}</p>
                )}

                {(testResults[server.name]?.status === 'success' || testResults[server.name]?.status === 'error') && (
                  <div className={`mt-3 rounded-lg px-3 py-2 ${
                    testResults[server.name]?.status === 'success'
                      ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-900/50'
                      : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-900/50'
                  }`}>
                    <p className={`text-sm ${
                      testResults[server.name]?.status === 'success' ? 'text-green-700 dark:text-green-300' : 'text-red-700 dark:text-red-300'
                    }`}>
                      {testResults[server.name]?.message}
                    </p>
                    {testResults[server.name]?.tools && testResults[server.name].tools!.length > 0 && (
                      <div className="mt-2 flex flex-wrap gap-1">
                        {testResults[server.name].tools!.map(tool => (
                          <span key={tool} className="inline-flex items-center rounded bg-green-100 dark:bg-green-900/40 px-1.5 py-0.5 text-xs text-green-700 dark:text-green-300">
                            {tool}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {server.env && Object.keys(server.env).length > 0 && (
                  <div className="mt-3 rounded-lg bg-secondary/50 px-3 py-2">
                    <p className="text-xs font-medium text-foreground/60 mb-1">{t('mcp.envVars')}</p>
                    <div className="flex flex-wrap gap-2">
                      {Object.keys(server.env).map(key => (
                        <span key={key} className="inline-flex items-center rounded bg-secondary px-1.5 py-0.5 text-xs text-foreground/70">
                          {key}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {server.headers && Object.keys(server.headers).length > 0 && (
                  <div className="mt-3 rounded-lg bg-secondary/50 px-3 py-2">
                    <p className="text-xs font-medium text-foreground/60 mb-1">{t('mcp.form.headers')}</p>
                    <div className="flex flex-wrap gap-2">
                      {Object.keys(server.headers).map(key => (
                        <span key={key} className="inline-flex items-center rounded bg-secondary px-1.5 py-0.5 text-xs text-foreground/70">
                          {key}
                        </span>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function EditIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
    </svg>
  );
}

function TrashIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
    </svg>
  );
}

function TestIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  );
}

function SpinnerIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
    </svg>
  );
}
