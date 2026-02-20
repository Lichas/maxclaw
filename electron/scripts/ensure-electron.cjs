#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const rootDir = path.resolve(__dirname, '..');
const electronDir = path.join(rootDir, 'node_modules', 'electron');
const installScript = path.join(electronDir, 'install.js');
const pathFile = path.join(electronDir, 'path.txt');

function getPlatformExecutable() {
  switch (process.platform) {
    case 'darwin':
      return 'Electron.app/Contents/MacOS/Electron';
    case 'win32':
      return 'electron.exe';
    default:
      return 'electron';
  }
}

function getExpectedBinaryPath() {
  if (!fs.existsSync(pathFile)) {
    return null;
  }

  const executablePath = fs.readFileSync(pathFile, 'utf8').trim();
  if (!executablePath) {
    return null;
  }

  if (process.env.ELECTRON_OVERRIDE_DIST_PATH) {
    return path.join(process.env.ELECTRON_OVERRIDE_DIST_PATH, executablePath);
  }

  return path.join(electronDir, 'dist', executablePath);
}

function isElectronInstalled() {
  const expectedBinary = getExpectedBinaryPath();
  return Boolean(expectedBinary && fs.existsSync(expectedBinary));
}

if (!fs.existsSync(installScript)) {
  console.error('[ensure-electron] node_modules/electron/install.js not found. Run npm install first.');
  process.exit(1);
}

if (isElectronInstalled()) {
  process.exit(0);
}

const env = { ...process.env };
const usesNpmMirror = (env.npm_config_registry || '').includes('npmmirror.com');

if (!env.ELECTRON_MIRROR && !env.npm_config_electron_mirror && usesNpmMirror) {
  env.ELECTRON_MIRROR = 'https://cdn.npmmirror.com/binaries/electron/';
}

console.log('[ensure-electron] Electron binary missing, running installer...');
if (env.ELECTRON_MIRROR) {
  console.log(`[ensure-electron] mirror: ${env.ELECTRON_MIRROR}`);
}

const installResult = spawnSync(process.execPath, [installScript], {
  cwd: electronDir,
  env,
  stdio: 'inherit',
});

if (installResult.status !== 0) {
  process.exit(installResult.status || 1);
}

if (!isElectronInstalled()) {
  const fallbackBinary = path.join(electronDir, 'dist', getPlatformExecutable());
  const fallbackExists = fs.existsSync(fallbackBinary);
  if (fallbackExists) {
    fs.writeFileSync(pathFile, getPlatformExecutable());
  }
}

if (!isElectronInstalled()) {
  console.error('[ensure-electron] Electron binary is still missing after install.');
  console.error('[ensure-electron] Remove electron/node_modules and run npm install again.');
  process.exit(1);
}

console.log('[ensure-electron] Electron binary is ready.');
