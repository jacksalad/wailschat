<script lang="ts" setup>
import {ref} from 'vue'
import {useSessionStore} from '../stores/session'
import {useMessageStore} from '../stores/message'
import {GripVertical} from 'lucide-vue-next'

const sessionStore = useSessionStore()
const messageStore = useMessageStore()

// Drag-and-drop state
const dragIndex = ref<number | null>(null)
const dropIndex = ref<number | null>(null)

async function selectSession(id: number) {
  if (sessionStore.currentSessionId === id) {
    sessionStore.switchSession(null)
    return
  }
  sessionStore.switchSession(id)
  await messageStore.loadHistory(id)
}

async function deleteSession(id: number, e: Event) {
  e.stopPropagation()
  messageStore.clearSession(id)
  await sessionStore.deleteSession(id)
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr)
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'})
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
  <div class="p-2 space-y-1">
    <div
      v-for="(session, index) in sessionStore.sessions"
      :key="session.id"
      draggable="true"
      @dragstart="onDragStart(index, $event)"
      @dragover="onDragOver(index, $event)"
      @dragleave="onDragLeave"
      @drop="onDrop(index, $event)"
      @dragend="onDragEnd"
      @click="selectSession(session.id)"
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
          <div class="text-sm font-medium truncate">{{ session.name }}</div>
          <div :class="['text-xs mt-0.5', session.id === sessionStore.currentSessionId ? 'text-blue-200' : 'text-slate-500 dark:text-slate-400']">{{ session.model }}</div>
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
  </div>
</template>
