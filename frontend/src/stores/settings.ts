import {defineStore} from 'pinia'
import {ref, computed} from 'vue'
import {GetSettings, SaveSettings, ReadImageAsBase64, GetSystemFonts, GetThemeCSS} from '../../wailsjs/go/main/App'

export type Theme = 'light' | 'dark'

export interface ShortcutBindings {
  new_chat: string
  clear_context: string
  focus_input: string
  toggle_sidebar: string
  search_messages: string
}

const DEFAULT_SHORTCUTS: ShortcutBindings = {
  new_chat: 'ctrl+n',
  clear_context: 'ctrl+l',
  focus_input: '/',
  toggle_sidebar: 'ctrl+b',
  search_messages: 'ctrl+f',
}

// Local file paths are stored with this prefix to distinguish from URLs
const LOCAL_PREFIX = 'local://'

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Map<string, string>>(new Map())

  // Error state for settings loading
  const settingsError = ref<string | null>(null)

  const systemPrompt = computed(() => settings.value.get('system_prompt') || 'You are a helpful assistant.')
  const fontFamily = computed(() => settings.value.get('font_family') || '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif')
  const fontSize = computed(() => settings.value.get('font_size') || '14')
  const chatWidth = computed(() => settings.value.get('chat_width') || '768')
  const theme = computed<Theme>(() => (settings.value.get('theme') as Theme) || 'dark')
  const customStyles = computed(() => settings.value.get('custom_styles') || '')
  const bgImage = computed(() => settings.value.get('bg_image') || '')
  const bgOpacity = computed(() => settings.value.get('bg_opacity') || '0.15')
  const sidebarWidth = computed(() => settings.value.get('sidebar_width') || '350')
  const toolEnabled = computed(() => settings.value.get('tool_enabled') || '0')
  const toolFileRead = computed(() => settings.value.get('tool_file_read') || '0')
  const toolFileWrite = computed(() => settings.value.get('tool_file_write') || '0')
  const toolShellExec = computed(() => settings.value.get('tool_shell_exec') || '0')
  const notifyOnComplete = computed(() => settings.value.get('notify_on_complete') || '0')
  const showMessageTime = computed(() => settings.value.get('show_message_time') || '0')
  const selectedTheme = computed(() => settings.value.get('selected_theme') || 'Default')

  const shortcuts = computed<ShortcutBindings>(() => {
    const raw = settings.value.get('shortcuts')
    if (!raw) return { ...DEFAULT_SHORTCUTS }
    try {
      return { ...DEFAULT_SHORTCUTS, ...JSON.parse(raw) }
    } catch {
      return { ...DEFAULT_SHORTCUTS }
    }
  })

  // System fonts cache (loaded once, reused across Settings modal open/close)
  const systemFonts = ref<string[]>([])
  const fontsLoaded = ref(false)

  function truncateFontName(name: string): string {
    if (name.length <= 30) return name
    const idx = name.indexOf(' ', 20)
    if (idx !== -1 && idx <= 30) {
      return name.slice(0, idx) + '...'
    }
    return name.slice(0, 30) + '...'
  }

  async function loadSystemFonts() {
    if (fontsLoaded.value) return
    try {
      const fonts = await GetSystemFonts() as string[]
      if (fonts && fonts.length > 0) {
        systemFonts.value = fonts.map(f => truncateFontName(f))
      }
    } catch (e) {
      console.error('Failed to load system fonts:', e)
    } finally {
      fontsLoaded.value = true
    }
  }

  // Multi-file cache for local file data URIs with TTL (not stored in DB, only in memory)
  // Key: file path, Value: {dataUri, timestamp}
  const _imageCache = ref<Map<string, {dataUri: string, timestamp: number}>>(new Map())
  // In-flight loading promises for dedup — prevents duplicate ReadImageAsBase64 RPC calls
  const _bgLoading = new Map<string, Promise<string>>()
  const CACHE_TTL_MS = 5 * 60 * 1000 // 5 minutes

  const hasError = computed(() => settingsError.value !== null)

  async function fetchSettings() {
    try {
      const s = await GetSettings() as Record<string, string>
      settings.value = new Map(Object.entries(s))
      settingsError.value = null

      // If a custom theme is selected, reload CSS from disk (source of truth)
      const themeName = settings.value.get('selected_theme')
      if (themeName && themeName !== 'Default') {
        try {
          const css = await GetThemeCSS(themeName) as string
          settings.value.set('custom_styles', css)
        } catch (e) {
          console.error('Failed to reload theme from disk:', e)
        }
      }

      // Start background image preload immediately — don't wait for applyToDOM()
      // This fires ReadImageAsBase64 in parallel with fetchProviders/fetchSessions
      if (settings.value.get('bg_image')) {
        applyBackgroundImage()
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e)
      console.error('fetchSettings error:', msg)
      settingsError.value = msg
    }
  }

  async function retryFetchSettings() {
    settingsError.value = null
    await fetchSettings()
  }

  async function saveSettings(newSettings: Record<string, string>) {
    try {
      await SaveSettings(newSettings)
      for (const [k, v] of Object.entries(newSettings)) {
        settings.value.set(k, v)
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e)
      console.error('saveSettings error:', msg)
      throw new Error(`保存设置失败: ${msg}`)
    }
  }

  async function applyToDOM() {
    const root = document.documentElement.style
    root.setProperty('--chat-font-family', fontFamily.value)
    root.setProperty('--chat-font-size', fontSize.value + 'px')
    root.setProperty('--chat-width', chatWidth.value + 'px')

    // Additional theme CSS variables for custom styling
    root.setProperty('--user-bubble-bg', 'rgba(37, 99, 235, 0.5)')
    root.setProperty('--user-bubble-text', '#ffffff')
    root.setProperty('--ai-bubble-bg-light', 'rgba(241, 245, 249, 0.5)')
    root.setProperty('--ai-bubble-bg-dark', 'rgba(51, 65, 85, 0.5)')
    root.setProperty('--ai-bubble-text', 'inherit')
    root.setProperty('--sidebar-bg-light', 'rgba(255, 255, 255, 0.5)')
    root.setProperty('--sidebar-bg-dark', 'rgba(30, 41, 59, 0.5)')
    root.setProperty('--input-bg-light', '#ffffff')
    root.setProperty('--input-bg-dark', '#334155')
    root.setProperty('--input-border-light', '#cbd5e1')
    root.setProperty('--input-border-dark', '#475569')
    root.setProperty('--input-focus-border', '#3b82f6')
    root.setProperty('--send-btn-bg', '#2563eb')
    root.setProperty('--send-btn-hover-bg', '#1d4ed8')
    root.setProperty('--stop-btn-bg', '#dc2626')
    root.setProperty('--stop-btn-hover-bg', '#b91c1c')
    root.setProperty('--new-chat-btn-bg', '#2563eb')
    root.setProperty('--new-chat-btn-hover-bg', '#1d4ed8')
    root.setProperty('--session-item-hover-bg-light', 'rgba(37, 99, 235, 0.1)')
    root.setProperty('--session-item-hover-bg-dark', 'rgba(100, 116, 139, 0.2)')
    root.setProperty('--header-bg-light', 'rgba(255, 255, 255, 0.5)')
    root.setProperty('--header-bg-dark', 'rgba(30, 41, 59, 0.5)')
    root.setProperty('--border-color-light', '#e2e8f0')
    root.setProperty('--border-color-dark', '#334155')
    root.setProperty('--text-primary-light', '#1e293b')
    root.setProperty('--text-primary-dark', '#e2e8f0')
    root.setProperty('--text-secondary-light', '#64748b')
    root.setProperty('--text-secondary-dark', '#94a3b8')
    root.setProperty('--shadow-sm', '0 1px 2px 0 rgb(0 0 0 / 0.05)')
    root.setProperty('--shadow-md', '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)')
    root.setProperty('--shadow-lg', '0 10px 15px -3px rgb(0 0 0 / 0.1), 0 4px 6px -4px rgb(0 0 0 / 0.1)')
    root.setProperty('--radius-sm', '0.25rem')
    root.setProperty('--radius-md', '0.5rem')
    root.setProperty('--radius-lg', '0.75rem')
    root.setProperty('--radius-xl', '1rem')
    root.setProperty('--transition-speed', '150ms')

    // Toggle dark class on <html> for Tailwind dark mode
    document.documentElement.classList.toggle('dark', theme.value === 'dark')

    // Inject custom styles with highest priority
    applyCustomStyles()

    // Apply background image (async for local files) - wait for it to complete
    await applyBackgroundImage()
  }

  function applyCustomStyles() {
    const css = customStyles.value
    let el = document.getElementById('custom-user-styles')
    if (!css) {
      if (el) el.remove()
      return
    }
    if (!el) {
      el = document.createElement('style')
      el.id = 'custom-user-styles'
      document.head.appendChild(el)
    }
    el.textContent = css
  }

  async function applyBackgroundImage(forceRefresh: boolean = false, newImagePath?: string) {
    // Always remove existing style element first to ensure CSS is re-applied
    const existingEl = document.getElementById('bg-image-styles')
    if (existingEl) {
      existingEl.remove()
    }

    // Use provided path if given, otherwise use reactive value
    const raw = newImagePath !== undefined ? newImagePath : bgImage.value
    const opacity = parseFloat(bgOpacity.value) || 0.15
    if (!raw) {
      return
    }

    let cssUrl: string
    if (raw.startsWith(LOCAL_PREFIX)) {
      // Local file — convert to data URI
      const filePath = raw.slice(LOCAL_PREFIX.length)

      // When forceRefresh or newImagePath is set, skip cache
      const cached = _imageCache.value.get(filePath)
      if (!forceRefresh && cached && (Date.now() - cached.timestamp) < CACHE_TTL_MS) {
        cssUrl = cached.dataUri
      } else {
        // Clear any existing loading promise to force fresh load
        _bgLoading.delete(filePath)
        try {
          const dataUri = await ReadImageAsBase64(filePath) as string
          _imageCache.value.set(filePath, { dataUri, timestamp: Date.now() })
          cssUrl = dataUri
        } catch (e) {
          console.error('Failed to read local background image:', e)
          return
        }
      }
    } else {
      // URL or data URI — use directly
      cssUrl = raw
    }

    // Create new style element and append to head
    const el = document.createElement('style')
    el.id = 'bg-image-styles'
    el.textContent = `
#app > div.flex.h-screen {
  position: relative;
  z-index: 0;
}
#app > div.flex.h-screen::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  background-image: url('${cssUrl}');
  background-size: cover;
  background-position: center;
  opacity: ${opacity};
  pointer-events: none;
}`
    document.head.appendChild(el)
  }

  async function setTheme(t: Theme) {
    await saveSettings({theme: t})
    await applyToDOM()
  }

  // Clear cache for a specific background image path
  function clearBgImageCache(filePath: string) {
    _imageCache.value.delete(filePath)
    _bgLoading.delete(filePath)
  }

  return {
    settings, settingsError, hasError, systemPrompt, fontFamily, fontSize, chatWidth, theme, customStyles,
    bgImage, bgOpacity, sidebarWidth, shortcuts, toolEnabled, toolFileRead, toolFileWrite, toolShellExec, notifyOnComplete, showMessageTime, selectedTheme,
    fetchSettings, retryFetchSettings, saveSettings, applyToDOM, applyCustomStyles, applyBackgroundImage, setTheme,
    systemFonts, fontsLoaded, loadSystemFonts, clearBgImageCache,
  }
})
