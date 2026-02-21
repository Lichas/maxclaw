# System Notifications Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement system-level notifications for task completions, new messages, and scheduled task execution results. Notifications should appear in the OS notification center and clicking them should bring the app to focus.

**Architecture:** Electron main process uses `electron-notification` module to show native notifications. Backend Gateway sends notification events via WebSocket or HTTP endpoint when tasks complete. Renderer process listens for notification events and triggers main process to show system notifications via IPC.

**Tech Stack:** Electron Notification API, WebSocket/HTTP events, IPC communication

---

## Prerequisites

- Electron app has IPC bridge configured
- Gateway can trigger events on task completion
- App has necessary OS permissions for notifications

---

### Task 1: Create Notification Service (Main Process)

**Files:**
- Create: `electron/src/main/notifications.ts`

**Step 1: Create notification manager**

```typescript
import { Notification, BrowserWindow } from 'electron';
import log from 'electron-log';

interface NotificationPayload {
  title: string;
  body: string;
  icon?: string;
  data?: {
    sessionKey?: string;
    taskId?: string;
    type: 'message' | 'task_complete' | 'scheduled_task';
  };
}

export class NotificationManager {
  private mainWindow: BrowserWindow;

  constructor(mainWindow: BrowserWindow) {
    this.mainWindow = mainWindow;
  }

  showNotification(payload: NotificationPayload): void {
    if (!Notification.isSupported()) {
      log.warn('Notifications not supported on this platform');
      return;
    }

    const notification = new Notification({
      title: payload.title,
      body: payload.body,
      icon: payload.icon || undefined,
      silent: false,
    });

    notification.on('click', () => {
      this.handleNotificationClick(payload);
    });

    notification.show();
    log.info(`Notification shown: ${payload.title}`);
  }

  private handleNotificationClick(payload: NotificationPayload): void {
    // Bring window to front
    if (this.mainWindow.isMinimized()) {
      this.mainWindow.restore();
    }
    this.mainWindow.show();
    this.mainWindow.focus();

    // Send event to renderer
    this.mainWindow.webContents.send('notification:clicked', payload.data);
  }

  // Check and request notification permissions
  async requestPermission(): Promise<boolean> {
    if (process.platform === 'darwin') {
      // macOS uses native notification permissions
      return Notification.isSupported();
    }
    return true;
  }
}
```

**Step 2: Commit**

```bash
git add electron/src/main/notifications.ts
git commit -m "feat(electron): add NotificationManager service"
```

---

### Task 2: Add IPC Handlers for Notifications

**Files:**
- Modify: `electron/src/main/ipc.ts`
- Modify: `electron/src/main/index.ts`

**Step 1: Initialize notification manager**

In `index.ts`:

```typescript
import { NotificationManager } from './notifications';

let notificationManager: NotificationManager;

async function initializeApp(): Promise<void> {
  // ... existing code ...

  mainWindow = createWindow();
  notificationManager = new NotificationManager(mainWindow);

  // Setup IPC handlers
  createIPCHandlers(mainWindow, gatewayManager, notificationManager);
}
```

**Step 2: Add IPC handlers**

In `ipc.ts`:

```typescript
import { NotificationManager } from './notifications';

export function createIPCHandlers(
  mainWindow: BrowserWindow,
  gatewayManager: GatewayManager,
  notificationManager: NotificationManager
): void {
  // ... existing handlers ...

  // Notification IPC
  ipcMain.handle('notification:show', (_, payload) => {
    notificationManager.showNotification(payload);
  });

  ipcMain.handle('notification:request-permission', async () => {
    return await notificationManager.requestPermission();
  });

  // Listen for Gateway events
  setInterval(async () => {
    await checkGatewayNotifications(gatewayManager, notificationManager);
  }, 5000);
}

async function checkGatewayNotifications(
  gatewayManager: GatewayManager,
  notificationManager: NotificationManager
): Promise<void> {
  try {
    const response = await fetch('http://localhost:18890/api/notifications/pending');
    if (!response.ok) return;

    const notifications = await response.json();
    for (const notif of notifications) {
      notificationManager.showNotification({
        title: notif.title,
        body: notif.body,
        data: notif.data,
      });

      // Mark as delivered
      await fetch(`http://localhost:18890/api/notifications/${notif.id}/delivered`, {
        method: 'POST',
      });
    }
  } catch (error) {
    // Gateway might not support notifications yet
    log.debug('Notification check failed:', error);
  }
}
```

**Step 3: Commit**

```bash
git add electron/src/main/ipc.ts electron/src/main/index.ts
git commit -m "feat(electron): integrate notification manager with IPC"
```

---

### Task 3: Create Backend Notification API

**Files:**
- Create: `internal/webui/notifications.go`
- Modify: `internal/webui/server.go`

**Step 1: Create notification store and handlers**

```go
package webui

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time `json:"createdAt"`
	Delivered bool      `json:"delivered"`
}

type NotificationStore struct {
	mu            sync.RWMutex
	notifications []Notification
	maxSize       int
}

func NewNotificationStore() *NotificationStore {
	return &NotificationStore{
		notifications: make([]Notification, 0),
		maxSize:       100,
	}
}

func (s *NotificationStore) Add(title, body string, data map[string]interface{}) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	notif := Notification{
		ID:        uuid.New().String()[:8],
		Title:     title,
		Body:      body,
		Data:      data,
		CreatedAt: time.Now(),
		Delivered: false,
	}

	s.notifications = append(s.notifications, notif)

	// Trim old notifications
	if len(s.notifications) > s.maxSize {
		s.notifications = s.notifications[len(s.notifications)-s.maxSize:]
	}

	return notif.ID
}

func (s *NotificationStore) GetPending() []Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pending := make([]Notification, 0)
	for _, n := range s.notifications {
		if !n.Delivered {
			pending = append(pending, n)
		}
	}
	return pending
}

func (s *NotificationStore) MarkDelivered(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.notifications {
		if s.notifications[i].ID == id {
			s.notifications[i].Delivered = true
			return
		}
	}
}

// HTTP Handlers
func (s *Server) handleGetPendingNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	pending := s.notificationStore.GetPending()
	writeJSON(w, pending)
}

func (s *Server) handleMarkNotificationDelivered(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := extractIDFromPath(r.URL.Path)
	s.notificationStore.MarkDelivered(id)
	writeJSON(w, map[string]bool{"ok": true})
}
```

**Step 2: Add to server.go**

```go
type Server struct {
    // ... existing fields ...
    notificationStore *NotificationStore
}

func NewServer(cfg *config.Config) *Server {
    // ... existing code ...
    s.notificationStore = NewNotificationStore()
    return s
}

func (s *Server) registerRoutes() {
    // ... existing routes ...
    mux.HandleFunc("/api/notifications/pending", s.handleGetPendingNotifications)
    mux.HandleFunc("/api/notifications/", s.handleMarkNotificationDelivered)
}
```

**Step 3: Commit**

```bash
git add internal/webui/notifications.go internal/webui/server.go
git commit -m "feat(api): add notification store and API endpoints"
```

---

### Task 4: Trigger Notifications on Events

**Files:**
- Modify: `internal/cron/scheduler.go`
- Modify: `internal/agent/loop.go`

**Step 1: Add notification trigger to cron**

```go
func (s *Scheduler) executeJob(job *Job) {
	// ... existing execution code ...

	// Send notification on completion
	s.server.notificationStore.Add(
		"定时任务完成",
		fmt.Sprintf("任务 \"%s\" 执行完成", job.Title),
		map[string]interface{}{
			"type": "scheduled_task",
			"jobId": job.ID,
		},
	)
}
```

**Step 2: Add notification for new messages**

```go
func (b *AgentLoop) handleMessage(msg InboundMessage) {
	// ... existing code ...

	// Notify if from external channel
	if msg.Channel != "desktop" {
		b.server.notificationStore.Add(
			"新消息",
			fmt.Sprintf("收到来自 %s 的新消息", msg.Channel),
			map[string]interface{}{
				"type": "message",
				"sessionKey": msg.SessionKey,
				"channel": msg.Channel,
			},
		)
	}
}
```

**Step 3: Commit**

```bash
git add internal/cron/scheduler.go internal/agent/loop.go
git commit -m "feat: trigger notifications on task/message events"
```

---

### Task 5: Frontend Notification Settings

**Files:**
- Modify: `electron/src/renderer/views/SettingsView.tsx`
- Modify: `electron/src/renderer/store/index.ts`

**Step 1: Add notification settings UI**

Add to SettingsView:

```typescript
const [notificationsEnabled, setNotificationsEnabled] = useState(true);

useEffect(() => {
  // Request permission on mount
  if (notificationsEnabled) {
    window.electronAPI.system.requestNotificationPermission();
  }
}, []);

// In JSX, add section:
<section className="mb-8">
  <h2 className="text-lg font-semibold mb-4">{t('settings.notifications')}</h2>
  <div className="space-y-4">
    <label className="flex items-center justify-between">
      <span className="text-sm font-medium">{t('settings.notifications.enable')}</span>
      <input
        type="checkbox"
        checked={notificationsEnabled}
        onChange={(e) => {
          setNotificationsEnabled(e.target.checked);
          window.electronAPI.system.setNotificationsEnabled(e.target.checked);
        }}
        className="w-4 h-4"
      />
    </label>
  </div>
</section>
```

**Step 2: Add translations**

```typescript
// zh
'settings.notifications': '通知',
'settings.notifications.enable': '启用系统通知',

// en
'settings.notifications': 'Notifications',
'settings.notifications.enable': 'Enable system notifications',
```

**Step 3: Commit**

```bash
git add electron/src/renderer/views/SettingsView.tsx electron/src/renderer/i18n/index.ts
git commit -m "feat(electron): add notification settings UI"
```

---

### Task 6: Test and Document

**Step 1: Test scenarios**

1. Enable notifications in settings
2. Run a scheduled task - verify notification appears
3. Send message from Telegram - verify notification appears
4. Click notification - verify app comes to front
5. Disable notifications - verify no notifications appear

**Step 2: Update documentation**

Update CHANGELOG.md, Electron_STATUS.md, Electron_PRD.md.

**Step 3: Commit**

```bash
git add docs/ CHANGELOG.md
git commit -m "docs: update for system notifications feature"
```

---

## Summary

After completing this plan:
- System notifications work on all supported platforms
- Notifications triggered on task completion and new messages
- Clicking notification brings app to focus
- Users can enable/disable notifications in settings
- Backend tracks pending/delivered notifications
