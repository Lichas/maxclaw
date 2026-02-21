# Global Shortcuts Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement global keyboard shortcuts (e.g., Cmd/Ctrl+Shift+Space) to quickly show/hide the app window from anywhere in the OS.

**Architecture:** Electron main process registers global shortcuts using `electron-global-shortcut` API. When shortcut is triggered, toggle window visibility. Settings UI allows users to configure shortcut keys. Shortcuts are persisted in app config.

**Tech Stack:** Electron globalShortcut API, IPC communication

---

## Prerequisites

- Electron main process has access to window instance
- Settings UI can modify shortcuts configuration

---

### Task 1: Create Shortcut Service

**Files:**
- Create: `electron/src/main/shortcuts.ts`
- Modify: `electron/src/main/index.ts`

**Step 1: Create shortcut manager**

```typescript
import { globalShortcut, BrowserWindow } from 'electron';
import log from 'electron-log';

export interface ShortcutConfig {
  toggleWindow: string;
  newChat: string;
}

export class ShortcutManager {
  private mainWindow: BrowserWindow;
  private currentShortcuts: Map<string, string> = new Map();

  constructor(mainWindow: BrowserWindow) {
    this.mainWindow = mainWindow;
  }

  register(config: ShortcutConfig): void {
    // Unregister existing shortcuts
    this.unregisterAll();

    // Register toggle window shortcut
    if (config.toggleWindow) {
      const registered = globalShortcut.register(config.toggleWindow, () => {
        this.toggleWindow();
      });
      if (registered) {
        this.currentShortcuts.set('toggleWindow', config.toggleWindow);
        log.info(`Registered shortcut: ${config.toggleWindow}`);
      } else {
        log.error(`Failed to register shortcut: ${config.toggleWindow}`);
      }
    }

    // Register new chat shortcut
    if (config.newChat) {
      const registered = globalShortcut.register(config.newChat, () => {
        this.mainWindow.webContents.send('shortcut:newChat');
      });
      if (registered) {
        this.currentShortcuts.set('newChat', config.newChat);
        log.info(`Registered shortcut: ${config.newChat}`);
      }
    }
  }

  unregisterAll(): void {
    globalShortcut.unregisterAll();
    this.currentShortcuts.clear();
    log.info('Unregistered all shortcuts');
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
}
```

**Step 2: Integrate into main process**

```typescript
// In index.ts
import { ShortcutManager } from './shortcuts';

let shortcutManager: ShortcutManager;

function initializeApp() {
  mainWindow = createWindow();
  shortcutManager = new ShortcutManager(mainWindow);

  // Load and register shortcuts from config
  const config = store.get('shortcuts') || {
    toggleWindow: 'CommandOrControl+Shift+Space',
    newChat: 'CommandOrControl+N',
  };
  shortcutManager.register(config);
}

// Cleanup on quit
app.on('will-quit', () => {
  shortcutManager?.unregisterAll();
});
```

**Step 3: Commit**

```bash
git add electron/src/main/shortcuts.ts electron/src/main/index.ts
git commit -m "feat(electron): add global shortcuts support"
```

---

### Task 2: Create Shortcut Settings UI

**Files:**
- Modify: `electron/src/renderer/views/SettingsView.tsx`

**Step 1: Add shortcut configuration UI**

```typescript
const [shortcuts, setShortcuts] = useState({
  toggleWindow: 'CommandOrControl+Shift+Space',
  newChat: 'CommandOrControl+N',
});

const handleShortcutChange = (key: string, value: string) => {
  const updated = { ...shortcuts, [key]: value };
  setShortcuts(updated);
  window.electronAPI.config.set({ shortcuts: updated });
  window.electronAPI.shortcuts.register(updated);
};

// In JSX:
<section className="mb-8">
  <h2 className="text-lg font-semibold mb-4">Keyboard Shortcuts</h2>
  <div className="space-y-4">
    <div className="flex items-center justify-between">
      <label className="text-sm font-medium">Toggle Window</label>
      <input
        type="text"
        value={shortcuts.toggleWindow}
        onChange={(e) => handleShortcutChange('toggleWindow', e.target.value)}
        className="bg-secondary rounded-lg px-3 py-2 text-sm font-mono"
        placeholder="Cmd+Shift+Space"
      />
    </div>
    <div className="flex items-center justify-between">
      <label className="text-sm font-medium">New Chat</label>
      <input
        type="text"
        value={shortcuts.newChat}
        onChange={(e) => handleShortcutChange('newChat', e.target.value)}
        className="bg-secondary rounded-lg px-3 py-2 text-sm font-mono"
        placeholder="Cmd+N"
      />
    </div>
  </div>
  <p className="mt-2 text-xs text-foreground/50">
    Use "CommandOrControl" for cross-platform shortcuts
  </p>
</section>
```

**Step 2: Commit**

```bash
git add electron/src/renderer/views/SettingsView.tsx
git commit -m "feat(electron): add shortcuts settings UI"
```

---

### Task 3: Test and Document

Test scenarios:
1. Use shortcut to show/hide window
2. Change shortcut in settings
3. Verify new shortcut works immediately
4. Test on different platforms (macOS/Windows/Linux)

Update documentation.
