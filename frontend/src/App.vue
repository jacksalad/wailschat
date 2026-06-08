<script lang="ts" setup>
import {onMounted, onUnmounted, ref} from 'vue'
import Sidebar from './components/Sidebar.vue'
import ChatWindow from './components/ChatWindow.vue'
import {useProviderStore} from './stores/provider'
import {useSessionStore} from './stores/session'
import {useSettingsStore} from './stores/settings'
import {useMessageStore} from './stores/message'
import {useSearchStore} from './stores/search'
import {ToggleFullscreen} from '../wailsjs/go/main/App'

const providerStore = useProviderStore()
const sessionStore = useSessionStore()
const settingsStore = useSettingsStore()
const messageStore = useMessageStore()

const sidebarVisible = ref(true)
const sidebarWidth = ref(Number(settingsStore.sidebarWidth) || 350)
const isResizing = ref(false)

async function newChat() {
  if (providerStore.providers.length === 0) return
  const p = providerStore.getCurrentProvider()
  if (!p) return
  const model = p.models[0] || ''
  await sessionStore.createSession(p.id, 'New Chat', model)
}

function parseBinding(binding: string): { ctrl: boolean; alt: boolean; shift: boolean; meta: boolean; key: string } {
  const parts = binding.toLowerCase().split('+')
  return {
    ctrl: parts.includes('ctrl'),
    alt: parts.includes('alt'),
    shift: parts.includes('shift'),
    meta: parts.includes('meta'),
    key: parts[parts.length - 1],
  }
}

function matchesShortcut(e: KeyboardEvent, binding: string): boolean {
  const b = parseBinding(binding)
  return e.ctrlKey === b.ctrl && e.altKey === b.alt && e.shiftKey === b.shift && e.metaKey === b.meta && e.key.toLowerCase() === b.key
}

function onGlobalKeydown(e: KeyboardEvent) {
  // F11 fullscreen toggle
  if (e.key === 'F11') {
    e.preventDefault()
    ToggleFullscreen()
    return
  }

  const shortcuts = settingsStore.shortcuts
  const target = e.target as HTMLElement
  const inInput = target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable

  // New Chat (Ctrl+N) — always active
  if (matchesShortcut(e, shortcuts.new_chat)) {
    e.preventDefault()
    newChat()
    return
  }

  // Toggle Sidebar (Ctrl+B) — always active
  if (matchesShortcut(e, shortcuts.toggle_sidebar)) {
    e.preventDefault()
    sidebarVisible.value = !sidebarVisible.value
    return
  }

  // Clear Context (Ctrl+L) — only when a session is active
  if (matchesShortcut(e, shortcuts.clear_context) && sessionStore.currentSessionId) {
    e.preventDefault()
    messageStore.clearHistory(sessionStore.currentSessionId)
    return
  }

  // Search Messages (Ctrl+F) — toggle search bar
  if (matchesShortcut(e, shortcuts.search_messages) && sessionStore.currentSessionId) {
    e.preventDefault()
    const searchStore = useSearchStore()
    if (searchStore.isOpen) {
      searchStore.closeSearch()
    } else {
      searchStore.openSearch()
    }
    return
  }

  // Focus Input (/) — skip when typing in input fields
  if (!inInput && matchesShortcut(e, shortcuts.focus_input)) {
    e.preventDefault()
    const textarea = document.querySelector<HTMLTextAreaElement>('.chat-input-textarea')
    if (textarea) textarea.focus()
    return
  }
}

// Sidebar resize handlers
let resizeRaf = 0

function onResizeStart(e: MouseEvent) {
  e.preventDefault()
  isResizing.value = true
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}

function onMouseMove(e: MouseEvent) {
  if (!isResizing.value) return
  const newWidth = Math.min(500, Math.max(200, e.clientX))
  if (resizeRaf) return
  resizeRaf = requestAnimationFrame(() => {
    resizeRaf = 0
    sidebarWidth.value = newWidth
  })
}

function onMouseUp() {
  if (!isResizing.value) return
  isResizing.value = false
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
  // Persist sidebar width
  settingsStore.saveSettings({ sidebar_width: String(sidebarWidth.value) })
}

onMounted(async () => {
  try {
    await settingsStore.fetchSettings()
    settingsStore.applyToDOM()
    sidebarWidth.value = Number(settingsStore.sidebarWidth) || 450
    // Fetch providers and sessions in parallel (both are independent DB queries)
    // Prompts are lazy-loaded on first use in ChatWindow
    await Promise.all([
      providerStore.fetchProviders(),
      sessionStore.fetchSessions(),
    ])
    // Preload system fonts in background for Settings font picker
    settingsStore.loadSystemFonts()
  } catch (e) {
    console.error('Failed to initialize stores:', e)
  }
  document.addEventListener('keydown', onGlobalKeydown)
  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
})
onUnmounted(() => {
  document.removeEventListener('keydown', onGlobalKeydown)
  document.removeEventListener('mousemove', onMouseMove)
  document.removeEventListener('mouseup', onMouseUp)
})
</script>

<template>
  <div class="app-container flex h-screen bg-slate-50 dark:bg-slate-900 text-slate-800 dark:text-slate-200">
    <Sidebar v-show="sidebarVisible" :style="{ width: sidebarWidth + 'px', flexShrink: 0 }" class="flex-shrink-0 sidebar" />
    <!-- Resize handle -->
    <div
      v-show="sidebarVisible"
      class="resize-handle w-1 cursor-col-resize hover:bg-blue-400 dark:hover:bg-blue-500 transition-colors flex-shrink-0"
      :class="{ 'bg-blue-400 dark:bg-blue-500': isResizing }"
      @mousedown="onResizeStart"
    ></div>
    <ChatWindow class="flex-1 min-w-0 chat-window" />
  </div>
</template>
