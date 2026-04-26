import {defineStore} from 'pinia'
import {ref} from 'vue'
import {PromptList, PromptCreate, PromptUpdate, PromptDelete, PromptSetDefault} from '../../wailsjs/go/main/App'
import {model} from '../../wailsjs/go/models'

export type Prompt = model.Prompt

export const usePromptStore = defineStore('prompt', () => {
  const prompts = ref<Prompt[]>([])
  const loading = ref(false)

  async function fetchPrompts() {
    loading.value = true
    try {
      prompts.value = (await PromptList() || []) as Prompt[]
    } catch (e) {
      console.error('fetchPrompts error:', e)
    } finally {
      loading.value = false
    }
  }

  async function createPrompt(opts: {name: string; content: string; category?: string; is_default?: boolean}): Promise<Prompt> {
    const p = new model.Prompt({
      id: 0,
      name: opts.name,
      content: opts.content,
      category: opts.category || '',
      is_default: opts.is_default || false,
      sort_order: 0,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    })
    const result = await PromptCreate(p) as Prompt
    await fetchPrompts()
    return result
  }

  async function updatePrompt(prompt: Prompt) {
    await PromptUpdate(prompt)
    await fetchPrompts()
  }

  async function deletePrompt(id: number) {
    await PromptDelete(id)
    await fetchPrompts()
  }

  async function setDefault(id: number) {
    await PromptSetDefault(id)
    await fetchPrompts()
  }

  function getDefault(): Prompt | undefined {
    return prompts.value.find(p => p.is_default)
  }

  function getByID(id: number): Prompt | undefined {
    return prompts.value.find(p => p.id === id)
  }

  function getCategories(): string[] {
    const cats = new Set(prompts.value.map(p => p.category).filter(c => c))
    return Array.from(cats).sort()
  }

  return {
    prompts, loading,
    fetchPrompts, createPrompt, updatePrompt, deletePrompt, setDefault,
    getDefault, getByID, getCategories,
  }
})
