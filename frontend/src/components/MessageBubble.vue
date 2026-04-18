<script lang="ts" setup>
import {computed, ref} from 'vue'
import MarkdownMessage from './MarkdownMessage.vue'
import {Copy, RefreshCw, BarChart3, Check, X, ChevronDown, ChevronRight, Loader2, Wrench, Folder, Terminal} from 'lucide-vue-next'
import type {PerformanceStats, MCPToolCall, MCPToolResult} from '../stores/message'

const props = defineProps<{
  message: {
    id: string | number
    session_id: number
    role: string
    content: string
    images?: string
    created_at: string
    tool_calls?: MCPToolCall[]
    tool_results?: MCPToolResult[]
  }
  streaming?: boolean
  stats?: PerformanceStats
  isLastAssistant?: boolean
}>()

const emit = defineEmits<{
  retry: [messageId: string]
  retryFromUser: [messageId: string]
}>()

const isUser = computed(() => props.message.role === 'user')
const images = computed(() => {
  if (!props.message.images) return []
  try {
    return JSON.parse(props.message.images)
  } catch {
    return []
  }
})

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
</script>

<template>
  <div :class="['message-wrapper flex', isUser ? 'justify-end' : 'justify-start']">
    <div :class="[
      'message-bubble max-w-[85%] rounded-2xl px-4 py-3',
      isUser
        ? 'user-bubble bg-blue-600/50 text-white'
        : 'ai-bubble bg-slate-100/50 dark:bg-slate-700/50 text-slate-800 dark:text-slate-200',
    ]">
      <!-- Images -->
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
      <!-- Text content -->
      <div v-if="isUser" class="message-content whitespace-pre-wrap">{{ message.content }}</div>
      <div v-else class="message-content-wrapper">
        <MarkdownMessage :content="message.content" :streaming="streaming" class="message-content" />
        <span v-if="streaming" class="typing-cursor-bar"></span>
      </div>

      <!-- Action buttons for user messages (bottom-right) -->
      <div v-if="isUser" class="message-actions flex justify-end items-center gap-1 mt-1">
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

      <!-- Action buttons for assistant messages -->
      <div v-if="!isUser && !streaming" class="message-actions flex items-center gap-1 mt-2 -mb-1 relative">
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
    </div>
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
</style>
