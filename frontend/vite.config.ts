import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    // The Go binary embeds backend/cmd/portsleuthd/web — output there.
    outDir: '../backend/cmd/portsleuthd/web',
    emptyOutDir: true,
    sourcemap: false,
    rollupOptions: {
      output: {
        // Hashed chunks make caching safe.
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]',
      },
    },
  },
  server: {
    port: 5173,
    // Dev: proxy API + WebSocket to the Go backend on :1234.
    proxy: {
      '/api': {
        target: 'http://localhost:1234',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:1234',
        ws: true,
        changeOrigin: true,
      },
      '/metrics': 'http://localhost:1234',
    },
  },
})
