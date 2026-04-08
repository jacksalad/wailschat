<script lang="ts" setup>
import { ref, nextTick, computed, onMounted, onUnmounted } from 'vue'
import { useSettingsStore } from '../stores/settings'
import { Send, Square, ImagePlus } from 'lucide-vue-next'
import { extractFilename } from '../utils/filepath'

interface FileRef {
  path: string
  filename: string
}

const props = defineProps<{
  disabled: boolean
  isStreaming: boolean
}>()

const emit = defineEmits<{
  (e: 'send', content: string, images: string[]): void
  (e: 'cancel'): void
}>()

const settingsStore = useSettingsStore()
const inputText = ref('')
const textarea = ref<HTMLTextAreaElement | null>(null)
const selectedImages = ref<string[]>([])
const fileRefs = ref<FileRef[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const chatWidthPx = computed(() => settingsStore.chatWidth + 'px')

function addFiles(paths: string[]) {
  for (const path of paths) {
    if (path && !fileRefs.value.some(f => f.path === path)) {
      fileRefs.value.push({ path, filename: extractFilename(path) })
    }
  }
}

function removeFileRef(index: number) {
  fileRefs.value.splice(index, 1)
}

defineExpose({ addFiles })

function autoResize() {
  if (textarea.value) {
    textarea.value.style.height = 'auto'
    const newHeight = Math.min(textarea.value.scrollHeight, 200)
    textarea.value.style.height = newHeight + 'px'
    textarea.value.style.overflowY = newHeight >= 200 ? 'auto' : 'hidden'
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

function handleFileSelect(e: Event) {
  const target = e.target as HTMLInputElement
  const files = target.files
  if (!files) return

  Array.from(files).forEach(file => {
    const reader = new FileReader()
    reader.onload = (event) => {
      const result = event.target?.result as string
      if (result) {
        selectedImages.value.push(result)
      }
    }
    reader.readAsDataURL(file)
  })

  // Reset input
  target.value = ''
}

function handlePaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items) return

  Array.from(items).forEach(item => {
    if (item.type.startsWith('image/')) {
      const file = item.getAsFile()
      if (file) {
        const reader = new FileReader()
        reader.onload = (event) => {
          const result = event.target?.result as string
          if (result) {
            selectedImages.value.push(result)
          }
        }
        reader.readAsDataURL(file)
      }
    }
  })
}

function removeImage(index: number) {
  selectedImages.value.splice(index, 1)
}

function send() {
  const text = inputText.value.trim()
  if ((!text && selectedImages.value.length === 0 && fileRefs.value.length === 0) || props.disabled) return

  let finalContent = ''
  if (fileRefs.value.length > 0) {
    finalContent = fileRefs.value.map(f => '@`' + f.path + '`').join('\n')
    if (text) finalContent += '\n\n' + text
  } else {
    finalContent = text
  }

  emit('send', finalContent, selectedImages.value)
  inputText.value = ''
  selectedImages.value = []
  fileRefs.value = []
  nextTick(() => autoResize())
}

onMounted(() => {
  textarea.value?.addEventListener('paste', handlePaste)
})
onUnmounted(() => {
  textarea.value?.removeEventListener('paste', handlePaste)
})
</script>

<template>
  <div class="chat-input-area border-t border-slate-200 dark:border-slate-700 bg-white/50 dark:bg-slate-800/50 p-4">
    <div class="mx-auto input-container" :style="{ maxWidth: chatWidthPx }">
      <!-- Image preview -->
      <div v-if="selectedImages.length > 0" class="image-preview-container mb-3 flex flex-wrap gap-2">
        <div v-for="(img, index) in selectedImages" :key="index" class="image-preview-item relative group">
          <img :src="img" class="h-20 w-20 object-cover rounded-lg border border-slate-300 dark:border-slate-600" />
          <button @click="removeImage(index)"
            class="image-remove-btn absolute -top-2 -right-2 bg-red-500 text-white rounded-full w-5 h-5 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity text-xs">
            ×
          </button>
        </div>
      </div>

      <!-- File references -->
      <div v-if="fileRefs.length > 0" class="mb-3 flex flex-wrap gap-2">
        <div v-for="(ref, index) in fileRefs" :key="ref.path"
          class="group inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-700"
          :title="ref.path">
          <svg class="w-3.5 h-3.5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
              d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <span class="truncate max-w-[200px]">@{{ ref.filename }}</span>
          <button @click="removeFileRef(index)"
            class="ml-0.5 opacity-0 group-hover:opacity-100 transition-opacity text-blue-400 hover:text-blue-600 dark:hover:text-blue-200 text-sm leading-none">
            ×
          </button>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <input ref="fileInput" type="file" accept="image/*" multiple @change="handleFileSelect" class="hidden" />
        <button @click="fileInput?.click()" :disabled="disabled"
          class="image-upload-btn shrink-0 p-3 bg-slate-200 dark:bg-slate-600 hover:bg-slate-300 dark:hover:bg-slate-500 disabled:opacity-50 disabled:cursor-not-allowed rounded-xl transition-colors text-slate-700 dark:text-slate-200"
          title="Add image">
          <ImagePlus class="w-5 h-5" />
        </button>
        <textarea ref="textarea" v-model="inputText" @input="autoResize" @keydown="handleKeydown" :disabled="disabled"
          :placeholder="disabled ? 'Waiting for response...' : 'Type a message... (Enter to send, Shift+Enter for newline)'"
          rows="1"
          class="chat-input-textarea input-textarea flex-1 px-4 py-3 bg-white dark:bg-slate-700 rounded-xl border border-slate-300 dark:border-slate-600 focus:border-blue-500 focus:outline-none resize-none text-slate-800 dark:text-slate-200 placeholder-slate-400 dark:placeholder-slate-500 disabled:opacity-50" />
        <button v-if="!isStreaming" @click="send"
          :disabled="disabled || (!inputText.trim() && selectedImages.length === 0 && fileRefs.length === 0)"
          class="send-btn shrink-0 p-3 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-xl transition-colors text-white"
          title="Send message">
          <Send class="w-5 h-5" />
        </button>
        <button v-else @click="emit('cancel')"
          class="stop-btn shrink-0 p-3 bg-red-600 hover:bg-red-700 rounded-xl transition-colors text-white"
          title="Stop generating">
          <Square class="w-5 h-5" />
        </button>
      </div>
    </div>
  </div>
</template>
