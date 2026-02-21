const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const electronPath = path.join(__dirname, '..', 'node_modules', 'electron', 'dist', 'Electron.app');

if (!fs.existsSync(electronPath)) {
  console.log('Electron not found, installing...');
  try {
    execSync('npm install electron@28.0.0', {
      cwd: path.join(__dirname, '..'),
      stdio: 'inherit'
    });
    console.log('Electron installed successfully');
  } catch (error) {
    console.error('Failed to install Electron:', error.message);
    process.exit(1);
  }
} else {
  console.log('Electron already installed');
}
