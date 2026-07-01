import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  // NOTE: do NOT add `build.rollupOptions.output.manualChunks` for mermaid/
  // katex. Forcing these into their own chunk pulls shared helpers (e.g. Vue's
  // defineAsyncComponent wrapper) into that chunk, which the entry then imports
  // statically — Vite responds by injecting <link rel="modulepreload"> for the
  // mermaid chunk, defeating the lazy-load and preloading ~640KB on startup.
  // Rollup's automatic code-splitting already splits mermaid/katex into
  // separate dynamic chunks since they are only reached via import().
})
