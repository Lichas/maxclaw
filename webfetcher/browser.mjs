#!/usr/bin/env node
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { chromium } from 'playwright';

const DEFAULT_TIMEOUT_MS = 30000;
const DEFAULT_MAX_CHARS = 8000;
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

function normalizeText(raw) {
  if (typeof raw !== 'string') {
    return '';
  }
  return raw.replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim();
}

function asInt(input, fallback) {
  if (Number.isFinite(input)) {
    return Number(input);
  }
  return fallback;
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

function resolveChromeUserDataDir(userDataDir, profileName) {
  const expanded = expandUserPath(userDataDir);
  if (expanded) {
    return path.resolve(expanded);
  }
  return path.join(os.homedir(), '.maxclaw', 'browser', profileName, 'user-data');
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

function resolveLinuxChromeExecutables(channel) {
  if (String(channel || '').toLowerCase().startsWith('msedge')) {
    return ['microsoft-edge', 'microsoft-edge-stable', 'microsoft-edge-beta', 'microsoft-edge-dev'];
  }
  return ['google-chrome', 'google-chrome-stable', 'chromium-browser', 'chromium'];
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
    // continue
  }

  const hostUserDataDir = resolveHostChromeUserDataDir(chrome.hostUserDataDir, chrome.channel);
  if (!hostUserDataDir) {
    throw new Error('cannot resolve host Chrome user data directory for this platform');
  }
  await fs.mkdir(hostUserDataDir, { recursive: true });

  const startupArgs = [`--remote-debugging-port=${port}`, `--user-data-dir=${hostUserDataDir}`];
  if (process.platform === 'darwin') {
    const appName = resolveChromeAppName(chrome.channel);
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
        // try next
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

function sanitizeSessionId(input) {
  const raw = String(input || 'default');
  const normalized = raw.replace(/[^a-zA-Z0-9._-]+/g, '_').replace(/^_+|_+$/g, '');
  return normalized || 'default';
}

function sessionStatePath(sessionId) {
  return path.join(os.homedir(), '.maxclaw', 'browser', 'sessions', `${sanitizeSessionId(sessionId)}.json`);
}

async function readSessionState(sessionId) {
  const file = sessionStatePath(sessionId);
  try {
    const data = await fs.readFile(file, 'utf8');
    const parsed = JSON.parse(data);
    if (parsed && typeof parsed === 'object') {
      if (!Array.isArray(parsed.lastSnapshot)) {
        parsed.lastSnapshot = [];
      }
      if (typeof parsed.activeTab !== 'number') {
        parsed.activeTab = 0;
      }
      if (typeof parsed.lastURL !== 'string') {
        parsed.lastURL = '';
      }
      return parsed;
    }
  } catch {
    // ignore
  }
  return { activeTab: 0, lastSnapshot: [], lastURL: '' };
}

async function writeSessionState(sessionId, state) {
  const file = sessionStatePath(sessionId);
  await fs.mkdir(path.dirname(file), { recursive: true });
  await fs.writeFile(file, JSON.stringify(state, null, 2), 'utf8');
}

function normalizeChromeConfig(raw) {
  const input = raw && typeof raw === 'object' ? raw : {};
  const profileName = sanitizeProfileName(input.profileName);
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
    autoStartCDP: typeof input.autoStartCDP === 'boolean' ? input.autoStartCDP : true,
    hostUserDataDir: typeof input.hostUserDataDir === 'string' ? input.hostUserDataDir.trim() : '',
    launchTimeoutMs,
  };
}

function normalizeRequest(raw) {
  return {
    action: typeof raw.action === 'string' ? raw.action.trim().toLowerCase() : '',
    url: typeof raw.url === 'string' ? raw.url.trim() : '',
    selector: typeof raw.selector === 'string' ? raw.selector.trim() : '',
    ref: asInt(raw.ref, 0),
    text: typeof raw.text === 'string' ? raw.text : '',
    act: typeof raw.act === 'string' ? raw.act.trim().toLowerCase() : 'click',
    x: Math.max(-1, asInt(raw.x, -1)),
    y: Math.max(-1, asInt(raw.y, -1)),
    key: typeof raw.key === 'string' && raw.key.trim() ? raw.key.trim() : 'Enter',
    waitMs: Math.max(0, asInt(raw.waitMs, 1200)),
    tabAction: typeof raw.tabAction === 'string' ? raw.tabAction.trim().toLowerCase() : 'list',
    tabIndex: Math.max(0, asInt(raw.tabIndex, 0)),
    path: typeof raw.path === 'string' ? raw.path.trim() : '',
    fullPage: typeof raw.fullPage === 'boolean' ? raw.fullPage : true,
    maxChars: Math.min(50000, Math.max(200, asInt(raw.maxChars, DEFAULT_MAX_CHARS))),
    timeoutMs: Math.min(180000, Math.max(1000, asInt(raw.timeoutMs, DEFAULT_TIMEOUT_MS))),
    sessionId: sanitizeSessionId(raw.sessionId || 'default'),
    chrome: normalizeChromeConfig(raw.chrome),
  };
}

function actionNeedsURL(action) {
  return action === 'navigate';
}

function isBlankPageURL(raw) {
  const value = String(raw || '').trim().toLowerCase();
  return value === '' || value === 'about:blank';
}

async function restorePageIfNeeded(req, page, state) {
  if (req.url || req.action === 'navigate' || req.action === 'tabs') {
    return false;
  }

  if (!isBlankPageURL(page.url())) {
    return false;
  }

  const fallbackURL = typeof state.lastURL === 'string' ? state.lastURL.trim() : '';
  if (!fallbackURL) {
    return false;
  }

  await page.goto(fallbackURL, { waitUntil: DEFAULT_WAIT_UNTIL, timeout: req.timeoutMs });
  await page.waitForTimeout(900).catch(() => {});
  return true;
}

async function openBrowserContext(req) {
  const chrome = req.chrome;
  const warnings = [];

  if (chrome.cdpEndpoint) {
    try {
      const browser = await chromium.connectOverCDP(chrome.cdpEndpoint, { timeout: req.timeoutMs });
      const context = browser.contexts()[0];
      if (!context) {
        throw new Error('no browser context found via CDP');
      }
      return { mode: 'cdp', browser, context, warnings };
    } catch (cdpErr) {
      warnings.push(`CDP attach failed: ${cdpErr && cdpErr.message ? cdpErr.message : String(cdpErr)}`);
      if (chrome.autoStartCDP) {
        try {
          const autoStart = await ensureHostChromeCDP(chrome);
          if (autoStart.started) {
            warnings.push('auto-started host Chrome for CDP takeover');
          }
          const browser = await chromium.connectOverCDP(chrome.cdpEndpoint, { timeout: req.timeoutMs });
          const context = browser.contexts()[0];
          if (!context) {
            throw new Error('no browser context found via CDP');
          }
          return { mode: 'cdp', browser, context, warnings };
        } catch (autoErr) {
          warnings.push(`auto-start host Chrome failed: ${autoErr && autoErr.message ? autoErr.message : String(autoErr)}`);
        }
      }
    }
  }

  const userDataDir = resolveChromeUserDataDir(chrome.userDataDir, chrome.profileName);
  await fs.mkdir(userDataDir, { recursive: true });
  const context = await chromium.launchPersistentContext(userDataDir, {
    headless: chrome.headless,
    channel: chrome.channel,
    args: DEFAULT_CHROME_ARGS,
    viewport: { width: 1360, height: 900 },
  });
  return { mode: 'profile', browser: null, context, warnings };
}

function clampTabIndex(index, count) {
  if (count <= 0) {
    return 0;
  }
  if (!Number.isFinite(index) || index < 0) {
    return 0;
  }
  if (index >= count) {
    return count - 1;
  }
  return index;
}

async function activePage(context, state) {
  let pages = context.pages();
  if (!pages.length) {
    const created = await context.newPage();
    pages = [created];
  }

  const activeIndex = clampTabIndex(asInt(state.activeTab, 0), pages.length);
  return { page: pages[activeIndex], activeIndex };
}

async function collectSnapshot(page, maxChars) {
  const payload = await page.evaluate((maxLen) => {
    function collapse(v) {
      return (v || '').replace(/\s+/g, ' ').trim();
    }

    function cssPath(el) {
      if (!el || !el.tagName) {
        return '';
      }
      if (el.id) {
        return `#${el.id.replace(/"/g, '\\"')}`;
      }
      const segments = [];
      let cur = el;
      let depth = 0;
      while (cur && cur.nodeType === Node.ELEMENT_NODE && depth < 4) {
        let part = cur.tagName.toLowerCase();
        if (cur.classList && cur.classList.length) {
          part += `.${Array.from(cur.classList).slice(0, 2).join('.')}`;
        }
        const parent = cur.parentElement;
        if (parent) {
          const siblings = Array.from(parent.children).filter((c) => c.tagName === cur.tagName);
          if (siblings.length > 1) {
            part += `:nth-of-type(${siblings.indexOf(cur) + 1})`;
          }
        }
        segments.unshift(part);
        cur = parent;
        depth += 1;
      }
      return segments.join(' > ');
    }

    const title = collapse(document.title || '');
    const text = collapse(document.body ? (document.body.innerText || document.body.textContent || '') : '');
    const refs = [];
    const nodes = Array.from(
      document.querySelectorAll('a,button,input,textarea,select,[role="button"],[contenteditable="true"]')
    ).slice(0, 80);

    nodes.forEach((el, idx) => {
      const label = collapse(
        el.innerText ||
          el.textContent ||
          el.getAttribute('aria-label') ||
          el.getAttribute('placeholder') ||
          el.getAttribute('name') ||
          ''
      );
      refs.push({
        ref: idx + 1,
        tag: el.tagName.toLowerCase(),
        label: label.slice(0, 160),
        selector: cssPath(el),
      });
    });

    return {
      title,
      text: text.slice(0, maxLen),
      refs,
      url: location.href,
    };
  }, maxChars);

  return {
    title: normalizeText(payload.title || ''),
    text: normalizeText(payload.text || ''),
    refs: Array.isArray(payload.refs) ? payload.refs : [],
    url: payload.url || page.url(),
  };
}

function summaryWithWarnings(summary, warnings) {
  if (!warnings || !warnings.length) {
    return summary;
  }
  return `[warning] ${warnings.join('; ')}\n\n${summary}`;
}

async function summarizeTabs(context, activeIndex) {
  const pages = context.pages();
  const list = [];
  for (let i = 0; i < pages.length; i += 1) {
    const page = pages[i];
    let title = '';
    try {
      title = normalizeText(await page.title());
    } catch {
      title = '';
    }
    list.push({
      index: i,
      active: i === activeIndex,
      title,
      url: page.url(),
    });
  }
  return list;
}

async function runAction(req, context, state, warnings) {
  if (actionNeedsURL(req.action) && !req.url) {
    throw new Error(`url is required for action: ${req.action}`);
  }

  const { page, activeIndex } = await activePage(context, state);
  state.activeTab = activeIndex;
  const restored = await restorePageIfNeeded(req, page, state).catch(() => false);
  if (restored) {
    warnings.push(`restored previous page: ${page.url()}`);
  }

  if (req.action === 'navigate') {
    await page.goto(req.url, { waitUntil: DEFAULT_WAIT_UNTIL, timeout: req.timeoutMs });
    await page.waitForTimeout(900).catch(() => {});
    const title = normalizeText(await page.title().catch(() => ''));
    state.lastSnapshot = [];
    state.lastURL = page.url();
    const summary = `Navigated to ${page.url()}\nTitle: ${title || '(empty)'}`;
    return { summary: summaryWithWarnings(summary, warnings), data: { url: page.url(), title } };
  }

  if (req.action === 'snapshot') {
    if (req.url) {
      await page.goto(req.url, { waitUntil: DEFAULT_WAIT_UNTIL, timeout: req.timeoutMs });
      await page.waitForTimeout(900).catch(() => {});
    }
    if (isBlankPageURL(page.url())) {
      throw new Error('snapshot requires an active page; call navigate(url) first or pass url in snapshot');
    }
    const snapshot = await collectSnapshot(page, req.maxChars);
    state.lastSnapshot = snapshot.refs;
    state.lastURL = snapshot.url;
    const refLines = snapshot.refs
      .slice(0, 25)
      .map((item) => `[${item.ref}] <${item.tag}> ${item.label || '(no label)'} ${item.selector ? ` selector=${item.selector}` : ''}`)
      .join('\n');
    const summary = [
      `Snapshot URL: ${snapshot.url}`,
      `Title: ${snapshot.title || '(empty)'}`,
      `Refs: ${snapshot.refs.length}`,
      refLines,
      '',
      `Text: ${snapshot.text || '(empty)'}`,
    ]
      .filter(Boolean)
      .join('\n');
    return {
      summary: summaryWithWarnings(summary, warnings),
      data: { url: snapshot.url, title: snapshot.title, refs: snapshot.refs.length },
    };
  }

  if (req.action === 'screenshot') {
    if (req.url) {
      await page.goto(req.url, { waitUntil: DEFAULT_WAIT_UNTIL, timeout: req.timeoutMs });
      await page.waitForTimeout(900).catch(() => {});
    }
    if (isBlankPageURL(page.url())) {
      throw new Error('screenshot requires an active page; call navigate(url) first or pass url in screenshot');
    }
    const outputPath = req.path
      ? path.resolve(expandUserPath(req.path))
      : path.join(os.homedir(), '.maxclaw', 'browser', 'screenshots', `${req.sessionId}-${Date.now()}.png`);
    await fs.mkdir(path.dirname(outputPath), { recursive: true });
    await page.screenshot({ path: outputPath, fullPage: req.fullPage });
    state.lastURL = page.url();
    const summary = `Screenshot saved: ${outputPath}`;
    return { summary: summaryWithWarnings(summary, warnings), data: { path: outputPath, url: page.url() } };
  }

  if (req.action === 'act') {
    const sub = req.act || 'click';
    let selector = req.selector;
    if (!selector && req.ref > 0 && Array.isArray(state.lastSnapshot)) {
      const found = state.lastSnapshot.find((item) => Number(item.ref) === req.ref);
      if (found && typeof found.selector === 'string') {
        selector = found.selector;
      }
    }

    if (sub !== 'wait' && sub !== 'click_xy' && !selector) {
      throw new Error('selector or ref is required for act(click/type/press)');
    }
    if (isBlankPageURL(page.url()) && sub !== 'wait') {
      throw new Error('act requires an active page; call navigate(url) first or pass url in a preceding action');
    }

    if (sub === 'click') {
      await page.click(selector, { timeout: req.timeoutMs });
    } else if (sub === 'click_xy') {
      if (req.x < 0 || req.y < 0) {
        throw new Error('x and y are required for act(click_xy)');
      }
      await page.mouse.click(req.x, req.y);
    } else if (sub === 'type') {
      await page.fill(selector, req.text || '', { timeout: req.timeoutMs });
    } else if (sub === 'press') {
      await page.press(selector, req.key || 'Enter', { timeout: req.timeoutMs });
    } else if (sub === 'wait') {
      await page.waitForTimeout(Math.max(0, req.waitMs || 1200));
    } else {
      throw new Error(`unsupported act: ${sub}`);
    }

    await page.waitForTimeout(Math.max(300, Math.min(req.waitMs || 700, 4000))).catch(() => {});
    const title = normalizeText(await page.title().catch(() => ''));
    state.lastURL = page.url();
    const summary = `Action ${sub} completed on ${page.url()}\nTitle: ${title || '(empty)'}`;
    return { summary: summaryWithWarnings(summary, warnings), data: { url: page.url(), title, action: sub } };
  }

  if (req.action === 'tabs') {
    const tabAction = req.tabAction || 'list';
    if (tabAction === 'new') {
      const newPage = await context.newPage();
      state.activeTab = context.pages().length - 1;
      if (req.url) {
        await newPage.goto(req.url, { waitUntil: DEFAULT_WAIT_UNTIL, timeout: req.timeoutMs });
      }
    } else if (tabAction === 'switch') {
      const pages = context.pages();
      if (!pages.length) {
        throw new Error('no tabs available');
      }
      if (req.tabIndex < 0 || req.tabIndex >= pages.length) {
        throw new Error(`tab_index out of range: ${req.tabIndex}`);
      }
      state.activeTab = req.tabIndex;
      await pages[req.tabIndex].bringToFront().catch(() => {});
    } else if (tabAction === 'close') {
      const pages = context.pages();
      if (!pages.length) {
        throw new Error('no tabs available');
      }
      if (req.tabIndex < 0 || req.tabIndex >= pages.length) {
        throw new Error(`tab_index out of range: ${req.tabIndex}`);
      }
      await pages[req.tabIndex].close({ runBeforeUnload: false });
      const left = context.pages().length;
      state.activeTab = clampTabIndex(state.activeTab, left);
    } else if (tabAction !== 'list') {
      throw new Error(`unsupported tab_action: ${tabAction}`);
    }

    const tabs = await summarizeTabs(context, clampTabIndex(state.activeTab, context.pages().length));
    if (tabs.length > 0) {
      const currentTab = tabs.find((t) => t.active) || tabs[0];
      if (currentTab && currentTab.url && !isBlankPageURL(currentTab.url)) {
        state.lastURL = currentTab.url;
      }
    }
    const lines = tabs.map((t) => `${t.active ? '*' : ' '}[${t.index}] ${t.title || '(untitled)'} ${t.url}`);
    const summary = `Tabs (${tabs.length})\n${lines.join('\n')}`;
    return { summary: summaryWithWarnings(summary, warnings), data: { tabs } };
  }

  throw new Error(`unsupported action: ${req.action}`);
}

async function main() {
  const raw = (await readStdin()).trim();
  if (!raw) {
    writeResult({ ok: false, error: 'missing request body' });
    return;
  }

  let reqBody;
  try {
    reqBody = JSON.parse(raw);
  } catch {
    writeResult({ ok: false, error: 'invalid JSON request' });
    return;
  }
  const req = normalizeRequest(reqBody);
  if (!req.action) {
    writeResult({ ok: false, error: 'action is required' });
    return;
  }

  let opened;
  try {
    const state = await readSessionState(req.sessionId);
    opened = await openBrowserContext(req);
    const result = await runAction(req, opened.context, state, opened.warnings);
    await writeSessionState(req.sessionId, state);
    writeResult({ ok: true, summary: result.summary, data: result.data });
  } catch (err) {
    const message = err && typeof err.message === 'string' ? err.message : String(err);
    writeResult({ ok: false, error: message });
  } finally {
    if (opened) {
      if (opened.mode === 'profile' && opened.context) {
        await opened.context.close().catch(() => {});
      }
      if (opened.mode === 'cdp' && opened.browser) {
        await opened.browser.close().catch(() => {});
      }
    }
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
