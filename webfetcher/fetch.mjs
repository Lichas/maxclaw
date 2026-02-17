#!/usr/bin/env node
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { chromium } from 'playwright';

const DEFAULT_UA =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36';
const DEFAULT_WAIT_UNTIL = 'domcontentloaded';
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

function normalizeChromeConfig(raw) {
  const input = raw && typeof raw === 'object' ? raw : {};
  const profileName = sanitizeProfileName(input.profileName);
  return {
    cdpEndpoint: typeof input.cdpEndpoint === 'string' ? input.cdpEndpoint.trim() : '',
    profileName,
    userDataDir: typeof input.userDataDir === 'string' ? input.userDataDir.trim() : '',
    channel: typeof input.channel === 'string' && input.channel.trim() ? input.channel.trim() : 'chrome',
    headless: typeof input.headless === 'boolean' ? input.headless : true,
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
  if (chrome.cdpEndpoint) {
    try {
      const result = await fetchWithChromeCDP(req, chrome);
      return result;
    } catch (err) {
      const message = err && typeof err.message === 'string' ? err.message : String(err);
      const fallback = await fetchWithChromeProfile(req, chrome);
      const warning = `[warning] CDP attach failed, fallback to managed profile: ${message}`;
      const mergedText = fallback.text ? `${warning}\n\n${fallback.text}` : warning;
      return { ...fallback, text: mergedText };
    }
  }
  return await fetchWithChromeProfile(req, chrome);
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
    const message = err && typeof err.message === 'string' ? err.message : String(err);
    if (normalized.mode === 'chrome' && normalized.chrome.cdpEndpoint) {
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
