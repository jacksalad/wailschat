import {defineStore} from 'pinia'
import {ref} from 'vue'
import {
  AddProvider,
  GetProviders,
  UpdateProvider,
  DeleteProvider,
  TestConnection,
} from '../../wailsjs/go/main/App'

export interface Provider {
  id: number
  name: string
  api_key: string
  base_url: string
  models: string[]
  is_default: boolean
  created_at: string
}

export const useProviderStore = defineStore('provider', () => {
  const providers = ref<Provider[]>([])
  const currentProviderId = ref<number | null>(null)
  const loading = ref(false)

  async function fetchProviders() {
    loading.value = true
    try {
      providers.value = (await GetProviders() || []) as Provider[]
      if (currentProviderId.value === null && providers.value.length > 0) {
        const def = providers.value.find(p => p.is_default)
        currentProviderId.value = def ? def.id : providers.value[0].id
      }
    } catch (e) {
        console.error('fetchProviders error:', e)
      } finally {
      loading.value = false
    }
  }

  async function addProvider(name: string, apiKey: string, baseURL: string, models: string[], isDefault: boolean) {
    await AddProvider(name, apiKey, baseURL, models, isDefault)
    await fetchProviders()
  }

  async function updateProvider(id: number, name: string, apiKey: string, baseURL: string, models: string[], isDefault: boolean) {
    await UpdateProvider(id, name, apiKey, baseURL, models, isDefault)
    await fetchProviders()
  }

  async function deleteProvider(id: number) {
    await DeleteProvider(id)
    if (currentProviderId.value === id) {
      currentProviderId.value = providers.value.length > 0 ? providers.value[0].id : null
    }
    await fetchProviders()
  }

  async function testConnection(baseURL: string, apiKey: string, model: string): Promise<string | null> {
    try {
      await TestConnection(baseURL, apiKey, model)
      return null
    } catch (e: any) {
      return e.toString()
    }
  }

  function getCurrentProvider(): Provider | undefined {
    return providers.value.find(p => p.id === currentProviderId.value)
  }

  return {
    providers, currentProviderId, loading,
    fetchProviders, addProvider, updateProvider, deleteProvider, testConnection, getCurrentProvider,
  }
})
