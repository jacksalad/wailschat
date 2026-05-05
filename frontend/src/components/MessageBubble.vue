<script lang="ts" setup>
import {computed, ref, inject, nextTick, type Ref} from 'vue'
import MarkdownMessage from './MarkdownMessage.vue'
import ContextMenu, {type MenuItem} from './ContextMenu.vue'
import {Copy, RefreshCw, BarChart3, Check, X, ChevronDown, ChevronRight, Loader2, Wrench, Folder, Terminal, Brain, ClipboardCopy, Quote, Pencil} from 'lucide-vue-next'
import type {PerformanceStats, MCPToolCall, MCPToolResult} from '../stores/message'
import {useSettingsStore} from '../stores/settings'
import {formatRelativeTime} from '../utils/format'

const props = defineProps<{
  message: {
    id: string | number
    session_id: number
    role: string
    content: string
    reasoning_content?: string
    images?: string
    created_at: string
    tool_calls?: MCPToolCall[]
    tool_results?: MCPToolResult[]
  }
  streaming?: boolean
  streamingReasoning?: string
  stats?: PerformanceStats
  isLastAssistant?: boolean
  searchQuery?: string
}>()

const emit = defineEmits<{
  retry: [messageId: string]
  retryFromUser: [messageId: string]
  quote: [content: string]
  editAndResend: [messageId: string, newContent: string, newImages: string[]]
}>()

const isUser = computed(() => props.message.role === 'user')
const settingsStore = useSettingsStore()
const showMessageTime = computed(() => settingsStore.showMessageTime === '1' || settingsStore.showMessageTime === 'true')
const images = computed(() => {
  if (!props.message.images) return []
  try {
    return JSON.parse(props.message.images)
  } catch {
    return []
  }
})

// Reasoning content: from saved message or from streaming state
const reasoningText = computed(() => {
  return props.message.reasoning_content || props.streamingReasoning || ''
})

const hasReasoning = computed(() => reasoningText.value.length > 0)

// Search highlighting for user messages (plain text)
const userHighlightedSegments = computed(() => {
  const q = props.searchQuery?.trim()
  if (!q || !isUser.value) return [{text: props.message.content, highlight: false}]
  const segments: {text: string; highlight: boolean}[] = []
  const content = props.message.content
  const lower = content.toLowerCase()
  const queryLower = q.toLowerCase()
  let lastIdx = 0
  let searchFrom = 0
  while (searchFrom < content.length) {
    const idx = lower.indexOf(queryLower, searchFrom)
    if (idx === -1) break
    if (idx > lastIdx) segments.push({text: content.slice(lastIdx, idx), highlight: false})
    segments.push({text: content.slice(idx, idx + q.length), highlight: true})
    lastIdx = idx + q.length
    searchFrom = lastIdx
  }
  if (lastIdx < content.length) segments.push({text: content.slice(lastIdx), highlight: false})
  if (segments.length === 0) segments.push({text: content, highlight: false})
  return segments
})

// Reasoning section state
const showReasoning = ref(false)

// Auto-expand reasoning during streaming, collapse when done
// Default: collapsed for saved messages, expanded during streaming
const reasoningExpanded = computed(() => {
  if (props.streaming) return true
  return showReasoning.value
})

// Format reasoning content length for display
function formatReasoningLength(text: string): string {
  const len = text.length
  if (len < 1000) return `${len} chars`
  if (len < 1000000) return `${(len / 1000).toFixed(1)}K chars`
  return `${(len / 1000000).toFixed(1)}M chars`
}

// MCP tool calls state
const showToolCalls = ref(false)

// Copy state
const copied = ref(false)
async function copyContent() {
  try {
    await navigator.clipboard.writeText(props.message.content)
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  } catch (e) {
    console.error('Copy failed:', e)
  }
}

// Edit state (shared via provide/inject to ensure only one edit at a time)
const editingMessageId = inject<Ref<string | null>>('editingMessageId', ref(null))
const isEditing = computed(() => isUser.value && editingMessageId.value === String(props.message.id))
const editContent = ref('')
const editTextarea = ref<HTMLTextAreaElement | null>(null)

function startEdit() {
  editingMessageId.value = String(props.message.id)
  editContent.value = props.message.content
  nextTick(() => {
    autoResizeEdit()
    editTextarea.value?.focus()
  })
}

function cancelEdit() {
  editingMessageId.value = null
  editContent.value = ''
}

function saveEdit() {
  const trimmed = editContent.value.trim()
  if (!trimmed && images.value.length === 0) return
  emit('editAndResend', String(props.message.id), trimmed, images.value)
  editingMessageId.value = null
  editContent.value = ''
}

function autoResizeEdit() {
  const el = editTextarea.value
  if (!el) return
  el.style.height = 'auto'
  const newHeight = Math.min(el.scrollHeight, 200)
  el.style.height = newHeight + 'px'
  el.style.overflowY = newHeight >= 200 ? 'auto' : 'hidden'
}

function handleEditKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    saveEdit()
  }
  if (e.key === 'Escape') {
    cancelEdit()
  }
}

// Stats popup
const showStats = ref(false)

function formatTime(seconds: number): string {
  if (seconds < 1) return (seconds * 1000).toFixed(0) + 'ms'
  return seconds.toFixed(1) + 's'
}

function formatDuration(ms: number): string {
  if (ms < 1000) return ms + 'ms'
  return (ms / 1000).toFixed(2) + 's'
}

// Extract clean tool name from "serverID___toolName" format
function cleanToolName(fqName: string): string {
  const idx = fqName.indexOf('___')
  if (idx >= 0 && idx + 3 < fqName.length) {
    return fqName.substring(idx + 3)
  }
  return fqName
}

// Get icon component for tool type
function getToolIcon(toolName: string) {
  if (toolName === 'file_read' || toolName === 'file_write') {
    return Folder
  }
  if (toolName === 'shell_exec') {
    return Terminal
  }
  return Wrench
}

// Check if tool is built-in
function isBuiltInTool(toolName: string): boolean {
  return ['file_read', 'file_write', 'shell_exec'].includes(toolName)
}

function formatArgs(args: string): string {
  if (!args) return ''
  try {
    return JSON.stringify(JSON.parse(args), null, 2)
  } catch {
    return args
  }
}

function formatResult(result: string): string {
  try {
    const parsed = JSON.parse(result)
    if (typeof parsed === 'object') {
      return JSON.stringify(parsed, null, 2)
    }
    return result
  } catch {
    return result
  }
}

// Context menu
const menuVisible = ref(false)
const menuX = ref(0)
const menuY = ref(0)
const menuItems = ref<MenuItem[]>([])
const markdownRef = ref<InstanceType<typeof MarkdownMessage> | null>(null)
const menuSelectedText = ref('')

function onContextMenu(e: MouseEvent) {
  e.preventDefault()
  menuX.value = e.clientX
  menuY.value = e.clientY

  const sel = window.getSelection()
  const selected = sel?.toString().trim() || ''
  menuSelectedText.value = selected

  const copyTarget = selected || props.message.content
  const quoteTarget = selected || props.message.content

  if (isUser.value) {
    menuItems.value = [
      { label: 'Edit', icon: Pencil, action: () => startEdit() },
      { label: 'Copy', icon: Copy, action: () => { navigator.clipboard.writeText(copyTarget).catch(() => {}) } },
      { label: 'Quote', icon: Quote, action: () => emit('quote', quoteTarget) },
      { divider: true },
      { label: 'Retry from here', icon: RefreshCw, action: () => emit('retryFromUser', String(props.message.id)) },
    ]
  } else {
    const items: MenuItem[] = [
      { label: 'Copy', icon: Copy, action: () => { navigator.clipboard.writeText(selected || props.message.content).catch(() => {}) } },
      { label: 'Copy Text', icon: ClipboardCopy, action: () => { navigator.clipboard.writeText(selected || getPlainText()).catch(() => {}) } },
      { label: 'Quote', icon: Quote, action: () => emit('quote', quoteTarget) },
    ]
    if (props.isLastAssistant && !props.streaming) {
      items.push({ divider: true })
      items.push({ label: 'Retry', icon: RefreshCw, action: () => emit('retry', String(props.message.id)) })
    }
    menuItems.value = items
  }
  menuVisible.value = true
}

function getPlainText(): string {
  const el = markdownRef.value?.$el as HTMLElement | undefined
  return (el?.textContent || '').trim()
}


</script>

<template>
  <div class="message-wrapper flex flex-col">
    <div v-if="showMessageTime && message.created_at" class="text-center text-[10px] text-slate-400 dark:text-slate-500 mb-1 mt-2">
      {{ formatRelativeTime(message.created_at) }}
    </div>
    <div :class="['flex', isUser ? 'justify-end' : 'justify-start']">
    <div :class="[
      'message-bubble rounded-2xl px-4 py-3',
      isEditing
        ? 'w-[80%]'
        : 'max-w-[85%]',
      isUser
        ? 'user-bubble bg-blue-600/50 text-white'
        : 'ai-bubble bg-slate-100/50 dark:bg-slate-700/50 text-slate-800 dark:text-slate-200',
    ]" @contextmenu="onContextMenu">
      <!-- User message: view mode or edit mode -->
      <template v-if="isUser">
        <!-- View mode -->
        <template v-if="!isEditing">
          <div v-if="images.length > 0" class="message-images flex flex-wrap gap-2 mb-2">
            <img
              v-for="(img, index) in images"
              :key="index"
              :src="img"
              class="max-w-[200px] max-h-[200px] rounded-lg object-contain"
              alt="Attached image"
              loading="lazy"
              decoding="async"
            />
          </div>
          <div class="message-content whitespace-pre-wrap"><template v-for="(seg, i) in userHighlightedSegments" :key="i"><mark v-if="seg.highlight" class="search-match">{{ seg.text }}</mark><span v-else>{{ seg.text }}</span></template></div>
          <!-- Action buttons for user messages (bottom-right) -->
          <div class="message-actions flex justify-end items-center gap-1 mt-1">
            <button
              v-if="!streaming"
              @click="startEdit"
              class="edit-btn p-1 rounded-md text-blue-300 hover:text-white hover:bg-blue-500/40 transition-colors"
              title="Edit"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              @click="copyContent"
              class="copy-btn p-1 rounded-md text-blue-300 hover:text-white hover:bg-blue-500/40 transition-colors"
              title="Copy"
            >
              <Check v-if="copied" class="w-3.5 h-3.5 text-green-300" />
              <Copy v-else class="w-3.5 h-3.5" />
            </button>
            <button
              @click="emit('retryFromUser', String(message.id))"
              class="retry-btn p-1 rounded-md text-blue-300 hover:text-white hover:bg-blue-500/40 transition-colors"
              title="Retry from here"
            >
              <RefreshCw class="w-3.5 h-3.5" />
            </button>
          </div>
        </template>
        <!-- Edit mode -->
        <template v-else>
          <div v-if="images.length > 0" class="message-images flex flex-wrap gap-2 mb-2">
            <img
              v-for="(img, index) in images"
              :key="index"
              :src="img"
              class="max-w-[200px] max-h-[200px] rounded-lg object-contain"
              alt="Attached image"
              loading="lazy"
              decoding="async"
            />
          </div>
          <textarea
            ref="editTextarea"
            v-model="editContent"
            @input="autoResizeEdit"
            @keydown="handleEditKeydown"
            class="w-full bg-white/20 border border-blue-300/50 rounded-lg px-3 py-2 text-white placeholder-blue-200/50 resize-none focus:outline-none focus:border-blue-200/70 text-sm"
            rows="1"
            placeholder="Edit message..."
          ></textarea>
          <div class="flex justify-end items-center gap-2 mt-2">
            <button
              @click="cancelEdit"
              class="px-3 py-1.5 text-xs rounded-lg text-blue-200 hover:text-white hover:bg-blue-500/30 transition-colors"
            >
              Cancel
            </button>
            <button
              @click="saveEdit"
              :disabled="!editContent.trim() && images.length === 0"
              class="px-3 py-1.5 text-xs rounded-lg bg-blue-500/60 hover:bg-blue-500/80 text-white disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Save &amp; Submit
            </button>
          </div>
        </template>
      </template>
      <!-- Assistant message content -->
      <template v-else>
        <div v-if="images.length > 0" class="message-images flex flex-wrap gap-2 mb-2">
          <img
            v-for="(img, index) in images"
            :key="index"
            :src="img"
            class="max-w-[200px] max-h-[200px] rounded-lg object-contain"
            alt="Attached image"
            loading="lazy"
            decoding="async"
          />
        </div>
        <div class="message-content-wrapper">
        <!-- Reasoning section (collapsible) -->
        <div v-if="hasReasoning" class="reasoning-section mb-3">
          <button
            @click="showReasoning = !showReasoning"
            class="reasoning-toggle flex items-center gap-1.5 text-xs font-medium w-full text-left py-1.5 px-2 rounded-lg hover:bg-slate-200/50 dark:hover:bg-slate-600/50 transition-colors"
          >
            <Brain class="w-3.5 h-3.5 text-violet-500 dark:text-violet-400" />
            <span class="text-violet-600 dark:text-violet-400">
              <template v-if="streaming && !message.content">Thinking</template>
              <template v-else>Thought for {{ formatReasoningLength(reasoningText) }}</template>
            </span>
            <Loader2 v-if="streaming && !message.content" class="w-3 h-3 animate-spin text-violet-500 dark:text-violet-400" />
            <component :is="reasoningExpanded ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500 ml-auto" />
          </button>
          <div v-if="reasoningExpanded" class="reasoning-content mt-1.5 pl-2 border-l-2 border-violet-300/50 dark:border-violet-600/50">
            <div class="text-xs text-slate-500 dark:text-slate-400 italic whitespace-pre-wrap max-h-[300px] overflow-y-auto">{{ reasoningText }}</div>
          </div>
        </div>
        <!-- Main content -->
        <MarkdownMessage ref="markdownRef" :content="message.content" :streaming="streaming" :search-query="searchQuery" class="message-content" />
        <span v-if="streaming && message.content" class="typing-cursor-bar"></span>
        </div>

        <!-- Action buttons for assistant messages -->
        <div v-if="!streaming" class="message-actions flex items-center gap-1 mt-2 -mb-1 relative">
        <button
          @click="copyContent"
          class="copy-btn p-1 rounded-md text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-200 dark:hover:bg-slate-600 transition-colors"
          title="Copy"
        >
          <Check v-if="copied" class="w-3.5 h-3.5 text-green-500" />
          <Copy v-else class="w-3.5 h-3.5" />
        </button>
        <button
          v-if="isLastAssistant"
          @click="emit('retry', String(message.id))"
          class="retry-btn p-1 rounded-md text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-200 dark:hover:bg-slate-600 transition-colors"
          title="Retry"
        >
          <RefreshCw class="w-3.5 h-3.5" />
        </button>
        <button
          @click="showStats = !showStats"
          :class="[
            'stats-btn p-1 rounded-md transition-colors',
            showStats
              ? 'text-blue-500 bg-blue-50 dark:bg-blue-900/30'
              : 'text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-200 dark:hover:bg-slate-600'
          ]"
          title="Performance"
        >
          <BarChart3 class="w-3.5 h-3.5" />
        </button>

        <!-- Stats popup -->
        <div
          v-if="showStats"
          class="stats-popup absolute bottom-full left-0 mb-2 bg-white/50 dark:bg-slate-800/50 rounded-lg shadow-lg border border-slate-200 dark:border-slate-600 p-3 min-w-[180px] z-50"
        >
          <div class="flex items-center justify-between mb-2">
            <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">Performance</span>
            <button @click="showStats = false" class="text-slate-400 hover:text-slate-600 dark:hover:text-white">
              <X class="w-3 h-3" />
            </button>
          </div>
          <div v-if="stats" class="space-y-1.5 text-xs">
            <div class="flex justify-between">
              <span class="text-slate-500 dark:text-slate-400">Input tokens</span>
              <span class="font-mono text-slate-800 dark:text-slate-200">{{ stats.input_tokens || '—' }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-slate-500 dark:text-slate-400">Output tokens</span>
              <span class="font-mono text-slate-800 dark:text-slate-200">{{ stats.output_tokens || '—' }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-slate-500 dark:text-slate-400">First token</span>
              <span class="font-mono text-slate-800 dark:text-slate-200">{{ stats.first_token_time ? formatTime(stats.first_token_time) : '—' }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-slate-500 dark:text-slate-400">Total time</span>
              <span class="font-mono text-slate-800 dark:text-slate-200">{{ stats.total_time ? formatTime(stats.total_time) : '—' }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-slate-500 dark:text-slate-400">Speed</span>
              <span class="font-mono text-slate-800 dark:text-slate-200">{{ stats.speed ? stats.speed.toFixed(1) + ' T/s' : '—' }}</span>
            </div>
          </div>
          <div v-else class="text-xs text-slate-400 dark:text-slate-500">
            No stats available for this message
          </div>
        </div>
      </div>

      <!-- Tool Calls Section -->
      <div v-if="!isUser && message.tool_calls && message.tool_calls.length > 0" class="tool-calls-panel mt-3 border-t border-slate-200 dark:border-slate-600 pt-3">
        <button
          @click="showToolCalls = !showToolCalls"
          class="tool-calls-toggle flex items-center gap-1.5 text-xs font-medium text-slate-600 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200 transition-colors"
        >
          <component :is="showToolCalls ? ChevronDown : ChevronRight" class="w-3.5 h-3.5" />
          <Wrench class="w-3.5 h-3.5" />
          <span>Tools ({{ message.tool_calls.length }})</span>
        </button>

        <div v-if="showToolCalls" class="tool-calls-list mt-2 space-y-2">
          <div
            v-for="(toolCall, idx) in message.tool_calls"
            :key="toolCall.id || idx"
            class="tool-call-item bg-slate-50 dark:bg-slate-800 rounded-lg p-2 text-xs"
          >
            <div class="flex items-center justify-between mb-1">
              <div class="flex items-center gap-1.5 min-w-0">
                <component :is="getToolIcon(cleanToolName(toolCall.name))" class="w-3 h-3 flex-shrink-0" :class="[
                  isBuiltInTool(cleanToolName(toolCall.name)) ? 'text-purple-500 dark:text-purple-400' : 'text-blue-600 dark:text-blue-400'
                ]" />
                <span class="tool-call-name font-mono font-medium truncate" :class="[
                  isBuiltInTool(cleanToolName(toolCall.name)) ? 'text-purple-600 dark:text-purple-400' : 'text-blue-600 dark:text-blue-400'
                ]">{{ cleanToolName(toolCall.name) }}</span>
                <span v-if="toolCall.server_name || message.tool_results?.[idx]?.server_name" class="text-[10px] text-slate-400 dark:text-slate-500 shrink-0">({{ toolCall.server_name || message.tool_results?.[idx]?.server_name }})</span>
              </div>
              <span v-if="message.tool_results?.[idx]" :class="[
                'tool-call-status text-[10px] px-1.5 py-0.5 rounded shrink-0',
                message.tool_results[idx].error
                  ? 'bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400'
                  : 'bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400'
              ]">
                {{ message.tool_results[idx].error ? 'Error' : 'Success' }}
                ({{ formatDuration(message.tool_results[idx].duration_ms) }})
              </span>
            </div>
            <div class="text-slate-500 dark:text-slate-400 mb-1">Arguments:</div>
            <pre class="tool-call-args bg-slate-100 dark:bg-slate-700 rounded p-1.5 overflow-x-auto text-[11px] text-slate-700 dark:text-slate-300">{{ formatArgs(toolCall.arguments) }}</pre>
            <template v-if="message.tool_results?.[idx]">
              <div class="text-slate-500 dark:text-slate-400 mt-1 mb-1">Result:</div>
              <pre :class="[
                'tool-call-result bg-slate-100 dark:bg-slate-700 rounded p-1.5 overflow-x-auto text-[11px]',
                message.tool_results[idx].error
                  ? 'text-red-600 dark:text-red-400'
                  : 'text-slate-700 dark:text-slate-300'
              ]">{{ message.tool_results[idx].error || formatResult(message.tool_results[idx].result) }}</pre>
            </template>
          </div>
        </div>
      </div>
      </template>
    </div>
    </div>
    <ContextMenu :visible="menuVisible" :x="menuX" :y="menuY" :items="menuItems" @close="menuVisible = false" />
  </div>
</template>

<style scoped>
.typing-cursor-bar {
  display: inline-block;
  width: 2px;
  height: 1em;
  margin-left: 2px;
  vertical-align: text-bottom;
  background: #60a5fa;
  animation: cursor-blink 1s infinite;
}
@keyframes cursor-blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}
:deep(.search-match) {
  background-color: rgba(255, 212, 59, 0.4);
  border-radius: 2px;
  padding: 0 1px;
}
</style>
