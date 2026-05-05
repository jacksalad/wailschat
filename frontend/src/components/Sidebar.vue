<script lang="ts" setup>
import SessionList from './SessionList.vue'
import SettingsModal from './SettingsModal.vue'
import {useSessionStore} from '../stores/session'
import {useProviderStore} from '../stores/provider'
import {useSearchStore} from '../stores/search'
import {ref} from 'vue'
import { MessageSquarePlus, Settings, Search } from 'lucide-vue-next'

const sessionStore = useSessionStore()
const providerStore = useProviderStore()
const searchStore = useSearchStore()
const showSettings = ref(false)

async function newChat() {
  if (providerStore.providers.length === 0) {
    showSettings.value = true
    return
  }
  const p = providerStore.getCurrentProvider()
  if (!p) return
  const model = p.models[0] || ''
  await sessionStore.createSession(p.id, 'New Chat', model)
}
</script>

<template>
  <div class="sidebar-wrapper flex flex-col h-full">
    <div class="sidebar-container flex flex-col h-full bg-white/50 dark:bg-slate-800/50 border-r border-slate-200 dark:border-slate-700">
      <!-- Header -->
      <div class="sidebar-header p-4 border-b border-slate-200 dark:border-slate-700">
        <button
          @click="newChat"
          class="new-chat-btn w-full p-2 bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors text-white flex items-center justify-center gap-2"
        >
          <MessageSquarePlus class="w-5 h-5" />
          <span>New Chat</span>
        </button>
      </div>

      <!-- Session search (shown with Ctrl+F) -->
      <div v-if="searchStore.isOpen" class="px-4 py-2 border-b border-slate-200 dark:border-slate-700">
        <div class="relative">
          <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-slate-400 pointer-events-none" />
          <input
            :value="searchStore.query"
            @input="searchStore.setQuery(($event.target as HTMLInputElement).value)"
            placeholder="Search sessions..."
            class="w-full pl-8 pr-3 py-1.5 bg-white dark:bg-slate-700 rounded-lg border border-slate-200 dark:border-slate-600 text-sm text-slate-800 dark:text-slate-200 placeholder-slate-400 focus:border-blue-500 focus:outline-none transition-colors"
          />
        </div>
      </div>

      <!-- Session List -->
      <SessionList class="session-list flex-1 overflow-y-auto" :filter="searchStore.isOpen ? searchStore.query : ''" />

      <!-- Footer: Settings -->
      <div class="sidebar-footer p-4 border-t border-slate-200 dark:border-slate-700">
        <button
          @click="showSettings = true"
          class="settings-btn w-full p-2 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 rounded-lg transition-colors flex items-center justify-center gap-2 text-slate-700 dark:text-slate-200"
        >
          <Settings class="w-5 h-5" />
          <span>Settings</span>
        </button>
      </div>

    </div>

    <!-- Settings Modal - Teleported outside sidebar to body to ensure proper z-index stacking -->
    <Teleport to="body">
      <SettingsModal v-if="showSettings" @close="showSettings = false" />
    </Teleport>
  </div>
</template>
