import React, { useEffect, useState, useCallback } from 'react';
import { CustomSelect } from '../components/CustomSelect';
import { CronBuilder } from '../components/CronBuilder';
import { ExecutionHistory } from '../components/ExecutionHistory';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { useTranslation } from '../i18n';

interface CronJob {
  id: string;
  title: string;
  prompt: string;
  schedule: string;
  scheduleType: 'once' | 'every' | 'cron';
  workDir?: string;
  enabled: boolean;
  createdAt: string;
  lastRun?: string;
  nextRun?: string;
  executionMode?: 'safe' | 'ask' | 'auto';
  channels?: string[];
}

interface JobFormData {
  title: string;
  prompt: string;
  scheduleType: 'once' | 'every' | 'cron';
  scheduleValue: string;
  workDir: string;
  executionMode: 'safe' | 'ask' | 'auto';
  channels: string[];
}

export function ScheduledTasksView() {
  const { t } = useTranslation();
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [showHistory, setShowHistory] = useState(false);
  const [selectedJobId, setSelectedJobId] = useState<string | undefined>(undefined);
  const [formData, setFormData] = useState<JobFormData>({
    title: '',
    prompt: '',
    scheduleType: 'cron',
    scheduleValue: '0 9 * * *',
    workDir: '',
    executionMode: 'ask',
    channels: ['desktop']
  });
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [jobToDelete, setJobToDelete] = useState<string | null>(null);
  const [editingJob, setEditingJob] = useState<CronJob | null>(null);
  const channelOptions = [
    { value: 'desktop', label: t('scheduled.channel.name.desktop') },
    { value: 'telegram', label: t('scheduled.channel.name.telegram') },
    { value: 'discord', label: t('scheduled.channel.name.discord') },
    { value: 'slack', label: t('scheduled.channel.name.slack') },
    { value: 'email', label: t('scheduled.channel.name.email') },
    { value: 'websocket', label: t('scheduled.channel.name.websocket') }
  ];

  const fetchJobs = useCallback(async (showLoading = false) => {
    try {
      if (showLoading) {
        setLoading(true);
      }
      const response = await fetch('http://localhost:18890/api/cron');
      if (!response.ok) throw new Error(t('scheduled.error.load'));
      const data = await response.json();
      setJobs(data.jobs || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('scheduled.error.load'));
    } finally {
      if (showLoading) {
        setLoading(false);
      }
    }
  }, [t]);

  useEffect(() => {
    void fetchJobs(true);
    const timer = setInterval(() => void fetchJobs(false), 30000);
    return () => clearInterval(timer);
  }, [fetchJobs]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const payload = {
        title: formData.title,
        prompt: formData.prompt,
        [formData.scheduleType === 'every' ? 'every' : formData.scheduleType === 'once' ? 'at' : 'cron']: formData.scheduleValue,
        workDir: formData.workDir || undefined,
        executionMode: formData.executionMode,
        channels: formData.channels
      };

      const url = editingJob
        ? `http://localhost:18890/api/cron/${editingJob.id}`
        : 'http://localhost:18890/api/cron';
      const method = editingJob ? 'PUT' : 'POST';

      const response = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      if (!response.ok) throw new Error(editingJob ? t('scheduled.error.update') : t('scheduled.error.create'));
      setShowForm(false);
      setEditingJob(null);
      setFormData({
        title: '',
        prompt: '',
        scheduleType: 'cron',
        scheduleValue: '0 9 * * *',
        workDir: '',
        executionMode: 'ask',
        channels: ['desktop']
      });
      void fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : (editingJob ? t('scheduled.error.update') : t('scheduled.error.create')));
    }
  };

  const editJob = (job: CronJob) => {
    setEditingJob(job);
    setFormData({
      title: job.title,
      prompt: job.prompt,
      scheduleType: job.scheduleType,
      scheduleValue: job.schedule,
      workDir: job.workDir || '',
      executionMode: job.executionMode || 'ask',
      channels: job.channels || ['desktop']
    });
    setShowForm(true);
  };

  const cancelEdit = () => {
    setShowForm(false);
    setEditingJob(null);
    setFormData({
      title: '',
      prompt: '',
      scheduleType: 'cron',
      scheduleValue: '0 9 * * *',
      workDir: '',
      executionMode: 'ask',
      channels: ['desktop']
    });
  };

  const toggleJob = async (id: string, enabled: boolean) => {
    try {
      const response = await fetch(`http://localhost:18890/api/cron/${id}/${enabled ? 'disable' : 'enable'}`, {
        method: 'POST'
      });
      if (!response.ok) throw new Error(t('scheduled.error.toggle'));
      void fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('scheduled.error.toggle'));
    }
  };

  const deleteJob = async (id: string) => {
    setJobToDelete(id);
    setDeleteDialogOpen(true);
  };

  const runJobNow = async (id: string) => {
    try {
      const response = await fetch(`http://localhost:18890/api/cron/${id}/run`, {
        method: 'POST'
      });
      if (!response.ok) throw new Error(t('scheduled.error.run'));
      // Refresh job list to show execution status
      void fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('scheduled.error.run'));
    }
  };

  const confirmDeleteJob = async () => {
    if (!jobToDelete) return;
    try {
      const response = await fetch(`http://localhost:18890/api/cron/${jobToDelete}`, {
        method: 'DELETE'
      });
      if (!response.ok) throw new Error(t('scheduled.error.delete'));
      void fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('scheduled.error.delete'));
    }
    setDeleteDialogOpen(false);
    setJobToDelete(null);
  };

  const viewJobHistory = (id: string) => {
    setSelectedJobId(id);
    setShowHistory(true);
  };

  const getScheduleLabel = (job: CronJob) => {
    if (job.scheduleType === 'every') return t('scheduled.schedule.every').replace('{value}', job.schedule);
    if (job.scheduleType === 'once') return t('scheduled.schedule.once').replace('{value}', job.schedule);
    return t('scheduled.schedule.cron').replace('{value}', job.schedule);
  };

  const getExecutionModeLabel = (mode?: 'safe' | 'ask' | 'auto') => {
    if (mode === 'auto') return t('scheduled.executionMode.auto.short');
    if (mode === 'safe') return t('scheduled.executionMode.safe.short');
    return t('scheduled.executionMode.ask.short');
  };

  return (
    <div className="h-full overflow-y-auto bg-background p-6">
      <div className="mx-auto max-w-4xl">
        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-foreground">{t('scheduled.title')}</h1>
            <p className="mt-1 text-sm text-foreground/55">{t('scheduled.subtitle')}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => {
                setSelectedJobId(undefined);
                setShowHistory(true);
              }}
              className="rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-secondary"
            >
              {t('scheduled.executionHistory')}
            </button>
            <button
              onClick={() => {
                if (showForm && editingJob) {
                  cancelEdit();
                } else {
                  setShowForm(!showForm);
                }
              }}
              className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
            >
              {showForm ? (editingJob ? t('scheduled.cancelEdit') : t('scheduled.cancel')) : `+ ${t('scheduled.newTask')}`}
            </button>
          </div>
        </div>

        {error && (
          <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {error}
          </div>
        )}

        {showForm && (
          <form onSubmit={handleSubmit} className="mb-6 rounded-xl border border-border bg-background p-5 shadow-sm">
            <h3 className="mb-4 text-base font-semibold">{editingJob ? t('scheduled.edit') : t('scheduled.create')}</h3>
            <div className="space-y-4">
              <div>
                <label className="mb-1.5 block text-sm font-medium text-foreground">{t('scheduled.name')}</label>
                <input
                  type="text"
                  value={formData.title}
                  onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                  placeholder={t('scheduled.name.placeholder')}
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                  required
                />
              </div>

              <div>
                <label className="mb-1.5 block text-sm font-medium text-foreground">{t('scheduled.prompt')}</label>
                <textarea
                  value={formData.prompt}
                  onChange={(e) => setFormData({ ...formData, prompt: e.target.value })}
                  placeholder={t('scheduled.prompt.placeholder')}
                  rows={4}
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                  required
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="mb-1.5 block text-sm font-medium text-foreground">{t('scheduled.scheduleType')}</label>
                  <CustomSelect
                    value={formData.scheduleType}
                    onChange={(value) => {
                      const type = value as JobFormData['scheduleType'];
                      setFormData({
                        ...formData,
                        scheduleType: type,
                        scheduleValue: type === 'cron' ? '0 9 * * *' : type === 'every' ? '3600000' : ''
                      });
                    }}
                    options={[
                      { value: 'cron', label: t('scheduled.type.cron') },
                      { value: 'every', label: t('scheduled.type.every') },
                      { value: 'once', label: t('scheduled.type.once') }
                    ]}
                    size="md"
                  />
                </div>

                {formData.scheduleType === 'cron' ? (
                  <div className="form-group">
                    <label className="mb-1.5 block text-sm font-medium text-foreground">{t('scheduled.cronExpression')}</label>
                    <CronBuilder
                      value={formData.scheduleValue}
                      onChange={(value) => setFormData({ ...formData, scheduleValue: value })}
                    />
                  </div>
                ) : (
                  <div>
                    <label className="mb-1.5 block text-sm font-medium text-foreground">
                      {formData.scheduleType === 'every' ? t('scheduled.intervalMs') : t('scheduled.execTime')}
                    </label>
                    <input
                      type="text"
                      value={formData.scheduleValue}
                      onChange={(e) => setFormData({ ...formData, scheduleValue: e.target.value })}
                  placeholder={formData.scheduleType === 'every' ? t('scheduled.intervalMs.placeholder') : t('scheduled.execTime.placeholder')}
                      className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                      required
                    />
                  </div>
                )}
              </div>

              <div>
                <label className="mb-1.5 block text-sm font-medium text-foreground">{t('scheduled.workdir')}</label>
                <input
                  type="text"
                  value={formData.workDir}
                  onChange={(e) => setFormData({ ...formData, workDir: e.target.value })}
                  placeholder={t('scheduled.workdir.placeholder')}
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                />
              </div>

              <div>
                <label className="mb-1.5 block text-sm font-medium text-foreground">{t('scheduled.executionMode')}</label>
                <CustomSelect
                  value={formData.executionMode}
                  onChange={(value) => setFormData({ ...formData, executionMode: value as 'safe' | 'ask' | 'auto' })}
                  options={[
                    { value: 'safe', label: t('scheduled.executionMode.safe') },
                    { value: 'ask', label: t('scheduled.executionMode.ask') },
                    { value: 'auto', label: t('scheduled.executionMode.auto') }
                  ]}
                  size="md"
                />
                <p className="mt-1 text-xs text-foreground/50">
                  {formData.executionMode === 'safe' && t('scheduled.executionMode.safe.desc')}
                  {formData.executionMode === 'ask' && t('scheduled.executionMode.ask.desc')}
                  {formData.executionMode === 'auto' && t('scheduled.executionMode.auto.desc')}
                </p>
              </div>

              <div>
                <label className="mb-1.5 block text-sm font-medium text-foreground">{t('scheduled.channels')}</label>
                <div className="flex flex-wrap gap-2">
                  {channelOptions.map((option) => (
                    <button
                      key={option.value}
                      type="button"
                      aria-pressed={formData.channels.includes(option.value)}
                      onClick={() => {
                        const newChannels = formData.channels.includes(option.value)
                          ? (formData.channels.length === 1 ? formData.channels : formData.channels.filter(c => c !== option.value))
                          : [...formData.channels, option.value];
                        setFormData({ ...formData, channels: newChannels });
                      }}
                      className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-sm font-medium transition-all ${
                        formData.channels.includes(option.value)
                          ? 'border-primary bg-primary text-primary-foreground shadow-sm ring-2 ring-primary/20'
                          : 'border-border bg-background text-foreground/80 hover:bg-secondary'
                      }`}
                    >
                      {formData.channels.includes(option.value) && (
                        <svg className="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor" aria-hidden>
                          <path fillRule="evenodd" d="M16.704 5.29a1 1 0 010 1.42l-7.2 7.2a1 1 0 01-1.415 0l-3-3a1 1 0 111.414-1.415L8.8 11.79l6.49-6.5a1 1 0 011.414 0z" clipRule="evenodd" />
                        </svg>
                      )}
                      {option.label}
                    </button>
                  ))}
                </div>
                <p className="mt-1 text-xs text-foreground/50">
                  {formData.channels.length === 1 && formData.channels[0] === 'desktop'
                    ? t('scheduled.channel.desktop.desc')
                    : t('scheduled.channels.desc').replace('{channels}', formData.channels.map((channel) => {
                      const match = channelOptions.find((option) => option.value === channel);
                      return match ? match.label : channel;
                    }).join(', '))}
                </p>
                <p className="mt-1 text-xs text-foreground/40">{t('scheduled.channels.multiHint')}</p>
              </div>

              <div className="flex justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={cancelEdit}
                  className="rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-secondary"
                >
                  {t('scheduled.cancel')}
                </button>
                <button
                  type="submit"
                  className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
                >
                  {editingJob ? t('scheduled.saveChanges') || t('common.save') : t('scheduled.create')}
                </button>
              </div>
            </div>
          </form>
        )}

        {loading && jobs.length === 0 ? (
          <div className="py-12 text-center text-foreground/50">{t('scheduled.loading')}</div>
        ) : jobs.length === 0 ? (
          <div className="py-12 text-center">
            <p className="text-foreground/50">{t('scheduled.empty')}</p>
            <p className="mt-1 text-sm text-foreground/40">{t('scheduled.empty.hint')}</p>
          </div>
        ) : (
          <div className="space-y-3">
            {jobs.map((job) => (
              <div
                key={job.id}
                className={`rounded-xl border bg-background p-4 shadow-sm transition-opacity ${
                  job.enabled ? 'border-border' : 'border-border/50 opacity-60'
                }`}
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold text-foreground truncate">{job.title}</h3>
                      <span
                        className={`shrink-0 rounded-full px-2 py-0.5 text-xs ${
                          job.enabled
                            ? 'bg-green-100 text-green-700'
                            : 'bg-gray-100 text-gray-600'
                        }`}
                      >
                        {job.enabled ? t('scheduled.enabled') : t('scheduled.disabled')}
                      </span>
                    </div>
                    <p className="mt-1 text-sm text-foreground/70 line-clamp-2">{job.prompt}</p>
                    <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-foreground/50">
                      <span className="inline-flex items-center gap-1">
                        <ClockIcon className="h-3.5 w-3.5" />
                        {getScheduleLabel(job)}
                      </span>
                      {job.workDir && (
                        <span className="inline-flex items-center gap-1">
                          <FolderIcon className="h-3.5 w-3.5" />
                          {job.workDir}
                        </span>
                      )}
                      <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 ${
                        job.executionMode === 'auto'
                          ? 'bg-purple-100 text-purple-700'
                          : job.executionMode === 'safe'
                          ? 'bg-blue-100 text-blue-700'
                          : 'bg-yellow-100 text-yellow-700'
                      }`}>
                        {getExecutionModeLabel(job.executionMode)}
                      </span>
                      {job.channels && job.channels.length > 0 && (
                        <span className="inline-flex items-center gap-1">
                          <ChannelsIcon className="h-3.5 w-3.5" />
                          {job.channels.map((channel) => {
                            const match = channelOptions.find((option) => option.value === channel);
                            return match ? match.label : channel;
                          }).join(', ')}
                        </span>
                      )}
                      {job.lastRun && (
                        <span>{t('scheduled.lastRun')}: {new Date(job.lastRun).toLocaleString()}</span>
                      )}
                      {job.nextRun && job.enabled && (
                        <span>{t('scheduled.nextRun')}: {new Date(job.nextRun).toLocaleString()}</span>
                      )}
                    </div>
                  </div>
                  <div className="ml-4 flex shrink-0 items-center gap-2">
                    <button
                      onClick={() => void runJobNow(job.id)}
                      className="rounded-lg bg-green-500/10 px-3 py-1.5 text-xs font-medium text-green-600 hover:bg-green-500/20"
                      title={t('scheduled.runNow.title')}
                    >
                      ▶ {t('scheduled.runNow')}
                    </button>
                    <button
                      onClick={() => void viewJobHistory(job.id)}
                      className="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-foreground hover:bg-secondary"
                      title={t('scheduled.history.title')}
                    >
                      {t('scheduled.history')}
                    </button>
                    <button
                      onClick={() => editJob(job)}
                      className="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-foreground hover:bg-secondary"
                      title={t('scheduled.edit.title')}
                    >
                      {t('common.edit')}
                    </button>
                    <button
                      onClick={() => void toggleJob(job.id, job.enabled)}
                      className={`rounded-lg px-3 py-1.5 text-xs font-medium ${
                        job.enabled
                          ? 'border border-border text-foreground hover:bg-secondary'
                          : 'bg-primary text-primary-foreground hover:bg-primary/90'
                      }`}
                    >
                      {job.enabled ? t('scheduled.disable') || t('common.disable') : t('scheduled.enable') || t('common.enable')}
                    </button>
                    <button
                      onClick={() => void deleteJob(job.id)}
                      className="rounded-lg border border-red-200 px-3 py-1.5 text-xs font-medium text-red-600 hover:bg-red-50"
                      title={t('scheduled.delete.title')}
                    >
                      {t('common.delete')}
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Execution History Panel */}
        {showHistory && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
            <div className="max-h-[80vh] w-full max-w-3xl overflow-hidden rounded-xl border border-border bg-background shadow-lg">
              <div className="flex items-center justify-between border-b border-border px-4 py-3">
                <div>
                  <h3 className="font-semibold text-foreground">{t('scheduled.executionHistory')}</h3>
                  <p className="text-xs text-foreground/50">
                    {selectedJobId
                      ? jobs.find((j) => j.id === selectedJobId)?.title || t('scheduled.executionHistory')
                      : t('scheduled.executionHistory.all')}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  {selectedJobId && (
                    <button
                      onClick={() => setSelectedJobId(undefined)}
                      className="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-foreground hover:bg-secondary"
                    >
                      {t('scheduled.executionHistory.viewAll')}
                    </button>
                  )}
                  <button
                    onClick={() => setShowHistory(false)}
                    className="rounded-lg p-1.5 text-foreground/50 hover:bg-secondary hover:text-foreground"
                  >
                    <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </div>
              <div className="max-h-[60vh] overflow-y-auto p-4">
                <ExecutionHistory jobId={selectedJobId} />
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        isOpen={deleteDialogOpen}
        title={t('scheduled.delete.title')}
        message={t('scheduled.delete.confirm')}
        confirmText={t('common.delete')}
        cancelText={t('common.cancel')}
        onConfirm={confirmDeleteJob}
        onCancel={() => {
          setDeleteDialogOpen(false);
          setJobToDelete(null);
        }}
        variant="danger"
      />
    </div>
  );
}

function ClockIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );
}

function FolderIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
    </svg>
  );
}

function ChannelsIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
    </svg>
  );
}
