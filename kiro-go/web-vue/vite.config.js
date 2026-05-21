import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'url'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/admin':    { target: 'http://localhost:8990', changeOrigin: true, secure: false },
      '/user/api/': { target: 'http://localhost:8990', changeOrigin: true, secure: false },
      '/v1':       { target: 'http://localhost:8990', changeOrigin: true, secure: false },
      '/health':   { target: 'http://localhost:8990', changeOrigin: true, secure: false },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
