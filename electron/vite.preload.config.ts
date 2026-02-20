import { defineConfig } from 'vite';
import path from 'path';

export default defineConfig({
  build: {
    lib: {
      entry: path.resolve(__dirname, 'src/preload/index.ts'),
      formats: ['cjs'],
      fileName: () => 'preload.js'
    },
    outDir: 'dist/main',
    emptyOutDir: false,
    rollupOptions: {
      external: ['electron']
    }
  }
});
