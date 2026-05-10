import {defineStore} from 'pinia'
import {ref} from 'vue'
import {
  CreateSession,
  GetSessions,
  DeleteSession,
  UpdateSession,
  ReorderSessions,
} from '../../wailsjs/go/main/App'
import {EventsOn} from '../../wailsjs/runtime/runtime'

export interface Session {
  id: number
  provider_id: number
  name: string
  model: string
  prompt_id: number | null
  created_at: string
  updated_at: string
}

export const useSessionStore = defineStore('session', () => {
  const sessions = ref<Session[]>([])
  const currentSessionId = ref<number | null>(null)
  const loading = ref(false)

  async function fetchSessions() {
    loading.value = true
    try {
      sessions.value = (await GetSessions() || []) as Session[]
    } catch (e) {
      console.error('fetchSessions error:', e)
    } finally {
      loading.value = false
    }
  }

  async function createSession(providerID: number, name: string, model: string, promptID?: number | null): Promise<Session> {
    const pid = promptID !== undefined && promptID !== null ? promptID : null
    const sess = await CreateSession(providerID, name, model, pid) as Session
    sessions.value.unshift(sess)
    currentSessionId.value = sess.id
    return sess
  }

  async function deleteSession(id: number) {
    await DeleteSession(id)
    sessions.value = sessions.value.filter(s => s.id !== id)
    if (currentSessionId.value === id) {
      currentSessionId.value = sessions.value.length > 0 ? sessions.value[0].id : null
    }
  }

  function switchSession(id: number | null) {
    currentSessionId.value = id
  }

  function getCurrentSession(): Session | undefined {
    return sessions.value.find(s => s.id === currentSessionId.value)
  }

  function updateSessionName(id: number, name: string) {
    const s = sessions.value.find(s => s.id === id)
    if (s) s.name = name
  }

  async function updateSessionModel(sessionId: number, providerId: number, model: string) {
    const s = sessions.value.find(s => s.id === sessionId)
    if (!s) return
    await UpdateSession(sessionId, providerId, s.name, model, s.prompt_id)
    s.provider_id = providerId
    s.model = model
  }

  async function updateSessionPrompt(sessionId: number, promptId: number | null) {
    const s = sessions.value.find(s => s.id === sessionId)
    if (!s) return
    await UpdateSession(sessionId, s.provider_id, s.name, s.model, promptId)
    s.prompt_id = promptId
  }

  async function renameSession(sessionId: number, newName: string) {
    const s = sessions.value.find(s => s.id === sessionId)
    if (!s) return
    await UpdateSession(sessionId, s.provider_id, newName, s.model, s.prompt_id)
    s.name = newName
  }

  // Move a session to the top of the local list (for UI responsiveness)
  function moveToTop(sessionId: number) {
    const session = sessions.value.find(s => s.id === sessionId)
    if (!session) return
    session.updated_at = new Date().toISOString()
    const idx = sessions.value.findIndex(s => s.id === sessionId)
    if (idx > 0) {
      sessions.value.splice(idx, 1)
      sessions.value.unshift(session)
    }
  }

  // Persist new session order after drag-and-drop
  async function reorderSessions(orderedIds: number[]) {
    // Optimistically update local state
    const map = new Map(sessions.value.map(s => [s.id, s]))
    sessions.value = orderedIds
      .map(id => map.get(id))
      .filter((s): s is Session => s !== undefined)
    // Persist to backend
    try {
      await ReorderSessions(orderedIds)
    } catch (e) {
      console.error('reorderSessions error:', e)
      // Revert on error
      await fetchSessions()
    }
  }

  // Listen for auto-generated title updates from backend
  EventsOn('session_renamed', (sid: number, newName: string) => {
    updateSessionName(sid, newName)
  })

  return {
    sessions, currentSessionId, loading,
    fetchSessions, createSession, deleteSession, switchSession, getCurrentSession, updateSessionName, updateSessionModel, updateSessionPrompt, renameSession,
    moveToTop, reorderSessions,
  }
})
