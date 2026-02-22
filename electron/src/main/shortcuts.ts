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
