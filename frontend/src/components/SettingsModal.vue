<script lang="ts" setup>
import {ref, onMounted, onBeforeUnmount, computed} from 'vue'
import {useProviderStore, type Provider} from '../stores/provider'
import {useSettingsStore, type Theme, type ShortcutBindings} from '../stores/settings'
import {GetModels, GetDefaultStyles, OpenFileDialog} from '../../wailsjs/go/main/App'
import {model} from '../../wailsjs/go/models'
import {Server, Settings2, MessageSquare, Palette, Keyboard, Plug} from 'lucide-vue-next'

const emit = defineEmits<{close: []}>()

const providerStore = useProviderStore()
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

// --- Prompt settings state ---
const systemPrompt = ref(settingsStore.systemPrompt)
const promptSaved = ref(false)

// --- Styles settings state ---
const stylesCSS = ref('')
const defaultCSS = ref('')
const stylesLoading = ref(false)
const stylesSaved = ref(false)

// --- Shortcuts settings state ---
const shortcutBindings = ref<ShortcutBindings>({ ...settingsStore.shortcuts })
const recordingKey = ref<string | null>(null) // which shortcut is being recorded
const shortcutsSaved = ref(false)

const shortcutList: { key: keyof ShortcutBindings; label: string; description: string }[] = [
  { key: 'new_chat', label: 'New Chat', description: 'Create a new chat session' },
  { key: 'clear_context', label: 'Clear Context', description: 'Clear all messages in current chat' },
  { key: 'focus_input', label: 'Focus Input', description: 'Focus the chat input box' },
  { key: 'toggle_sidebar', label: 'Toggle Sidebar', description: 'Show or hide the sidebar' },
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

// MCP connection status
const mcpConnectionStatus = ref<Record<string, 'connected' | 'disconnected' | 'connecting' | 'error'>>({})
const mcpConnectionError = ref<Record<string, string>>({})

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
    mcpConnectionStatus.value = statuses
  } catch (e) {
    console.error('Failed to load MCP server statuses:', e)
  }
}

// Connect to an MCP server
async function connectMCPServer(id: string) {
  mcpConnectionStatus.value[id] = 'connecting'
  mcpConnectionError.value[id] = ''
  try {
    const {MCPServerConnect} = await import('../../wailsjs/go/main/App')
    await MCPServerConnect(id)
    mcpConnectionStatus.value[id] = 'connected'
  } catch (e: any) {
    mcpConnectionStatus.value[id] = 'error'
    mcpConnectionError.value[id] = e.toString()
  }
}

// Disconnect from an MCP server
async function disconnectMCPServer(id: string) {
  try {
    const {MCPServerDisconnect} = await import('../../wailsjs/go/main/App')
    await MCPServerDisconnect(id)
    mcpConnectionStatus.value[id] = 'disconnected'
  } catch (e) {
    console.error('Failed to disconnect MCP server:', e)
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
onMounted(async () => {
  await loadMCPServers()
  await loadMCPServerStatuses()
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

async function savePrompt() {
  await settingsStore.saveSettings({
    system_prompt: systemPrompt.value,
  })
  promptSaved.value = true
  setTimeout(() => { promptSaved.value = false }, 2000)
}

async function saveShortcuts() {
  await settingsStore.saveSettings({
    shortcuts: JSON.stringify(shortcutBindings.value),
  })
  shortcutsSaved.value = true
  setTimeout(() => { shortcutsSaved.value = false }, 2000)
}

function resetShortcuts() {
  shortcutBindings.value = { new_chat: 'ctrl+n', clear_context: 'ctrl+l', focus_input: '/', toggle_sidebar: 'ctrl+b' }
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
})

async function saveStyles() {
  await settingsStore.saveSettings({
    custom_styles: stylesCSS.value,
  })
  settingsStore.applyCustomStyles()
  stylesSaved.value = true
  setTimeout(() => { stylesSaved.value = false }, 2000)
}

function resetStyles() {
  stylesCSS.value = defaultCSS.value
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
              System prompt prepended to every conversation. This sets the AI's behavior and persona.
            </div>
            <textarea
              v-model="systemPrompt"
              placeholder="Default system prompt for all chats..."
              class="flex-1 w-full px-3 py-2 bg-white dark:bg-slate-700 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none resize-none text-slate-800 dark:text-white placeholder-slate-400 min-h-[350px]"
            ></textarea>
            <div class="flex justify-end flex-shrink-0">
              <button @click="savePrompt" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm font-medium transition-colors text-white">
                {{ promptSaved ? 'Saved!' : 'Save' }}
              </button>
            </div>
          </div>
        </template>

        <!-- Styles Tab -->
        <template v-if="activeTab === 'styles'">
          <div class="space-y-3 h-full flex flex-col">
            <div class="text-xs text-slate-500 dark:text-slate-400 flex-shrink-0">
              Custom CSS styles with highest priority. Edits here override all default styles. Refer to
              <code class="bg-slate-100 dark:bg-slate-700 px-1 rounded">styles-doc.md</code> for available selectors and examples.
            </div>
            <textarea
              v-model="stylesCSS"
              :disabled="stylesLoading"
              spellcheck="false"
              class="flex-1 w-full px-3 py-2 bg-white dark:bg-slate-900 rounded-lg border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none resize-none text-slate-800 dark:text-slate-200 placeholder-slate-400 font-mono text-xs leading-relaxed min-h-[400px]"
            ></textarea>
            <div class="flex gap-2">
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
                    <!-- Connection status -->
                    <span
                      v-if="server.enabled"
                      :class="[
                        'px-1.5 py-0.5 text-xs rounded',
                        mcpConnectionStatus[server.id] === 'connected' ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400' :
                        mcpConnectionStatus[server.id] === 'connecting' ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400' :
                        mcpConnectionStatus[server.id] === 'error' ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400' :
                        'bg-slate-100 dark:bg-slate-600 text-slate-500 dark:text-slate-400'
                      ]"
                    >
                      {{ mcpConnectionStatus[server.id] === 'connected' ? 'Connected' :
                         mcpConnectionStatus[server.id] === 'connecting' ? 'Connecting...' :
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
                    v-if="server.enabled && mcpConnectionStatus[server.id] !== 'connected'"
                    @click="connectMCPServer(server.id)"
                    :disabled="mcpConnectionStatus[server.id] === 'connecting'"
                    class="text-green-600 dark:text-green-400 hover:text-green-700 dark:hover:text-green-300 text-sm disabled:opacity-50"
                  >
                    {{ mcpConnectionStatus[server.id] === 'connecting' ? 'Connecting...' : 'Connect' }}
                  </button>
                  <button
                    v-if="server.enabled && mcpConnectionStatus[server.id] === 'connected'"
                    @click="disconnectMCPServer(server.id)"
                    class="text-orange-600 dark:text-orange-400 hover:text-orange-700 dark:hover:text-orange-300 text-sm"
                  >
                    Disconnect
                  </button>
                  <button @click="openMCPEdit(server)" class="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 text-sm">Edit</button>
                  <button @click="deleteMCPServer(server.id)" class="text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 text-sm">Delete</button>
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
