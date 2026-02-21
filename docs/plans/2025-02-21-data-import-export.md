# Data Import/Export Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow users to export and import Gateway configuration and session data for backup and migration.

**Architecture:** Export creates a JSON/zip file with config, sessions, and skills. Import reads the file and restores data, with merge or replace options.

**Tech Stack:** File system APIs, JSON serialization, zip compression

---

### Task 1: Export Functionality

**Files:**
- Modify: `electron/src/main/ipc.ts`

```typescript
ipcMain.handle('data:export', async () => {
  const result = await dialog.showSaveDialog({
    defaultPath: 'nanobot-backup.zip',
    filters: [{ name: 'ZIP', extensions: ['zip'] }],
  });

  if (result.canceled) return { cancelled: true };

  // Export config
  const config = await fetch('http://localhost:18890/api/config').then(r => r.json());

  // Export sessions
  const sessions = await fetch('http://localhost:18890/api/sessions').then(r => r.json());

  // Create zip
  const JSZip = require('jszip');
  const zip = new JSZip();
  zip.file('config.json', JSON.stringify(config, null, 2));
  zip.file('sessions.json', JSON.stringify(sessions, null, 2));

  const buffer = await zip.generateAsync({ type: 'nodebuffer' });
  await fs.promises.writeFile(result.filePath, buffer);

  return { success: true, path: result.filePath };
});
```

---

### Task 2: Import Functionality

```typescript
ipcMain.handle('data:import', async (_, filePath) => {
  const data = await fs.promises.readFile(filePath);
  const JSZip = require('jszip');
  const zip = await JSZip.loadAsync(data);

  const config = JSON.parse(await zip.file('config.json').async('text'));

  // Import to Gateway
  await fetch('http://localhost:18890/api/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  });

  return { success: true };
});
```

---

### Task 3: Settings UI

Add Export/Import buttons in Settings > Data Management section.

---

### Task 4: Testing

Test export and import roundtrip, verify data integrity.
