<script lang="ts" setup>
import {ref, reactive, onMounted, onBeforeUnmount, computed, nextTick} from 'vue'
import {useProviderStore, type Provider} from '../stores/provider'
import {usePromptStore, type Prompt} from '../stores/prompt'
import {useSettingsStore, type Theme, type ShortcutBindings} from '../stores/settings'
import {GetModels, GetDefaultStyles, OpenFileDialog, GetThemes, GetThemeCSS, SaveThemeCSS, OpenThemeFolder} from '../../wailsjs/go/main/App'
import {model} from '../../wailsjs/go/models'
import {Server, Settings2, MessageSquare, Palette, Keyboard, Plug} from 'lucide-vue-next'

const emit = defineEmits<{close: []}>()

const providerStore = useProviderStore()
const promptStore = usePromptStore()
const settingsStore = useSettingsStore()

const activeTab = ref<'general' | 'providers' | 'prompt' | 'styles' | 'shortcuts' | 'mcp'>('general')

// Track if user is dragging to select text (to prevent accidental close)
const isDragging = ref(false)

function onMouseDown() {
  isDragging.value = false
}

function onMouseMove() {
  isDragging.value = true
}

function onOverlayClick(e: MouseEvent) {
  // Only close if not dragging to select text
  if (!isDragging.value) {
    emit('close')
  }
}

// --- General settings state ---
const fontFamily = ref(settingsStore.fontFamily)
const fontSize = ref(Number(settingsStore.fontSize))
const chatWidth = ref(Number(settingsStore.chatWidth))
const selectedTheme = ref<Theme>(settingsStore.theme)
const bgImage = ref(settingsStore.bgImage)
const bgOpacity = ref(Number(settingsStore.bgOpacity))
const toolEnabled = ref(settingsStore.toolEnabled === '1' || settingsStore.toolEnabled === 'true')
const toolFileRead = ref(settingsStore.toolFileRead === '1' || settingsStore.toolFileRead === 'true')
const toolFileWrite = ref(settingsStore.toolFileWrite === '1' || settingsStore.toolFileWrite === 'true')
const toolShellExec = ref(settingsStore.toolShellExec === '1' || settingsStore.toolShellExec === 'true')
const toolProvideSelection = ref(settingsStore.toolProvideSelection === '1' || settingsStore.toolProvideSelection === 'true')
const notifyOnComplete = ref(settingsStore.notifyOnComplete === '1' || settingsStore.notifyOnComplete === 'true')
const showMessageTime = ref(settingsStore.showMessageTime === '1' || settingsStore.showMessageTime === 'true')
const generalSaved = ref(false)

// System fonts (from cached store)
const systemFonts = computed(() => settingsStore.systemFonts)
const fontsLoading = computed(() => !settingsStore.fontsLoaded)
const fontDropdownOpen = ref(false)
const fontSearch = ref('')
const fontDropdownRef = ref<HTMLElement | null>(null)

const SYSTEM_DEFAULT = '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'

const filteredFonts = computed(() => {
  const q = fontSearch.value.trim().toLowerCase()
  if (!q) return systemFonts.value
  return systemFonts.value.filter(f => f.toLowerCase().includes(q))
})

// MCP form validity check
const isMCPServerValid = computed(() => {
  if (!mcpName.value.trim()) return false
  if (mcpTransport.value === 'stdio' && !mcpCommand.value.trim()) return false
  if (mcpTransport.value === 'http' && !mcpURL.value.trim()) return false
  return true
})

function selectFont(f: string) {
  fontFamily.value = f
  fontDropdownOpen.value = false
  fontSearch.value = ''
}

const currentFontLabel = computed(() => {
  if (fontFamily.value === SYSTEM_DEFAULT) return 'System Default'
  return systemFonts.value.find(f => f === fontFamily.value) || fontFamily.value
})

// Close dropdown on outside click
function onFontClickOutside(e: MouseEvent) {
  if (fontDropdownRef.value && !fontDropdownRef.value.contains(e.target as Node)) {
    fontDropdownOpen.value = false
    fontSearch.value = ''
  }
}
onMounted(() => document.addEventListener('click', onFontClickOutside))
onBeforeUnmount(() => document.removeEventListener('click', onFontClickOutside))

// --- Prompt management state ---
const promptList = ref<Prompt[]>([])
const promptLoading = ref(false)
const promptShowForm = ref(false)
const promptEditingId = ref<number | null>(null)
const promptName = ref('')
const promptCategory = ref('')
const promptContent = ref('')
const promptIsDefault = ref(false)
const promptSaving = ref(false)

async function loadPrompts() {
  // If store already has data from App.vue preload, use it directly (instant)
  if (promptStore.prompts.length > 0) {
    promptList.value = promptStore.prompts
    return
  }
  promptLoading.value = true
  try {
    await promptStore.fetchPrompts()
    promptList.value = promptStore.prompts
  } catch (e) {
    console.error('Failed to load prompts:', e)
  } finally {
    promptLoading.value = false
  }
}

function openPromptAdd() {
  promptEditingId.value = null
  promptName.value = ''
  promptCategory.value = ''
  promptContent.value = ''
  promptIsDefault.value = promptList.value.length === 0
  promptShowForm.value = true
}

function openPromptEdit(p: Prompt) {
  promptEditingId.value = p.id
  promptName.value = p.name
  promptCategory.value = p.category
  promptContent.value = p.content
  promptIsDefault.value = p.is_default
  promptShowForm.value = true
}

async function savePromptItem() {
  if (promptSaving.value || !promptName.value.trim() || !promptContent.value.trim()) return
  promptSaving.value = true
  try {
    if (promptEditingId.value) {
      const p = new model.Prompt({
        id: promptEditingId.value,
        name: promptName.value.trim(),
        content: promptContent.value,
        category: promptCategory.value.trim(),
        is_default: promptIsDefault.value,
        sort_order: 0,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      })
      await promptStore.updatePrompt(p)
    } else {
      await promptStore.createPrompt({
        name: promptName.value.trim(),
        content: promptContent.value,
        category: promptCategory.value.trim(),
        is_default: promptIsDefault.value,
      })
    }
    promptList.value = promptStore.prompts
    promptShowForm.value = false
  } catch (e) {
    console.error('Failed to save prompt:', e)
  } finally {
    promptSaving.value = false
  }
}

async function deletePromptItem(id: number) {
  try {
    await promptStore.deletePrompt(id)
    promptList.value = promptStore.prompts
  } catch (e) {
    console.error('Failed to delete prompt:', e)
  }
}

async function setPromptDefault(id: number) {
  try {
    await promptStore.setDefault(id)
    promptList.value = promptStore.prompts
  } catch (e) {
    console.error('Failed to set default prompt:', e)
  }
}

function truncateContent(content: string, maxLen = 100): string {
  if (!content) return ''
  const text = content.replace(/\n/g, ' ').trim()
  return text.length > maxLen ? text.slice(0, maxLen) + '...' : text
}

// --- Styles settings state ---
const stylesCSS = ref('')
const defaultCSS = ref('')
const stylesLoading = ref(false)
const stylesSaved = ref(false)

// --- Theme selector state ---
interface ThemeInfo { name: string; isDefault: boolean; css: string }
const themes = ref<ThemeInfo[]>([])
const currentTheme = ref<string>('Default')
const themesLoading = ref(false)

async function loadThemes() {
  themesLoading.value = true
  themeColorCache.value.clear()
  try {
    themes.value = await GetThemes() as ThemeInfo[]
    currentTheme.value = settingsStore.selectedTheme || 'Default'
  } catch (e) {
    console.error('Failed to load themes:', e)
  } finally {
    themesLoading.value = false
  }
}

async function selectTheme(themeName: string) {
  currentTheme.value = themeName
  try {
    const css = await GetThemeCSS(themeName) as string
    stylesCSS.value = css
  } catch (e) {
    console.error('Failed to load theme CSS:', e)
  }
}

async function openThemeFolderAction() {
  try {
    await OpenThemeFolder()
  } catch (e) {
    console.error('Failed to open theme folder:', e)
  }
}

async function refreshThemes() {
  await loadThemes()
}

// Cache extracted theme colors to avoid re-parsing CSS on every render
const themeColorCache = ref<Map<string, ThemeColors>>(new Map())

function getThemeColors(themeName: string, css: string): ThemeColors {
  const cached = themeColorCache.value.get(themeName)
  if (cached) return cached
  const colors = extractThemeColors(css, themeName)
  themeColorCache.value.set(themeName, colors)
  return colors
}

interface ThemeColors {
  bodyBg: string
  bodyText: string
  sidebarBg: string
  headerBg: string
  userBubble: string
  aiBubble: string
  accent: string
  inputBg: string
  borderColor: string
}

const DEFAULT_LIGHT_COLORS: ThemeColors = {
  bodyBg: '#f8fafc',
  bodyText: '#1e293b',
  sidebarBg: '#f1f5f9',
  headerBg: '#ffffff',
  userBubble: 'rgba(37, 99, 235, 0.5)',
  aiBubble: 'rgba(241, 245, 249, 0.5)',
  accent: '#2563eb',
  inputBg: '#ffffff',
  borderColor: '#e2e8f0',
}

function extractFirstColor(css: string, selectors: string[], props: string[]): string | null {
  for (const sel of selectors) {
    const escaped = sel.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    // Match selector block — handle both single-line and multi-line
    const blockRe = new RegExp(escaped + '\\s*\\{([^}]*)\\}', 's')
    const blockMatch = css.match(blockRe)
    if (blockMatch) {
      for (const prop of props) {
        const propRe = new RegExp(prop + '\\s*:\\s*([^;}]+)', 'i')
        const propMatch = blockMatch[1].match(propRe)
        if (propMatch) {
          const val = propMatch[1].trim()
          if (val.startsWith('var(')) {
            const fb = val.match(/var\([^,]+,\s*([^)]+)\)/)
            if (fb) return fb[1].trim()
            continue
          }
          // For background shorthand, extract the first color token
          if (prop === 'background' || prop === 'background-image') {
            const colorToken = val.match(/(#[0-9a-fA-F]{3,8}|rgba?\([^)]+\)|hsla?\([^)]+\))/)
            if (colorToken) return colorToken[1]
            continue
          }
          return val
        }
      }
    }
  }
  return null
}

// Generate a deterministic hue from a string (0-360)
function nameToHue(name: string): number {
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return ((hash % 360) + 360) % 360
}

// Build a full color palette from a single hue
function hueToPalette(hue: number): ThemeColors {
  const h = hue
  return {
    bodyBg: `hsl(${h}, 15%, 96%)`,
    bodyText: `hsl(${h}, 20%, 15%)`,
    sidebarBg: `hsl(${h}, 30%, 20%)`,
    headerBg: `hsl(${h}, 15%, 98%)`,
    userBubble: `hsla(${(h + 200) % 360}, 65%, 55%, 0.5)`,
    aiBubble: `hsla(${h}, 15%, 92%, 0.5)`,
    accent: `hsl(${(h + 200) % 360}, 70%, 50%)`,
    inputBg: `hsl(${h}, 15%, 99%)`,
    borderColor: `hsl(${h}, 10%, 88%)`,
  }
}

function extractThemeColors(css: string, themeName: string): ThemeColors {
  const props = ['background-color', 'background']
  const c = { ...DEFAULT_LIGHT_COLORS }
  let extracted = 0

  // Body background
  const bodyBg = extractFirstColor(css, ['html, body', 'html body', 'body', 'html', ':root'], props)
  if (bodyBg) { c.bodyBg = bodyBg; extracted++ }

  // Body text color
  const bodyText = extractFirstColor(css, ['html, body', 'html body', 'body', 'html', ':root'], ['color'])
  if (bodyText) { c.bodyText = bodyText; extracted++ }

  // Dark / sidebar background — dark mode body or sidebar-specific
  const darkBg = extractFirstColor(css,
    ['html.dark, html.dark body', 'html.dark body', 'html.dark', '.sidebar-container', '.sidebar'],
    props)
  if (darkBg) { c.sidebarBg = darkBg; extracted++ }

  // Chat header
  const headerBg = extractFirstColor(css, ['.chat-header', '.header'], props)
  if (headerBg) { c.headerBg = headerBg; extracted++ }

  // User bubble
  const userBubble = extractFirstColor(css, ['.user-bubble', '.message.user', '.msg-user'], props)
  if (userBubble) { c.userBubble = userBubble; extracted++ }

  // AI bubble
  const aiBubble = extractFirstColor(css, ['.ai-bubble', '.message.assistant', '.msg-ai'], props)
  if (aiBubble) { c.aiBubble = aiBubble; extracted++ }

  // Accent color (buttons, highlights)
  const accent = extractFirstColor(css, ['.send-btn', '.new-chat-btn', '.accent', 'button'], props)
  if (accent) { c.accent = accent; extracted++ }

  // Input background
  const inputBg = extractFirstColor(css, ['.input-textarea', '.chat-input-area', '.chat-input', 'textarea'], props)
  if (inputBg) { c.inputBg = inputBg; extracted++ }

  // Border color
  const border = extractFirstColor(css, ['.chat-input-area', '.chat-header', '.border', '*'], ['border-color', 'border'])
  if (border) { c.borderColor = border; extracted++ }

  // If extraction found very little, use name-based palette as primary colors
  // so each custom theme is visually distinct even with minimal CSS
  if (extracted < 3 && themeName !== 'Default') {
    const palette = hueToPalette(nameToHue(themeName))
    // Only override colors that weren't successfully extracted
    if (extracted === 0) return palette
    // Partial: blend extracted with name-based palette
    if (!bodyBg) c.bodyBg = palette.bodyBg
    if (!bodyText) c.bodyText = palette.bodyText
    if (!darkBg) c.sidebarBg = palette.sidebarBg
    if (!headerBg) c.headerBg = palette.headerBg
    if (!userBubble) c.userBubble = palette.userBubble
    if (!aiBubble) c.aiBubble = palette.aiBubble
    if (!accent) c.accent = palette.accent
    if (!inputBg) c.inputBg = palette.inputBg
    if (!border) c.borderColor = palette.borderColor
  }

  return c
}

// --- Shortcuts settings state ---
const shortcutBindings = ref<ShortcutBindings>({ ...settingsStore.shortcuts })
const recordingKey = ref<string | null>(null) // which shortcut is being recorded
const shortcutsSaved = ref(false)

const shortcutList: { key: keyof ShortcutBindings; label: string; description: string }[] = [
  { key: 'new_chat', label: 'New Chat', description: 'Create a new chat session' },
  { key: 'clear_context', label: 'Clear Context', description: 'Clear all messages in current chat' },
  { key: 'focus_input', label: 'Focus Input', description: 'Focus the chat input box' },
  { key: 'toggle_sidebar', label: 'Toggle Sidebar', description: 'Show or hide the sidebar' },
  { key: 'search_messages', label: 'Search Messages', description: 'Search within the current chat' },
]

// --- MCP Servers settings state ---
const mcpServers = ref<model.MCPServer[]>([])
const mcpLoading = ref(false)
const mcpShowForm = ref(false)
const mcpEditingId = ref<string | null>(null)
const mcpName = ref('')
const mcpTransport = ref<'stdio' | 'http'>('stdio')
const mcpCommand = ref('')
const mcpURL = ref('')
const mcpEnabled = ref(true)
const mcpAuthToken = ref('')
const mcpEnvVars = ref<{key: string, value: string}[]>([])
const mcpNewEnvKey = ref('')
const mcpNewEnvValue = ref('')
const mcpTestResult = ref<model.MCPServerTestResult | null>(null)
const mcpTesting = ref(false)
const mcpSaving = ref(false)

// MCP connection status (reactive for deep property tracking)
const mcpConnectionStatus = reactive<Record<string, 'connected' | 'disconnected' | 'connecting' | 'disconnecting' | 'error'>>({})
const mcpConnectionError = reactive<Record<string, string>>({})

// MCP form validation
const mcpValidationError = ref<string | null>(null)

function validateMCPForm(): boolean {
  mcpValidationError.value = null

  if (!mcpName.value.trim()) {
    mcpValidationError.value = 'Server name is required'
    return false
  }

  if (mcpTransport.value === 'stdio' && !mcpCommand.value.trim()) {
    mcpValidationError.value = 'Command is required for stdio transport'
    return false
  }

  if (mcpTransport.value === 'http' && !mcpURL.value.trim()) {
    mcpValidationError.value = 'URL is required for HTTP transport'
    return false
  }

  if (mcpTransport.value === 'http' && mcpURL.value.trim()) {
    try {
      new URL(mcpURL.value.trim())
    } catch {
      mcpValidationError.value = 'Please enter a valid URL'
      return false
    }
  }

  return true
}

// Load MCP servers from backend
async function loadMCPServers() {
  mcpLoading.value = true
  try {
    const {MCPServerList} = await import('../../wailsjs/go/main/App')
    mcpServers.value = await MCPServerList() as model.MCPServer[]
  } catch (e) {
    console.error('Failed to load MCP servers:', e)
  } finally {
    mcpLoading.value = false
  }
}

// Load connection statuses for all servers
async function loadMCPServerStatuses() {
  try {
    const {MCPServerGetAllStatuses} = await import('../../wailsjs/go/main/App')
    const statuses = await MCPServerGetAllStatuses() as Record<string, 'connected' | 'disconnected' | 'connecting' | 'error'>
    // Merge into reactive object (clear stale keys first)
    Object.keys(mcpConnectionStatus).forEach(k => delete mcpConnectionStatus[k])
    Object.assign(mcpConnectionStatus, statuses)
  } catch (e) {
    console.error('Failed to load MCP server statuses:', e)
  }
}

// Connect to an MCP server — only updates status, never reloads the list
async function connectMCPServer(id: string) {
  mcpConnectionStatus[id] = 'connecting'
  mcpConnectionError[id] = ''
  try {
    const {MCPServerConnect} = await import('../../wailsjs/go/main/App')
    await MCPServerConnect(id)
    mcpConnectionStatus[id] = 'connected'
  } catch (e: any) {
    mcpConnectionStatus[id] = 'error'
    mcpConnectionError[id] = e.toString()
  }
}

// Disconnect from an MCP server — only updates status, never reloads the list
async function disconnectMCPServer(id: string) {
  mcpConnectionStatus[id] = 'disconnecting'
  try {
    const {MCPServerDisconnect} = await import('../../wailsjs/go/main/App')
    await MCPServerDisconnect(id)
    mcpConnectionStatus[id] = 'disconnected'
  } catch (e) {
    mcpConnectionStatus[id] = 'error'
    mcpConnectionError[id] = e instanceof Error ? e.message : String(e)
  }
}

function openMCPAdd() {
  mcpEditingId.value = null
  mcpName.value = ''
  mcpTransport.value = 'stdio'
  mcpCommand.value = ''
  mcpURL.value = ''
  mcpEnabled.value = true
  mcpAuthToken.value = ''
  mcpEnvVars.value = []
  mcpTestResult.value = null
  mcpValidationError.value = null
  mcpShowForm.value = true
}

function openMCPEdit(server: model.MCPServer) {
  mcpEditingId.value = server.id
  mcpName.value = server.name
  mcpTransport.value = server.transport as 'stdio' | 'http'
  mcpCommand.value = server.command || ''
  mcpURL.value = server.url || ''
  mcpEnabled.value = server.enabled
  mcpAuthToken.value = server.auth_token || ''
  mcpEnvVars.value = Object.entries(server.env || {}).map(([key, value]) => ({key, value}))
  mcpTestResult.value = null
  mcpValidationError.value = null
  mcpShowForm.value = true
}

function addMCPEnvVar() {
  if (mcpNewEnvKey.value.trim()) {
    mcpEnvVars.value.push({key: mcpNewEnvKey.value.trim(), value: mcpNewEnvValue.value})
    mcpNewEnvKey.value = ''
    mcpNewEnvValue.value = ''
  }
}

function removeMCPEnvVar(index: number) {
  mcpEnvVars.value.splice(index, 1)
}

async function saveMCPServer() {
  if (mcpSaving.value) return
  if (!validateMCPForm()) return

  mcpSaving.value = true

  const env: Record<string, string> = {}
  for (const {key, value} of mcpEnvVars.value) {
    if (key.trim()) env[key.trim()] = value
  }

  const server = new model.MCPServer({
    id: mcpEditingId.value || crypto.randomUUID(),
    name: mcpName.value.trim(),
    transport: mcpTransport.value,
    command: mcpTransport.value === 'stdio' ? mcpCommand.value : '',
    url: mcpTransport.value === 'http' ? mcpURL.value : '',
    env,
    enabled: mcpEnabled.value,
    auth_token: mcpAuthToken.value,
    created_at: new Date().toISOString(),
  })

  try {
    const {MCPServerCreate, MCPServerUpdate} = await import('../../wailsjs/go/main/App')
    if (mcpEditingId.value) {
      await MCPServerUpdate(server)
    } else {
      await MCPServerCreate(server)
    }
    await loadMCPServers()
    await loadMCPServerStatuses()
    mcpShowForm.value = false
  } catch (e) {
    console.error('Failed to save MCP server:', e)
  } finally {
    mcpSaving.value = false
  }
}

async function deleteMCPServer(id: string) {
  try {
    const {MCPServerDelete} = await import('../../wailsjs/go/main/App')
    await MCPServerDelete(id)
    // Clean up status for deleted server
    delete mcpConnectionStatus[id]
    delete mcpConnectionError[id]
    await loadMCPServers()
  } catch (e) {
    console.error('Failed to delete MCP server:', e)
  }
}

async function testMCPServer() {
  const env: Record<string, string> = {}
  for (const {key, value} of mcpEnvVars.value) {
    if (key.trim()) env[key.trim()] = value
  }

  const server = new model.MCPServer({
    id: mcpEditingId.value || crypto.randomUUID(),
    name: mcpName.value.trim(),
    transport: mcpTransport.value,
    command: mcpTransport.value === 'stdio' ? mcpCommand.value : '',
    url: mcpTransport.value === 'http' ? mcpURL.value : '',
    env,
    enabled: mcpEnabled.value,
    auth_token: mcpAuthToken.value,
    created_at: new Date().toISOString(),
  })

  mcpTesting.value = true
  mcpTestResult.value = null
  try {
    const {MCPServerTest} = await import('../../wailsjs/go/main/App')
    mcpTestResult.value = await MCPServerTest(server) as model.MCPServerTestResult
  } catch (e: any) {
    mcpTestResult.value = new model.MCPServerTestResult({success: false, error: e.toString()})
  } finally {
    mcpTesting.value = false
  }
}

// Load MCP servers when component mounts
onMounted(() => {
  // All three are independent — load in parallel
  loadPrompts()
  loadMCPServers()
  loadMCPServerStatuses()
})

function formatShortcut(binding: string): string {
  return binding
    .replace('ctrl+', 'Ctrl+')
    .replace('alt+', 'Alt+')
    .replace('shift+', 'Shift+')
    .replace('meta+', 'Meta+')
    .toUpperCase()
}

function startRecording(key: string) {
  recordingKey.value = key
}

function cancelRecording() {
  recordingKey.value = null
}

function handleShortcutKeydown(e: KeyboardEvent) {
  if (!recordingKey.value) return
  e.preventDefault()
  e.stopPropagation()

  // Escape to cancel
  if (e.key === 'Escape') {
    recordingKey.value = null
    return
  }

  // Build binding string
  const parts: string[] = []
  if (e.ctrlKey) parts.push('ctrl')
  if (e.altKey) parts.push('alt')
  if (e.shiftKey) parts.push('shift')
  if (e.metaKey) parts.push('meta')

  const key = e.key.toLowerCase()
  // Ignore lone modifier presses
  if (['control', 'alt', 'shift', 'meta'].includes(key)) return

  parts.push(key)
  const binding = parts.join('+')
  shortcutBindings.value[recordingKey.value as keyof ShortcutBindings] = binding
  recordingKey.value = null
}

async function saveGeneral() {
  // Check if bg_image changed - if so, clear its cache to force refresh
  const oldBgImage = settingsStore.bgImage
  const bgImageChanged = oldBgImage !== bgImage.value
  if (bgImageChanged) {
    // Clear cache for old image if it was a local file
    if (oldBgImage.startsWith(LOCAL_PREFIX)) {
      const oldPath = oldBgImage.slice(LOCAL_PREFIX.length)
      settingsStore.clearBgImageCache(oldPath)
    }
    // Clear cache for new image if it was a local file
    if (bgImage.value.startsWith(LOCAL_PREFIX)) {
      const newPath = bgImage.value.slice(LOCAL_PREFIX.length)
      settingsStore.clearBgImageCache(newPath)
    }
  }

  await settingsStore.saveSettings({
    font_family: fontFamily.value,
    font_size: String(fontSize.value),
    chat_width: String(chatWidth.value),
    theme: selectedTheme.value,
    bg_image: bgImage.value,
    bg_opacity: String(bgOpacity.value),
    tool_enabled: toolEnabled.value ? '1' : '0',
    tool_file_read: toolFileRead.value ? '1' : '0',
    tool_file_write: toolFileWrite.value ? '1' : '0',
    tool_shell_exec: toolShellExec.value ? '1' : '0',
    tool_provide_selection: toolProvideSelection.value ? '1' : '0',
    notify_on_complete: notifyOnComplete.value ? '1' : '0',
    show_message_time: showMessageTime.value ? '1' : '0',
  })

  // Force refresh background image if it changed
  if (bgImageChanged) {
    await settingsStore.applyBackgroundImage(true, bgImage.value)
  }

  // Apply font and other style changes immediately
  await settingsStore.applyToDOM()

  generalSaved.value = true
  setTimeout(() => { generalSaved.value = false }, 1000)
}

async function saveShortcuts() {
  await settingsStore.saveSettings({
    shortcuts: JSON.stringify(shortcutBindings.value),
  })
  shortcutsSaved.value = true
  setTimeout(() => { shortcutsSaved.value = false }, 2000)
}

function resetShortcuts() {
  shortcutBindings.value = { new_chat: 'ctrl+n', clear_context: 'ctrl+l', focus_input: '/', toggle_sidebar: 'ctrl+b', search_messages: 'ctrl+f' }
}

const LOCAL_PREFIX = 'local://'

// Display value for the input field: strips local:// prefix to show clean path
const bgImageDisplay = computed({
  get: () => bgImage.value.startsWith(LOCAL_PREFIX) ? bgImage.value.slice(LOCAL_PREFIX.length) : bgImage.value,
  set: (v: string) => { bgImage.value = v }
})

async function browseImage() {
  try {
    const path = await OpenFileDialog() as string
    if (path) {
      bgImage.value = LOCAL_PREFIX + path
    }
  } catch (e) {
    console.error('File dialog error:', e)
  }
}

onMounted(async () => {
  stylesLoading.value = true
  // Sync shortcuts from store
  shortcutBindings.value = { ...settingsStore.shortcuts }
  // Preload fonts if not already cached
  settingsStore.loadSystemFonts()

  try {
    defaultCSS.value = await GetDefaultStyles() as string
    // Use saved custom styles if they exist, otherwise show default
    stylesCSS.value = settingsStore.customStyles || defaultCSS.value
  } catch (e) {
    console.error('Failed to load default styles:', e)
  } finally {
    stylesLoading.value = false
  }

  // Load theme list after initial styles are ready
  await loadThemes()
})

async function saveStyles() {
  await settingsStore.saveSettings({
    custom_styles: stylesCSS.value,
    selected_theme: currentTheme.value,
  })
  // If editing a custom theme, also save to disk
  if (currentTheme.value !== 'Default') {
    try {
      await SaveThemeCSS(currentTheme.value, stylesCSS.value)
    } catch (e) {
      console.error('Failed to save theme file:', e)
    }
  }
  settingsStore.applyCustomStyles()
  stylesSaved.value = true
  setTimeout(() => { stylesSaved.value = false }, 2000)
}

function resetStyles() {
  stylesCSS.value = defaultCSS.value
  currentTheme.value = 'Default'
}

// --- Provider management state ---
const showForm = ref(false)
const editingId = ref<number | null>(null)
const name = ref('')
const apiKey = ref('')
const baseURL = ref('https://api.openai.com')
const modelList = ref<string[]>([])
const isDefault = ref(false)
const testResult = ref<string | null>(null)
const testing = ref(false)
const newModelInput = ref('')

// Get Models state
const showModelFetcher = ref(false)
const fetchingModels = ref(false)
const fetchedModels = ref<string[]>([])
const fetchError = ref<string | null>(null)
const modelFilter = ref('')

const filteredFetchedModels = computed(() => {
  const prefix = modelFilter.value.trim().toLowerCase()
  if (!prefix) return fetchedModels.value
  return fetchedModels.value.filter(m => m.toLowerCase().includes(prefix))
})

function openAdd() {
  editingId.value = null
  name.value = ''
  apiKey.value = ''
  baseURL.value = 'https://api.openai.com'
  modelList.value = []
  isDefault.value = providerStore.providers.length === 0
  testResult.value = null
  showForm.value = true
}

function openEdit(p: Provider) {
  editingId.value = p.id
  name.value = p.name
  apiKey.value = p.api_key
  baseURL.value = p.base_url
  modelList.value = [...p.models]
  isDefault.value = p.is_default
  testResult.value = null
  showForm.value = true
}

function addModel() {
  const m = newModelInput.value.trim()
  if (m && !modelList.value.includes(m)) {
    modelList.value.push(m)
  }
  newModelInput.value = ''
}

function removeModel(index: number) {
  modelList.value.splice(index, 1)
}

async function fetchAvailableModels() {
  if (!baseURL.value.trim() || !apiKey.value.trim()) {
    fetchError.value = 'Please fill in Base URL and API Key first'
    return
  }
  fetchingModels.value = true
  fetchError.value = null
  fetchedModels.value = []
  modelFilter.value = ''
  showModelFetcher.value = true
  try {
    const models = await GetModels(baseURL.value.trim(), apiKey.value.trim()) as string[]
    fetchedModels.value = (models || []).sort()
  } catch (e: any) {
    fetchError.value = e.toString()
  } finally {
    fetchingModels.value = false
  }
}

function addFetchedModel(m: string) {
  if (!modelList.value.includes(m)) {
    modelList.value.push(m)
  }
}

function addAllFetchedModels() {
  for (const m of fetchedModels.value) {
    if (!modelList.value.includes(m)) {
      modelList.value.push(m)
    }
  }
  showModelFetcher.value = false
}

async function save() {
  if (!name.value.trim() || !apiKey.value.trim() || !baseURL.value.trim() || modelList.value.length === 0) return

  if (editingId.value) {
    await providerStore.updateProvider(editingId.value, name.value.trim(), apiKey.value.trim(), baseURL.value.trim(), modelList.value, isDefault.value)
  } else {
    await providerStore.addProvider(name.value.trim(), apiKey.value.trim(), baseURL.value.trim(), modelList.value, isDefault.value)
  }
  showForm.value = false
}

async function test() {
  if (!baseURL.value.trim() || !apiKey.value.trim() || modelList.value.length === 0) {
    testResult.value = 'Please fill in base URL, API key, and at least one model'
    return
  }
  testing.value = true
  testResult.value = null
  const err = await providerStore.testConnection(baseURL.value.trim(), apiKey.value.trim(), modelList.value[0])
  testResult.value = err || 'Connection successful!'
  testing.value = false
}

async function remove(id: number) {
  await providerStore.deleteProvider(id)
}
</script>

<template>
  <div
    class="settings-modal-overlay fixed inset-0 z-50 flex items-center justify-center bg-black/30 dark:bg-black/50"
    @click.self="onOverlayClick"
    @mousedown="onMouseDown"
    @mousemove="onMouseMove"
  >
    <div class="settings-modal-content bg-white dark:bg-slate-800 rounded-xl w-[900px] h-[700px] max-h-[90vh] border border-slate-200 dark:border-slate-600 flex flex-col shadow-xl">
      <!-- Header -->
      <div class="flex items-center justify-between p-4 border-b border-slate-200 dark:border-slate-700">
        <div class="flex gap-1">
          <button
            @click="activeTab = 'general'; showForm = false"
            :class="['px-3 py-1.5 rounded-lg text-sm font-medium transition-colors inline-flex items-center gap-1.5',
              activeTab === 'general'
                ? 'bg-slate-200 dark:bg-slate-600 text-slate-800 dark:text-white'
                : 'text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-white']"
          >
            <Settings2 class="w-4 h-4" />
            General
          </button>
          <button
            @click="activeTab = 'providers'; showForm = false"
            :class="['px-3 py-1.5 rounded-lg text-sm font-medium transition-colors inline-flex items-center gap-1.5',
              activeTab === 'providers'
                ? 'bg-slate-200 dark:bg-slate-600 text-slate-800 dark:text-white'
                : 'text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-white']"
          >
            <Server class="w-4 h-4" />
            API Providers
          </button>
          <button
            @click="activeTab = 'prompt'; showForm = false"
            :class="['px-3 py-1.5 rounded-lg text-sm font-medium transition-colors inline-flex items-center gap-1.5',
              activeTab === 'prompt'
                ? 'bg-slate-200 dark:bg-slate-600 text-slate-800 dark:text-white'
                : 'text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-white']"
          >
            <MessageSquare class="w-4 h-4" />
            Prompt
          </button>
          <button
            @click="activeTab = 'styles'; showForm = false"
            :class="['px-3 py-1.5 rounded-lg text-sm font-medium transition-colors inline-flex items-center gap-1.5',
              activeTab === 'styles'
                ? 'bg-slate-200 dark:bg-slate-600 text-slate-800 dark:text-white'
                : 'text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-white']"
          >
            <Palette class="w-4 h-4" />
            Styles
          </button>
          <button
            @click="activeTab = 'shortcuts'; showForm = false"
            :class="['px-3 py-1.5 rounded-lg text-sm font-medium transition-colors inline-flex items-center gap-1.5',
              activeTab === 'shortcuts'
                ? 'bg-slate-200 dark:bg-slate-600 text-slate-800 dark:text-white'
                : 'text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-white']"
          >
            <Keyboard class="w-4 h-4" />
            Shortcuts
          </button>
          <button
            @click="activeTab = 'mcp'; showForm = false"
            :class="['px-3 py-1.5 rounded-lg text-sm font-medium transition-colors inline-flex items-center gap-1.5',
              activeTab === 'mcp'
                ? 'bg-slate-200 dark:bg-slate-600 text-slate-800 dark:text-white'
                : 'text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-white']"
          >
            <Plug class="w-4 h-4" />
            MCP
          </button>
        </div>
        <button @click="emit('close')" class="text-slate-400 hover:text-slate-600 dark:hover:text-white text-xl">&times;</button>
      </div>

      <!-- Content -->
      <div class="overflow-y-auto p-4 flex-1">
        <!-- General Tab -->
        <template v-if="activeTab === 'general'">
          <div class="flex flex-col h-full">
            <div class="space-y-4 flex-1 overflow-y-auto pr-1">
              <!-- Theme -->
              <div>
                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Theme</label>
                <div class="flex gap-2">
                  <button
                    @click="selectedTheme = 'light'"
                    :class="['flex-1 px-4 py-2.5 rounded-lg border text-sm font-medium transition-all',
                      selectedTheme === 'light'
                        ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                        : 'border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-400 hover:border-slate-300 dark:hover:border-slate-500']"
                  >&#9728; Light</button>
                  <button
                    @click="selectedTheme = 'dark'"
                    :class="['flex-1 px-4 py-2.5 rounded-lg border text-sm font-medium transition-all',
                      selectedTheme === 'dark'
                        ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                        : 'border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-400 hover:border-slate-300 dark:hover:border-slate-500']"
                  >&#127769; Dark</button>
                </div>
              </div>

              <div ref="fontDropdownRef" class="relative">
                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Font Family</label>
                <div
                  @click.stop="fontDropdownOpen = !fontDropdownOpen; if (fontDropdownOpen) fontSearch = ''"
                  class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus-within:border-blue-500 text-slate-800 dark:text-white cursor-text flex items-center justify-between"
                >
                  <input
                    v-if="fontDropdownOpen"
                    v-model="fontSearch"
                    placeholder="Search fonts..."
                    class="flex-1 bg-transparent outline-none text-slate-800 dark:text-white placeholder-slate-400 text-sm"
                    @keydown.escape="fontDropdownOpen = false"
                  />
                  <span v-else class="truncate text-sm">{{ currentFontLabel }}</span>
                  <svg class="w-4 h-4 text-slate-400 flex-shrink-0 ml-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
                </div>
                <div v-if="fontDropdownOpen && !fontsLoading" class="absolute z-50 w-full mt-1 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 shadow-lg max-h-60 overflow-y-auto">
                  <button
                    @click="selectFont(SYSTEM_DEFAULT)"
                    :class="['w-full text-left px-3 py-2 text-sm hover:bg-blue-50 dark:hover:bg-slate-600 transition-colors',
                      fontFamily === SYSTEM_DEFAULT ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 font-medium' : 'text-slate-800 dark:text-white']"
                  >System Default</button>
                  <div v-if="filteredFonts.length > 0" class="border-t border-slate-200 dark:border-slate-600">
                    <button
                      v-for="f in filteredFonts"
                      :key="f"
                      @click="selectFont(f)"
                      :style="{ fontFamily: f }"
                      :class="['w-full text-left px-3 py-1.5 text-sm hover:bg-blue-50 dark:hover:bg-slate-600 transition-colors truncate',
                        fontFamily === f ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 font-medium' : 'text-slate-800 dark:text-white']"
                    >{{ f }}</button>
                  </div>
                  <div v-if="filteredFonts.length === 0 && systemFonts.length > 0" class="px-3 py-2 text-xs text-slate-400 text-center">No matching fonts</div>
                </div>
                <div v-if="fontsLoading" class="text-xs text-slate-400 mt-1">Loading fonts...</div>
              </div>

              <div class="flex gap-4">
                <div class="flex-1">
                  <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Font Size (px)</label>
                  <input
                    v-model.number="fontSize"
                    type="number"
                    min="10"
                    max="24"
                    class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white"
                  />
                </div>
                <div class="flex-1">
                  <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Chat Width (px)</label>
                  <input
                    v-model.number="chatWidth"
                    type="number"
                    min="400"
                    max="2000"
                    class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white"
                  />
                </div>
              </div>

              <!-- Background Image -->
              <div>
                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Background Image</label>
                <div class="flex gap-2">
                  <input
                    v-model="bgImageDisplay"
                    placeholder="Enter image URL, or click Browse to pick a local file"
                    class="flex-1 px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400"
                  />
                  <button @click="browseImage" class="px-3 py-2 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-sm text-slate-700 dark:text-white flex-shrink-0">
                    Browse...
                  </button>
                </div>
              </div>

              <div>
                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">
                  Background Opacity: {{ bgOpacity.toFixed(2) }}
                </label>
                <input
                  v-model.number="bgOpacity"
                  type="range"
                  min="0"
                  max="1"
                  step="0.01"
                  class="w-full accent-blue-500"
                />
              </div>

              <!-- Built-in Tools -->
              <div class="space-y-3">
                <div>
                  <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Built-in Tools</label>
                  <p class="text-xs text-slate-500 dark:text-slate-400">
                    Enable or disable individual tools that AI can use
                  </p>
                </div>

                <!-- Tool: File Read -->
                <div class="flex items-center justify-between pl-2">
                  <div>
                    <label class="block text-sm font-medium text-slate-700 dark:text-slate-300">File Read</label>
                    <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">Allow AI to read local files</p>
                  </div>
                  <button
                    @click="toolFileRead = !toolFileRead"
                    :class="[
                      'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                      toolFileRead ? 'bg-blue-600' : 'bg-slate-300 dark:bg-slate-600'
                    ]"
                  >
                    <span
                      :class="[
                        'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                        toolFileRead ? 'translate-x-6' : 'translate-x-1'
                      ]"
                    />
                  </button>
                </div>

                <!-- Tool: File Write -->
                <div class="flex items-center justify-between pl-2">
                  <div>
                    <label class="block text-sm font-medium text-slate-700 dark:text-slate-300">File Write</label>
                    <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">Allow AI to create or overwrite files</p>
                  </div>
                  <button
                    @click="toolFileWrite = !toolFileWrite"
                    :class="[
                      'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                      toolFileWrite ? 'bg-blue-600' : 'bg-slate-300 dark:bg-slate-600'
                    ]"
                  >
                    <span
                      :class="[
                        'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                        toolFileWrite ? 'translate-x-6' : 'translate-x-1'
                      ]"
                    />
                  </button>
                </div>

                <!-- Tool: Shell Exec -->
                <div class="flex items-center justify-between pl-2">
                  <div>
                    <label class="block text-sm font-medium text-slate-700 dark:text-slate-300">Shell Exec</label>
                    <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">Allow AI to execute shell commands</p>
                  </div>
                  <button
                    @click="toolShellExec = !toolShellExec"
                    :class="[
                      'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                      toolShellExec ? 'bg-blue-600' : 'bg-slate-300 dark:bg-slate-600'
                    ]"
                  >
                    <span
                      :class="[
                        'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                        toolShellExec ? 'translate-x-6' : 'translate-x-1'
                      ]"
                    />
                  </button>
                </div>
              </div>

              <!-- Tool: Provide Selection -->
              <div class="flex items-center justify-between pl-2">
                <div>
                  <label class="block text-sm font-medium text-slate-700 dark:text-slate-300">Provide Selection</label>
                  <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">Allow AI to present choices (radio/checkbox) for interactive selection</p>
                </div>
                <button
                  @click="toolProvideSelection = !toolProvideSelection"
                  :class="[
                    'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                    toolProvideSelection ? 'bg-blue-600' : 'bg-slate-300 dark:bg-slate-600'
                  ]"
                >
                  <span
                    :class="[
                      'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                      toolProvideSelection ? 'translate-x-6' : 'translate-x-1'
                    ]"
                  />
                </button>
              </div>

              <!-- Completion Notification -->
              <div class="flex items-center justify-between pl-2">
                <div>
                  <label class="block text-sm font-medium text-slate-700 dark:text-slate-300">Completion Notification</label>
                  <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">Show a Windows notification when AI finishes responding</p>
                </div>
                <button
                  @click="notifyOnComplete = !notifyOnComplete"
                  :class="[
                    'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                    notifyOnComplete ? 'bg-blue-600' : 'bg-slate-300 dark:bg-slate-600'
                  ]"
                >
                  <span
                    :class="[
                      'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                      notifyOnComplete ? 'translate-x-6' : 'translate-x-1'
                    ]"
                  />
                </button>
              </div>

              <!-- Show Message Timestamps -->
              <div class="flex items-center justify-between pl-2">
                <div>
                  <label class="block text-sm font-medium text-slate-700 dark:text-slate-300">Show Message Timestamps</label>
                  <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">Display timestamps on chat messages</p>
                </div>
                <button
                  @click="showMessageTime = !showMessageTime"
                  :class="[
                    'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                    showMessageTime ? 'bg-blue-600' : 'bg-slate-300 dark:bg-slate-600'
                  ]"
                >
                  <span
                    :class="[
                      'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                      showMessageTime ? 'translate-x-6' : 'translate-x-1'
                    ]"
                  />
                </button>
              </div>
            </div>

            <div class="pt-4 border-t border-slate-200 dark:border-slate-700 mt-4">
              <button @click="saveGeneral" class="w-full px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm font-medium transition-colors text-white">
                {{ generalSaved ? 'Saved!' : 'Save' }}
              </button>
            </div>
          </div>
        </template>

        <!-- Providers Tab -->
        <template v-if="activeTab === 'providers'">
          <div class="space-y-2 mb-4">
            <div
              v-for="p in providerStore.providers"
              :key="p.id"
              class="flex items-center justify-between p-3 bg-slate-50 dark:bg-slate-700 rounded-lg"
            >
              <div class="min-w-0 flex-1">
                <div class="font-medium text-slate-800 dark:text-white">{{ p.name }}</div>
                <div class="text-xs text-slate-500 dark:text-slate-400 truncate">{{ p.base_url }} &middot; {{ p.models.join(', ') }}</div>
              </div>
              <div class="flex gap-2 flex-shrink-0 ml-2">
                <button @click="openEdit(p)" class="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 text-sm">Edit</button>
                <button @click="remove(p.id)" class="text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 text-sm">Delete</button>
              </div>
            </div>
            <div v-if="providerStore.providers.length === 0" class="text-center text-slate-400 dark:text-slate-500 text-sm py-4">
              No providers configured
            </div>
          </div>

          <div v-if="showForm" class="border-t border-slate-200 dark:border-slate-600 pt-4 space-y-3">
            <h4 class="font-medium text-slate-800 dark:text-white">{{ editingId ? 'Edit' : 'Add' }} Provider</h4>
            <input v-model="name" placeholder="Name (e.g. OpenAI, DeepSeek)" class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400" />
            <input v-model="apiKey" type="password" placeholder="API Key" class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400" />
            <input v-model="baseURL" placeholder="Base URL (e.g. https://api.openai.com)" class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400" />

            <!-- Models: dynamic list -->
            <div>
              <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Models</label>
              <div class="flex gap-2 mb-2">
                <input
                  v-model="newModelInput"
                  placeholder="Model name (e.g. gpt-4o)"
                  class="flex-1 px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400 text-sm"
                  @keydown.enter.prevent="addModel"
                />
                <button @click="addModel" class="px-3 py-2 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-sm text-slate-700 dark:text-white flex-shrink-0">Add</button>
                <button @click="fetchAvailableModels" :disabled="fetchingModels" class="px-3 py-2 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 rounded-lg text-sm text-white flex-shrink-0">
                  {{ fetchingModels ? 'Fetching...' : 'Get Models' }}
                </button>
              </div>
              <div v-if="modelList.length > 0" class="flex flex-wrap gap-1.5">
                <span
                  v-for="(m, idx) in modelList"
                  :key="idx"
                  class="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 text-xs rounded-full"
                >
                  {{ m }}
                  <button @click="removeModel(idx)" class="hover:text-red-500 dark:hover:text-red-400 font-bold leading-none">&times;</button>
                </span>
              </div>
              <div v-else class="text-xs text-slate-400 dark:text-slate-500">No models added yet</div>
            </div>

            <!-- Model fetcher popup -->
            <div v-if="showModelFetcher" class="border border-slate-200 dark:border-slate-600 rounded-lg p-3 bg-slate-50 dark:bg-slate-900/50">
              <div class="flex items-center justify-between mb-2">
                <span class="text-sm font-medium text-slate-700 dark:text-slate-300">Available Models ({{ filteredFetchedModels.length }}/{{ fetchedModels.length }})</span>
                <div class="flex gap-2">
                  <button v-if="fetchedModels.length > 0" @click="addAllFetchedModels" class="text-xs px-2 py-1 bg-blue-600 hover:bg-blue-700 rounded text-white">Add All</button>
                  <button @click="showModelFetcher = false" class="text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-300">&times; Close</button>
                </div>
              </div>
              <input
                v-if="!fetchingModels && !fetchError && fetchedModels.length > 0"
                v-model="modelFilter"
                placeholder="Filter models by prefix..."
                class="w-full px-2 py-1.5 mb-2 bg-white dark:bg-slate-700 rounded border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-xs text-slate-800 dark:text-white placeholder-slate-400"
              />
              <div v-if="fetchingModels" class="text-sm text-slate-500 dark:text-slate-400 py-2">Fetching models...</div>
              <div v-else-if="fetchError" class="text-sm text-red-600 dark:text-red-400 py-2">{{ fetchError }}</div>
              <div v-else class="max-h-48 overflow-y-auto space-y-0.5">
                <button
                  v-for="m in filteredFetchedModels"
                  :key="m"
                  @click="addFetchedModel(m)"
                  :class="['w-full text-left px-2 py-1.5 text-xs rounded hover:bg-blue-50 dark:hover:bg-slate-700 transition-colors flex items-center justify-between group',
                    modelList.includes(m) ? 'text-slate-400 dark:text-slate-500 line-through' : 'text-slate-700 dark:text-slate-300']"
                >
                  <span class="truncate">{{ m }}</span>
                  <span v-if="!modelList.includes(m)" class="text-blue-500 opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0 ml-1">+Add</span>
                  <span v-else class="text-slate-400 flex-shrink-0 ml-1">added</span>
                </button>
                <div v-if="filteredFetchedModels.length === 0" class="text-xs text-slate-400 dark:text-slate-500 text-center py-2">No models match filter</div>
              </div>
            </div>

            <label class="flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300">
              <input v-model="isDefault" type="checkbox" class="rounded" />
              Set as default provider
            </label>
            <div v-if="testResult" :class="['text-sm p-2 rounded', testResult.includes('success') ? 'bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-400' : 'bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-400']">
              {{ testResult }}
            </div>
            <div class="flex justify-between">
              <button @click="test" :disabled="testing" class="px-3 py-2 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-sm text-slate-700 dark:text-white">
                {{ testing ? 'Testing...' : 'Test Connection' }}
              </button>
              <div class="flex gap-2">
                <button @click="showForm = false" class="px-3 py-2 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-sm text-slate-700 dark:text-white">Cancel</button>
                <button @click="save" class="px-3 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm text-white">Save</button>
              </div>
            </div>
          </div>

          <button v-if="!showForm" @click="openAdd" class="w-full px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm font-medium transition-colors text-white">
            + Add Provider
          </button>
        </template>

        <!-- Prompt Tab -->
        <template v-if="activeTab === 'prompt'">
          <div class="space-y-3 h-full flex flex-col">
            <div class="text-xs text-slate-500 dark:text-slate-400 flex-shrink-0">
              Manage system prompts for your AI conversations. Set one as default, or assign specific prompts to individual sessions.
            </div>

            <!-- Prompt list -->
            <div v-if="!promptLoading && promptList.length > 0 && !promptShowForm" class="space-y-2 flex-1 overflow-y-auto">
              <div
                v-for="p in promptList"
                :key="p.id"
                class="flex items-start justify-between p-3 bg-slate-50 dark:bg-slate-700 rounded-lg"
              >
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="font-medium text-slate-800 dark:text-white">{{ p.name }}</span>
                    <span v-if="p.is_default" class="px-1.5 py-0.5 text-xs bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 rounded">Default</span>
                    <span v-if="p.category" class="px-1.5 py-0.5 text-xs bg-slate-100 dark:bg-slate-600 text-slate-500 dark:text-slate-400 rounded">{{ p.category }}</span>
                  </div>
                  <div class="text-xs text-slate-500 dark:text-slate-400 mt-1 line-clamp-2">{{ truncateContent(p.content) }}</div>
                </div>
                <div class="flex gap-2 flex-shrink-0 ml-2">
                  <button v-if="!p.is_default" @click="setPromptDefault(p.id)" class="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 text-sm">Set Default</button>
                  <button @click="openPromptEdit(p)" class="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 text-sm">Edit</button>
                  <button @click="deletePromptItem(p.id)" class="text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 text-sm">Delete</button>
                </div>
              </div>
            </div>

            <div v-if="promptLoading" class="text-center text-slate-400 dark:text-slate-500 text-sm py-4">
              Loading prompts...
            </div>

            <div v-if="!promptLoading && promptList.length === 0 && !promptShowForm" class="text-center text-slate-400 dark:text-slate-500 text-sm py-4">
              No prompts yet. Add one to get started.
            </div>

            <!-- Add/Edit form -->
            <div v-if="promptShowForm" class="border-t border-slate-200 dark:border-slate-600 pt-4 space-y-3 flex-1 flex flex-col">
              <h4 class="font-medium text-slate-800 dark:text-white">{{ promptEditingId ? 'Edit' : 'Add' }} Prompt</h4>
              <div class="flex gap-3">
                <div class="flex-1">
                  <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Name</label>
                  <input v-model="promptName" placeholder="e.g. Code Reviewer, Translator" class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400" />
                </div>
                <div class="w-1/3">
                  <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Category</label>
                  <input v-model="promptCategory" placeholder="e.g. Coding" class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400" />
                </div>
              </div>
              <div class="flex-1 flex flex-col">
                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Prompt Content</label>
                <textarea
                  v-model="promptContent"
                  placeholder="Enter system prompt content..."
                  class="flex-1 w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none resize-none text-slate-800 dark:text-white placeholder-slate-400 min-h-[200px]"
                ></textarea>
              </div>
              <label class="flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300">
                <input v-model="promptIsDefault" type="checkbox" class="rounded" />
                Set as default prompt
              </label>
              <div class="flex justify-end gap-2">
                <button @click="promptShowForm = false" class="px-3 py-2 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-sm text-slate-700 dark:text-white">Cancel</button>
                <button @click="savePromptItem" :disabled="promptSaving || !promptName.trim() || !promptContent.trim()" class="px-3 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm text-white disabled:bg-slate-300 dark:disabled:bg-slate-600 disabled:cursor-not-allowed">{{ promptSaving ? 'Saving...' : 'Save' }}</button>
              </div>
            </div>

            <button v-if="!promptShowForm" @click="openPromptAdd" class="w-full px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm font-medium transition-colors text-white">
              + Add Prompt
            </button>
          </div>
        </template>

        <!-- Styles Tab -->
        <template v-if="activeTab === 'styles'">
          <div class="space-y-3 h-full flex flex-col">
            <div class="text-xs text-slate-500 dark:text-slate-400 flex-shrink-0">
              Select a theme or edit CSS directly. Place custom <code class="bg-slate-100 dark:bg-slate-700 px-1 rounded">.css</code> files in the themes folder to add new themes.
            </div>

            <!-- Theme selector grid -->
            <div v-if="!themesLoading" class="grid grid-cols-3 gap-2 flex-shrink-0">
              <div
                v-for="theme in themes"
                :key="theme.name"
                @click="selectTheme(theme.name)"
                :class="[
                  'relative p-2 rounded-lg border-2 cursor-pointer transition-all',
                  currentTheme === theme.name
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 shadow-sm'
                    : 'border-slate-200 dark:border-slate-600 hover:border-slate-300 dark:hover:border-slate-500 bg-white dark:bg-slate-700'
                ]"
              >
                <!-- Selected indicator -->
                <div v-if="currentTheme === theme.name" class="absolute top-1 right-1 z-10">
                  <svg class="w-3.5 h-3.5 text-blue-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
                  </svg>
                </div>

                <!-- Mini chat preview -->
                <div
                  class="rounded overflow-hidden h-16 flex"
                  :style="{backgroundColor: getThemeColors(theme.name, theme.css).bodyBg, borderColor: getThemeColors(theme.name, theme.css).borderColor, borderWidth: '1px'}"
                >
                  <!-- Mini sidebar -->
                  <div class="w-5 flex-shrink-0 flex flex-col gap-px p-px" :style="{backgroundColor: getThemeColors(theme.name, theme.css).sidebarBg}">
                    <div class="rounded-sm h-1.5 w-full mt-0.5" :style="{backgroundColor: getThemeColors(theme.name, theme.css).accent, opacity: 0.8}"></div>
                    <div class="rounded-sm h-1 w-3/4 mt-1" :style="{backgroundColor: getThemeColors(theme.name, theme.css).bodyText, opacity: 0.15}"></div>
                    <div class="rounded-sm h-1 w-full mt-0.5" :style="{backgroundColor: getThemeColors(theme.name, theme.css).bodyText, opacity: 0.1}"></div>
                    <div class="rounded-sm h-1 w-2/3 mt-0.5" :style="{backgroundColor: getThemeColors(theme.name, theme.css).bodyText, opacity: 0.1}"></div>
                  </div>
                  <!-- Mini chat area -->
                  <div class="flex-1 flex flex-col justify-center gap-1 px-1" :style="{backgroundColor: getThemeColors(theme.name, theme.css).bodyBg}">
                    <!-- AI bubble -->
                    <div class="rounded-sm h-2.5 w-3/4 self-start" :style="{backgroundColor: getThemeColors(theme.name, theme.css).aiBubble}"></div>
                    <!-- User bubble -->
                    <div class="rounded-sm h-2.5 w-3/5 self-end" :style="{backgroundColor: getThemeColors(theme.name, theme.css).userBubble}"></div>
                  </div>
                </div>

                <!-- Theme name + label -->
                <div class="flex items-baseline justify-between mt-1">
                  <div class="text-xs font-medium text-slate-800 dark:text-white truncate">
                    {{ theme.name }}
                  </div>
                  <span class="text-[9px] text-slate-400 dark:text-slate-500 flex-shrink-0 ml-1">
                    {{ theme.isDefault ? 'Built-in' : 'Custom' }}
                  </span>
                </div>
              </div>
            </div>

            <div v-if="themesLoading" class="text-center text-slate-400 dark:text-slate-500 text-sm py-2">
              Loading themes...
            </div>

            <!-- Theme folder button + refresh -->
            <div class="flex gap-2 flex-shrink-0 items-center">
              <button @click="openThemeFolderAction" class="px-3 py-1.5 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-xs font-medium transition-colors text-slate-700 dark:text-white">
                Open Theme Folder
              </button>
              <button @click="refreshThemes" class="px-3 py-1.5 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-xs font-medium transition-colors text-slate-700 dark:text-white">
                Refresh
              </button>
              <div class="flex-1"></div>
              <span class="text-[10px] text-slate-400 dark:text-slate-500">
                {{ themes.filter(t => !t.isDefault).length }} custom theme(s)
              </span>
            </div>

            <!-- CSS textarea editor -->
            <textarea
              v-model="stylesCSS"
              :disabled="stylesLoading"
              spellcheck="false"
              class="flex-1 w-full px-3 py-2 bg-white dark:bg-slate-900 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none resize-none text-slate-800 dark:text-slate-200 placeholder-slate-400 font-mono text-xs leading-relaxed min-h-[200px]"
            ></textarea>

            <!-- Action buttons -->
            <div class="flex gap-2 flex-shrink-0">
              <button @click="resetStyles" class="px-4 py-2 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-sm font-medium transition-colors text-slate-700 dark:text-white">
                Reset to Default
              </button>
              <div class="flex-1"></div>
              <button @click="saveStyles" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm font-medium transition-colors text-white">
                {{ stylesSaved ? 'Saved!' : 'Save' }}
              </button>
            </div>
          </div>
        </template>

        <!-- Shortcuts Tab -->
        <template v-if="activeTab === 'shortcuts'">
          <div class="space-y-3 h-full flex flex-col" @keydown="handleShortcutKeydown">
            <div class="text-xs text-slate-500 dark:text-slate-400 flex-shrink-0">
              Click a shortcut to rebind it. Press the desired key combination, or press Escape to cancel.
            </div>
            <div class="space-y-2 flex-1 overflow-y-auto">
              <div
                v-for="item in shortcutList"
                :key="item.key"
                class="flex items-center justify-between p-3 bg-slate-50 dark:bg-slate-700 rounded-lg"
              >
                <div>
                  <div class="font-medium text-sm text-slate-800 dark:text-white">{{ item.label }}</div>
                  <div class="text-xs text-slate-500 dark:text-slate-400">{{ item.description }}</div>
                </div>
                <button
                  @click="recordingKey === item.key ? cancelRecording() : startRecording(item.key)"
                  :class="['min-w-[120px] px-3 py-1.5 rounded-lg text-sm font-mono border transition-colors text-center',
                    recordingKey === item.key
                      ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 animate-pulse'
                      : 'border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 hover:border-slate-400 dark:hover:border-slate-500']"
                >
                  <template v-if="recordingKey === item.key">Press key...</template>
                  <template v-else>{{ formatShortcut(shortcutBindings[item.key]) }}</template>
                </button>
              </div>
            </div>
            <div class="flex gap-2 flex-shrink-0 pt-2">
              <button @click="resetShortcuts" class="px-4 py-2 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-sm font-medium transition-colors text-slate-700 dark:text-white">
                Reset to Default
              </button>
              <div class="flex-1"></div>
              <button @click="saveShortcuts" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm font-medium transition-colors text-white">
                {{ shortcutsSaved ? 'Saved!' : 'Save' }}
              </button>
            </div>
          </div>
        </template>

        <!-- MCP Servers Tab -->
        <template v-if="activeTab === 'mcp'">
          <div class="space-y-3">
            <div class="text-xs text-slate-500 dark:text-slate-400">
              MCP (Model Context Protocol) servers provide tools that AI models can use. Configure local (stdio) or remote (HTTP) servers.
            </div>

            <!-- Server list -->
            <div v-if="!mcpLoading && mcpServers.length > 0" class="space-y-2 mb-4">
              <div
                v-for="server in mcpServers"
                :key="server.id"
                class="flex items-center justify-between p-3 bg-slate-50 dark:bg-slate-700 rounded-lg"
              >
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="font-medium text-slate-800 dark:text-white">{{ server.name }}</span>
                    <span v-if="server.enabled" class="px-1.5 py-0.5 text-xs bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded">Enabled</span>
                    <span v-else class="px-1.5 py-0.5 text-xs bg-slate-100 dark:bg-slate-600 text-slate-500 dark:text-slate-400 rounded">Disabled</span>
                    <!-- Connection status badge -->
                    <span
                      v-if="server.enabled"
                      :class="[
                        'px-1.5 py-0.5 text-xs rounded transition-colors duration-200',
                        mcpConnectionStatus[server.id] === 'connected' ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400' :
                        mcpConnectionStatus[server.id] === 'connecting' ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400' :
                        mcpConnectionStatus[server.id] === 'disconnecting' ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400' :
                        mcpConnectionStatus[server.id] === 'error' ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400' :
                        'bg-slate-100 dark:bg-slate-600 text-slate-500 dark:text-slate-400'
                      ]"
                    >
                      {{ mcpConnectionStatus[server.id] === 'connected' ? 'Connected' :
                         mcpConnectionStatus[server.id] === 'connecting' ? 'Connecting...' :
                         mcpConnectionStatus[server.id] === 'disconnecting' ? 'Disconnecting...' :
                         mcpConnectionStatus[server.id] === 'error' ? 'Error' : 'Disconnected' }}
                    </span>
                  </div>
                  <div class="text-xs text-slate-500 dark:text-slate-400 truncate">
                    {{ server.transport === 'stdio' ? server.command : server.url }}
                  </div>
                  <div v-if="mcpConnectionError[server.id]" class="text-xs text-red-500 dark:text-red-400 mt-1">
                    {{ mcpConnectionError[server.id] }}
                  </div>
                </div>
                <div class="flex gap-2 flex-shrink-0 ml-2">
                  <button
                    v-if="mcpConnectionStatus[server.id] !== 'connected' && mcpConnectionStatus[server.id] !== 'connecting' && mcpConnectionStatus[server.id] !== 'disconnecting'"
                    @click="connectMCPServer(server.id)"
                    class="text-green-600 dark:text-green-400 hover:text-green-700 dark:hover:text-green-300 text-sm"
                  >
                    Connect
                  </button>
                  <button
                    v-if="mcpConnectionStatus[server.id] === 'connected' || mcpConnectionStatus[server.id] === 'disconnecting'"
                    @click="disconnectMCPServer(server.id)"
                    :disabled="mcpConnectionStatus[server.id] === 'disconnecting'"
                    class="text-orange-600 dark:text-orange-400 hover:text-orange-700 dark:hover:text-orange-300 text-sm disabled:opacity-50"
                  >
                    {{ mcpConnectionStatus[server.id] === 'disconnecting' ? 'Disconnecting...' : 'Disconnect' }}
                  </button>
                  <button @click="openMCPEdit(server)" :disabled="mcpConnectionStatus[server.id] === 'connecting' || mcpConnectionStatus[server.id] === 'disconnecting'" class="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 text-sm disabled:opacity-50">Edit</button>
                  <button @click="deleteMCPServer(server.id)" :disabled="mcpConnectionStatus[server.id] === 'connecting' || mcpConnectionStatus[server.id] === 'disconnecting'" class="text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 text-sm disabled:opacity-50">Delete</button>
                </div>
              </div>
            </div>

            <div v-if="mcpLoading" class="text-center text-slate-400 dark:text-slate-500 text-sm py-4">
              Loading MCP servers...
            </div>

            <div v-if="!mcpLoading && mcpServers.length === 0 && !mcpShowForm" class="text-center text-slate-400 dark:text-slate-500 text-sm py-4">
              No MCP servers configured
            </div>

            <!-- Add/Edit form -->
            <div v-if="mcpShowForm" class="border-t border-slate-200 dark:border-slate-600 pt-4 space-y-3">
              <h4 class="font-medium text-slate-800 dark:text-white">{{ mcpEditingId ? 'Edit' : 'Add' }} MCP Server</h4>

              <!-- Validation error -->
              <div v-if="mcpValidationError" class="text-sm p-3 bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-400 rounded">
                {{ mcpValidationError }}
              </div>

              <input v-model="mcpName" placeholder="Server name (e.g. File System, GitHub)" class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400" />

              <!-- Transport type -->
              <div>
                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Transport Type</label>
                <div class="flex gap-2">
                  <button
                    @click="mcpTransport = 'stdio'"
                    :class="['flex-1 px-4 py-2 rounded-lg border text-sm font-medium transition-all',
                      mcpTransport === 'stdio'
                        ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                        : 'border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-400 hover:border-slate-300']"
                  >Stdio (Local)</button>
                  <button
                    @click="mcpTransport = 'http'"
                    :class="['flex-1 px-4 py-2 rounded-lg border text-sm font-medium transition-all',
                      mcpTransport === 'http'
                        ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                        : 'border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-400 hover:border-slate-300']"
                  >HTTP (Remote)</button>
                </div>
              </div>

              <!-- Stdio fields -->
              <div v-if="mcpTransport === 'stdio'">
                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">Command</label>
                <input v-model="mcpCommand" placeholder="e.g. npx -y @modelcontextprotocol/server-filesystem /tmp" class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400" />

                <!-- Env variables -->
                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1 mt-3">Environment Variables</label>
                <div class="flex gap-2 mb-2">
                  <input v-model="mcpNewEnvKey" placeholder="KEY" class="flex-1 px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400 text-sm" />
                  <input v-model="mcpNewEnvValue" placeholder="value" class="flex-1 px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400 text-sm" />
                  <button @click="addMCPEnvVar" class="px-3 py-2 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-sm text-slate-700 dark:text-white">Add</button>
                </div>
                <div v-if="mcpEnvVars.length > 0" class="flex flex-wrap gap-1.5">
                  <span
                    v-for="(env, idx) in mcpEnvVars"
                    :key="idx"
                    class="inline-flex items-center gap-1 px-2 py-1 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 text-xs rounded-full"
                  >
                    {{ env.key }}={{ env.value }}
                    <button @click="removeMCPEnvVar(idx)" class="hover:text-red-500 dark:hover:text-red-400 font-bold leading-none">&times;</button>
                  </span>
                </div>
              </div>

              <!-- HTTP fields -->
              <div v-if="mcpTransport === 'http'">
                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1">URL</label>
                <input v-model="mcpURL" placeholder="e.g. https://api.github.com/mcp" class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400" />

                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1 mt-3">Auth Token (Optional)</label>
                <input v-model="mcpAuthToken" type="password" placeholder="Bearer token or API key" class="w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none text-slate-800 dark:text-white placeholder-slate-400" />
              </div>

              <label class="flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300">
                <input v-model="mcpEnabled" type="checkbox" class="rounded" />
                Enable this server
              </label>

              <!-- Test result -->
              <div v-if="mcpTestResult" :class="['text-sm p-3 rounded', mcpTestResult.success ? 'bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-400' : 'bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-400']">
                <div v-if="mcpTestResult.success">
                  <div class="font-medium mb-1">Connection successful!</div>
                  <div v-if="mcpTestResult.tools && mcpTestResult.tools.length > 0" class="text-xs">
                    Tools available: {{ mcpTestResult.tools.map(t => t.name).join(', ') }}
                  </div>
                </div>
                <div v-else>
                  <div class="font-medium">Connection failed</div>
                  <div class="text-xs">{{ mcpTestResult.error }}</div>
                </div>
              </div>

              <div class="flex justify-between">
                <button @click="testMCPServer" :disabled="mcpTesting || (mcpTransport === 'stdio' && !mcpCommand) || (mcpTransport === 'http' && !mcpURL)" class="px-3 py-2 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-sm text-slate-700 dark:text-white disabled:opacity-50">
                  {{ mcpTesting ? 'Testing...' : 'Test Connection' }}
                </button>
                <div class="flex gap-2">
                  <button @click="mcpShowForm = false" class="px-3 py-2 bg-slate-100 dark:bg-slate-600 hover:bg-slate-200 dark:hover:bg-slate-500 rounded-lg text-sm text-slate-700 dark:text-white">Cancel</button>
                  <button @click="saveMCPServer" :disabled="!isMCPServerValid || mcpSaving" class="px-3 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm text-white disabled:bg-slate-300 dark:disabled:bg-slate-600 disabled:cursor-not-allowed">{{ mcpSaving ? 'Saving...' : 'Save' }}</button>
                </div>
              </div>
            </div>

            <button v-if="!mcpShowForm" @click="openMCPAdd" class="w-full px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm font-medium transition-colors text-white">
              + Add MCP Server
            </button>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
