import React, { useEffect, useState, useCallback } from 'react';
import { CustomSelect } from '../components/CustomSelect';
import { CronBuilder } from '../components/CronBuilder';
import { ExecutionHistory } from '../components/ExecutionHistory';
import { ConfirmDialog } from '../components/ConfirmDialog';

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
}

interface JobFormData {
  title: string;
  prompt: string;
  scheduleType: 'once' | 'every' | 'cron';
  scheduleValue: string;
  workDir: string;
}

export function ScheduledTasksView() {
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
    workDir: ''
  });
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [jobToDelete, setJobToDelete] = useState<string | null>(null);

  const fetchJobs = useCallback(async () => {
    try {
      setLoading(true);
      const response = await fetch('http://localhost:18890/api/cron');
      if (!response.ok) throw new Error('Failed to fetch jobs');
      const data = await response.json();
      setJobs(data.jobs || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchJobs();
    const timer = setInterval(() => void fetchJobs(), 30000);
    return () => clearInterval(timer);
  }, [fetchJobs]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const response = await fetch('http://localhost:18890/api/cron', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: formData.title,
          prompt: formData.prompt,
          [formData.scheduleType === 'every' ? 'every' : formData.scheduleType === 'once' ? 'at' : 'cron']: formData.scheduleValue,
          workDir: formData.workDir || undefined
        })
      });
      if (!response.ok) throw new Error('Failed to create job');
      setShowForm(false);
      setFormData({
        title: '',
        prompt: '',
        scheduleType: 'cron',
        scheduleValue: '0 9 * * *',
        workDir: ''
      });
      void fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建失败');
    }
  };

  const toggleJob = async (id: string, enabled: boolean) => {
    try {
      const response = await fetch(`http://localhost:18890/api/cron/${id}/${enabled ? 'disable' : 'enable'}`, {
        method: 'POST'
      });
      if (!response.ok) throw new Error('Failed to toggle job');
      void fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败');
    }
  };

  const deleteJob = async (id: string) => {
    setJobToDelete(id);
    setDeleteDialogOpen(true);
  };

  const confirmDeleteJob = async () => {
    if (!jobToDelete) return;
    try {
      const response = await fetch(`http://localhost:18890/api/cron/${jobToDelete}`, {
        method: 'DELETE'
      });
      if (!response.ok) throw new Error('Failed to delete job');
      void fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败');
    }
    setDeleteDialogOpen(false);
    setJobToDelete(null);
  };

  const viewJobHistory = (id: string) => {
    setSelectedJobId(id);
    setShowHistory(true);
  };

  const getScheduleLabel = (job: CronJob) => {
    if (job.scheduleType === 'every') return `每 ${job.schedule}`;
    if (job.scheduleType === 'once') return `一次性: ${job.schedule}`;
    return `Cron: ${job.schedule}`;
  };

  return (
    <div className="h-full overflow-y-auto bg-background p-6">
      <div className="mx-auto max-w-4xl">
        <div className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-foreground">定时任务</h1>
            <p className="mt-1 text-sm text-foreground/55">创建和管理自动化 AI 任务</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => {
                setSelectedJobId(undefined);
                setShowHistory(true);
              }}
              className="rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-secondary"
            >
              执行历史
            </button>
            <button
              onClick={() => setShowForm(!showForm)}
              className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
            >
              {showForm ? '取消' : '+ 新建任务'}
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
            <h3 className="mb-4 text-base font-semibold">创建新任务</h3>
            <div className="space-y-4">
              <div>
                <label className="mb-1.5 block text-sm font-medium text-foreground">任务名称</label>
                <input
                  type="text"
                  value={formData.title}
                  onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                  placeholder="例如：每日工作总结"
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                  required
                />
              </div>

              <div>
                <label className="mb-1.5 block text-sm font-medium text-foreground">提示词</label>
                <textarea
                  value={formData.prompt}
                  onChange={(e) => setFormData({ ...formData, prompt: e.target.value })}
                  placeholder="输入给 AI 的指令..."
                  rows={4}
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                  required
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="mb-1.5 block text-sm font-medium text-foreground">调度类型</label>
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
                      { value: 'cron', label: 'Cron 表达式' },
                      { value: 'every', label: '周期执行' },
                      { value: 'once', label: '一次性' }
                    ]}
                    size="md"
                  />
                </div>

                {formData.scheduleType === 'cron' ? (
                  <div className="form-group">
                    <label className="mb-1.5 block text-sm font-medium text-foreground">Cron 表达式</label>
                    <CronBuilder
                      value={formData.scheduleValue}
                      onChange={(value) => setFormData({ ...formData, scheduleValue: value })}
                    />
                  </div>
                ) : (
                  <div>
                    <label className="mb-1.5 block text-sm font-medium text-foreground">
                      {formData.scheduleType === 'every' ? '间隔(毫秒)' : '执行时间'}
                    </label>
                    <input
                      type="text"
                      value={formData.scheduleValue}
                      onChange={(e) => setFormData({ ...formData, scheduleValue: e.target.value })}
                      placeholder={formData.scheduleType === 'every' ? '3600000' : '2026-02-21T09:00:00'}
                      className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                      required
                    />
                  </div>
                )}
              </div>

              <div>
                <label className="mb-1.5 block text-sm font-medium text-foreground">工作目录 (可选)</label>
                <input
                  type="text"
                  value={formData.workDir}
                  onChange={(e) => setFormData({ ...formData, workDir: e.target.value })}
                  placeholder="~/workspace"
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-foreground/40 focus:border-primary/40 focus:outline-none"
                />
              </div>

              <div className="flex justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowForm(false)}
                  className="rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-secondary"
                >
                  取消
                </button>
                <button
                  type="submit"
                  className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
                >
                  创建任务
                </button>
              </div>
            </div>
          </form>
        )}

        {loading && jobs.length === 0 ? (
          <div className="py-12 text-center text-foreground/50">加载中...</div>
        ) : jobs.length === 0 ? (
          <div className="py-12 text-center">
            <p className="text-foreground/50">暂无定时任务</p>
            <p className="mt-1 text-sm text-foreground/40">点击右上角创建新任务</p>
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
                        {job.enabled ? '启用' : '禁用'}
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
                      {job.lastRun && (
                        <span>上次执行: {new Date(job.lastRun).toLocaleString()}</span>
                      )}
                      {job.nextRun && job.enabled && (
                        <span>下次执行: {new Date(job.nextRun).toLocaleString()}</span>
                      )}
                    </div>
                  </div>
                  <div className="ml-4 flex shrink-0 items-center gap-2">
                    <button
                      onClick={() => void viewJobHistory(job.id)}
                      className="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-foreground hover:bg-secondary"
                      title="查看执行历史"
                    >
                      历史
                    </button>
                    <button
                      onClick={() => void toggleJob(job.id, job.enabled)}
                      className={`rounded-lg px-3 py-1.5 text-xs font-medium ${
                        job.enabled
                          ? 'border border-border text-foreground hover:bg-secondary'
                          : 'bg-primary text-primary-foreground hover:bg-primary/90'
                      }`}
                    >
                      {job.enabled ? '禁用' : '启用'}
                    </button>
                    <button
                      onClick={() => void deleteJob(job.id)}
                      className="rounded-lg border border-red-200 px-3 py-1.5 text-xs font-medium text-red-600 hover:bg-red-50"
                    >
                      删除
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
                  <h3 className="font-semibold text-foreground">执行历史</h3>
                  <p className="text-xs text-foreground/50">
                    {selectedJobId
                      ? jobs.find((j) => j.id === selectedJobId)?.title || '任务执行记录'
                      : '所有任务执行记录'}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  {selectedJobId && (
                    <button
                      onClick={() => setSelectedJobId(undefined)}
                      className="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-foreground hover:bg-secondary"
                    >
                      查看全部
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
        title="删除定时任务"
        message="确定要删除这个定时任务吗？此操作不可恢复。"
        confirmText="删除"
        cancelText="取消"
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
