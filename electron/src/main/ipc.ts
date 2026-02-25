import { ipcMain, BrowserWindow, dialog, shell, app } from 'electron';
import { spawn as spawnPty, type IPty } from 'node-pty';
import { execFile } from 'node:child_process';
import { createRequire } from 'node:module';
import { promisify } from 'node:util';
import fs from 'fs';
import path from 'path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import Store from 'electron-store';
import AutoLaunch from 'auto-launch';
import log from 'electron-log';
import JSZip from 'jszip';
import { autoUpdater } from 'electron-updater';
import { GatewayManager } from './gateway';
import { NotificationManager } from './notifications';
import { ShortcutManager } from './shortcuts';

interface AppConfig {
  theme: 'light' | 'dark' | 'system';
  language: 'zh' | 'en';
  autoLaunch: boolean;
  minimizeToTray: boolean;
  shortcuts: Record<string, string>;
}

const configStore = new Store<AppConfig>({
  name: 'app-config',
  defaults: {
    theme: 'system',
    language: 'zh',
    autoLaunch: false,
    minimizeToTray: true,
    shortcuts: {
      toggleWindow: 'CommandOrControl+Shift+N',
      newChat: 'CommandOrControl+N'
    }
  }
});

const autoLauncher = new AutoLaunch({
  name: 'Maxclaw',
  path: app.getPath('exe')
});
const nodeRequire = createRequire(import.meta.url);
const execFileAsync = promisify(execFile);

let handlersRegistered = false;
let currentMainWindow: BrowserWindow | null = null;
let gatewayStatusTimer: NodeJS.Timeout | null = null;
const terminalProcesses = new Map<string, IPty>();

const MAX_TEXT_PREVIEW_BYTES = 1024 * 1024;

const markdownExtensions = new Set(['.md', '.markdown', '.mdown']);
const textExtensions = new Set([
  '.txt',
  '.log',
  '.csv',
  '.ts',
  '.tsx',
  '.js',
  '.jsx',
  '.mjs',
  '.cjs',
  '.json',
  '.jsonl',
  '.yaml',
  '.yml',
  '.toml',
  '.xml',
  '.html',
  '.css',
  '.scss',
  '.go',
  '.py',
  '.java',
  '.rb',
  '.rs',
  '.c',
  '.cc',
  '.cpp',
  '.h',
  '.hpp',
  '.sh',
  '.zsh',
  '.bash',
  '.sql'
]);
const imageExtensions = new Set(['.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp', '.svg']);
const videoExtensions = new Set(['.mp4', '.webm', '.mov', '.m4v']);
const audioExtensions = new Set(['.mp3', '.wav', '.m4a', '.ogg', '.flac']);
const officeExtensions = new Set(['.docx', '.pptx', '.xlsx']);

type PreviewKind = 'markdown' | 'text' | 'image' | 'pdf' | 'audio' | 'video' | 'office' | 'binary';

interface FileResolveOptions {
  workspace?: string;
  sessionKey?: string;
}

interface FilePreviewResult {
  success: boolean;
  inputPath: string;
  resolvedPath?: string;
  kind?: PreviewKind;
  extension?: string;
  fileUrl?: string;
  content?: string;
  truncated?: boolean;
  size?: number;
  error?: string;
}

interface OpenPathResult {
  success: boolean;
  resolvedPath?: string;
  openedPath?: string;
  error?: string;
}

interface FileExistsResult {
  exists: boolean;
  resolvedPath?: string;
  isFile?: boolean;
  error?: string;
}

interface FileListEntry {
  name: string;
  path: string;
  type: 'file' | 'directory';
  size?: number;
  modifiedTime?: string;
}

interface FileListResult {
  success: boolean;
  entries?: FileListEntry[];
  error?: string;
}

function sendTerminalData(sessionKey: string, chunk: string): void {
  if (currentMainWindow && !currentMainWindow.isDestroyed()) {
    currentMainWindow.webContents.send('terminal:data', { sessionKey, chunk });
  }
}

function sendTerminalExit(sessionKey: string, code: number | null, signal: string | null): void {
  if (currentMainWindow && !currentMainWindow.isDestroyed()) {
    currentMainWindow.webContents.send('terminal:exit', { sessionKey, code, signal });
  }
}

function parseShellExecutable(raw: string | undefined): string | undefined {
  if (!raw) {
    return undefined;
  }

  const value = raw.trim();
  if (value === '') {
    return undefined;
  }

  const [command] = value.split(/\s+/);
  return command || undefined;
}

function resolveShellCandidates(): Array<{ command: string; args: string[] }> {
  if (process.platform === 'win32') {
    return [
      { command: 'powershell.exe', args: ['-NoLogo'] },
      { command: 'cmd.exe', args: [] }
    ];
  }

  const candidates: Array<{ command: string; args: string[] }> = [];
  const appendCandidate = (command: string | undefined) => {
    if (!command) {
      return;
    }
    const exists = command.startsWith('/') ? fs.existsSync(command) : true;
    if (!exists) {
      return;
    }
    if (!candidates.some((candidate) => candidate.command === command)) {
      candidates.push({ command, args: ['-i'] });
    }
  };

  appendCandidate(parseShellExecutable(process.env.SHELL));
  appendCandidate('/bin/zsh');
  appendCandidate('/bin/bash');
  appendCandidate('/bin/sh');

  return candidates;
}

function ensurePtySpawnHelperExecutable(): void {
  if (process.platform === 'win32') {
    return;
  }

  const helperCandidates = new Set<string>();

  try {
    const packageJson = nodeRequire.resolve('node-pty/package.json');
    const packageDir = path.dirname(packageJson);
    helperCandidates.add(path.join(packageDir, 'prebuilds', `${process.platform}-${process.arch}`, 'spawn-helper'));
  } catch (error) {
    log.warn('Failed to ensure node-pty spawn-helper executable:', error);
  }

  helperCandidates.add(
    path.join(process.cwd(), 'node_modules', 'node-pty', 'prebuilds', `${process.platform}-${process.arch}`, 'spawn-helper')
  );
  helperCandidates.add(
    path.join(
      process.resourcesPath,
      'app.asar.unpacked',
      'node_modules',
      'node-pty',
      'prebuilds',
      `${process.platform}-${process.arch}`,
      'spawn-helper'
    )
  );

  for (const helperPath of helperCandidates) {
    try {
      if (!fs.existsSync(helperPath)) {
        continue;
      }

      const stats = fs.statSync(helperPath);
      if ((stats.mode & 0o111) === 0) {
        fs.chmodSync(helperPath, 0o755);
        log.info('Updated node-pty spawn-helper permission:', helperPath);
      }
    } catch (error) {
      log.warn('Failed to update node-pty spawn-helper permission:', helperPath, error);
    }
  }
}

function buildPtyEnv(): Record<string, string> {
  const env: Record<string, string> = {};
  for (const [key, value] of Object.entries(process.env)) {
    if (typeof value === 'string') {
      env[key] = value;
    }
  }

  if (!env.TERM) {
    env.TERM = 'xterm-256color';
  }
  if (!env.LANG) {
    env.LANG = 'en_US.UTF-8';
  }

  return env;
}

function resolveTerminalCwd(): string {
  const home = process.env.HOME || app.getPath('home');
  if (home && fs.existsSync(home)) {
    return home;
  }
  return process.cwd();
}

function normalizeSessionKey(value: unknown): string {
  if (typeof value !== 'string') {
    return 'default';
  }
  const normalized = value.trim();
  return normalized === '' ? 'default' : normalized;
}

function sanitizeSessionKey(input: string): string {
  if (!input) {
    return 'default';
  }

  let out = '';
  for (const char of input) {
    if (
      (char >= 'a' && char <= 'z') ||
      (char >= 'A' && char <= 'Z') ||
      (char >= '0' && char <= '9') ||
      char === '-' ||
      char === '_'
    ) {
      out += char;
    } else {
      out += '_';
    }
  }

  return out || 'default';
}

function trimPathToken(raw: string): string {
  let value = (raw || '').trim();
  value = value.replace(/^`+|`+$/g, '');
  value = value.replace(/^['"]+|['"]+$/g, '');
  // 特殊处理：如果原始值是 "." 或 ".."，保留它们
  if (value === '.' || value === '..') {
    return value;
  }
  value = value.replace(/[),.;]+$/g, '');
  return value.trim();
}

function resolveSessionOutputDir(workspace: string | undefined, sessionKey: string | undefined): string | undefined {
  const root = (workspace || '').trim();
  if (!root) {
    return undefined;
  }
  const normalizedSession = sanitizeSessionKey((sessionKey || '').trim());
  return path.join(root, '.sessions', normalizedSession);
}

function resolveLocalFilePath(inputPath: string, options?: FileResolveOptions): string {
  const raw = trimPathToken(inputPath);
  if (!raw) {
    throw new Error('empty path');
  }

  let candidate = raw;
  if (candidate.startsWith('file://')) {
    candidate = fileURLToPath(candidate);
  } else if (candidate.startsWith('~/')) {
    candidate = path.join(app.getPath('home'), candidate.slice(2));
  }

  if (path.isAbsolute(candidate)) {
    return path.normalize(candidate);
  }

  const workspace = (options?.workspace || '').trim();
  const baseDir = resolveSessionOutputDir(workspace, options?.sessionKey) || workspace || process.cwd();
  return path.resolve(baseDir, candidate);
}

function checkFileExists(targetPath: string, options?: FileResolveOptions): FileExistsResult {
  try {
    const resolvedPath = resolveLocalFilePath(targetPath, options);
    const stat = fs.statSync(resolvedPath);
    return {
      exists: true,
      isFile: stat.isFile(),
      resolvedPath
    };
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if ((error as NodeJS.ErrnoException | undefined)?.code === 'ENOENT') {
      return { exists: false };
    }
    return {
      exists: false,
      error: message
    };
  }
}

async function listDirectory(dirPath: string, options?: FileResolveOptions): Promise<FileListResult> {
  try {
    const resolvedPath = resolveLocalFilePath(dirPath, options);
    const stat = await fs.promises.stat(resolvedPath);

    if (!stat.isDirectory()) {
      return {
        success: false,
        error: 'Path is not a directory'
      };
    }

    const entries = await fs.promises.readdir(resolvedPath, { withFileTypes: true });
    const result: FileListEntry[] = [];

    for (const entry of entries) {
      // 隐藏文件和目录不显示
      if (entry.name.startsWith('.')) {
        continue;
      }

      const entryPath = path.join(resolvedPath, entry.name);
      const entryStat = await fs.promises.stat(entryPath);

      result.push({
        name: entry.name,
        path: entryPath,
        type: entry.isDirectory() ? 'directory' : 'file',
        size: entryStat.size,
        modifiedTime: entryStat.mtime.toISOString()
      });
    }

    // 按类型排序：目录在前，然后按名称排序
    result.sort((a, b) => {
      if (a.type !== b.type) {
        return a.type === 'directory' ? -1 : 1;
      }
      return a.name.localeCompare(b.name);
    });

    return {
      success: true,
      entries: result
    };
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error)
    };
  }
}

function detectPreviewKind(ext: string): PreviewKind {
  if (markdownExtensions.has(ext)) {
    return 'markdown';
  }
  if (textExtensions.has(ext)) {
    return 'text';
  }
  if (imageExtensions.has(ext)) {
    return 'image';
  }
  if (videoExtensions.has(ext)) {
    return 'video';
  }
  if (audioExtensions.has(ext)) {
    return 'audio';
  }
  if (ext === '.pdf') {
    return 'pdf';
  }
  if (officeExtensions.has(ext)) {
    return 'office';
  }
  return 'binary';
}

function decodeXmlEntities(input: string): string {
  return input
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'");
}

function stripOfficeXML(xml: string): string {
  const withBreaks = xml
    .replace(/<\/w:p>/gi, '\n')
    .replace(/<\/a:p>/gi, '\n')
    .replace(/<w:tab\/>/gi, '\t')
    .replace(/<br\s*\/?>/gi, '\n');

  const withoutTags = withBreaks.replace(/<[^>]+>/g, '');
  const decoded = decodeXmlEntities(withoutTags);
  return decoded
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .join('\n');
}

function slideNumber(entry: string): number {
  const matched = entry.match(/slide(\d+)\.xml$/i);
  if (!matched) {
    return Number.MAX_SAFE_INTEGER;
  }
  return Number.parseInt(matched[1], 10);
}

async function listZipEntries(filePath: string): Promise<string[]> {
  const { stdout } = await execFileAsync('unzip', ['-Z1', filePath], { maxBuffer: 4 * 1024 * 1024 });
  return stdout
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
}

async function readZipEntry(filePath: string, entry: string): Promise<string> {
  const { stdout } = await execFileAsync('unzip', ['-p', filePath, entry], { maxBuffer: 8 * 1024 * 1024 });
  return stdout;
}

async function extractOfficePreviewText(filePath: string, extension: string): Promise<string> {
  const entries = await listZipEntries(filePath);

  if (extension === '.docx') {
    if (!entries.includes('word/document.xml')) {
      return '';
    }
    const xml = await readZipEntry(filePath, 'word/document.xml');
    return stripOfficeXML(xml);
  }

  if (extension === '.pptx') {
    const slides = entries
      .filter((entry) => /^ppt\/slides\/slide\d+\.xml$/i.test(entry))
      .sort((a, b) => slideNumber(a) - slideNumber(b));
    if (slides.length === 0) {
      return '';
    }

    const chunks: string[] = [];
    for (let index = 0; index < slides.length; index += 1) {
      const xml = await readZipEntry(filePath, slides[index]);
      const text = stripOfficeXML(xml);
      if (!text) {
        continue;
      }
      chunks.push(`Slide ${index + 1}\n${text}`);
    }
    return chunks.join('\n\n');
  }

  if (extension === '.xlsx') {
    const chunks: string[] = [];
    if (entries.includes('xl/sharedStrings.xml')) {
      const xml = await readZipEntry(filePath, 'xl/sharedStrings.xml');
      const text = stripOfficeXML(xml);
      if (text) {
        chunks.push(text);
      }
    }

    const sheets = entries
      .filter((entry) => /^xl\/worksheets\/sheet\d+\.xml$/i.test(entry))
      .sort();
    for (const sheet of sheets.slice(0, 3)) {
      const xml = await readZipEntry(filePath, sheet);
      const text = stripOfficeXML(xml);
      if (text) {
        chunks.push(text);
      }
    }

    return chunks.join('\n\n');
  }

  return '';
}

async function buildFilePreview(inputPath: string, options?: FileResolveOptions): Promise<FilePreviewResult> {
  try {
    const resolvedPath = resolveLocalFilePath(inputPath, options);
    const stat = await fs.promises.stat(resolvedPath);
    if (!stat.isFile()) {
      return {
        success: false,
        inputPath,
        resolvedPath,
        error: 'Path is not a file'
      };
    }

    const extension = path.extname(resolvedPath).toLowerCase();
    const kind = detectPreviewKind(extension);
    const baseResult: FilePreviewResult = {
      success: true,
      inputPath,
      resolvedPath,
      extension,
      kind,
      size: stat.size,
      fileUrl: `${pathToFileURL(resolvedPath).toString()}?t=${Date.now()}`
    };

    if (kind === 'markdown' || kind === 'text') {
      const content = await fs.promises.readFile(resolvedPath, 'utf8');
      if (Buffer.byteLength(content, 'utf8') > MAX_TEXT_PREVIEW_BYTES) {
        const truncated = content.slice(0, MAX_TEXT_PREVIEW_BYTES);
        return {
          ...baseResult,
          content: `${truncated}\n\n... (preview truncated)`,
          truncated: true
        };
      }

      return {
        ...baseResult,
        content
      };
    }

    if (kind === 'office') {
      try {
        const officeText = await extractOfficePreviewText(resolvedPath, extension);
        return {
          ...baseResult,
          content: officeText || '无法提取可预览文本，请使用“打开文件”查看完整内容。'
        };
      } catch (error) {
        return {
          ...baseResult,
          content: `暂不支持直接渲染该 Office 文件。\n${error instanceof Error ? error.message : String(error)}`
        };
      }
    }

    return baseResult;
  } catch (error) {
    return {
      success: false,
      inputPath,
      error: error instanceof Error ? error.message : String(error)
    };
  }
}

async function openInFolder(inputPath: string, options?: FileResolveOptions): Promise<OpenPathResult> {
  try {
    const resolvedPath = resolveLocalFilePath(inputPath, options);
    const stat = await fs.promises.stat(resolvedPath);
    const targetDir = stat.isDirectory() ? resolvedPath : path.dirname(resolvedPath);
    const openErr = await shell.openPath(targetDir);
    if (openErr) {
      return { success: false, resolvedPath, openedPath: targetDir, error: openErr };
    }
    return { success: true, resolvedPath, openedPath: targetDir };
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error)
    };
  }
}

let shortcutManagerInstance: ShortcutManager | null = null;

export function createIPCHandlers(
  mainWindow: BrowserWindow,
  gatewayManager: GatewayManager,
  notificationManager?: NotificationManager,
  shortcutManager?: ShortcutManager
): void {
  // Store shortcut manager reference for IPC handlers
  if (shortcutManager) {
    shortcutManagerInstance = shortcutManager;
  }
  currentMainWindow = mainWindow;
  mainWindow.once('closed', () => {
    if (currentMainWindow === mainWindow) {
      currentMainWindow = null;
    }
    for (const [, terminalProcess] of terminalProcesses) {
      try {
        terminalProcess.kill();
      } catch (error) {
        log.warn('Failed to stop terminal process on window close:', error);
      }
    }
    terminalProcesses.clear();
  });

  if (handlersRegistered) {
    return;
  }
  handlersRegistered = true;

  // Gateway IPC
  ipcMain.handle('gateway:getStatus', () => gatewayManager.getStatus());

  ipcMain.handle('gateway:restart', async () => {
    try {
      await gatewayManager.restart();
      return { success: true };
    } catch (error) {
      log.error('Failed to restart gateway:', error);
      return { success: false, error: String(error) };
    }
  });

  // Shortcuts IPC
  ipcMain.handle('shortcuts:update', (_, config) => {
    shortcutManagerInstance?.register(config);
    return { success: true };
  });

  ipcMain.handle('shortcuts:get', () => {
    return Object.fromEntries(shortcutManagerInstance?.getCurrentShortcuts() || []);
  });

  // Config IPC
  ipcMain.handle('config:get', () => configStore.get());

  ipcMain.handle('config:set', (_, config: Partial<AppConfig>) => {
    const current = configStore.get();
    const updated = { ...current, ...config };
    configStore.set(updated);

    // Handle auto-launch
    if (config.autoLaunch !== undefined) {
      if (config.autoLaunch) {
        autoLauncher.enable();
      } else {
        autoLauncher.disable();
      }
    }

    // Notify renderer of config change
    if (currentMainWindow && !currentMainWindow.isDestroyed()) {
      currentMainWindow.webContents.send('config:change', updated);
    }

    return updated;
  });

  // System IPC
  ipcMain.handle('notification:show', (_, payload) => {
    if (notificationManager) {
      notificationManager.showNotification(payload);
    }
  });

  ipcMain.handle('notification:request-permission', async () => {
    if (notificationManager) {
      return await notificationManager.requestPermission();
    }
    return false;
  });

  ipcMain.handle('system:openExternal', (_, url: string) => {
    shell.openExternal(url);
  });

  ipcMain.handle('system:openPath', async (_, targetPath: string, options?: FileResolveOptions) => {
    try {
      const resolvedPath = resolveLocalFilePath(targetPath, options);
      const openErr = await shell.openPath(resolvedPath);
      if (openErr) {
        return { success: false, error: openErr, resolvedPath };
      }
      return { success: true, resolvedPath };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : String(error)
      };
    }
  });

  ipcMain.handle('system:openInFolder', async (_, targetPath: string, options?: FileResolveOptions) => {
    return openInFolder(targetPath, options);
  });

  ipcMain.handle('system:previewFile', async (_, targetPath: string, options?: FileResolveOptions) => {
    return buildFilePreview(targetPath, options);
  });

  ipcMain.handle('system:fileExists', async (_, targetPath: string, options?: FileResolveOptions) => {
    return checkFileExists(targetPath, options);
  });

  ipcMain.handle('system:listDirectory', async (_, dirPath: string, options?: FileResolveOptions) => {
    return listDirectory(dirPath, options);
  });

  ipcMain.handle('system:selectFolder', async () => {
    const targetWindow = currentMainWindow && !currentMainWindow.isDestroyed() ? currentMainWindow : undefined;
    const result = await dialog.showOpenDialog(targetWindow, {
      properties: ['openDirectory'],
      title: 'Select Folder'
    });

    if (result.canceled || result.filePaths.length === 0) {
      return null;
    }

    return result.filePaths[0];
  });

  ipcMain.handle('system:selectFile', async (_, filters) => {
    const targetWindow = currentMainWindow && !currentMainWindow.isDestroyed() ? currentMainWindow : undefined;
    const result = await dialog.showOpenDialog(targetWindow, {
      properties: ['openFile'],
      filters: filters || [{ name: 'All Files', extensions: ['*'] }],
      title: 'Select File'
    });

    if (result.canceled || result.filePaths.length === 0) {
      return null;
    }

    return result.filePaths[0];
  });

  // Terminal IPC
  ipcMain.handle(
    'terminal:start',
    async (
      _,
      sessionKeyOrOptions?: string | { cols?: number; rows?: number },
      maybeOptions?: { cols?: number; rows?: number }
    ) => {
      const key = normalizeSessionKey(typeof sessionKeyOrOptions === 'string' ? sessionKeyOrOptions : undefined);
      const options = typeof sessionKeyOrOptions === 'string' ? maybeOptions : sessionKeyOrOptions;
      const existing = terminalProcesses.get(key);
      if (existing) {
        if (options?.cols && options?.rows) {
          existing.resize(options.cols, options.rows);
        }
        return { success: true, alreadyRunning: true };
      }

      ensurePtySpawnHelperExecutable();

      const shellCandidates = resolveShellCandidates();
      const cols = options?.cols && options.cols > 0 ? options.cols : 120;
      const rows = options?.rows && options.rows > 0 ? options.rows : 28;
      const cwd = resolveTerminalCwd();
      const env = buildPtyEnv();
      let usedShell = '';
      let lastError = '';
      let ptyProcess: IPty | null = null;

      for (const shellCandidate of shellCandidates) {
        try {
          ptyProcess = spawnPty(shellCandidate.command, shellCandidate.args, {
            name: 'xterm-256color',
            cols,
            rows,
            cwd,
            env
          });
          usedShell = shellCandidate.command;
          break;
        } catch (error) {
          const message = error instanceof Error ? error.message : String(error);
          lastError = message;
          log.warn(`Failed to start terminal shell candidate ${shellCandidate.command}:`, message);
        }
      }

      if (!ptyProcess) {
        const fallbackError = lastError || 'no available shell candidates';
        log.error('Failed to start terminal shell:', fallbackError);
        return { success: false, error: fallbackError };
      }

      terminalProcesses.set(key, ptyProcess);

      ptyProcess.onData((data: string) => {
        sendTerminalData(key, data);
      });

      ptyProcess.onExit(({ exitCode, signal }) => {
        sendTerminalExit(key, exitCode, signal !== undefined && signal !== null ? String(signal) : null);
        terminalProcesses.delete(key);
      });

      sendTerminalData(key, `\r\n[terminal] started with ${usedShell}\r\n`);
      return { success: true, shell: usedShell };
    }
  );

  ipcMain.handle('terminal:input', async (_, sessionKeyOrValue: string, maybeValue?: string) => {
    const key = maybeValue === undefined ? 'default' : normalizeSessionKey(sessionKeyOrValue);
    const value = maybeValue === undefined ? sessionKeyOrValue : maybeValue;
    const terminalProcess = terminalProcesses.get(key);
    if (!terminalProcess) {
      return { success: false, error: 'terminal not running' };
    }
    if (typeof value !== 'string') {
      return { success: false, error: 'invalid terminal input' };
    }

    terminalProcess.write(value);
    return { success: true };
  });

  ipcMain.handle('terminal:resize', async (_, sessionKeyOrCols: string | number, maybeCols: number, maybeRows?: number) => {
    const key = typeof sessionKeyOrCols === 'string' ? normalizeSessionKey(sessionKeyOrCols) : 'default';
    const cols = typeof sessionKeyOrCols === 'string' ? maybeCols : sessionKeyOrCols;
    const rows = typeof sessionKeyOrCols === 'string' ? maybeRows : maybeCols;
    const terminalProcess = terminalProcesses.get(key);
    if (!terminalProcess) {
      return { success: false, error: 'terminal not running' };
    }
    if (typeof cols === 'number' && typeof rows === 'number' && cols > 0 && rows > 0) {
      terminalProcess.resize(cols, rows);
      return { success: true };
    }
    return { success: false, error: 'invalid cols/rows' };
  });

  ipcMain.handle('terminal:stop', async (_, sessionKey?: string) => {
    const key = normalizeSessionKey(sessionKey);
    const terminalProcess = terminalProcesses.get(key);
    if (terminalProcess) {
      terminalProcess.kill();
      terminalProcesses.delete(key);
    }
    return { success: true };
  });

  // Gateway status polling - notify renderer
  gatewayStatusTimer = setInterval(() => {
    const status = gatewayManager.getStatus();
    if (currentMainWindow && !currentMainWindow.isDestroyed()) {
      currentMainWindow.webContents.send('gateway:status-change', status);
    }
  }, 5000);

  // Notification polling from Gateway
  if (notificationManager) {
    const notificationTimer = setInterval(async () => {
      try {
        const response = await fetch('http://localhost:18890/api/notifications/pending');
        if (!response.ok) return;

        const notifications = await response.json();
        for (const notif of notifications) {
          notificationManager.showNotification({
            title: notif.title,
            body: notif.body,
            data: notif.data
          });

          // Mark as delivered
          await fetch(`http://localhost:18890/api/notifications/${notif.id}/delivered`, {
            method: 'POST'
          });
        }
      } catch (error) {
        // Gateway might not support notifications yet
        log.debug('Notification check failed:', error);
      }
    }, 5000);

    notificationTimer.unref();
  }

  // Keep Node process from being blocked by this timer on shutdown.
  gatewayStatusTimer.unref();

  // Update IPC handlers
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

  // Data export/import IPC
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
}
