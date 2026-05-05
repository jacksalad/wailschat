<script lang="ts" setup>
import {ref, onMounted, onBeforeUnmount, nextTick} from 'vue'
import {useSearchStore} from '../stores/search'
import {ChevronUp, ChevronDown, X} from 'lucide-vue-next'

const searchStore = useSearchStore()
const searchInput = ref<HTMLInputElement | null>(null)

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault()
    e.stopPropagation()
    searchStore.closeSearch()
    returnFocus()
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (e.shiftKey) {
      searchStore.prevMatch()
    } else {
      searchStore.nextMatch()
    }
  }
}

function onClose() {
  searchStore.closeSearch()
  returnFocus()
}

function returnFocus() {
  nextTick(() => {
    const textarea = document.querySelector<HTMLTextAreaElement>('.chat-input-textarea')
    if (textarea) textarea.focus()
  })
}

onMounted(() => {
  nextTick(() => {
    searchInput.value?.focus()
    searchInput.value?.select()
  })
})

onBeforeUnmount(() => {
  returnFocus()
})
</script>

<template>
  <div class="search-bar absolute top-2 right-3 z-30 animate-in slide-in-from-top-2 duration-150">
    <div class="flex items-center gap-1 bg-white/90 dark:bg-slate-800/90 backdrop-blur-sm border border-slate-200 dark:border-slate-600 rounded-lg shadow-lg px-2 py-1.5">
      <input
        ref="searchInput"
        :value="searchStore.query"
        @input="searchStore.setQuery(($event.target as HTMLInputElement).value)"
        @keydown="onKeydown"
        placeholder="Find in chat..."
        class="w-44 bg-transparent text-sm text-slate-800 dark:text-slate-200 placeholder-slate-400 outline-none"
      />
      <span class="text-xs text-slate-400 dark:text-slate-500 whitespace-nowrap min-w-[36px] text-center select-none">{{ searchStore.currentMatchDisplay }}</span>
      <button
        @click="searchStore.prevMatch()"
        :disabled="searchStore.totalMatches === 0"
        class="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-500 dark:text-slate-400 disabled:opacity-30 disabled:cursor-default transition-colors"
        title="Previous (Shift+Enter)"
      >
        <ChevronUp class="w-3.5 h-3.5" />
      </button>
      <button
        @click="searchStore.nextMatch()"
        :disabled="searchStore.totalMatches === 0"
        class="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-500 dark:text-slate-400 disabled:opacity-30 disabled:cursor-default transition-colors"
        title="Next (Enter)"
      >
        <ChevronDown class="w-3.5 h-3.5" />
      </button>
      <button
        @click="onClose"
        class="p-1 rounded hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-500 dark:text-slate-400 transition-colors"
        title="Close (Escape)"
      >
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  </div>
</template>
