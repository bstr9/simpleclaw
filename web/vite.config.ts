import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  server: {
    port: 5173,
    proxy: {
      '/message': {
        target: 'http://localhost:9899',
        changeOrigin: true
      },
      '/stream': {
        target: 'http://localhost:9899',
        changeOrigin: true
      },
      '/upload': {
        target: 'http://localhost:9899',
        changeOrigin: true
      },
      '/uploads': {
        target: 'http://localhost:9899',
        changeOrigin: true
      },
      '/config': {
        target: 'http://localhost:9899',
        changeOrigin: true
      },
      '/api': {
        target: 'http://localhost:9899',
        changeOrigin: true
      },
      '/admin/api': {
        target: 'http://localhost:9899',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    minify: 'esbuild',
    rollupOptions: {
      output: {
        manualChunks: {
          'element-plus': ['element-plus'],
          'vendor': ['vue', 'vue-router', 'pinia']
        }
      }
    }
  }
})
