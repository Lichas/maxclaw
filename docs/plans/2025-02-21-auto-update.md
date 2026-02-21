# Auto-Update Mechanism Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement automatic update checking and installation using electron-updater, with user notifications and manual update triggers.

**Architecture:** electron-updater polls GitHub releases for new versions. When update available, show notification to user. User can choose to install now or later. Download happens in background, install on app restart.

**Tech Stack:** electron-updater, GitHub releases, IPC communication

---

### Task 1: Configure electron-updater

**Files:**
- Modify: `electron/package.json`
- Modify: `electron/electron-builder.yml`
- Modify: `electron/src/main/index.ts`

```typescript
import { autoUpdater } from 'electron-updater';
import log from 'electron-log';

// Configure auto-updater
autoUpdater.logger = log;
autoUpdater.checkForUpdatesAndNotify();

// Check every hour
setInterval(() => {
  autoUpdater.checkForUpdatesAndNotify();
}, 60 * 60 * 1000);

autoUpdater.on('update-available', () => {
  mainWindow?.webContents.send('update:available');
});

autoUpdater.on('update-downloaded', () => {
  mainWindow?.webContents.send('update:downloaded');
});
```

---

### Task 2: Add Update UI

**Files:**
- Modify: `electron/src/renderer/views/SettingsView.tsx`

```typescript
const [updateStatus, setUpdateStatus] = useState<'checking' | 'available' | 'downloaded' | 'none'>('none');

useEffect(() => {
  window.electronAPI.update.onAvailable(() => setUpdateStatus('available'));
  window.electronAPI.update.onDownloaded(() => setUpdateStatus('downloaded'));
}, []);

const handleInstallUpdate = () => {
  window.electronAPI.update.install();
};
```

---

### Task 3: GitHub Releases Setup

Configure GitHub repository for releases with signed binaries.

---

### Task 4: Testing

Test update flow:
1. Publish test release
2. Verify app detects update
3. Verify download and install works
