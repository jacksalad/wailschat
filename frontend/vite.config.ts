import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          mermaid: ['mermaid'],
          katex: ['katex'],
          'highlight-lib': ['highlight.js/lib/core'],
        }
      }
    }
  }
})
