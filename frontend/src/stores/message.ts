import {defineStore} from 'pinia'
import {ref} from 'vue'
import {
  GetHistory,
  SendMessage,
  CancelMessage,
  RetryMessage,
  RetryFromUserMessage,
  EditAndResendMessage,
  ClearSessionMessages,
} from '../../wailsjs/go/main/App'
import {EventsOn, EventsOff} from '../../wailsjs/runtime/runtime'
import {useSessionStore} from './session'

export interface PerformanceStats {
  input_tokens: number
  output_tokens: number
  first_token_time: number
  total_time: number
  speed: number
}

export interface MCPToolCall {
  id: string
  name: string
  server_name?: string
  arguments: string
}

export interface MCPToolResult {
  tool_name: string
  server_name?: string
  result: string
  error: string
  duration_ms: number
}

export interface Message {
  id: string  // Changed from number to string to support UUID
  session_id: number
  role: string
  content: string
  reasoning_content?: string  // Reasoning/thinking content from models like DeepSeek R1
  images?: string
  stats?: string     // JSON string from DB
  created_at: string
  tool_calls?: MCPToolCall[]
  tool_results?: MCPToolResult[]
}

// Generate unique ID using crypto.randomUUID() to avoid collision in high concurrency
function generateMessageId(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID()
  }
  // Fallback for environments without crypto.randomUUID
  return Date.now().toString(36) + Math.random().toString(36).substring(2, 15)
}

export const useMessageStore = defineStore('message', () => {
  const messages = ref<Map<number, Message[]>>(new Map())
  const streamingContent = ref('')
  const streamingReasoning = ref('')
  const isStreaming = ref(false)
  const streamingSessionId = ref<number | null>(null)
  const loadedSessions = ref<Set<number>>(new Set())
  // Transient stats for the current streaming response (not yet saved)
  const liveStats = ref<PerformanceStats | null>(null)
  // MCP tool call state
  const activeToolCalls = ref<Map<number, MCPToolCall[]>>(new Map())
  const activeToolResults = ref<Map<number, MCPToolResult[]>>(new Map())

  async function loadHistory(sessionId: number) {
    if (loadedSessions.value.has(sessionId)) return
    const raw = await GetHistory(sessionId) || []
    // Parse JSON string fields back to arrays
    // Ensure id is always a string for consistent comparison
    const history: Message[] = raw.map((m: any) => ({
      id: String(m.id),
      session_id: m.session_id,
      role: m.role,
      content: m.content,
      reasoning_content: m.reasoning_content || undefined,
      images: m.images,
      stats: m.stats,
      created_at: m.created_at,
      tool_calls: m.tool_calls ? JSON.parse(m.tool_calls) : undefined,
      tool_results: m.tool_results ? JSON.parse(m.tool_results) : undefined,
    }))
    messages.value.set(sessionId, history)
    loadedSessions.value.add(sessionId)
  }

  function setupStreamListeners(sessionId: number) {
    let pendingStats: PerformanceStats | null = null

    EventsOn('message_chunk', (sid: number, chunk: string) => {
      if (sid === streamingSessionId.value) {
        streamingContent.value += chunk
      }
    })
    EventsOn('message_reasoning', (sid: number, chunk: string) => {
      if (sid === streamingSessionId.value) {
        streamingReasoning.value += chunk
      }
    })
    EventsOn('message_stats', (sid: number, stats: PerformanceStats) => {
      if (sid === streamingSessionId.value) {
        pendingStats = stats
        liveStats.value = stats
      }
    })
    EventsOn('mcp_tool_call_start', (sid: number, data: {tool_calls: any[], server_names?: Record<string, string>}) => {
      if (sid === streamingSessionId.value) {
        const serverNames = data.server_names || {}
        // Transform from OpenAI nested format to flat MCPToolCall format
        const calls: MCPToolCall[] = data.tool_calls.map(tc => {
          const fqName = tc.function?.name || tc.name || ''
          const serverID = fqName.includes('___') ? fqName.split('___')[0] : ''
          return {
            id: tc.id || '',
            name: fqName,
            server_name: serverNames[serverID] || '',
            arguments: tc.function?.arguments || tc.arguments || '',
          }
        })
        activeToolCalls.value.set(sid, calls)
        activeToolResults.value.set(sid, [])
      }
    })
    EventsOn('mcp_tool_result', (sid: number, result: MCPToolResult) => {
      if (sid === streamingSessionId.value) {
        const results = activeToolResults.value.get(sid) || []
        results.push(result)
        activeToolResults.value.set(sid, [...results])
      }
    })
    EventsOn('message_done', (sid: number, savedMessageId?: string) => {
      if (sid === streamingSessionId.value) {
        const toolCalls = activeToolCalls.value.get(sid) || []
        const toolResults = activeToolResults.value.get(sid) || []
        const statsJSON = pendingStats ? JSON.stringify(pendingStats) : ''
        // Use the database ID returned by backend if available, otherwise generate UUID
        const assistantMsg: Message = {
          id: savedMessageId || generateMessageId(),
          session_id: sid,
          role: 'assistant',
          content: streamingContent.value,
          reasoning_content: streamingReasoning.value || undefined,
          stats: statsJSON,
          created_at: new Date().toISOString(),
          tool_calls: toolCalls.length > 0 ? toolCalls : undefined,
          tool_results: toolResults.length > 0 ? toolResults : undefined,
        }
        const msgs = messages.value.get(sid) || []
        msgs.push(assistantMsg)
        messages.value.set(sid, [...msgs])

        streamingContent.value = ''
        streamingReasoning.value = ''
        liveStats.value = null
        isStreaming.value = false
        streamingSessionId.value = null
        pendingStats = null
        activeToolCalls.value.delete(sid)
        activeToolResults.value.delete(sid)
        cleanupEvents()
      }
    })
    EventsOn('message_error', (sid: number, errMsg: string) => {
      if (sid === streamingSessionId.value) {
        const errorMsg: Message = {
          id: generateMessageId(),
          session_id: sid,
          role: 'assistant',
          content: `**Error:** ${errMsg}`,
          created_at: new Date().toISOString(),
        }
        const msgs = messages.value.get(sid) || []
        msgs.push(errorMsg)
        messages.value.set(sid, [...msgs])
        streamingContent.value = ''
        streamingReasoning.value = ''
        liveStats.value = null
        isStreaming.value = false
        streamingSessionId.value = null
        activeToolCalls.value.delete(sid)
        activeToolResults.value.delete(sid)
        cleanupEvents()
      }
    })
  }

  async function sendMessage(sessionId: number, content: string, images: string[] = []) {
    // Cleanup old listeners before setting up new ones to prevent memory leak
    cleanupEvents()

    // Add user message optimistically with unique UUID
    const optimisticId = generateMessageId()
    const userMsg: Message = {
      id: optimisticId,
      session_id: sessionId,
      role: 'user',
      content,
      images: JSON.stringify(images),
      created_at: new Date().toISOString(),
    }
    const sessionMessages = messages.value.get(sessionId) || []
    sessionMessages.push(userMsg)
    messages.value.set(sessionId, [...sessionMessages])

    // Start streaming
    streamingContent.value = ''
    streamingReasoning.value = ''
    isStreaming.value = true
    streamingSessionId.value = sessionId
    liveStats.value = null

    // Cleanup again right before setup to ensure no leftover listeners
    cleanupEvents()
    setupStreamListeners(sessionId)

    // Move session to top of list
    const sessionStore = useSessionStore()
    sessionStore.moveToTop(sessionId)

    try {
      const savedId = await SendMessage(sessionId, content, images)
      // Update user message ID from optimistic UUID to real DB ID
      if (savedId) {
        const msgs = messages.value.get(sessionId) || []
        const lastMsg = msgs[msgs.length - 1]
        if (lastMsg && lastMsg.role === 'user' && lastMsg.id === optimisticId) {
          lastMsg.id = savedId
          messages.value.set(sessionId, [...msgs])
        }
      }
    } catch (e: any) {
      streamingContent.value = ''
      streamingReasoning.value = ''
      liveStats.value = null
      isStreaming.value = false
      streamingSessionId.value = null
      cleanupEvents()
      throw e
    }
  }

  async function retryMessage(sessionId: number, messageId: string) {
    // Cleanup old listeners before setting up new ones to prevent memory leak
    cleanupEvents()

    // Remove this message and all after it from local state
    const msgs = messages.value.get(sessionId) || []
    const idx = msgs.findIndex(m => m.id === messageId)
    if (idx !== -1) {
      messages.value.set(sessionId, msgs.slice(0, idx))
    }

    // Invalidate history cache so next load fetches fresh data
    loadedSessions.value.delete(sessionId)

    // Start streaming
    streamingContent.value = ''
    streamingReasoning.value = ''
    isStreaming.value = true
    streamingSessionId.value = sessionId
    liveStats.value = null

    setupStreamListeners(sessionId)

    // Move session to top of list
    const sessionStore = useSessionStore()
    sessionStore.moveToTop(sessionId)

    try {
      await RetryMessage(sessionId, messageId)
    } catch (e: any) {
      streamingContent.value = ''
      streamingReasoning.value = ''
      liveStats.value = null
      isStreaming.value = false
      streamingSessionId.value = null
      cleanupEvents()
      throw e
    }
  }

  async function retryFromUserMessage(sessionId: number, messageId: string) {
    // Cleanup old listeners
    cleanupEvents()

    // Save user message content and images before removing
    const msgs = messages.value.get(sessionId) || []
    const msg = msgs.find(m => m.id === messageId)
    if (!msg) return
    const savedContent = msg.content
    const savedImages = msg.images || '[]'

    // Remove this user message and all after from local state
    const idx = msgs.findIndex(m => m.id === messageId)
    if (idx !== -1) {
      messages.value.set(sessionId, msgs.slice(0, idx))
    }

    // Invalidate history cache
    loadedSessions.value.delete(sessionId)

    // Re-add user message optimistically
    const optimisticId = generateMessageId()
    const userMsg: Message = {
      id: optimisticId,
      session_id: sessionId,
      role: 'user',
      content: savedContent,
      images: savedImages,
      created_at: new Date().toISOString(),
    }
    const remainingMsgs = messages.value.get(sessionId) || []
    remainingMsgs.push(userMsg)
    messages.value.set(sessionId, [...remainingMsgs])

    // Start streaming
    streamingContent.value = ''
    streamingReasoning.value = ''
    isStreaming.value = true
    streamingSessionId.value = sessionId
    liveStats.value = null

    setupStreamListeners(sessionId)

    // Move session to top of list
    const sessionStore = useSessionStore()
    sessionStore.moveToTop(sessionId)

    try {
      const newId = await RetryFromUserMessage(sessionId, messageId)
      // Update user message ID to real DB ID
      if (newId) {
        const currentMsgs = messages.value.get(sessionId) || []
        const lastMsg = currentMsgs[currentMsgs.length - 1]
        if (lastMsg && lastMsg.role === 'user' && lastMsg.id === optimisticId) {
          lastMsg.id = newId
          messages.value.set(sessionId, [...currentMsgs])
        }
      }
    } catch (e: any) {
      streamingContent.value = ''
      streamingReasoning.value = ''
      liveStats.value = null
      isStreaming.value = false
      streamingSessionId.value = null
      cleanupEvents()
      throw e
    }
  }

  async function editAndResendMessage(sessionId: number, messageId: string, newContent: string, newImages: string[] = []) {
    cleanupEvents()

    const msgs = messages.value.get(sessionId) || []
    const idx = msgs.findIndex(m => m.id === messageId)
    if (idx !== -1) {
      messages.value.set(sessionId, msgs.slice(0, idx))
    }

    loadedSessions.value.delete(sessionId)

    const optimisticId = generateMessageId()
    const userMsg: Message = {
      id: optimisticId,
      session_id: sessionId,
      role: 'user',
      content: newContent,
      images: JSON.stringify(newImages),
      created_at: new Date().toISOString(),
    }
    const remainingMsgs = messages.value.get(sessionId) || []
    remainingMsgs.push(userMsg)
    messages.value.set(sessionId, [...remainingMsgs])

    streamingContent.value = ''
    streamingReasoning.value = ''
    isStreaming.value = true
    streamingSessionId.value = sessionId
    liveStats.value = null

    setupStreamListeners(sessionId)

    const sessionStore = useSessionStore()
    sessionStore.moveToTop(sessionId)

    try {
      const newId = await EditAndResendMessage(sessionId, messageId, newContent, newImages)
      if (newId) {
        const currentMsgs = messages.value.get(sessionId) || []
        const lastMsg = currentMsgs[currentMsgs.length - 1]
        if (lastMsg && lastMsg.role === 'user' && lastMsg.id === optimisticId) {
          lastMsg.id = newId
          messages.value.set(sessionId, [...currentMsgs])
        }
      }
    } catch (e: any) {
      streamingContent.value = ''
      streamingReasoning.value = ''
      liveStats.value = null
      isStreaming.value = false
      streamingSessionId.value = null
      cleanupEvents()
      throw e
    }
  }

  function cancelStream() {
    if (streamingSessionId.value !== null) {
      CancelMessage(streamingSessionId.value)
    }
  }

  function cleanupEvents() {
    EventsOff('message_chunk')
    EventsOff('message_reasoning')
    EventsOff('message_done')
    EventsOff('message_error')
    EventsOff('message_stats')
    EventsOff('mcp_tool_call_start')
    EventsOff('mcp_tool_result')
  }

  function getMessages(sessionId: number): Message[] {
    return messages.value.get(sessionId) || []
  }

  /** Parse stats JSON from a saved message */
  function parseStats(raw: string | undefined): PerformanceStats | undefined {
    if (!raw) return undefined
    try {
      const s = JSON.parse(raw)
      if (s && (s.input_tokens || s.output_tokens || s.total_time || s.first_token_time || s.speed)) {
        return s as PerformanceStats
      }
    } catch { /* ignore */ }
    return undefined
  }

  function getStats(messageId: string, sessionId: number): PerformanceStats | undefined {
    // For the currently streaming message, use live stats
    if (isStreaming.value && streamingSessionId.value === sessionId) {
      return liveStats.value ?? undefined
    }
    // For saved messages, parse from the message's stats JSON
    const msgs = messages.value.get(sessionId) || []
    const msg = msgs.find(m => m.id === messageId)
    return parseStats(msg?.stats)
  }

  function clearSession(sessionId: number) {
    messages.value.delete(sessionId)
    loadedSessions.value.delete(sessionId)
  }

  async function clearHistory(sessionId: number) {
    await ClearSessionMessages(sessionId)
    clearSession(sessionId)
  }

  function getActiveToolCalls(sessionId: number): MCPToolCall[] {
    return activeToolCalls.value.get(sessionId) || []
  }

  function getActiveToolResults(sessionId: number): MCPToolResult[] {
    return activeToolResults.value.get(sessionId) || []
  }

  return {
    messages, streamingContent, streamingReasoning, isStreaming, streamingSessionId,
    loadHistory, sendMessage, retryMessage, retryFromUserMessage, editAndResendMessage, cancelStream,
    getMessages, getStats, parseStats, clearSession, clearHistory,
    getActiveToolCalls, getActiveToolResults,
  }
})
