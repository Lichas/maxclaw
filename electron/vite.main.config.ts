import { defineConfig } from 'vite';
import path from 'path';

export default defineConfig({
  build: {
    ssr: true,
    lib: {
      entry: path.resolve(__dirname, 'src/main/index.ts'),
      formats: ['cjs'],
      fileName: () => 'index.js'
    },
    outDir: 'dist/main',
    emptyOutDir: false,
    rollupOptions: {
      external: ['electron', 'electron-log', 'electron-store', 'electron-updater', 'auto-launch', 'child_process', 'path', 'os', 'fs', 'http']
    }
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@main': path.resolve(__dirname, './src/main'),
      '@shared': path.resolve(__dirname, './src/shared')
    }
  }
});
