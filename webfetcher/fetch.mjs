#!/usr/bin/env node
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { chromium } from 'playwright';

const DEFAULT_UA =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36';
const DEFAULT_WAIT_UNTIL = 'domcontentloaded';
const DEFAULT_CDP_LAUNCH_TIMEOUT_MS = 15000;
const DEFAULT_CHROME_ARGS = [
  '--disable-sync',
  '--disable-background-networking',
  '--disable-component-update',
  '--disable-features=Translate,MediaRouter',
  '--disable-session-crashed-bubble',
  '--hide-crash-restore-bubble',
  '--password-store=basic',
  '--disable-blink-features=AutomationControlled',
  '--no-first-run',
  '--no-default-browser-check',
];

function readStdin() {
  return new Promise((resolve) => {
    let data = '';
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => {
      data += chunk;
    });
    process.stdin.on('end', () => resolve(data));
    process.stdin.on('error', () => resolve(data));
  });
}

function writeResult(payload) {
  process.stdout.write(JSON.stringify(payload));
}

function normalizeMode(mode) {
  if (typeof mode !== 'string') {
    return 'browser';
  }
  const normalized = mode.trim().toLowerCase();
  return normalized === 'chrome' ? 'chrome' : 'browser';
}

function normalizeWaitUntil(waitUntil) {
  if (typeof waitUntil !== 'string') {
    return DEFAULT_WAIT_UNTIL;
  }
  const value = waitUntil.trim();
  if (!value) {
    return DEFAULT_WAIT_UNTIL;
  }
  return value;
}

function sanitizeProfileName(input) {
  if (typeof input !== 'string') {
    return 'chrome';
  }
  const normalized = input
    .trim()
    .replace(/[^a-zA-Z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return normalized || 'chrome';
}

function expandUserPath(inputPath) {
  if (typeof inputPath !== 'string') {
    return '';
  }
  const raw = inputPath.trim();
  if (!raw) {
    return '';
  }
  if (raw === '~') {
    return os.homedir();
  }
  if (raw.startsWith('~/')) {
    return path.join(os.homedir(), raw.slice(2));
  }
  return raw;
}

function resolveChromeUserDataDir(userDataDir, profileName) {
  const expanded = expandUserPath(userDataDir);
  if (expanded) {
    return path.resolve(expanded);
  }
  return path.join(os.homedir(), '.nanobot', 'browser', profileName, 'user-data');
}

function resolveHostChromeUserDataDir(userDataDir, channel) {
  const expanded = expandUserPath(userDataDir);
  if (expanded) {
    return path.resolve(expanded);
  }

  if (process.platform === 'darwin') {
    if (String(channel || '').toLowerCase().startsWith('msedge')) {
      return path.join(os.homedir(), 'Library', 'Application Support', 'Microsoft Edge');
    }
    return path.join(os.homedir(), 'Library', 'Application Support', 'Google', 'Chrome');
  }

  if (process.platform === 'linux') {
    if (String(channel || '').toLowerCase().startsWith('msedge')) {
      return path.join(os.homedir(), '.config', 'microsoft-edge');
    }
    return path.join(os.homedir(), '.config', 'google-chrome');
  }

  return '';
}

function resolveChromeAppName(channel) {
  switch (String(channel || '').trim().toLowerCase()) {
    case 'chrome-beta':
      return 'Google Chrome Beta';
    case 'chrome-dev':
      return 'Google Chrome Dev';
    case 'chrome-canary':
      return 'Google Chrome Canary';
    case 'msedge':
      return 'Microsoft Edge';
    case 'msedge-beta':
      return 'Microsoft Edge Beta';
    case 'msedge-dev':
      return 'Microsoft Edge Dev';
    case 'msedge-canary':
      return 'Microsoft Edge Canary';
    default:
      return 'Google Chrome';
  }
}

function endpointHttpBase(cdpEndpoint) {
  if (typeof cdpEndpoint !== 'string' || !cdpEndpoint.trim()) {
    return '';
  }

  const raw = cdpEndpoint.trim();
  const withScheme = /^[a-z]+:\/\//i.test(raw) ? raw : `http://${raw}`;
  const parsed = new URL(withScheme);
  if (parsed.protocol === 'ws:') {
    parsed.protocol = 'http:';
  } else if (parsed.protocol === 'wss:') {
    parsed.protocol = 'https:';
  }
  parsed.pathname = '';
  parsed.search = '';
  parsed.hash = '';
  return parsed.toString().replace(/\/$/, '');
}

function isLoopbackHost(hostname) {
  const host = String(hostname || '').toLowerCase();
  return host === '127.0.0.1' || host === 'localhost' || host === '::1';
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function escapeAppleScriptString(input) {
  return String(input || '').replace(/\\/g, '\\\\').replace(/"/g, '\\"');
}

function runCommandCapture(command, args, timeoutMs = 10000) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: ['ignore', 'pipe', 'pipe'] });
    let stdout = '';
    let stderr = '';
    let settled = false;

    const timer = setTimeout(() => {
      if (settled) {
        return;
      }
      settled = true;
      child.kill('SIGKILL');
      reject(new Error(`command timeout: ${command}`));
    }, timeoutMs);

    child.stdout.on('data', (chunk) => {
      stdout += chunk.toString('utf8');
    });
    child.stderr.on('data', (chunk) => {
      stderr += chunk.toString('utf8');
    });

    child.on('error', (err) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      reject(err);
    });

    child.on('exit', (code) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      if (code === 0) {
        resolve({ stdout, stderr });
        return;
      }
      reject(new Error((stderr || '').trim() || `command exit code ${code}: ${command}`));
    });
  });
}

async function fetchWithHostChromeAppleScript(req, chrome) {
  if (process.platform !== 'darwin') {
    throw new Error('host takeover via AppleScript is only supported on macOS');
  }

  const appName = resolveChromeAppName(chrome.channel);
  const url = escapeAppleScriptString(req.url);
  const sep = '__NANOBOT_SEP__';

  const jsTitle = "document.title || ''";
  const jsText = "document.body ? document.body.innerText : ''";

  const scriptLines = [
    `tell application "${escapeAppleScriptString(appName)}"`,
    'activate',
    'if (count of windows) = 0 then',
    'make new window',
    'end if',
    `set URL of active tab of front window to "${url}"`,
    'end tell',
    'delay 1.8',
    `tell application "${escapeAppleScriptString(appName)}"`,
    `set pageTitle to execute active tab of front window javascript "${escapeAppleScriptString(jsTitle)}"`,
    `set pageText to execute active tab of front window javascript "${escapeAppleScriptString(jsText)}"`,
    `return pageTitle & linefeed & "${sep}" & linefeed & pageText`,
    'end tell',
  ];

  const args = [];
  for (const line of scriptLines) {
    args.push('-e', line);
  }

  const result = await runCommandCapture('osascript', args, Math.max(req.timeoutMs, 12000));
  const output = (result.stdout || '').replace(/\r/g, '\n');
  const marker = `\n${sep}\n`;
  const idx = output.indexOf(marker);
  if (idx === -1) {
    throw new Error('host Chrome takeover returned unexpected payload');
  }

  const title = normalizeText(output.slice(0, idx));
  const text = normalizeText(output.slice(idx + marker.length));
  return { url: req.url, title, text };
}

function errorMessage(err) {
  return err && typeof err.message === 'string' ? err.message : String(err);
}

function mapHostTakeoverError(err) {
  const raw = errorMessage(err);
  const lower = raw.toLowerCase();

  if (lower.includes('executing javascript through applescript is turned off')) {
    return (
      'host takeover failed: Chrome blocks AppleScript JavaScript execution. ' +
      'Enable "View > Developer > Allow JavaScript from Apple Events" in Chrome, then retry'
    );
  }

  if (lower.includes('not authorized to send apple events')) {
    return (
      'host takeover failed: macOS denied Automation permission. ' +
      'Allow your agent app under System Settings > Privacy & Security > Automation, then retry'
    );
  }

  return `host takeover failed: ${raw}`;
}

async function readDevToolsVersion(cdpEndpoint, timeoutMs = 1500) {
  const base = endpointHttpBase(cdpEndpoint);
  if (!base) {
    throw new Error('invalid cdp endpoint');
  }

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(`${base}/json/version`, { signal: controller.signal });
    if (!response.ok) {
      throw new Error(`CDP version endpoint HTTP ${response.status}`);
    }
    const payload = await response.json();
    const wsUrl = typeof payload.webSocketDebuggerUrl === 'string' ? payload.webSocketDebuggerUrl.trim() : '';
    if (!wsUrl) {
      throw new Error('missing webSocketDebuggerUrl in CDP version response');
    }
    return payload;
  } finally {
    clearTimeout(timer);
  }
}

function launchDetached(command, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { detached: true, stdio: 'ignore' });
    let settled = false;
    child.on('error', (err) => {
      if (settled) {
        return;
      }
      settled = true;
      reject(err);
    });
    child.unref();
    setTimeout(() => {
      if (settled) {
        return;
      }
      settled = true;
      resolve();
    }, 120);
  });
}

function resolveLinuxChromeExecutables(channel) {
  if (String(channel || '').toLowerCase().startsWith('msedge')) {
    return ['microsoft-edge', 'microsoft-edge-stable', 'microsoft-edge-beta', 'microsoft-edge-dev'];
  }
  return ['google-chrome', 'google-chrome-stable', 'chromium-browser', 'chromium'];
}

async function waitForCDPReady(cdpEndpoint, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  let lastError = '';

  while (Date.now() < deadline) {
    try {
      await readDevToolsVersion(cdpEndpoint, 1200);
      return;
    } catch (err) {
      lastError = err && typeof err.message === 'string' ? err.message : String(err);
      await sleep(350);
    }
  }

  throw new Error(`CDP endpoint not ready after ${timeoutMs}ms (${lastError})`);
}

async function ensureHostChromeCDP(chrome) {
  const base = endpointHttpBase(chrome.cdpEndpoint);
  if (!base) {
    throw new Error('invalid cdpEndpoint; cannot auto-start host Chrome');
  }

  const endpointURL = new URL(base);
  if (!isLoopbackHost(endpointURL.hostname)) {
    throw new Error('auto-start host Chrome requires loopback cdpEndpoint');
  }

  const port = endpointURL.port || '9222';
  const launchTimeoutMs = Number.isFinite(chrome.launchTimeoutMs) && chrome.launchTimeoutMs > 0
    ? chrome.launchTimeoutMs
    : DEFAULT_CDP_LAUNCH_TIMEOUT_MS;

  try {
    await readDevToolsVersion(base, 1200);
    return { started: false };
  } catch {
    // not ready yet; continue auto-start
  }

  const hostUserDataDir = resolveHostChromeUserDataDir(chrome.hostUserDataDir, chrome.channel);
  if (!hostUserDataDir) {
    throw new Error('cannot resolve host Chrome user data directory for this platform');
  }
  await fs.mkdir(hostUserDataDir, { recursive: true });

  const startupArgs = [
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${hostUserDataDir}`,
  ];

  if (process.platform === 'darwin') {
    const appName = resolveChromeAppName(chrome.channel);
    // Use `-na` to force a new app instance with our launch args (including CDP port).
    await launchDetached('open', ['-na', appName, '--args', ...startupArgs]);
  } else if (process.platform === 'linux') {
    let launched = false;
    const candidates = resolveLinuxChromeExecutables(chrome.channel);
    for (const command of candidates) {
      try {
        await launchDetached(command, startupArgs);
        launched = true;
        break;
      } catch {
        // try next executable
      }
    }
    if (!launched) {
      throw new Error('failed to launch Chrome/Chromium executable on linux');
    }
  } else {
    throw new Error(`auto-start host Chrome not supported on platform: ${process.platform}`);
  }

  await waitForCDPReady(base, launchTimeoutMs);
  return { started: true };
}

function normalizeChromeConfig(raw) {
  const input = raw && typeof raw === 'object' ? raw : {};
  const profileName = sanitizeProfileName(input.profileName);
  const hasAutoStartCDP = Object.prototype.hasOwnProperty.call(input, 'autoStartCDP');
  const launchTimeoutMs =
    Number.isFinite(input.launchTimeoutMs) && Number(input.launchTimeoutMs) > 0
      ? Number(input.launchTimeoutMs)
      : DEFAULT_CDP_LAUNCH_TIMEOUT_MS;
  return {
    cdpEndpoint: typeof input.cdpEndpoint === 'string' ? input.cdpEndpoint.trim() : '',
    profileName,
    userDataDir: typeof input.userDataDir === 'string' ? input.userDataDir.trim() : '',
    channel: typeof input.channel === 'string' && input.channel.trim() ? input.channel.trim() : 'chrome',
    headless: typeof input.headless === 'boolean' ? input.headless : true,
    autoStartCDP: hasAutoStartCDP ? Boolean(input.autoStartCDP) : true,
    takeoverExisting: typeof input.takeoverExisting === 'boolean' ? input.takeoverExisting : false,
    hostUserDataDir: typeof input.hostUserDataDir === 'string' ? input.hostUserDataDir.trim() : '',
    launchTimeoutMs,
  };
}

function normalizeRequest(raw) {
  const timeoutMs = Number.isFinite(raw.timeoutMs) && raw.timeoutMs > 0 ? raw.timeoutMs : 30000;
  const userAgent = typeof raw.userAgent === 'string' && raw.userAgent.trim() ? raw.userAgent : DEFAULT_UA;
  const waitUntil = normalizeWaitUntil(raw.waitUntil);
  const mode = normalizeMode(raw.mode);
  return {
    url: raw.url,
    timeoutMs,
    userAgent,
    waitUntil,
    mode,
    chrome: normalizeChromeConfig(raw.chrome),
  };
}

function browserContextOptions(req) {
  return {
    userAgent: req.userAgent,
    locale: 'en-US',
    viewport: { width: 1280, height: 720 },
    extraHTTPHeaders: {
      'Accept-Language': 'en-US,en;q=0.9',
    },
  };
}

function normalizeText(raw) {
  if (typeof raw !== 'string') {
    return '';
  }
  return raw.replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim();
}

async function readPage(page, req) {
  await page.goto(req.url, { waitUntil: req.waitUntil, timeout: req.timeoutMs });

  // Let SPA content hydrate before reading visible text.
  await page.waitForTimeout(1200).catch(() => {});

  const { title, text } = await page.evaluate(() => {
    const pageTitle = (document.title || '').trim();
    const candidates = ['main', '[role="main"]', 'article', '#react-root', '#layers', 'body'];
    const pieces = [];

    for (const selector of candidates) {
      const node = document.querySelector(selector);
      if (!node) {
        continue;
      }
      const value = (node.innerText || node.textContent || '').trim();
      if (value) {
        pieces.push(value);
      }
      if (pieces.length >= 3) {
        break;
      }
    }

    const merged = pieces.join('\n\n').trim();
    return { title: pageTitle, text: merged };
  });

  return { url: req.url, title: normalizeText(title), text: normalizeText(text) };
}

async function fetchWithBrowserMode(req) {
  let browser;
  try {
    browser = await chromium.launch({ headless: true });
    const context = await browser.newContext(browserContextOptions(req));
    const page = await context.newPage();
    return await readPage(page, req);
  } finally {
    if (browser) {
      await browser.close();
    }
  }
}

async function fetchWithChromeCDP(req, chrome) {
  let browser;
  let page;
  try {
    browser = await chromium.connectOverCDP(chrome.cdpEndpoint, { timeout: req.timeoutMs });
    let context = browser.contexts()[0];
    if (!context) {
      context = await browser.newContext(browserContextOptions(req));
    }
    page = await context.newPage();
    return await readPage(page, req);
  } finally {
    if (page) {
      await page.close().catch(() => {});
    }
    if (browser) {
      await browser.close().catch(() => {});
    }
  }
}

async function fetchWithChromeProfile(req, chrome) {
  let context;
  const userDataDir = resolveChromeUserDataDir(chrome.userDataDir, chrome.profileName);
  await fs.mkdir(userDataDir, { recursive: true });

  try {
    context = await chromium.launchPersistentContext(userDataDir, {
      ...browserContextOptions(req),
      channel: chrome.channel,
      headless: chrome.headless,
      args: DEFAULT_CHROME_ARGS,
    });
    const page = await context.newPage();
    return await readPage(page, req);
  } finally {
    if (context) {
      await context.close().catch(() => {});
    }
  }
}

async function fetchWithChromeMode(req) {
  const chrome = req.chrome;
  const warnings = [];

  if (chrome.cdpEndpoint) {
    try {
      return await fetchWithChromeCDP(req, chrome);
    } catch (cdpErr) {
      warnings.push(`CDP attach failed: ${errorMessage(cdpErr)}`);
    }
  }

  if (chrome.takeoverExisting) {
    if (process.platform === 'darwin') {
      try {
        const takeover = await fetchWithHostChromeAppleScript(req, chrome);
        const mergedWarning = warnings.length
          ? `[warning] ${warnings.join('; ')}; used host Chrome AppleScript takeover`
          : '[warning] used host Chrome AppleScript takeover';
        const mergedText = takeover.text ? `${mergedWarning}\n\n${takeover.text}` : mergedWarning;
        return { ...takeover, text: mergedText };
      } catch (takeoverErr) {
        warnings.push(mapHostTakeoverError(takeoverErr));
      }
    } else {
      warnings.push(`host takeover requires macOS AppleScript, unsupported on ${process.platform}`);
    }

    const detail = warnings.length ? ` ${warnings.join('; ')}` : '';
    throw new Error(
      'takeoverExisting=true requires direct access to your current Chrome session, but takeover did not succeed.' +
        detail
    );
  }

  if (chrome.cdpEndpoint && chrome.autoStartCDP) {
    try {
      const autoStart = await ensureHostChromeCDP(chrome);
      if (autoStart.started) {
        warnings.push('auto-started host Chrome for CDP takeover');
      } else {
        warnings.push('host Chrome CDP endpoint already ready');
      }
      const connected = await fetchWithChromeCDP(req, chrome);
      const merged = connected.text ? `[warning] ${warnings.join('; ')}\n\n${connected.text}` : `[warning] ${warnings.join('; ')}`;
      return { ...connected, text: merged };
    } catch (autoErr) {
      warnings.push(`auto-start host Chrome failed: ${errorMessage(autoErr)}`);
    }
  }

  const fallback = await fetchWithChromeProfile(req, chrome);
  if (!warnings.length) {
    return fallback;
  }
  const mergedWarning = `[warning] ${warnings.join('; ')}; fallback to managed profile`;
  const mergedText = fallback.text ? `${mergedWarning}\n\n${fallback.text}` : mergedWarning;
  return { ...fallback, text: mergedText };
}

async function main() {
  const raw = (await readStdin()).trim();
  if (!raw) {
    writeResult({ ok: false, error: 'missing request body' });
    return;
  }

  let req;
  try {
    req = JSON.parse(raw);
  } catch (err) {
    writeResult({ ok: false, error: 'invalid JSON request' });
    return;
  }

  const url = req.url;
  if (!url || typeof url !== 'string') {
    writeResult({ ok: false, error: 'url is required' });
    return;
  }
  const normalized = normalizeRequest(req);
  try {
    const result =
      normalized.mode === 'chrome'
        ? await fetchWithChromeMode(normalized)
        : await fetchWithBrowserMode(normalized);
    const title = normalizeText(result.title || '');
    const text = normalizeText(result.text || '');
    if (!title && !text) {
      writeResult({
        ok: false,
        error:
          'page rendered empty content (likely auth wall / anti-bot challenge / blank SPA shell). ' +
          'Try authenticated Chrome CDP session or reuse a logged-in persistent profile.',
      });
      return;
    }
    writeResult({ ok: true, url, title, text });
  } catch (err) {
    const message = errorMessage(err);
    if (normalized.mode === 'chrome' && normalized.chrome.cdpEndpoint && !normalized.chrome.takeoverExisting) {
      writeResult({
        ok: false,
        error:
          message +
          ' (CDP connect failed; start Chrome with --remote-debugging-port=9222, or remove chrome.cdpEndpoint to use a managed profile)',
      });
      return;
    }
    writeResult({ ok: false, error: message });
  }
}

process.on('unhandledRejection', (err) => {
  const message = err && typeof err.message === 'string' ? err.message : String(err);
  writeResult({ ok: false, error: message });
});

process.on('uncaughtException', (err) => {
  const message = err && typeof err.message === 'string' ? err.message : String(err);
  writeResult({ ok: false, error: message });
});

main();
