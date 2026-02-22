# 剩余功能完整实现计划（含托盘图标修复）

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 完成所有 HIGH 和 LOW 优先级剩余功能，并修复系统托盘图标不显示的问题。

**Architecture:**
1. **Cron Visual Builder**: 前端可视化组件，支持分钟/小时/天/周/月的可视化选择，实时生成 Cron 表达式
2. **Execution History**: 扩展 Cron Service，添加执行日志存储和 API，前端展示详细执行记录
3. **Global Shortcuts**: Electron globalShortcut API 注册系统级快捷键，支持快速呼出/隐藏窗口
4. **Auto-Update**: electron-updater 集成，自动检查 GitHub releases 并提示更新
5. **Data Import/Export**: ZIP 格式导出/导入配置和会话数据
6. **Tray Icon Fix**: 修复托盘图标路径，使用正确尺寸的图标文件

**Tech Stack:** React, TypeScript, Electron globalShortcut, electron-updater, JSZip, cron-parser

---

## 已有参考计划

- Global Shortcuts: 参考 `docs/plans/2025-02-21-global-shortcuts.md`
- Auto-Update: 参考 `docs/plans/2025-02-21-auto-update.md`
- Data Import/Export: 参考 `docs/plans/2025-02-21-data-import-export.md`

---

### Task 1: Fix System Tray Icon

**问题**: `tray.ts` 引用 `tray-icon.png` 但该文件不存在，只有 `icon.png`

**Files:**
- Modify: `electron/src/main/tray.ts`
- Create: `electron/assets/tray-icon.png` (从 icon.png 生成 16x16 和 32x32 版本)

**Step 1: Check current icon situation**

Run:
```bash
ls -la electron/assets/
file electron/assets/icon.png
```

Expected: Shows `icon.png` exists but no `tray-icon.png`

**Step 2: Create tray icon**

Run:
```bash
cd electron/assets
# Create 16x16 tray icon for macOS/Windows
sips -z 16 16 icon.png --out tray-icon.png || cp icon.png tray-icon.png
# Also create 32x32 for HiDPI
sips -z 32 32 icon.png --out tray-icon@2x.png || echo "sips not available"
```

If `sips` not available (non-macOS), manually copy and resize:
```bash
cp icon.png tray-icon.png
```

**Step 3: Update tray.ts to handle icon correctly**

Modify `electron/src/main/tray.ts`:

```typescript
import { Tray, Menu, BrowserWindow, nativeImage, app } from 'electron';
import path from 'path';
import log from 'electron-log';
import fs from 'fs';

let tray: Tray | null = null;

export function initializeTray(mainWindow: BrowserWindow): void {
  try {
    const trayIcon = createTrayIcon();
    if (!trayIcon) {
      log.warn('Could not create tray icon, skipping tray initialization');
      return;
    }

    tray = new Tray(trayIcon);
    tray.setToolTip('Maxclaw AI Assistant');

    updateTrayMenu(mainWindow);

    // Handle tray click (show window)
    tray.on('click', () => {
      showWindow(mainWindow);
    });

    // Handle double-click
    tray.on('double-click', () => {
      showWindow(mainWindow);
    });

    log.info('System tray initialized successfully');
  } catch (error) {
    log.error('Failed to initialize tray:', error);
  }
}

function createTrayIcon(): nativeImage | null {
  // Try different icon paths in order of preference
  const iconPaths = [
    path.join(__dirname, '../../assets/tray-icon.png'),
    path.join(__dirname, '../../assets/icon.png'),
    path.join(process.resourcesPath, 'assets/tray-icon.png'),
    path.join(process.resourcesPath, 'assets/icon.png'),
  ];

  for (const iconPath of iconPaths) {
    try {
      if (fs.existsSync(iconPath)) {
        const icon = nativeImage.createFromPath(iconPath);

        // Resize for tray (16x16 for standard, 32x32 for HiDPI)
        const size = process.platform === 'darwin' ? 16 : 16;
        let trayIcon = icon.resize({ width: size, height: size });

        // macOS: Use template image for proper dark mode support
        if (process.platform === 'darwin') {
          trayIcon.setTemplateImage(true);
        }

        log.info(`Created tray icon from: ${iconPath}`);
        return trayIcon;
      }
    } catch (err) {
      log.debug(`Failed to create icon from ${iconPath}:`, err);
    }
  }

  log.error('No valid tray icon found in any location');
  return null;
}

// ... rest of file unchanged
```

**Step 4: Verify tray is initialized in main process**

Check `electron/src/main/index.ts`:

```typescript
import { initializeTray, destroyTray } from './tray';

// In app ready handler, after window creation:
initializeTray(mainWindow);

// On app quit:
app.on('quit', () => {
  destroyTray();
});
```

**Step 5: Test tray icon**

Run:
```bash
cd electron && npm run dev
```

Expected: Tray icon should appear in system tray/taskbar.

**Step 6: Commit**

```bash
git add electron/src/main/tray.ts electron/assets/tray-icon.png
git commit -m "fix(electron): fix system tray icon not showing"
```

---

### Task 2: Cron Expression Visual Builder

**Files:**
- Create: `electron/src/renderer/components/CronBuilder.tsx`
- Create: `electron/src/renderer/components/CronBuilder.css`
- Modify: `electron/src/renderer/views/ScheduledTasksView.tsx`

**Step 1: Create CronBuilder component**

Create `electron/src/renderer/components/CronBuilder.tsx`:

```typescript
import React, { useState, useEffect } from 'react';
import './CronBuilder.css';

type CronPreset = 'custom' | 'minutely' | 'hourly' | 'daily' | 'weekly' | 'monthly';

interface CronBuilderProps {
  value: string;
  onChange: (value: string) => void;
}

interface CronParts {
  minute: string;
  hour: string;
  dayOfMonth: string;
  month: string;
  dayOfWeek: string;
}

const PRESETS: Record<CronPreset, string> = {
  custom: '* * * * *',
  minutely: '* * * * *',
  hourly: '0 * * * *',
  daily: '0 9 * * *',
  weekly: '0 9 * * 1',
  monthly: '0 9 1 * *',
};

const WEEKDAYS = [
  { value: '1', label: '周一' },
  { value: '2', label: '周二' },
  { value: '3', label: '周三' },
  { value: '4', label: '周四' },
  { value: '5', label: '周五' },
  { value: '6', label: '周六' },
  { value: '0', label: '周日' },
];

export function CronBuilder({ value, onChange }: CronBuilderProps) {
  const [preset, setPreset] = useState<CronPreset>('custom');
  const [parts, setParts] = useState<CronParts>({
    minute: '0',
    hour: '9',
    dayOfMonth: '*',
    month: '*',
    dayOfWeek: '*',
  });

  // Parse initial value
  useEffect(() => {
    const parsed = parseCron(value);
    if (parsed) {
      setParts(parsed);
      detectPreset(parsed);
    }
  }, []);

  const parseCron = (cron: string): CronParts | null => {
    const parts = cron.split(' ');
    if (parts.length !== 5) return null;
    return {
      minute: parts[0],
      hour: parts[1],
      dayOfMonth: parts[2],
      month: parts[3],
      dayOfWeek: parts[4],
    };
  };

  const detectPreset = (parts: CronParts) => {
    const cron = `${parts.minute} ${parts.hour} ${parts.dayOfMonth} ${parts.month} ${parts.dayOfWeek}`;
    for (const [presetName, presetCron] of Object.entries(PRESETS)) {
      if (presetCron === cron) {
        setPreset(presetName as CronPreset);
        return;
      }
    }
    setPreset('custom');
  };

  const updatePart = (key: keyof CronParts, value: string) => {
    const newParts = { ...parts, [key]: value };
    setParts(newParts);
    const cron = `${newParts.minute} ${newParts.hour} ${newParts.dayOfMonth} ${newParts.month} ${newParts.dayOfWeek}`;
    onChange(cron);
    detectPreset(newParts);
  };

  const handlePresetChange = (newPreset: CronPreset) => {
    setPreset(newPreset);
    if (newPreset !== 'custom') {
      const cron = PRESETS[newPreset];
      onChange(cron);
      const parsed = parseCron(cron);
      if (parsed) setParts(parsed);
    }
  };

  return (
    <div className="cron-builder">
      <div className="cron-presets">
        <label className="cron-label">执行频率</label>
        <div className="preset-buttons">
          {[
            { key: 'minutely', label: '每分钟' },
            { key: 'hourly', label: '每小时' },
            { key: 'daily', label: '每天' },
            { key: 'weekly', label: '每周' },
            { key: 'monthly', label: '每月' },
            { key: 'custom', label: '自定义' },
          ].map(({ key, label }) => (
            <button
              key={key}
              type="button"
              className={`preset-btn ${preset === key ? 'active' : ''}`}
              onClick={() => handlePresetChange(key as CronPreset)}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {preset === 'custom' && (
        <div className="cron-custom">
          <div className="cron-row">
            <div className="cron-field">
              <label>分钟 (0-59)</label>
              <input
                type="text"
                value={parts.minute}
                onChange={(e) => updatePart('minute', e.target.value)}
                placeholder="0"
              />
            </div>
            <div className="cron-field">
              <label>小时 (0-23)</label>
              <input
                type="text"
                value={parts.hour}
                onChange={(e) => updatePart('hour', e.target.value)}
                placeholder="9"
              />
            </div>
          </div>

          <div className="cron-row">
            <div className="cron-field">
              <label>日期 (1-31)</label>
              <input
                type="text"
                value={parts.dayOfMonth}
                onChange={(e) => updatePart('dayOfMonth', e.target.value)}
                placeholder="*"
              />
            </div>
            <div className="cron-field">
              <label>月份 (1-12)</label>
              <input
                type="text"
                value={parts.month}
                onChange={(e) => updatePart('month', e.target.value)}
                placeholder="*"
              />
            </div>
          </div>

          <div className="cron-weekday">
            <label>星期</label>
            <div className="weekday-buttons">
              {WEEKDAYS.map(({ value: dayValue, label }) => (
                <button
                  key={dayValue}
                  type="button"
                  className={`weekday-btn ${parts.dayOfWeek === dayValue ? 'active' : ''}`}
                  onClick={() => updatePart('dayOfWeek', parts.dayOfWeek === dayValue ? '*' : dayValue)}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

      <div className="cron-preview">
        <label>Cron 表达式</label>
        <code>{`${parts.minute} ${parts.hour} ${parts.dayOfMonth} ${parts.month} ${parts.dayOfWeek}`}</code>
      </div>

      <div className="cron-description">
        {getCronDescription(parts)}
      </div>
    </div>
  );
}

function getCronDescription(parts: CronParts): string {
  if (parts.minute === '*' && parts.hour === '*' && parts.dayOfMonth === '*' && parts.month === '*' && parts.dayOfWeek === '*') {
    return '每分钟执行一次';
  }
  if (parts.minute === '0' && parts.hour !== '*' && parts.dayOfMonth === '*' && parts.month === '*' && parts.dayOfWeek === '*') {
    return `每小时的 ${parts.minute} 分执行`;
  }
  if (parts.minute === '0' && parts.hour !== '*' && parts.dayOfMonth === '*' && parts.month === '*' && parts.dayOfWeek === '*') {
    return `每天 ${parts.hour}:00 执行`;
  }
  if (parts.minute === '0' && parts.hour !== '*' && parts.dayOfMonth === '*' && parts.month === '*' && parts.dayOfWeek !== '*') {
    const weekday = WEEKDAYS.find(w => w.value === parts.dayOfWeek)?.label || parts.dayOfWeek;
    return `每周${weekday.replace('周', '')} ${parts.hour}:00 执行`;
  }
  if (parts.minute === '0' && parts.hour !== '*' && parts.dayOfMonth === '1' && parts.month === '*' && parts.dayOfWeek === '*') {
    return `每月 1 日 ${parts.hour}:00 执行`;
  }
  return `在 ${parts.minute} 分 ${parts.hour} 时执行`;
}
```

**Step 2: Create CSS for CronBuilder**

Create `electron/src/renderer/components/CronBuilder.css`:

```css
.cron-builder {
  background: var(--secondary);
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 16px;
}

.cron-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--foreground);
  margin-bottom: 8px;
  display: block;
}

.preset-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}

.preset-btn {
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  color: var(--foreground);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}

.preset-btn:hover {
  background: var(--secondary);
}

.preset-btn.active {
  background: var(--primary);
  color: white;
  border-color: var(--primary);
}

.cron-custom {
  margin-bottom: 16px;
}

.cron-row {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
}

.cron-field {
  flex: 1;
}

.cron-field label {
  display: block;
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 4px;
}

.cron-field input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  color: var(--foreground);
  font-size: 14px;
  font-family: monospace;
}

.cron-weekday {
  margin-top: 12px;
}

.cron-weekday label {
  display: block;
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 8px;
}

.weekday-buttons {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.weekday-btn {
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--background);
  color: var(--foreground);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.weekday-btn:hover {
  background: var(--secondary);
}

.weekday-btn.active {
  background: var(--primary);
  color: white;
  border-color: var(--primary);
}

.cron-preview {
  background: var(--background);
  border-radius: 6px;
  padding: 12px;
  margin-top: 12px;
}

.cron-preview label {
  display: block;
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 4px;
}

.cron-preview code {
  font-family: monospace;
  font-size: 14px;
  color: var(--primary);
}

.cron-description {
  margin-top: 8px;
  font-size: 13px;
  color: var(--muted);
  font-style: italic;
}
```

**Step 3: Integrate CronBuilder into ScheduledTasksView**

Modify `electron/src/renderer/views/ScheduledTasksView.tsx`:

Add import:
```typescript
import { CronBuilder } from '../components/CronBuilder';
```

In the form section where `scheduleType === 'cron'`, replace the text input with:

```typescript
{formData.scheduleType === 'cron' && (
  <div className="form-group">
    <label>Cron 表达式</label>
    <CronBuilder
      value={formData.scheduleValue}
      onChange={(value) => setFormData({ ...formData, scheduleValue: value })}
    />
  </div>
)}
```

**Step 4: Test CronBuilder**

Run:
```bash
cd electron && npm run build
npm run dev
```

Test scenarios:
1. Click "新建任务" - verify CronBuilder appears for cron schedule type
2. Click different preset buttons - verify expression updates
3. Switch to custom - verify individual fields work
4. Select weekday buttons - verify expression updates

**Step 5: Commit**

```bash
git add electron/src/renderer/components/CronBuilder.* electron/src/renderer/views/ScheduledTasksView.tsx
git commit -m "feat(electron): add visual cron expression builder"
```

---

### Task 3: Execution History Logs

**Files:**
- Modify: `internal/cron/types.go`
- Modify: `internal/cron/service.go`
- Create: `internal/cron/history.go`
- Modify: `internal/webui/server.go` (add history endpoints)
- Create: `electron/src/renderer/components/ExecutionHistory.tsx`
- Modify: `electron/src/renderer/views/ScheduledTasksView.tsx`

**Step 1: Add execution history types**

Modify `internal/cron/types.go`:

```go
package cron

import "time"

// ExecutionRecord 任务执行记录
type ExecutionRecord struct {
	ID        string    `json:"id"`
	JobID     string    `json:"jobId"`
	JobTitle  string    `json:"jobTitle"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
	Status    string    `json:"status"` // running, success, failed
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
	Duration  int64     `json:"durationMs"` // milliseconds
}
```

**Step 2: Create history store**

Create `internal/cron/history.go`:

```go
package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HistoryStore 执行历史存储
type HistoryStore struct {
	records   []ExecutionRecord
	mu        sync.RWMutex
	storePath string
	maxSize   int
}

// NewHistoryStore 创建历史存储
func NewHistoryStore(storePath string) *HistoryStore {
	h := &HistoryStore{
		storePath: storePath,
		maxSize:   1000, // Keep last 1000 records
		records:   make([]ExecutionRecord, 0),
	}
	h.load()
	return h
}

// AddRecord 添加执行记录
func (h *HistoryStore) AddRecord(record ExecutionRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Assign ID if not set
	if record.ID == "" {
		record.ID = fmt.Sprintf("exec_%d", time.Now().UnixNano())
	}

	h.records = append(h.records, record)

	// Trim old records
	if len(h.records) > h.maxSize {
		h.records = h.records[len(h.records)-h.maxSize:]
	}

	h.save()
}

// UpdateRecord 更新执行记录
func (h *HistoryStore) UpdateRecord(id string, updates func(*ExecutionRecord)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := range h.records {
		if h.records[i].ID == id {
			updates(&h.records[i])
			h.save()
			return
		}
	}
}

// GetRecords 获取执行记录
func (h *HistoryStore) GetRecords(jobID string, limit int) []ExecutionRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []ExecutionRecord
	for i := len(h.records) - 1; i >= 0; i-- {
		if jobID == "" || h.records[i].JobID == jobID {
			result = append(result, h.records[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetRecord 获取单条记录
func (h *HistoryStore) GetRecord(id string) (*ExecutionRecord, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, r := range h.records {
		if r.ID == id {
			return &r, true
		}
	}
	return nil, false
}

func (h *HistoryStore) load() {
	data, err := os.ReadFile(h.storePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &h.records)
}

func (h *HistoryStore) save() error {
	data, err := json.MarshalIndent(h.records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.storePath, data, 0644)
}
```

**Step 3: Integrate history into service**

Modify `internal/cron/service.go`:

Add field to Service struct:
```go	type Service struct {
		// ... existing fields ...
		historyStore *HistoryStore
	}
```

In `NewService`, add:
```go
	historyPath := filepath.Join(filepath.Dir(storePath), "cron_history.json")
	s.historyStore = NewHistoryStore(historyPath)
```

Add getter method:
```go
// GetHistoryStore 获取历史存储
func (s *Service) GetHistoryStore() *HistoryStore {
	return s.historyStore
}
```

In the job execution, add history tracking:
```go
func (s *Service) executeJob(job *Job) {
	record := ExecutionRecord{
		ID:        fmt.Sprintf("exec_%d", time.Now().UnixNano()),
		JobID:     job.ID,
		JobTitle:  job.Title,
		StartedAt: time.Now(),
		Status:    "running",
	}
	s.historyStore.AddRecord(record)

	// Execute job
	start := time.Now()
	output, err := s.onJob(job)
	duration := time.Since(start).Milliseconds()

	// Update record
	now := time.Now()
	s.historyStore.UpdateRecord(record.ID, func(r *ExecutionRecord) {
		r.EndedAt = &now
		r.Duration = duration
		r.Output = output
		if err != nil {
			r.Status = "failed"
			r.Error = err.Error()
		} else {
			r.Status = "success"
		}
	})

	// Update job stats
	job.LastRun = &now
	job.LastResult = output
	if err != nil {
		job.LastError = err.Error()
	} else {
		job.LastError = ""
	}
}
```

**Step 4: Add API endpoints for history**

Modify `internal/webui/server.go`:

Add handlers:
```go
// 添加 cron 历史相关路由
mux.HandleFunc("/api/cron/history", s.handleGetCronHistory)
mux.HandleFunc("/api/cron/history/", s.handleGetCronHistoryDetail)
```

Add handler implementations:
```go
func (s *Server) handleGetCronHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Query().Get("jobId")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	if s.cronService == nil {
		writeJSON(w, map[string]interface{}{"records": []})
		return
	}

	records := s.cronService.GetHistoryStore().GetRecords(jobID, limit)
	writeJSON(w, map[string]interface{}{"records": records})
}

func (s *Server) handleGetCronHistoryDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := extractIDFromPath(r.URL.Path)
	if s.cronService == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	record, found := s.cronService.GetHistoryStore().GetRecord(id)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	writeJSON(w, record)
}
```

**Step 5: Create ExecutionHistory component**

Create `electron/src/renderer/components/ExecutionHistory.tsx`:

```typescript
import React, { useState, useEffect } from 'react';

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
}

export function ExecutionHistory({ jobId }: ExecutionHistoryProps) {
  const [records, setRecords] = useState<ExecutionRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedRecord, setSelectedRecord] = useState<ExecutionRecord | null>(null);

  useEffect(() => {
    fetchHistory();
    const timer = setInterval(fetchHistory, 5000);
    return () => clearInterval(timer);
  }, [jobId]);

  const fetchHistory = async () => {
    try {
      const url = jobId
        ? `http://localhost:18890/api/cron/history?jobId=${jobId}&limit=20`
        : 'http://localhost:18890/api/cron/history?limit=20';
      const response = await fetch(url);
      if (!response.ok) throw new Error('Failed to fetch history');
      const data = await response.json();
      setRecords(data.records || []);
    } catch (err) {
      console.error('Failed to fetch execution history:', err);
    }
  };

  const formatDuration = (ms: number): string => {
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
  };

  const formatTime = (iso: string): string => {
    const date = new Date(iso);
    return date.toLocaleString('zh-CN');
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success': return '✅';
      case 'failed': return '❌';
      case 'running': return '⏳';
      default: return '❓';
    }
  };

  if (records.length === 0) {
    return (
      <div className="execution-history empty">
        <p className="text-muted">暂无执行记录</p>
      </div>
    );
  }

  return (
    <div className="execution-history">
      <h4>执行历史</h4>
      <div className="history-list">
        {records.map((record) => (
          <div
            key={record.id}
            className={`history-item ${record.status}`}
            onClick={() => setSelectedRecord(record)}
          >
            <div className="history-header">
              <span className="status-icon">{getStatusIcon(record.status)}</span>
              <span className="history-title">{record.jobTitle}</span>
              <span className="history-time">{formatTime(record.startedAt)}</span>
            </div>
            <div className="history-meta">
              {record.endedAt && (
                <span className="duration">耗时: {formatDuration(record.durationMs)}</span>
              )}
              {record.status === 'running' && <span className="running">执行中...</span>}
            </div>
          </div>
        ))}
      </div>

      {selectedRecord && (
        <div className="history-detail-modal" onClick={() => setSelectedRecord(null)}>
          <div className="history-detail" onClick={(e) => e.stopPropagation()}>
            <h5>执行详情</h5>
            <div className="detail-row">
              <label>任务:</label>
              <span>{selectedRecord.jobTitle}</span>
            </div>
            <div className="detail-row">
              <label>状态:</label>
              <span>{getStatusIcon(selectedRecord.status)} {selectedRecord.status}</span>
            </div>
            <div className="detail-row">
              <label>开始时间:</label>
              <span>{formatTime(selectedRecord.startedAt)}</span>
            </div>
            {selectedRecord.endedAt && (
              <div className="detail-row">
                <label>结束时间:</label>
                <span>{formatTime(selectedRecord.endedAt)}</span>
              </div>
            )}
            {selectedRecord.durationMs > 0 && (
              <div className="detail-row">
                <label>耗时:</label>
                <span>{formatDuration(selectedRecord.durationMs)}</span>
              </div>
            )}
            {selectedRecord.output && (
              <div className="detail-section">
                <label>输出:</label>
                <pre className="output">{selectedRecord.output}</pre>
              </div>
            )}
            {selectedRecord.error && (
              <div className="detail-section">
                <label>错误:</label>
                <pre className="error">{selectedRecord.error}</pre>
              </div>
            )}
            <button className="close-btn" onClick={() => setSelectedRecord(null)}>
              关闭
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
```

**Step 6: Add CSS for ExecutionHistory**

Add to `electron/src/renderer/views/ScheduledTasksView.css` (or create new file):

```css
.execution-history {
  margin-top: 24px;
}

.execution-history.empty {
  text-align: center;
  padding: 24px;
  color: var(--muted);
}

.execution-history h4 {
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 12px;
  color: var(--foreground);
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.history-item {
  padding: 12px;
  background: var(--secondary);
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s;
}

.history-item:hover {
  background: var(--border);
}

.history-item.success {
  border-left: 3px solid #22c55e;
}

.history-item.failed {
  border-left: 3px solid #ef4444;
}

.history-item.running {
  border-left: 3px solid #3b82f6;
}

.history-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.status-icon {
  font-size: 14px;
}

.history-title {
  flex: 1;
  font-weight: 500;
  font-size: 14px;
}

.history-time {
  font-size: 12px;
  color: var(--muted);
}

.history-meta {
  font-size: 12px;
  color: var(--muted);
  padding-left: 22px;
}

/* Modal styles */
.history-detail-modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.history-detail {
  background: var(--background);
  border-radius: 12px;
  padding: 24px;
  max-width: 600px;
  max-height: 80vh;
  overflow: auto;
  width: 90%;
}

.history-detail h5 {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 16px;
}

.detail-row {
  display: flex;
  margin-bottom: 8px;
  font-size: 14px;
}

.detail-row label {
  width: 80px;
  color: var(--muted);
}

.detail-section {
  margin-top: 16px;
}

.detail-section label {
  display: block;
  color: var(--muted);
  margin-bottom: 8px;
  font-size: 14px;
}

.detail-section pre {
  background: var(--secondary);
  padding: 12px;
  border-radius: 6px;
  font-size: 12px;
  overflow-x: auto;
  max-height: 200px;
  overflow-y: auto;
}

.detail-section pre.error {
  background: #fef2f2;
  color: #dc2626;
}

.close-btn {
  margin-top: 16px;
  padding: 8px 16px;
  background: var(--primary);
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}
```

**Step 7: Integrate into ScheduledTasksView**

Modify `electron/src/renderer/views/ScheduledTasksView.tsx`:

Add import:
```typescript
import { ExecutionHistory } from '../components/ExecutionHistory';
```

In the job list or detail view, add:
```typescript
// In job detail or list section
<ExecutionHistory jobId={selectedJob?.id} />
```

**Step 8: Test execution history**

Run:
```bash
make build
./build/maxclaw gateway &
cd electron && npm run dev
```

Create a test cron job that runs every minute, verify:
1. Job executes
2. History records appear
3. Click history item shows details

**Step 9: Commit**

```bash
git add internal/cron/*.go internal/webui/server.go electron/src/renderer/components/ExecutionHistory.tsx
git commit -m "feat: add execution history logs for scheduled tasks"
```

---

### Task 4: Global Shortcuts

**参考**: `docs/plans/2025-02-21-global-shortcuts.md`

**Files:**
- Create: `electron/src/main/shortcuts.ts`
- Modify: `electron/src/main/index.ts`
- Modify: `electron/src/main/ipc.ts`
- Modify: `electron/src/renderer/views/SettingsView.tsx`

**Step 1: Create shortcut manager**

Create `electron/src/main/shortcuts.ts`:

```typescript
import { globalShortcut, BrowserWindow, app } from 'electron';
import log from 'electron-log';

export interface ShortcutConfig {
  toggleWindow: string;
  newChat: string;
}

const DEFAULT_SHORTCUTS: ShortcutConfig = {
  toggleWindow: 'CommandOrControl+Shift+Space',
  newChat: 'CommandOrControl+N',
};

export class ShortcutManager {
  private mainWindow: BrowserWindow;
  private currentShortcuts: Map<string, string> = new Map();

  constructor(mainWindow: BrowserWindow) {
    this.mainWindow = mainWindow;
  }

  register(config: Partial<ShortcutConfig>): void {
    // Unregister existing first
    this.unregisterAll();

    const merged = { ...DEFAULT_SHORTCUTS, ...config };

    // Register toggle window
    if (merged.toggleWindow) {
      try {
        const registered = globalShortcut.register(merged.toggleWindow, () => {
          this.toggleWindow();
        });
        if (registered) {
          this.currentShortcuts.set('toggleWindow', merged.toggleWindow);
          log.info(`Registered toggle shortcut: ${merged.toggleWindow}`);
        } else {
          log.error(`Failed to register toggle shortcut: ${merged.toggleWindow}`);
        }
      } catch (err) {
        log.error('Error registering toggle shortcut:', err);
      }
    }

    // Register new chat
    if (merged.newChat) {
      try {
        const registered = globalShortcut.register(merged.newChat, () => {
          this.mainWindow.show();
          this.mainWindow.focus();
          this.mainWindow.webContents.send('shortcut:newChat');
        });
        if (registered) {
          this.currentShortcuts.set('newChat', merged.newChat);
          log.info(`Registered new chat shortcut: ${merged.newChat}`);
        }
      } catch (err) {
        log.error('Error registering newChat shortcut:', err);
      }
    }
  }

  unregisterAll(): void {
    globalShortcut.unregisterAll();
    this.currentShortcuts.clear();
    log.info('Unregistered all global shortcuts');
  }

  private toggleWindow(): void {
    if (this.mainWindow.isVisible() && this.mainWindow.isFocused()) {
      this.mainWindow.hide();
    } else {
      this.mainWindow.show();
      this.mainWindow.focus();
    }
  }

  isRegistered(accelerator: string): boolean {
    return globalShortcut.isRegistered(accelerator);
  }

  getCurrentShortcuts(): Map<string, string> {
    return new Map(this.currentShortcuts);
  }
}

export { DEFAULT_SHORTCUTS };
```

**Step 2: Integrate into main process**

Modify `electron/src/main/index.ts`:

```typescript
import { ShortcutManager, DEFAULT_SHORTCUTS } from './shortcuts';

let shortcutManager: ShortcutManager | null = null;

// In openMainWindow function, after creating window:
function openMainWindow() {
  // ... existing code ...

  // Initialize shortcut manager
  if (!shortcutManager) {
    shortcutManager = new ShortcutManager(mainWindow);
    const shortcutsConfig = store.get('shortcuts') as Partial<typeof DEFAULT_SHORTCUTS> || {};
    shortcutManager.register(shortcutsConfig);
  }

  // ... rest of code ...
}

// Cleanup on quit
app.on('will-quit', () => {
  shortcutManager?.unregisterAll();
});
```

**Step 3: Add IPC for shortcut updates**

Modify `electron/src/main/ipc.ts`:

```typescript
// Add to createIPCHandlers:
ipcMain.handle('shortcuts:update', (_, config) => {
  shortcutManager?.register(config);
  return { success: true };
});

ipcMain.handle('shortcuts:get', () => {
  return Object.fromEntries(shortcutManager?.getCurrentShortcuts() || []);
});
```

**Step 4: Add shortcut settings UI**

Modify `electron/src/renderer/views/SettingsView.tsx`:

Add state and UI:
```typescript
const [shortcuts, setShortcuts] = useState({
  toggleWindow: 'CommandOrControl+Shift+Space',
  newChat: 'CommandOrControl+N',
});

useEffect(() => {
  // Load current shortcuts
  window.electronAPI.shortcuts?.get?.().then((current: Record<string, string>) => {
    setShortcuts(prev => ({ ...prev, ...current }));
  });
}, []);

const handleShortcutChange = (key: string, value: string) => {
  const updated = { ...shortcuts, [key]: value };
  setShortcuts(updated);
  window.electronAPI.config.set({ shortcuts: updated });
  window.electronAPI.shortcuts?.update?.(updated);
};

// Add to JSX:
<section className="mb-8">
  <h2 className="text-lg font-semibold mb-4">{t('settings.shortcuts')}</h2>
  <div className="space-y-4">
    <div className="flex items-center justify-between">
      <label className="text-sm font-medium">{t('settings.shortcuts.toggle')}</label>
      <input
        type="text"
        value={shortcuts.toggleWindow}
        onChange={(e) => handleShortcutChange('toggleWindow', e.target.value)}
        className="bg-secondary rounded-lg px-3 py-2 text-sm font-mono w-48"
        placeholder="Cmd+Shift+Space"
      />
    </div>
    <div className="flex items-center justify-between">
      <label className="text-sm font-medium">{t('settings.shortcuts.newChat')}</label>
      <input
        type="text"
        value={shortcuts.newChat}
        onChange={(e) => handleShortcutChange('newChat', e.target.value)}
        className="bg-secondary rounded-lg px-3 py-2 text-sm font-mono w-48"
        placeholder="Cmd+N"
      />
    </div>
  </div>
  <p className="mt-2 text-xs text-foreground/50">
    Use "CommandOrControl" for cross-platform shortcuts
  </p>
</section>
```

**Step 5: Add i18n translations**

Add to i18n files:
```typescript
// zh
'settings.shortcuts': '全局快捷键',
'settings.shortcuts.toggle': '显示/隐藏窗口',
'settings.shortcuts.newChat': '新建对话',

// en
'settings.shortcuts': 'Global Shortcuts',
'settings.shortcuts.toggle': 'Show/Hide Window',
'settings.shortcuts.newChat': 'New Chat',
```

**Step 6: Test shortcuts**

Run and test:
1. Use Cmd/Ctrl+Shift+Space to toggle window
2. Use Cmd/Ctrl+N to create new chat
3. Change shortcuts in settings and verify they work

**Step 7: Commit**

```bash
git add electron/src/main/shortcuts.ts electron/src/main/index.ts electron/src/main/ipc.ts electron/src/renderer/views/SettingsView.tsx
git commit -m "feat(electron): add global keyboard shortcuts"
```

---

### Task 5: Data Import/Export

**参考**: `docs/plans/2025-02-21-data-import-export.md`

**Files:**
- Modify: `electron/src/main/ipc.ts`
- Modify: `electron/src/renderer/views/SettingsView.tsx`
- Install: jszip

**Step 1: Install jszip**

```bash
cd electron && npm install jszip
```

**Step 2: Add IPC handlers**

Modify `electron/src/main/ipc.ts`:

```typescript
import JSZip from 'jszip';

// Export data
ipcMain.handle('data:export', async () => {
  try {
    const result = await dialog.showSaveDialog({
      defaultPath: `maxclaw-backup-${new Date().toISOString().split('T')[0]}.zip`,
      filters: [{ name: 'ZIP Archive', extensions: ['zip'] }],
    });

    if (result.canceled || !result.filePath) {
      return { cancelled: true };
    }

    // Fetch data from Gateway
    const [configRes, sessionsRes] = await Promise.all([
      fetch('http://localhost:18890/api/config'),
      fetch('http://localhost:18890/api/sessions'),
    ]);

    const [config, sessions] = await Promise.all([
      configRes.json(),
      sessionsRes.json(),
    ]);

    // Create ZIP
    const zip = new JSZip();
    zip.file('config.json', JSON.stringify(config, null, 2));
    zip.file('sessions.json', JSON.stringify(sessions, null, 2));
    zip.file('metadata.json', JSON.stringify({
      exportedAt: new Date().toISOString(),
      version: app.getVersion(),
    }, null, 2));

    const buffer = await zip.generateAsync({ type: 'nodebuffer' });
    await fs.promises.writeFile(result.filePath, buffer);

    return { success: true, path: result.filePath };
  } catch (error) {
    log.error('Export failed:', error);
    return { success: false, error: String(error) };
  }
});

// Import data
ipcMain.handle('data:import', async () => {
  try {
    const result = await dialog.showOpenDialog({
      filters: [{ name: 'ZIP Archive', extensions: ['zip'] }],
      properties: ['openFile'],
    });

    if (result.canceled || result.filePaths.length === 0) {
      return { cancelled: true };
    }

    const filePath = result.filePaths[0];
    const data = await fs.promises.readFile(filePath);
    const zip = await JSZip.loadAsync(data);

    // Read files from zip
    const configFile = zip.file('config.json');
    const sessionsFile = zip.file('sessions.json');

    if (!configFile) {
      return { success: false, error: 'Invalid backup file: config.json not found' };
    }

    const configText = await configFile.async('text');
    const config = JSON.parse(configText);

    // Import to Gateway
    const response = await fetch('http://localhost:18890/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config),
    });

    if (!response.ok) {
      throw new Error(`Failed to import config: ${response.statusText}`);
    }

    return { success: true };
  } catch (error) {
    log.error('Import failed:', error);
    return { success: false, error: String(error) };
  }
});
```

**Step 3: Add UI to Settings**

Modify `electron/src/renderer/views/SettingsView.tsx`:

```typescript
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

// Add to JSX:
<section className="mb-8">
  <h2 className="text-lg font-semibold mb-4">{t('settings.dataManagement')}</h2>
  <div className="space-y-4">
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
    <p className="text-xs text-muted">
      导出包含配置和会话数据，导入将覆盖当前配置
    </p>
  </div>
</section>
```

**Step 4: Add i18n**

```typescript
// zh
'settings.dataManagement': '数据管理',
'settings.export': '导出备份',
'settings.import': '导入备份',

// en
'settings.dataManagement': 'Data Management',
'settings.export': 'Export Backup',
'settings.import': 'Import Backup',
```

**Step 5: Test import/export**

Run and test:
1. Click Export - verify ZIP file created
2. Click Import - verify config restored

**Step 6: Commit**

```bash
git add electron/src/main/ipc.ts electron/src/renderer/views/SettingsView.tsx electron/package.json
git commit -m "feat(electron): add data import/export functionality"
```

---

### Task 6: Auto-Update Mechanism

**参考**: `docs/plans/2025-02-21-auto-update.md`

**Files:**
- Modify: `electron/package.json`
- Modify: `electron/electron-builder.yml`
- Modify: `electron/src/main/index.ts`
- Modify: `electron/src/renderer/views/SettingsView.tsx`

**Step 1: Configure electron-builder**

Modify `electron/electron-builder.yml`:

```yaml
appId: com.lichas.maxclaw
productName: Maxclaw
directories:
  output: dist
copyright: Copyright © 2026
publish:
  provider: github
  owner: Lichas
  repo: maxclaw
  releaseType: release
mac:
  category: public.app-category.productivity
  target:
    - dmg
    - zip
win:
  target:
    - nsis
linux:
  target:
    - AppImage
    - deb
```

**Step 2: Add auto-updater to main process**

Modify `electron/src/main/index.ts`:

```typescript
import { autoUpdater } from 'electron-updater';

// Configure auto-updater
function setupAutoUpdater() {
  // Skip in development
  if (isDev) {
    log.info('Auto-updater disabled in development');
    return;
  }

  autoUpdater.logger = log;
  autoUpdater.autoDownload = false; // Manual download

  autoUpdater.on('checking-for-update', () => {
    log.info('Checking for update...');
  });

  autoUpdater.on('update-available', (info) => {
    log.info('Update available:', info);
    mainWindow?.webContents.send('update:available', info);
  });

  autoUpdater.on('update-not-available', () => {
    log.info('Update not available');
  });

  autoUpdater.on('error', (err) => {
    log.error('Update error:', err);
  });

  autoUpdater.on('download-progress', (progress) => {
    mainWindow?.webContents.send('update:progress', progress);
  });

  autoUpdater.on('update-downloaded', (info) => {
    log.info('Update downloaded:', info);
    mainWindow?.webContents.send('update:downloaded', info);
  });

  // Check on startup
  setTimeout(() => {
    autoUpdater.checkForUpdates().catch(err => {
      log.error('Failed to check for updates:', err);
    });
  }, 5000);

  // Check every hour
  setInterval(() => {
    autoUpdater.checkForUpdates().catch(err => {
      log.error('Failed to check for updates:', err);
    });
  }, 60 * 60 * 1000);
}

// Call in app ready:
app.whenReady().then(() => {
  // ... existing code ...
  setupAutoUpdater();
});
```

**Step 3: Add IPC handlers**

Modify `electron/src/main/ipc.ts`:

```typescript
import { autoUpdater } from 'electron-updater';

ipcMain.handle('update:check', async () => {
  try {
    const result = await autoUpdater.checkForUpdates();
    return { success: true, updateInfo: result?.updateInfo };
  } catch (error) {
    return { success: false, error: String(error) };
  }
});

ipcMain.handle('update:download', async () => {
  try {
    await autoUpdater.downloadUpdate();
    return { success: true };
  } catch (error) {
    return { success: false, error: String(error) };
  }
});

ipcMain.handle('update:install', () => {
  autoUpdater.quitAndInstall();
});
```

**Step 4: Add update UI**

Modify `electron/src/renderer/views/SettingsView.tsx`:

```typescript
const [updateStatus, setUpdateStatus] = useState<'checking' | 'available' | 'downloading' | 'downloaded' | 'none'>('none');
const [updateInfo, setUpdateInfo] = useState<any>(null);

useEffect(() => {
  // Listen for update events
  window.electronAPI.update?.onAvailable?.((info: any) => {
    setUpdateStatus('available');
    setUpdateInfo(info);
  });

  window.electronAPI.update?.onDownloaded?.(() => {
    setUpdateStatus('downloaded');
  });

  window.electronAPI.update?.onProgress?.((progress: any) => {
    console.log('Download progress:', progress);
  });
}, []);

const handleCheckUpdate = async () => {
  setUpdateStatus('checking');
  const result = await window.electronAPI.update?.check?.();
  if (!result?.updateInfo) {
    setUpdateStatus('none');
    alert('当前已是最新版本');
  }
};

const handleDownload = async () => {
  setUpdateStatus('downloading');
  await window.electronAPI.update?.download?.();
};

const handleInstall = () => {
  window.electronAPI.update?.install?.();
};

// Add to JSX:
<section className="mb-8">
  <h2 className="text-lg font-semibold mb-4">{t('settings.updates')}</h2>
  <div className="space-y-4">
    {updateStatus === 'none' && (
      <button
        onClick={handleCheckUpdate}
        className="px-4 py-2 bg-secondary rounded-lg text-sm font-medium hover:bg-border transition-colors"
      >
        {t('settings.checkUpdate')}
      </button>
    )}
    {updateStatus === 'checking' && (
      <p className="text-sm text-muted">{t('settings.checking')}</p>
    )}
    {updateStatus === 'available' && updateInfo && (
      <div className="space-y-2">
        <p className="text-sm">
          新版本可用: {updateInfo.version}
        </p>
        <button
          onClick={handleDownload}
          className="px-4 py-2 bg-primary text-white rounded-lg text-sm font-medium"
        >
          {t('settings.downloadUpdate')}
        </button>
      </div>
    )}
    {updateStatus === 'downloading' && (
      <p className="text-sm text-muted">{t('settings.downloading')}</p>
    )}
    {updateStatus === 'downloaded' && (
      <div className="space-y-2">
        <p className="text-sm text-green-600">{t('settings.updateReady')}</p>
        <button
          onClick={handleInstall}
          className="px-4 py-2 bg-primary text-white rounded-lg text-sm font-medium"
        >
          {t('settings.installAndRestart')}
        </button>
      </div>
    )}
  </div>
</section>
```

**Step 5: Add i18n**

```typescript
// zh
'settings.updates': '自动更新',
'settings.checkUpdate': '检查更新',
'settings.checking': '正在检查...',
'settings.downloadUpdate': '下载更新',
'settings.downloading': '正在下载...',
'settings.updateReady': '更新已下载，准备安装',
'settings.installAndRestart': '安装并重启',

// en
'settings.updates': 'Auto Update',
'settings.checkUpdate': 'Check for Updates',
'settings.checking': 'Checking...',
'settings.downloadUpdate': 'Download Update',
'settings.downloading': 'Downloading...',
'settings.updateReady': 'Update ready to install',
'settings.installAndRestart': 'Install and Restart',
```

**Step 6: Commit**

```bash
git add electron/electron-builder.yml electron/src/main/index.ts electron/src/main/ipc.ts electron/src/renderer/views/SettingsView.tsx
git commit -m "feat(electron): add auto-update mechanism"
```

---

## Final Testing Checklist

Run complete test suite:

```bash
# Build and test
cd /Users/lua/git/nanobot-go
make build
make test

# Test Electron
cd electron
npm run build
npm run dev
```

Verify all features:
- [ ] System tray icon shows correctly
- [ ] Cron visual builder works
- [ ] Execution history shows and updates
- [ ] Global shortcuts (Cmd+Shift+Space, Cmd+N) work
- [ ] Data export creates valid ZIP
- [ ] Data import restores config
- [ ] Auto-update checks for updates

---

## Summary

This plan implements all remaining HIGH and LOW priority features:

| Feature | Status | File Count |
|---------|--------|------------|
| Tray Icon Fix | New | 2 files modified |
| Cron Visual Builder | New | 3 files |
| Execution History | New | 6 files |
| Global Shortcuts | Ref existing | 4 files |
| Data Import/Export | Ref existing | 3 files |
| Auto-Update | Ref existing | 4 files |

Total: ~22 files touched, 6 commits expected.
