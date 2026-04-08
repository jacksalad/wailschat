<script lang="ts" setup>
import {ref} from 'vue'
import {useProviderStore, type Provider} from '../stores/provider'

const emit = defineEmits<{
  close: []
}>()

const providerStore = useProviderStore()

const showForm = ref(false)
const editingId = ref<number | null>(null)
const name = ref('')
const apiKey = ref('')
const baseURL = ref('https://api.openai.com')
const models = ref('')
const isDefault = ref(false)
const testResult = ref<string | null>(null)
const testing = ref(false)

function openAdd() {
  editingId.value = null
  name.value = ''
  apiKey.value = ''
  baseURL.value = 'https://api.openai.com'
  models.value = ''
  isDefault.value = providerStore.providers.length === 0
  testResult.value = null
  showForm.value = true
}

function openEdit(p: Provider) {
  editingId.value = p.id
  name.value = p.name
  apiKey.value = p.api_key
  baseURL.value = p.base_url
  models.value = p.models.join(', ')
  isDefault.value = p.is_default
  testResult.value = null
  showForm.value = true
}

async function save() {
  const modelList = models.value.split(',').map(m => m.trim()).filter(Boolean)
  if (!name.value.trim() || !apiKey.value.trim() || !baseURL.value.trim() || modelList.length === 0) return

  if (editingId.value) {
    await providerStore.updateProvider(editingId.value, name.value.trim(), apiKey.value.trim(), baseURL.value.trim(), modelList, isDefault.value)
  } else {
    await providerStore.addProvider(name.value.trim(), apiKey.value.trim(), baseURL.value.trim(), modelList, isDefault.value)
  }
  showForm.value = false
}

async function test() {
  const modelList = models.value.split(',').map(m => m.trim()).filter(Boolean)
  if (!baseURL.value.trim() || !apiKey.value.trim() || modelList.length === 0) {
    testResult.value = 'Please fill in base URL, API key, and at least one model'
    return
  }
  testing.value = true
  testResult.value = null
  const err = await providerStore.testConnection(baseURL.value.trim(), apiKey.value.trim(), modelList[0])
  testResult.value = err || 'Connection successful!'
  testing.value = false
}

async function remove(id: number) {
  await providerStore.deleteProvider(id)
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" @click.self="emit('close')">
    <div class="bg-slate-800 rounded-xl p-6 w-[500px] max-h-[80vh] overflow-y-auto border border-slate-600">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-lg font-semibold">API Providers</h3>
        <button @click="emit('close')" class="text-slate-400 hover:text-white text-xl">&times;</button>
      </div>

      <!-- Provider List -->
      <div class="space-y-2 mb-4">
        <div
          v-for="p in providerStore.providers"
          :key="p.id"
          class="flex items-center justify-between p-3 bg-slate-700 rounded-lg"
        >
          <div>
            <div class="font-medium">{{ p.name }}</div>
            <div class="text-xs text-slate-400">{{ p.base_url }} &middot; {{ p.models.join(', ') }}</div>
          </div>
          <div class="flex gap-2">
            <button @click="openEdit(p)" class="text-blue-400 hover:text-blue-300 text-sm">Edit</button>
            <button @click="remove(p.id)" class="text-red-400 hover:text-red-300 text-sm">Delete</button>
          </div>
        </div>
        <div v-if="providerStore.providers.length === 0" class="text-center text-slate-500 text-sm py-4">
          No providers configured
        </div>
      </div>

      <!-- Add / Edit Form -->
      <div v-if="showForm" class="border-t border-slate-600 pt-4 space-y-3">
        <h4 class="font-medium">{{ editingId ? 'Edit' : 'Add' }} Provider</h4>
        <input v-model="name" placeholder="Name (e.g. OpenAI, DeepSeek)" class="w-full px-3 py-2 bg-slate-700 rounded-lg border border-slate-600 focus:border-blue-500 focus:outline-none" />
        <input v-model="apiKey" type="password" placeholder="API Key" class="w-full px-3 py-2 bg-slate-700 rounded-lg border border-slate-600 focus:border-blue-500 focus:outline-none" />
        <input v-model="baseURL" placeholder="Base URL (e.g. https://api.openai.com)" class="w-full px-3 py-2 bg-slate-700 rounded-lg border border-slate-600 focus:border-blue-500 focus:outline-none" />
        <input v-model="models" placeholder="Models (comma-separated, e.g. gpt-4o, gpt-3.5-turbo)" class="w-full px-3 py-2 bg-slate-700 rounded-lg border border-slate-600 focus:border-blue-500 focus:outline-none" />
        <label class="flex items-center gap-2 text-sm">
          <input v-model="isDefault" type="checkbox" class="rounded" />
          Set as default provider
        </label>

        <!-- Test result -->
        <div v-if="testResult" :class="['text-sm p-2 rounded', testResult.includes('success') ? 'bg-green-900/30 text-green-400' : 'bg-red-900/30 text-red-400']">
          {{ testResult }}
        </div>

        <div class="flex justify-between">
          <button @click="test" :disabled="testing" class="px-3 py-2 bg-slate-600 hover:bg-slate-500 rounded-lg text-sm">
            {{ testing ? 'Testing...' : 'Test Connection' }}
          </button>
          <div class="flex gap-2">
            <button @click="showForm = false" class="px-3 py-2 bg-slate-600 hover:bg-slate-500 rounded-lg text-sm">Cancel</button>
            <button @click="save" class="px-3 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm">Save</button>
          </div>
        </div>
      </div>

      <button v-if="!showForm" @click="openAdd" class="w-full px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded-lg text-sm font-medium transition-colors">
        + Add Provider
      </button>
    </div>
  </div>
</template>
