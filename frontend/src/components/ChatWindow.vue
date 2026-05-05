<script lang="ts" setup>
import {computed, watch, nextTick, ref, onMounted, onBeforeUnmount, provide} from 'vue'
import {useSessionStore} from '../stores/session'
import {useMessageStore, type PerformanceStats} from '../stores/message'
import {useProviderStore} from '../stores/provider'
import {usePromptStore} from '../stores/prompt'
import {useSettingsStore} from '../stores/settings'
import {useSearchStore} from '../stores/search'
import {Loader2, ChevronUp, ChevronDown, ArrowUp} from 'lucide-vue-next'
import {OnFileDrop, OnFileDropOff} from '../../wailsjs/runtime/runtime'
import MessageBubble from './MessageBubble.vue'
import ChatInput from './ChatInput.vue'
import SearchBar from './SearchBar.vue'
import logoUrl from '../assets/logo.png'

const sessionStore = useSessionStore()
const messageStore = useMessageStore()
const providerStore = useProviderStore()
const promptStore = usePromptStore()
const settingsStore = useSettingsStore()
const searchStore = useSearchStore()
const messagesContainer = ref<HTMLElement | null>(null)
const showModelPicker = ref(false)
const showPromptPicker = ref(false)
const modelPickerRef = ref<HTMLElement | null>(null)
const promptPickerRef = ref<HTMLElement | null>(null)
const chatInputRef = ref<InstanceType<typeof ChatInput> | null>(null)
const isDragOver = ref(false)
const quoteText = ref('')
const editingMessageId = ref<string | null>(null)
provide('editingMessageId', editingMessageId)

const chatWidthPx = computed(() => settingsStore.chatWidth + 'px')

const currentSession = computed(() => sessionStore.getCurrentSession())
const messages = computed(() => {
  const sid = sessionStore.currentSessionId
  if (!sid) return []
  return messageStore.getMessages(sid)
})

const currentProvider = computed(() => {
  const sess = currentSession.value
  if (!sess) return null
  return providerStore.providers.find(p => p.id === sess.provider_id)
})

function toggleModelPicker() {
  showModelPicker.value = !showModelPicker.value
}

async function selectModel(providerId: number, model: string) {
  const sid = sessionStore.currentSessionId
  if (!sid) return
  await sessionStore.updateSessionModel(sid, providerId, model)
  showModelPicker.value = false
}

function onClickOutside(e: MouseEvent) {
  if (modelPickerRef.value && !modelPickerRef.value.contains(e.target as Node)) {
    showModelPicker.value = false
  }
  if (promptPickerRef.value && !promptPickerRef.value.contains(e.target as Node)) {
    showPromptPicker.value = false
  }
}

// Prompt picker helpers
const currentPromptName = computed(() => {
  const sess = currentSession.value
  if (!sess) return 'Default'
  if (sess.prompt_id) {
    const p = promptStore.getByID(sess.prompt_id)
    return p ? p.name : 'Default'
  }
  const def = promptStore.getDefault()
  return def ? def.name : 'Default'
})

function togglePromptPicker() {
  showPromptPicker.value = !showPromptPicker.value
}

async function selectPrompt(promptId: number | null) {
  const sid = sessionStore.currentSessionId
  if (!sid) return
  await sessionStore.updateSessionPrompt(sid, promptId)
  showPromptPicker.value = false
}

onMounted(() => {
  document.addEventListener('click', onClickOutside)
  if (messagesContainer.value) {
    messagesContainer.value.addEventListener('scroll', onMessagesScroll, { passive: true })
  }

  // Register Wails native file drop handler
  OnFileDrop((x: number, y: number, paths: string[]) => {
    isDragOver.value = false
    if (paths.length > 0) chatInputRef.value?.addFiles(paths)
  }, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', onClickOutside)
  if (messagesContainer.value) {
    messagesContainer.value.removeEventListener('scroll', onMessagesScroll)
  }
  if (scrollUpdateRaf) {
    cancelAnimationFrame(scrollUpdateRaf)
    scrollUpdateRaf = 0
  }
  if (scrollRafId) {
    cancelAnimationFrame(scrollRafId)
    scrollRafId = 0
  }
  OnFileDropOff()
})

// Track which assistant message is the last one for retry
const lastAssistantMessageId = computed(() => {
  const msgs = messages.value
  for (let i = msgs.length - 1; i >= 0; i--) {
    if (msgs[i].role === 'assistant') return msgs[i].id
  }
  return null
})

// Pre-compute stats map to avoid JSON.parse per message per render
const statsMap = computed(() => {
  const map = new Map<string, PerformanceStats>()
  const sid = sessionStore.currentSessionId
  if (!sid) return map
  const msgs = messageStore.getMessages(sid)
  for (const msg of msgs) {
    const stats = messageStore.parseStats(msg.stats)
    if (stats) map.set(msg.id, stats)
  }
  return map
})

// --- Turn navigation ---
const currentTurnIndex = ref(0)
let scrollUpdateRaf = 0
let isNavigating = false

const turnAnchors = computed(() => {
  const result: number[] = []
  messages.value.forEach((msg, idx) => {
    if (msg.role === 'user') result.push(idx)
  })
  return result
})

const showTurnNav = computed(() => turnAnchors.value.length > 1)
const canGoPrev = computed(() => currentTurnIndex.value > 0)
const canGoNext = computed(() => currentTurnIndex.value < turnAnchors.value.length - 1)
const navOpacity = computed(() => settingsStore.bgOpacity)

function updateCurrentTurn() {
  const container = messagesContainer.value
  if (!container || turnAnchors.value.length === 0) return
  const anchors = turnAnchors.value
  const containerRect = container.getBoundingClientRect()
  const refPoint = containerRect.top + containerRect.height * 0.3
  for (let i = anchors.length - 1; i >= 0; i--) {
    const el = container.querySelector(`[data-msg-index="${anchors[i]}"]`) as HTMLElement
    if (el && el.getBoundingClientRect().top <= refPoint) {
      currentTurnIndex.value = i
      return
    }
  }
  currentTurnIndex.value = 0
}

function onMessagesScroll() {
  if (isNavigating) return
  if (scrollUpdateRaf) return
  scrollUpdateRaf = requestAnimationFrame(() => {
    scrollUpdateRaf = 0
    updateCurrentTurn()
  })
}

function scrollToTop() {
  const container = messagesContainer.value
  if (!container) return
  isNavigating = true
  smoothScrollTo(container, 0, 300, () => { isNavigating = false })
  currentTurnIndex.value = 0
}

function smoothScrollTo(container: HTMLElement, targetScrollTop: number, duration = 250, onComplete?: () => void) {
  const startTop = container.scrollTop
  const distance = targetScrollTop - startTop
  if (Math.abs(distance) < 5) {
    onComplete?.()
    return
  }
  const startTime = performance.now()
  function step(currentTime: number) {
    const elapsed = currentTime - startTime
    const progress = Math.min(elapsed / duration, 1)
    const eased = 1 - Math.pow(1 - progress, 3)
    container.scrollTop = startTop + distance * eased
    if (progress < 1) requestAnimationFrame(step)
    else onComplete?.()
  }
  requestAnimationFrame(step)
}

function navigateToTurn(direction: 'prev' | 'next') {
  const anchors = turnAnchors.value
  if (anchors.length === 0) return
  const newIndex = direction === 'prev'
    ? Math.max(0, currentTurnIndex.value - 1)
    : Math.min(anchors.length - 1, currentTurnIndex.value + 1)
  if (newIndex === currentTurnIndex.value) return
  const msgIdx = anchors[newIndex]
  const container = messagesContainer.value
  if (!container) return
  const el = container.querySelector(`[data-msg-index="${msgIdx}"]`) as HTMLElement
  if (!el) return
  const containerRect = container.getBoundingClientRect()
  const elRect = el.getBoundingClientRect()
  const targetTop = container.scrollTop + (elRect.top - containerRect.top) - 16
  isNavigating = true
  smoothScrollTo(container, Math.max(0, targetTop), 250, () => { isNavigating = false })
  currentTurnIndex.value = newIndex
}
// --- End turn navigation ---

// --- Search integration ---
function scrollToCurrentMatch() {
  const msgIdx = searchStore.currentMsgIdx
  if (msgIdx < 0 || !messagesContainer.value) return
  const container = messagesContainer.value
  const el = container.querySelector(`[data-msg-index="${msgIdx}"]`) as HTMLElement
  if (!el) return
  const containerRect = container.getBoundingClientRect()
  const elRect = el.getBoundingClientRect()
  const targetTop = container.scrollTop + (elRect.top - containerRect.top) - containerRect.height / 3
  isNavigating = true
  smoothScrollTo(container, Math.max(0, targetTop), 250, () => { isNavigating = false })
}

watch(() => searchStore.currentPos, () => {
  nextTick(() => scrollToCurrentMatch())
})

watch(() => searchStore.debouncedQuery, (q) => {
  if (q.trim()) {
    searchStore.computeMatches(messages.value)
    nextTick(() => scrollToCurrentMatch())
  }
})

watch(() => messages.value.length, () => {
  if (searchStore.debouncedQuery.trim()) {
    searchStore.computeMatches(messages.value)
  }
})
// --- End search integration ---

watch(() => sessionStore.currentSessionId, async (newId) => {
  if (searchStore.isOpen) searchStore.closeSearch()
  if (newId) {
    await messageStore.loadHistory(newId)
    await nextTick()
    scrollToBottom()
    currentTurnIndex.value = Math.max(0, turnAnchors.value.length - 1)
  }
})

watch(() => messages.value.length, async () => {
  await nextTick()
  scrollToBottom()
  currentTurnIndex.value = Math.max(0, turnAnchors.value.length - 1)
})

// RAF-throttled scroll during streaming — only scroll once per animation frame
let scrollRafId = 0

watch(() => messageStore.streamingContent, () => {
  if (scrollRafId) return // already scheduled
  scrollRafId = requestAnimationFrame(() => {
    scrollRafId = 0
    scrollToBottom()
  })
})

function scrollToBottom() {
  const el = messagesContainer.value
  if (el) {
    el.scrollTop = el.scrollHeight
  }
}

// --- File drag-and-drop ---
function onDragEnter(e: DragEvent) {
  e.preventDefault()
  if (e.dataTransfer?.types.includes('Files')) {
    isDragOver.value = true
  }
}

function onDragOver(e: DragEvent) {
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
}

function onDragLeave(e: DragEvent) {
  e.preventDefault()
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  if (
    e.clientX <= rect.left || e.clientX >= rect.right ||
    e.clientY <= rect.top || e.clientY >= rect.bottom
  ) {
    isDragOver.value = false
  }
}
// --- End file drag-and-drop ---

async function send(content: string, images: string[] = []) {
  const sid = sessionStore.currentSessionId
  if (!sid) return
  await messageStore.sendMessage(sid, content, images)
}

async function retryMessage(messageId: string) {
  const sid = sessionStore.currentSessionId
  if (!sid) return
  await messageStore.retryMessage(sid, messageId)
}

async function retryFromUser(messageId: string) {
  const sid = sessionStore.currentSessionId
  if (!sid) return
  await messageStore.retryFromUserMessage(sid, messageId)
}

async function editAndResend(messageId: string, newContent: string, newImages: string[]) {
  const sid = sessionStore.currentSessionId
  if (!sid) return
  await messageStore.editAndResendMessage(sid, messageId, newContent, newImages)
}

function onQuote(content: string) {
  quoteText.value = content
}
</script>

<template>
  <div class="chat-container relative flex flex-col h-full" style="--wails-drop-target: drop"
    @dragenter="onDragEnter" @dragover="onDragOver"
    @dragleave="onDragLeave">
    <!-- Header -->
    <div v-if="currentSession" class="chat-header px-6 py-3 border-b border-slate-200 dark:border-slate-700 bg-white/50 dark:bg-slate-800/50 flex items-center gap-3">
      <h2 class="chat-title text-lg font-semibold truncate">{{ currentSession.name }}</h2>
      <div v-if="currentProvider" class="model-picker relative" ref="modelPickerRef">
        <button
          @click.stop="toggleModelPicker"
          class="model-picker-btn text-xs px-2 py-1 bg-slate-100 dark:bg-slate-700 rounded-full text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-600 transition-colors flex items-center gap-1"
        >
          {{ currentProvider.name }} / {{ currentSession.model }}
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
        </button>
        <div v-if="showModelPicker" class="model-dropdown absolute top-full left-0 mt-1 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-600 rounded-lg shadow-lg z-50 min-w-[200px] max-h-[320px] overflow-y-auto py-1">
          <template v-for="p in providerStore.providers" :key="p.id">
            <div class="provider-group-label px-3 py-1.5 text-xs font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-wider">{{ p.name }}</div>
            <button
              v-for="m in p.models"
              :key="p.id + '/' + m"
              @click="selectModel(p.id, m)"
              :class="['model-option w-full text-left px-3 py-1.5 text-sm hover:bg-blue-50 dark:hover:bg-slate-700 transition-colors flex items-center justify-between',
                currentSession && p.id === currentSession.provider_id && m === currentSession.model ? 'text-blue-600 dark:text-blue-400 font-medium' : 'text-slate-700 dark:text-slate-300']"
            >
              <span class="truncate">{{ m }}</span>
              <svg v-if="currentSession && p.id === currentSession.provider_id && m === currentSession.model" class="w-4 h-4 flex-shrink-0 ml-2" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/></svg>
            </button>
          </template>
        </div>
      </div>
      <div v-if="currentProvider && promptStore.prompts.length > 0" class="prompt-picker relative" ref="promptPickerRef">
        <button
          @click.stop="togglePromptPicker"
          class="text-xs px-2 py-1 bg-slate-100 dark:bg-slate-700 rounded-full text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-600 transition-colors flex items-center gap-1"
        >
          {{ currentPromptName }}
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
        </button>
        <div v-if="showPromptPicker" class="absolute top-full left-0 mt-1 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-600 rounded-lg shadow-lg z-50 min-w-[180px] max-h-[280px] overflow-y-auto py-1">
          <button
            @click="selectPrompt(null)"
            :class="['w-full text-left px-3 py-1.5 text-sm hover:bg-blue-50 dark:hover:bg-slate-700 transition-colors flex items-center justify-between',
              !currentSession?.prompt_id ? 'text-blue-600 dark:text-blue-400 font-medium' : 'text-slate-700 dark:text-slate-300']"
          >
            <span>Use Default</span>
            <svg v-if="!currentSession?.prompt_id" class="w-4 h-4 flex-shrink-0 ml-2" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/></svg>
          </button>
          <div v-if="promptStore.prompts.length > 0" class="border-t border-slate-200 dark:border-slate-600 my-1"></div>
          <button
            v-for="p in promptStore.prompts"
            :key="p.id"
            @click="selectPrompt(p.id)"
            :class="['w-full text-left px-3 py-1.5 text-sm hover:bg-blue-50 dark:hover:bg-slate-700 transition-colors flex items-center justify-between',
              currentSession?.prompt_id === p.id ? 'text-blue-600 dark:text-blue-400 font-medium' : 'text-slate-700 dark:text-slate-300']"
          >
            <span class="truncate">{{ p.name }}<span v-if="p.is_default" class="text-slate-400 ml-1">(default)</span></span>
            <svg v-if="currentSession?.prompt_id === p.id" class="w-4 h-4 flex-shrink-0 ml-2" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/></svg>
          </button>
        </div>
      </div>
    </div>

    <!-- Messages area with navigation overlay -->
    <div class="relative flex-1 min-h-0">
      <!-- Search bar (Ctrl+F) -->
      <SearchBar v-if="searchStore.isOpen" />
      <!-- Drop overlay -->
      <div v-if="isDragOver"
        class="absolute inset-0 z-50 flex items-center justify-center bg-blue-500/10 dark:bg-blue-400/10 border-2 border-dashed border-blue-400 dark:border-blue-500 rounded-lg pointer-events-none">
        <div class="text-blue-600 dark:text-blue-400 text-lg font-medium">Drop files here</div>
      </div>
      <div ref="messagesContainer" class="chat-messages h-full overflow-y-auto">
        <div v-if="!currentSession" class="empty-state flex items-center justify-center h-full text-slate-400 dark:text-slate-500">
          <div class="text-center">
            <img :src="logoUrl" alt="WailsChat" class="w-40 h-40 mx-auto mb-4 rounded-2xl app-logo" />
            <p class="text-lg font-semibold text-slate-600 dark:text-slate-300">WailsChat</p>
            <p class="text-sm mt-2">Select a chat or create a new one to get started</p>
          </div>
        </div>

        <div v-else class="messages-list mx-auto px-6 py-4 space-y-4" :style="{maxWidth: chatWidthPx}">
          <div v-for="(msg, index) in messages" :key="msg.id" :data-msg-index="index">
            <MessageBubble
              v-memo="[msg.content, msg.images, msg.reasoning_content, msg.id === lastAssistantMessageId, settingsStore.showMessageTime, searchStore.debouncedQuery]"
              :message="msg"
              :stats="statsMap.get(msg.id)"
              :is-last-assistant="msg.id === lastAssistantMessageId"
              :search-query="searchStore.debouncedQuery"
              @retry="retryMessage"
              @retry-from-user="retryFromUser"
              @edit-and-resend="editAndResend"
              @quote="onQuote"
            />
          </div>
          <!-- Streaming message -->
          <MessageBubble
            v-if="messageStore.isStreaming && (messageStore.streamingContent || messageStore.streamingReasoning)"
            :message="{
              id: 'streaming',
              session_id: sessionStore.currentSessionId!,
              role: 'assistant',
              content: messageStore.streamingContent,
              created_at: '',
            }"
            :streaming="true"
            :streaming-reasoning="messageStore.streamingReasoning"
          />
          <!-- Loading indicator (no content and no reasoning yet) -->
          <div v-if="messageStore.isStreaming && !messageStore.streamingContent && !messageStore.streamingReasoning" class="loading-indicator flex justify-start">
            <div class="thinking-bubble bg-slate-100 dark:bg-slate-700 rounded-2xl px-4 py-3 text-slate-500 dark:text-slate-400">
              <span class="animate-pulse">Thinking...</span>
            </div>
          </div>
          <!-- Active MCP tool calls during streaming -->
          <div v-if="messageStore.isStreaming && messageStore.getActiveToolCalls(sessionStore.currentSessionId!).length > 0" class="mcp-tools-loading flex justify-start">
            <div class="mcp-loading-badge bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-2xl px-4 py-3 text-blue-600 dark:text-blue-400">
              <div class="flex items-center gap-2">
                <Loader2 class="w-4 h-4 animate-spin" />
                <span class="text-sm">Calling MCP tools...</span>
              </div>
              <div class="mt-1 text-xs text-blue-500 dark:text-blue-300">
                {{ messageStore.getActiveToolCalls(sessionStore.currentSessionId!).map(t => t.name).join(', ') }}
              </div>
            </div>
          </div>
        </div>
      </div>
      <!-- Turn navigation buttons -->
      <div v-if="showTurnNav" class="turn-nav">
        <button
          v-show="canGoPrev"
          class="turn-nav-btn w-9 h-9 rounded-full flex items-center justify-center bg-white/80 dark:bg-slate-700/80 border border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-300 backdrop-blur-sm cursor-pointer transition-all duration-200 hover:bg-white/95 dark:hover:bg-slate-700/95 hover:shadow-md"
          :style="{'--nav-opacity': navOpacity}"
          @click="navigateToTurn('prev')"
        >
          <ChevronUp class="w-5 h-5" />
        </button>
        <button
          v-show="canGoNext"
          class="turn-nav-btn w-9 h-9 rounded-full flex items-center justify-center bg-white/80 dark:bg-slate-700/80 border border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-300 backdrop-blur-sm cursor-pointer transition-all duration-200 hover:bg-white/95 dark:hover:bg-slate-700/95 hover:shadow-md"
          :style="{'--nav-opacity': navOpacity}"
          @click="navigateToTurn('next')"
        >
          <ChevronDown class="w-5 h-5" />
        </button>
      </div>
      <!-- Scroll to top button -->
      <div v-if="messages.length > 0" class="scroll-top-nav">
        <button
          class="turn-nav-btn w-9 h-9 rounded-full flex items-center justify-center bg-white/80 dark:bg-slate-700/80 border border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-300 backdrop-blur-sm cursor-pointer transition-all duration-200 hover:bg-white/95 dark:hover:bg-slate-700/95 hover:shadow-md"
          :style="{'--nav-opacity': navOpacity}"
          @click="scrollToTop"
        >
          <ArrowUp class="w-5 h-5" />
        </button>
      </div>
    </div>

    <!-- Input -->
    <ChatInput
      v-if="currentSession"
      ref="chatInputRef"
      :disabled="messageStore.isStreaming || editingMessageId !== null"
      @send="send"
      @cancel="messageStore.cancelStream"
      :is-streaming="messageStore.isStreaming"
      :quote-text="quoteText"
      @quote-consumed="quoteText = ''"
    />
  </div>
</template>

<style scoped>
.turn-nav {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  flex-direction: column;
  gap: 8px;
  z-index: 10;
}
.turn-nav-btn {
  opacity: var(--nav-opacity, 0.5);
}
.turn-nav-btn:hover {
  opacity: 1 !important;
}
.scroll-top-nav {
  position: absolute;
  right: 12px;
  bottom: 16px;
  z-index: 10;
}
</style>
