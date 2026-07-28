import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

const isTauriBuild = !!process.env.TAURI_ENV_PLATFORM

export default defineConfig({
  plugins: [
    vue(),
    {
      name: 'tauri-crossorigin-fix',
      transformIndexHtml: isTauriBuild
        ? (html) => html.replace(/crossorigin/g, '')
        : undefined,
    },
  ],
  base: isTauriBuild ? './' : '/app/',
  define: {
    __ROUTER_BASE__: JSON.stringify(isTauriBuild ? '/' : '/app/'),
    __TAURI_BUILD__: JSON.stringify(isTauriBuild),
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
    dedupe: ['vue'],
  },
  optimizeDeps: {
    // Linked file: packages — exclude so Vite loads fresh dist (avoids stale Dq* exports).
    include: ['reka-ui', 'codemirror', '@codemirror/language-data', '@codemirror/view', '@codemirror/state'],
    exclude: ['@danqing/dq-tokens', '@danqing/dq-ui', '@danqing/dq-shell'],
  },
  server: {
    port: Number(process.env.DQ_FRONTEND_PORT || 5801),
    strictPort: true,
    proxy: {
      '/api': { target: `http://127.0.0.1:${process.env.DQ_BACKEND_PORT || 7801}`, changeOrigin: true, ws: true },
    },
  },
  build: {
    outDir: '../out/frontend/dist',
    emptyOutDir: true,
  },
})
