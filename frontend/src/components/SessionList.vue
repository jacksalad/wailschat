<script lang="ts" setup>
import {ref, nextTick, computed} from 'vue'
import {useSessionStore} from '../stores/session'
import {useMessageStore} from '../stores/message'
import ContextMenu, {type MenuItem} from './ContextMenu.vue'
import {GripVertical, Pencil, Trash2, Eraser} from 'lucide-vue-next'
import {formatSessionTime} from '../utils/format'

const sessionStore = useSessionStore()
const messageStore = useMessageStore()

const props = defineProps<{
  filter?: string
}>()

const filteredSessions = computed(() => {
  if (!props.filter || !props.filter.trim()) return sessionStore.sessions
  const q = props.filter.trim().toLowerCase()
  return sessionStore.sessions.filter(s => s.name.toLowerCase().includes(q))
})

// Drag-and-drop state
const dragIndex = ref<number | null>(null)
const dropIndex = ref<number | null>(null)

// Context menu state
const menuVisible = ref(false)
const menuX = ref(0)
const menuY = ref(0)
const menuItems = ref<MenuItem[]>([])

// Rename state
const renamingSessionId = ref<number | null>(null)
const renameValue = ref('')
const renameInputRef = ref<HTMLInputElement | null>(null)

async function selectSession(id: number) {
  if (renamingSessionId.value === id) return
  if (sessionStore.currentSessionId === id) {
    sessionStore.switchSession(null)
    return
  }
  sessionStore.switchSession(id)
  await messageStore.loadHistory(id)
}

function deleteSession(id: number, e?: Event) {
  e?.stopPropagation()
  // Optimistically clear messages and remove session from list.
  // The store handles rollback on backend failure automatically.
  messageStore.clearSession(id)
  sessionStore.deleteSession(id) // fire-and-forget; store is optimistic
}

function onContextMenu(e: MouseEvent, session: { id: number; name: string }) {
  e.preventDefault()
  e.stopPropagation()
  menuX.value = e.clientX
  menuY.value = e.clientY
  menuItems.value = [
    { label: 'Rename', icon: Pencil, action: () => startRename(session.id, session.name) },
    { divider: true },
    { label: 'Clear History', icon: Eraser, action: () => messageStore.clearHistory(session.id) },
    { label: 'Delete', icon: Trash2, danger: true, action: () => deleteSession(session.id) },
  ]
  menuVisible.value = true
}

function startRename(sessionId: number, currentName: string) {
  renamingSessionId.value = sessionId
  renameValue.value = currentName
  nextTick(() => {
    renameInputRef.value?.focus()
    renameInputRef.value?.select()
  })
}

async function finishRename(sessionId: number) {
  const name = renameValue.value.trim()
  if (name && renamingSessionId.value !== null) {
    await sessionStore.renameSession(sessionId, name)
  }
  renamingSessionId.value = null
}

function cancelRename() {
  renamingSessionId.value = null
}

// Drag-and-drop handlers
function onDragStart(index: number, e: DragEvent) {
  dragIndex.value = index
  dropIndex.value = null
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(index))
  }
}

function onDragOver(index: number, e: DragEvent) {
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'move'
  }
  dropIndex.value = index
}

function onDragLeave() {
  dropIndex.value = null
}

async function onDrop(index: number, e: DragEvent) {
  e.preventDefault()
  if (dragIndex.value === null || dragIndex.value === index) {
    dragIndex.value = null
    dropIndex.value = null
    return
  }

  // Reorder sessions locally and persist
  const orderedIds = sessionStore.sessions.map(s => s.id)
  const [moved] = orderedIds.splice(dragIndex.value, 1)
  orderedIds.splice(index, 0, moved)

  await sessionStore.reorderSessions(orderedIds)

  dragIndex.value = null
  dropIndex.value = null
}

function onDragEnd() {
  dragIndex.value = null
  dropIndex.value = null
}
</script>

<template>
  <div>
    <div class="p-2 space-y-1">
      <div
        v-for="(session, index) in filteredSessions"
        :key="session.id"
        :draggable="!props.filter?.trim()"
        @dragstart="onDragStart(index, $event)"
        @dragover="onDragOver(index, $event)"
        @dragleave="onDragLeave"
        @drop="onDrop(index, $event)"
        @dragend="onDragEnd"
        @click="selectSession(session.id)"
        @contextmenu="(e: MouseEvent) => onContextMenu(e, session)"
        :class="[
          'group flex items-center justify-between px-3 py-2.5 rounded-lg cursor-pointer transition-colors',
          session.id === sessionStore.currentSessionId
            ? 'bg-blue-600/50 text-white'
            : 'hover:bg-slate-100/50 dark:hover:bg-slate-700/50 text-slate-700 dark:text-slate-300',
          dropIndex === index && dragIndex !== index ? 'border-t-2 border-blue-400' : '',
          dragIndex === index ? 'opacity-50' : '',
        ]"
      >
        <div class="flex items-center min-w-0 flex-1">
          <GripVertical class="w-3.5 h-3.5 mr-1.5 opacity-0 group-hover:opacity-40 text-slate-400 flex-shrink-0 cursor-grab" />
          <div class="min-w-0 flex-1">
            <template v-if="renamingSessionId === session.id">
              <input
                ref="renameInputRef"
                v-model="renameValue"
                class="rename-input w-full text-sm font-medium bg-white dark:bg-slate-600 border border-blue-400 rounded px-1 py-0.5 outline-none text-slate-800 dark:text-slate-200"
                @keydown.enter="finishRename(session.id)"
                @keydown.escape="cancelRename"
                @blur="finishRename(session.id)"
                @click.stop
              />
            </template>
            <template v-else>
              <div class="text-sm font-medium truncate">{{ session.name }}</div>
            </template>
            <div :class="['text-xs mt-0.5', session.id === sessionStore.currentSessionId ? 'text-blue-200' : 'text-slate-500 dark:text-slate-400']">{{ session.model }}</div>
            <div class="text-[10px] mt-0.5" :class="session.id === sessionStore.currentSessionId ? 'text-blue-300' : 'text-slate-400 dark:text-slate-500'">{{ formatSessionTime(session.updated_at) }}</div>
          </div>
        </div>
        <button
          @click="deleteSession(session.id, $event)"
          class="opacity-0 group-hover:opacity-100 ml-2 text-slate-400 hover:text-red-500 dark:text-slate-400 dark:hover:text-red-400 transition-opacity"
          title="Delete"
        >
          ✕
        </button>
      </div>
      <div v-if="sessionStore.sessions.length === 0" class="text-center text-slate-400 dark:text-slate-500 text-sm py-8">
        No chats yet
      </div>
      <div v-else-if="filteredSessions.length === 0" class="text-center text-slate-400 dark:text-slate-500 text-sm py-4">
        No matching sessions
      </div>
    </div>
    <ContextMenu :visible="menuVisible" :x="menuX" :y="menuY" :items="menuItems" @close="menuVisible = false" />
  </div>
</template>

<style scoped>
.rename-input {
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.3);
}
</style>
