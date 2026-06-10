import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/ws':         { target: 'ws://localhost:8080', ws: true, changeOrigin: true },
      '/vehicle/ws': { target: 'ws://localhost:8080', ws: true, changeOrigin: true },
      '/api':        { target: 'http://localhost:8080', rewrite: (p) => p.replace(/^\/api/, ''), changeOrigin: true },
      '/auth':       { target: 'http://localhost:8081', changeOrigin: true },
      '/sfu':        { target: 'http://localhost:8084', rewrite: (p) => p.replace(/^\/sfu/, ''), changeOrigin: true },
      '/telemetry':  { target: 'http://localhost:8083', changeOrigin: true },
      '/whep':       { target: 'http://localhost:8889', rewrite: (p) => p.replace(/^\/whep/, ''), changeOrigin: true },
    },
  },
})
