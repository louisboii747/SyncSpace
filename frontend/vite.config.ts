import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../backend/internal/frontend/dist',
    emptyOutDir: true,
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/devices': 'http://127.0.0.1:8384',
      '/device': 'http://127.0.0.1:8384',
      '/discovery': 'http://127.0.0.1:8384',
      '/pairing': 'http://127.0.0.1:8384',
      '/transfers': 'http://127.0.0.1:8384',
      '/health': 'http://127.0.0.1:8384',
      '/diagnostics': 'http://127.0.0.1:8384',
      '/dev': 'http://127.0.0.1:8384',
      '/ws': { target: 'ws://127.0.0.1:8384', ws: true },
    },
  },
})
