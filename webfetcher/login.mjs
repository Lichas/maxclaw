#!/usr/bin/env node
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import readline from 'node:readline';
import { chromium } from 'playwright';

const DEFAULT_URL = 'https://x.com';
const DEFAULT_PROFILE_NAME = 'chrome';
const DEFAULT_CHANNEL = 'chrome';

const LOGIN_CHROME_ARGS = [
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

function resolveUserDataDir(userDataDir, profileName) {
  const expanded = expandUserPath(userDataDir);
  if (expanded) {
    return path.resolve(expanded);
  }

  const profile = String(profileName || DEFAULT_PROFILE_NAME)
    .trim()
    .replace(/[^a-zA-Z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '') || DEFAULT_PROFILE_NAME;

  return path.join(os.homedir(), '.maxclaw', 'browser', profile, 'user-data');
}

function parseArgs(argv) {
  const options = {
    url: DEFAULT_URL,
    profileName: DEFAULT_PROFILE_NAME,
    userDataDir: '',
    channel: DEFAULT_CHANNEL,
    timeoutSec: 0,
  };

  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (token === '--url' && i + 1 < argv.length) {
      options.url = String(argv[++i] || DEFAULT_URL).trim() || DEFAULT_URL;
      continue;
    }
    if (token === '--profile-name' && i + 1 < argv.length) {
      options.profileName = String(argv[++i] || DEFAULT_PROFILE_NAME).trim() || DEFAULT_PROFILE_NAME;
      continue;
    }
    if (token === '--user-data-dir' && i + 1 < argv.length) {
      options.userDataDir = String(argv[++i] || '').trim();
      continue;
    }
    if (token === '--channel' && i + 1 < argv.length) {
      options.channel = String(argv[++i] || DEFAULT_CHANNEL).trim() || DEFAULT_CHANNEL;
      continue;
    }
    if (token === '--timeout-sec' && i + 1 < argv.length) {
      const parsed = Number.parseInt(argv[++i], 10);
      options.timeoutSec = Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
    }
  }

  return options;
}

function waitForDone(timeoutSec) {
  return new Promise((resolve) => {
    const rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout,
    });

    let settled = false;
    const finish = (reason) => {
      if (settled) {
        return;
      }
      settled = true;
      rl.close();
      resolve(reason);
    };

    let timer;
    if (timeoutSec > 0) {
      timer = setTimeout(() => finish('timeout'), timeoutSec * 1000);
    }

    rl.question('[maxclaw] Login complete? Press Enter to close this browser session...\n', () => {
      if (timer) {
        clearTimeout(timer);
      }
      finish('enter');
    });
  });
}

async function main() {
  const opts = parseArgs(process.argv.slice(2));
  const userDataDir = resolveUserDataDir(opts.userDataDir, opts.profileName);
  await fs.mkdir(userDataDir, { recursive: true });

  const context = await chromium.launchPersistentContext(userDataDir, {
    channel: opts.channel,
    headless: false,
    args: LOGIN_CHROME_ARGS,
    viewport: { width: 1280, height: 800 },
  });

  try {
    let page = context.pages()[0];
    if (!page) {
      page = await context.newPage();
    }
    await page.goto(opts.url, { waitUntil: 'domcontentloaded', timeout: 60000 });

    process.stdout.write(`[maxclaw] Opened ${opts.url}\n`);
    process.stdout.write(`[maxclaw] Managed profile directory: ${userDataDir}\n`);
    process.stdout.write('[maxclaw] Please log in manually in this browser window.\n');

    await waitForDone(opts.timeoutSec);
  } finally {
    await context.close().catch(() => {});
  }
}

main().catch((err) => {
  const message = err && typeof err.message === 'string' ? err.message : String(err);
  process.stderr.write(`[maxclaw] browser login failed: ${message}\n`);
  process.exitCode = 1;
});
