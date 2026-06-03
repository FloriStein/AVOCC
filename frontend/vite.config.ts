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
      '/ws': { target: 'ws://localhost:8080', ws: true, changeOrigin: true },
      '/vehicle/ws': { target: 'ws://localhost:8080', ws: true, changeOrigin: true },
      '/api': { target: 'http://localhost:8080', rewrite: (p) => p.replace(/^\/api/, ''), changeOrigin: true },
      '/auth': { target: 'http://localhost:8081', changeOrigin: true },
    },
  },
})
