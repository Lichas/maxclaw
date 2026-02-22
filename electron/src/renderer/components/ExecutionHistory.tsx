import React, { useEffect, useState, useCallback } from 'react';

interface ExecutionRecord {
  id: string;
  jobId: string;
  jobTitle: string;
  startedAt: string;
  endedAt?: string;
  status: 'running' | 'success' | 'failed';
  output: string;
  error?: string;
  durationMs: number;
}

interface ExecutionHistoryProps {
  jobId?: string;
  jobTitle?: string;
}

export function ExecutionHistory({ jobId, jobTitle }: ExecutionHistoryProps) {
  const [records, setRecords] = useState<ExecutionRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedRecord, setSelectedRecord] = useState<ExecutionRecord | null>(null);
  const [showDetail, setShowDetail] = useState(false);

  const fetchHistory = useCallback(async () => {
    try {
      setLoading(true);
      const url = new URL('http://localhost:18890/api/cron/history');
      if (jobId) {
        url.searchParams.set('jobId', jobId);
      }
      url.searchParams.set('limit', '50');

      const response = await fetch(url.toString());
      if (!response.ok) throw new Error('Failed to fetch history');
      const data = await response.json();
      setRecords(data.records || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [jobId]);

  useEffect(() => {
    void fetchHistory();
    const timer = setInterval(() => void fetchHistory(), 10000);
    return () => clearInterval(timer);
  }, [fetchHistory]);

  const fetchRecordDetail = async (id: string) => {
    try {
      const response = await fetch(`http://localhost:18890/api/cron/history/${id}`);
      if (!response.ok) throw new Error('Failed to fetch record detail');
      const data = await response.json();
      setSelectedRecord(data);
      setShowDetail(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载详情失败');
    }
  };

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    return `${(ms / 60000).toFixed(1)}m`;
  };

  const formatTime = (timeStr: string) => {
    const date = new Date(timeStr);
    return date.toLocaleString();
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success':
        return (
          <svg className="h-4 w-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
          </svg>
        );
      case 'failed':
        return (
          <svg className="h-4 w-4 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        );
      case 'running':
        return (
          <svg className="h-4 w-4 text-blue-500 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        );
      default:
        return null;
    }
  };

  const getStatusClass = (status: string) => {
    switch (status) {
      case 'success':
        return 'bg-green-50 text-green-700 border-green-200';
      case 'failed':
        return 'bg-red-50 text-red-700 border-red-200';
      case 'running':
        return 'bg-blue-50 text-blue-700 border-blue-200';
      default:
        return 'bg-gray-50 text-gray-700 border-gray-200';
    }
  };

  if (loading && records.length === 0) {
    return (
      <div className="py-8 text-center text-foreground/50">
        <div className="animate-spin h-6 w-6 border-2 border-primary border-t-transparent rounded-full mx-auto mb-2" />
        加载执行记录...
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      {records.length === 0 ? (
        <div className="py-8 text-center text-foreground/50">
          <HistoryIcon className="h-10 w-10 mx-auto mb-2 opacity-40" />
          <p>暂无执行记录</p>
        </div>
      ) : (
        <div className="space-y-2">
          {records.map((record) => (
            <div
              key={record.id}
              onClick={() => void fetchRecordDetail(record.id)}
              className="group cursor-pointer rounded-lg border border-border bg-background p-3 hover:bg-secondary/50 transition-colors"
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className={`shrink-0 rounded-full p-1.5 ${getStatusClass(record.status)}`}>
                    {getStatusIcon(record.status)}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-foreground">
                        {record.jobTitle || '未命名任务'}
                      </span>
                      <span className={`rounded-full px-2 py-0.5 text-xs border ${getStatusClass(record.status)}`}>
                        {record.status === 'success' ? '成功' : record.status === 'failed' ? '失败' : '运行中'}
                      </span>
                    </div>
                    <div className="mt-0.5 text-xs text-foreground/50">
                      {formatTime(record.startedAt)}
                      {record.durationMs > 0 && (
                        <span className="ml-2">耗时: {formatDuration(record.durationMs)}</span>
                      )}
                    </div>
                  </div>
                </div>
                <ChevronIcon className="h-4 w-4 text-foreground/30 group-hover:text-foreground/60" />
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Detail Modal */}
      {showDetail && selectedRecord && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="max-h-[80vh] w-full max-w-2xl overflow-hidden rounded-xl border border-border bg-background shadow-lg">
            <div className="flex items-center justify-between border-b border-border px-4 py-3">
              <div className="flex items-center gap-2">
                <div className={`rounded-full p-1 ${getStatusClass(selectedRecord.status)}`}>
                  {getStatusIcon(selectedRecord.status)}
                </div>
                <div>
                  <h3 className="font-medium text-foreground">{selectedRecord.jobTitle}</h3>
                  <p className="text-xs text-foreground/50">
                    {formatTime(selectedRecord.startedAt)}
                    {selectedRecord.endedAt && ` - ${formatTime(selectedRecord.endedAt)}`}
                  </p>
                </div>
              </div>
              <button
                onClick={() => setShowDetail(false)}
                className="rounded-lg p-1.5 text-foreground/50 hover:bg-secondary hover:text-foreground"
              >
                <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <div className="max-h-[60vh] overflow-y-auto p-4 space-y-4">
              {/* Status */}
              <div className="flex items-center gap-4 text-sm">
                <div>
                  <span className="text-foreground/50">状态:</span>
                  <span className={`ml-1.5 rounded-full px-2 py-0.5 text-xs border ${getStatusClass(selectedRecord.status)}`}>
                    {selectedRecord.status === 'success' ? '成功' : selectedRecord.status === 'failed' ? '失败' : '运行中'}
                  </span>
                </div>
                {selectedRecord.durationMs > 0 && (
                  <div>
                    <span className="text-foreground/50">耗时:</span>
                    <span className="ml-1.5 text-foreground">{formatDuration(selectedRecord.durationMs)}</span>
                  </div>
                )}
              </div>

              {/* Error */}
              {selectedRecord.error && (
                <div className="rounded-lg border border-red-200 bg-red-50 p-3">
                  <div className="text-xs font-medium text-red-700 mb-1">错误信息</div>
                  <pre className="text-sm text-red-600 whitespace-pre-wrap break-words">{selectedRecord.error}</pre>
                </div>
              )}

              {/* Output */}
              {selectedRecord.output && (
                <div>
                  <div className="text-xs font-medium text-foreground/50 mb-1.5">输出内容</div>
                  <div className="rounded-lg border border-border bg-secondary/50 p-3">
                    <pre className="text-sm text-foreground whitespace-pre-wrap break-words max-h-64 overflow-y-auto">
                      {selectedRecord.output}
                    </pre>
                  </div>
                </div>
              )}

              {/* No output message */}
              {!selectedRecord.output && !selectedRecord.error && (
                <div className="text-center py-8 text-foreground/50">
                  无输出内容
                </div>
              )}
            </div>

            <div className="border-t border-border px-4 py-3 flex justify-end">
              <button
                onClick={() => setShowDetail(false)}
                className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
              >
                关闭
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function HistoryIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );
}

function ChevronIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
    </svg>
  );
}
