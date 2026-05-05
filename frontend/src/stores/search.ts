import {defineStore} from 'pinia'
import {ref, computed} from 'vue'
import type {Message} from './message'

export const useSearchStore = defineStore('search', () => {
  const isOpen = ref(false)
  const query = ref('')
  const debouncedQuery = ref('')
  const matches = ref<{msgIdx: number}[]>([])
  const currentPos = ref(0)

  let debounceTimer: ReturnType<typeof setTimeout> | null = null

  const totalMatches = computed(() => matches.value.length)

  const currentMatchDisplay = computed(() => {
    if (matches.value.length === 0) return '0/0'
    return `${currentPos.value + 1}/${matches.value.length}`
  })

  const currentMsgIdx = computed(() => {
    if (matches.value.length === 0) return -1
    return matches.value[currentPos.value].msgIdx
  })

  function openSearch() {
    isOpen.value = true
  }

  function closeSearch() {
    isOpen.value = false
    query.value = ''
    debouncedQuery.value = ''
    matches.value = []
    currentPos.value = 0
    if (debounceTimer) {
      clearTimeout(debounceTimer)
      debounceTimer = null
    }
  }

  function setQuery(text: string) {
    query.value = text
    if (debounceTimer) clearTimeout(debounceTimer)
    if (!text.trim()) {
      debouncedQuery.value = ''
      matches.value = []
      currentPos.value = 0
      return
    }
    debounceTimer = setTimeout(() => {
      debouncedQuery.value = text
    }, 150)
  }

  function computeMatches(messages: Message[]) {
    const q = debouncedQuery.value.trim().toLowerCase()
    if (!q) {
      matches.value = []
      currentPos.value = 0
      return
    }
    const result: {msgIdx: number}[] = []
    for (let i = 0; i < messages.length; i++) {
      const content = messages[i].content.toLowerCase()
      if (content.includes(q)) {
        // Count occurrences for multiple matches within a message
        let pos = 0
        while ((pos = content.indexOf(q, pos)) !== -1) {
          result.push({msgIdx: i})
          pos += q.length
        }
      }
    }
    matches.value = result
    if (currentPos.value >= result.length) {
      currentPos.value = Math.max(0, result.length - 1)
    }
  }

  function nextMatch() {
    if (matches.value.length === 0) return
    currentPos.value = (currentPos.value + 1) % matches.value.length
  }

  function prevMatch() {
    if (matches.value.length === 0) return
    currentPos.value = (currentPos.value - 1 + matches.value.length) % matches.value.length
  }

  return {
    isOpen,
    query,
    debouncedQuery,
    matches,
    currentPos,
    totalMatches,
    currentMatchDisplay,
    currentMsgIdx,
    openSearch,
    closeSearch,
    setQuery,
    computeMatches,
    nextMatch,
    prevMatch,
  }
})
