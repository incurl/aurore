import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import wails from '@wailsio/runtime/plugins/vite';
import { fileURLToPath, URL } from 'node:url';

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [react(), wails('./bindings')],
  resolve: {
    alias: {
      // The vendored InvokeAI components use the upstream @platform/*,
      // @features/* and @theme/* module specifiers. They are rewritten to
      // the local shims in src/wrapped-invoke-ui.
      '@wrapped': fileURLToPath(new URL('./src/wrapped-invoke-ui', import.meta.url)),
    },
  },
});
