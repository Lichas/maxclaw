import { BrowserWindow, app, ipcMain } from 'electron';
import path from 'path';
import log from 'electron-log';

/**
 * Windows 平台特定集成
 * - 跳转列表 (Jump List)
 * - 任务栏进度条
 * - 缩略图工具栏
 */

// 设置 Windows 跳转列表
export function setupWindowsJumpList(): void {
  if (process.platform !== 'win32') {
    return;
  }

  try {
    app.setJumpList([
      {
        type: 'tasks',
        items: [
          {
            type: 'task',
            program: process.execPath,
            args: '--new-chat',
            iconPath: process.execPath,
            iconIndex: 0,
            title: 'New Chat',
            description: 'Start a new chat session'
          },
          {
            type: 'task',
            program: process.execPath,
            args: '--show-settings',
            iconPath: process.execPath,
            iconIndex: 0,
            title: 'Settings',
            description: 'Open settings'
          }
        ]
      },
      {
        type: 'frequent'
      },
      {
        type: 'recent'
      }
    ]);

    log.info('[windows] Jump list configured');
  } catch (error) {
    log.error('[windows] Failed to set jump list:', error);
  }
}

// Windows 任务栏进度条管理器
export class WindowsTaskbarManager {
  private window: BrowserWindow;
  private currentProgress = 0;

  constructor(window: BrowserWindow) {
    this.window = window;
    this.setupIPCHandlers();
  }

  private setupIPCHandlers(): void {
    // 设置进度条
    ipcMain.handle('windows:set-progress', (_, progress: number, mode?: 'normal' | 'error' | 'paused') => {
      this.setProgress(progress, mode);
    });

    // 清除进度条
    ipcMain.handle('windows:clear-progress', () => {
      this.clearProgress();
    });
  }

  setProgress(progress: number, mode: 'normal' | 'error' | 'paused' = 'normal'): void {
    if (process.platform !== 'win32' || this.window.isDestroyed()) {
      return;
    }

    // 限制范围在 0-1
    this.currentProgress = Math.max(0, Math.min(1, progress));

    try {
      // 将模式转换为 Electron 的格式
      const modeMap: Record<string, number> = {
        normal: 0,
        error: 2,
        paused: 1
      };

      this.window.setProgressBar(this.currentProgress, {
        mode: modeMap[mode] as 0 | 1 | 2
      });

      log.debug('[windows] Taskbar progress:', { progress: this.currentProgress, mode });
    } catch (error) {
      log.error('[windows] Failed to set progress:', error);
    }
  }

  clearProgress(): void {
    if (process.platform !== 'win32' || this.window.isDestroyed()) {
      return;
    }

    try {
      // -1 表示隐藏进度条
      this.window.setProgressBar(-1);
      this.currentProgress = 0;
      log.debug('[windows] Taskbar progress cleared');
    } catch (error) {
      log.error('[windows] Failed to clear progress:', error);
    }
  }
}

// 应用用户任务（用于 Windows 7+ 的任务栏）
export function setupWindowsUserTasks(): void {
  if (process.platform !== 'win32') {
    return;
  }

  try {
    app.setUserTasks([
      {
        program: process.execPath,
        arguments: '--new-chat',
        iconPath: process.execPath,
        iconIndex: 0,
        title: 'New Chat',
        description: 'Start a new conversation'
      },
      {
        program: process.execPath,
        arguments: '--quick-search',
        iconPath: process.execPath,
        iconIndex: 0,
        title: 'Quick Search',
        description: 'Search sessions'
      }
    ]);

    log.info('[windows] User tasks configured');
  } catch (error) {
    log.error('[windows] Failed to set user tasks:', error);
  }
}

// 处理 Windows 特定的命令行参数
export function handleWindowsCLIArgs(): void {
  if (process.platform !== 'win32') {
    return;
  }

  const args = process.argv.slice(1);

  if (args.includes('--new-chat')) {
    log.info('[windows] CLI arg: --new-chat');
    // 通过 IPC 发送给渲染进程处理
  }

  if (args.includes('--show-settings')) {
    log.info('[windows] CLI arg: --show-settings');
  }

  if (args.includes('--quick-search')) {
    log.info('[windows] CLI arg: --quick-search');
  }
}

// 初始化所有 Windows 集成功能
export function initializeWindowsIntegration(window: BrowserWindow): void {
  if (process.platform !== 'win32') {
    return;
  }

  setupWindowsJumpList();
  setupWindowsUserTasks();
  handleWindowsCLIArgs();

  // 创建任务栏进度管理器
  const taskbarManager = new WindowsTaskbarManager(window);

  log.info('[windows] Integration initialized');
}
