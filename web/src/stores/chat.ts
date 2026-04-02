import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Message, Session, SSEEvent } from '@/types'
import { chatApi, createSSEConnection } from '@/api'

const generateId = () => `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`

export const useChatStore = defineStore('chat', () => {
  const sessions = ref<Session[]>([])
  const currentSessionId = ref<string>(generateId())
  const messages = ref<Map<string, Message[]>>(new Map())
  const isLoading = ref(false)
  const streamingContent = ref('')
  const abortController = ref<AbortController | null>(null)

  const currentMessages = computed(() => {
    return messages.value.get(currentSessionId.value) || []
  })

  const currentSession = computed(() => {
    return sessions.find(s => s.id === currentSessionId.value)
  })

  function createSession(title = '新对话'): Session {
    const session: Session = {
      id: generateId(),
      title,
      createdAt: Date.now(),
      updatedAt: Date.now(),
      messageCount: 0
    }
    sessions.value.unshift(session)
    messages.value.set(session.id, [])
    saveToStorage()
    return session
  }

  function selectSession(sessionId: string) {
    if (sessions.value.some(s => s.id === sessionId)) {
      currentSessionId.value = sessionId
    }
  }

  function deleteSession(sessionId: string) {
    const index = sessions.value.findIndex(s => s.id === sessionId)
    if (index !== -1) {
      sessions.value.splice(index, 1)
      messages.value.delete(sessionId)
      if (currentSessionId.value === sessionId) {
        currentSessionId.value = sessions.value[0]?.id || generateId()
      }
      saveToStorage()
    }
  }

  function addUserMessage(content: string, sessionId?: string): Message {
    const sid = sessionId || currentSessionId.value
    const message: Message = {
      id: generateId(),
      role: 'user',
      content,
      timestamp: Date.now(),
      status: 'done'
    }
    
    const msgs = messages.value.get(sid) || []
    msgs.push(message)
    messages.value.set(sid, msgs)
    
    updateSessionInfo(sid, content)
    saveToStorage()
    return message
  }

  function addAssistantMessage(sessionId?: string): Message {
    const sid = sessionId || currentSessionId.value
    const message: Message = {
      id: generateId(),
      role: 'assistant',
      content: '',
      timestamp: Date.now(),
      status: 'pending'
    }
    
    const msgs = messages.value.get(sid) || []
    msgs.push(message)
    messages.value.set(sid, msgs)
    return message
  }

  function updateMessage(sessionId: string, messageId: string, updates: Partial<Message>) {
    const msgs = messages.value.get(sessionId)
    if (msgs) {
      const index = msgs.findIndex(m => m.id === messageId)
      if (index !== -1) {
        msgs[index] = { ...msgs[index], ...updates }
      }
    }
  }

  function updateSessionInfo(sessionId: string, firstMessage: string) {
    const session = sessions.value.find(s => s.id === sessionId)
    if (session) {
      if (!session.title || session.title === '新对话') {
        session.title = firstMessage.slice(0, 30) + (firstMessage.length > 30 ? '...' : '')
      }
      session.updatedAt = Date.now()
      session.messageCount = (messages.value.get(sessionId)?.length || 0)
    }
  }

  async function sendMessage(content: string, attachments?: Array<{ file_path: string; file_name: string; file_type: string }>) {
    if (!content.trim() && !attachments?.length) return

    const sessionId = currentSessionId.value
    addUserMessage(content, sessionId)
    const assistantMsg = addAssistantMessage(sessionId)
    
    isLoading.value = true
    streamingContent.value = ''
    
    try {
      const response = await chatApi.sendMessage({
        session_id: sessionId,
        message: content,
        stream: true,
        attachments
      })

      if (response.stream) {
        await handleStream(sessionId, assistantMsg.id, response.request_id)
      }
    } catch (error) {
      updateMessage(sessionId, assistantMsg.id, {
        content: '抱歉，发生了错误，请稍后重试。',
        status: 'error'
      })
    } finally {
      isLoading.value = false
      streamingContent.value = ''
      saveToStorage()
    }
  }

  async function handleStream(sessionId: string, messageId: string, requestId: string) {
    return new Promise<void>((resolve, reject) => {
      const eventSource = createSSEConnection(requestId)
      
      eventSource.onmessage = (event) => {
        try {
          const data: SSEEvent = JSON.parse(event.data)
          handleSSEEvent(sessionId, messageId, data, eventSource, resolve)
        } catch {
          // ignore parse errors
        }
      }

      eventSource.onerror = () => {
        eventSource.close()
        updateMessage(sessionId, messageId, { status: 'error' })
        reject(new Error('SSE connection error'))
      }
    })
  }

  function handleSSEEvent(
    sessionId: string,
    messageId: string,
    event: SSEEvent,
    eventSource: EventSource,
    resolve: () => void
  ) {
    switch (event.type) {
      case 'delta':
        streamingContent.value += event.content || ''
        updateMessage(sessionId, messageId, {
          content: streamingContent.value,
          status: 'streaming'
        })
        break
        
      case 'done':
        updateMessage(sessionId, messageId, {
          content: event.content || streamingContent.value,
          status: 'done'
        })
        eventSource.close()
        resolve()
        break
        
      case 'error':
        updateMessage(sessionId, messageId, {
          content: event.content || '发生错误',
          status: 'error'
        })
        eventSource.close()
        resolve()
        break
        
      case 'tool_start':
        updateMessage(sessionId, messageId, {
          status: 'streaming'
        })
        break
        
      case 'tool_end':
        // 可选：显示工具调用结果
        break
    }
  }

  function cancelRequest() {
    abortController.value?.abort()
    isLoading.value = false
  }

  function clearMessages(sessionId?: string) {
    const sid = sessionId || currentSessionId.value
    messages.value.set(sid, [])
    const session = sessions.value.find(s => s.id === sid)
    if (session) {
      session.messageCount = 0
    }
    saveToStorage()
  }

  function saveToStorage() {
    try {
      const data = {
        sessions: sessions.value,
        messages: Object.fromEntries(messages.value)
      }
      localStorage.setItem('chat_data', JSON.stringify(data))
    } catch {
      // ignore storage errors
    }
  }

  function loadFromStorage() {
    try {
      const stored = localStorage.getItem('chat_data')
      if (stored) {
        const data = JSON.parse(stored)
        sessions.value = data.sessions || []
        messages.value = new Map(Object.entries(data.messages || {}))
        if (sessions.value.length > 0) {
          currentSessionId.value = sessions.value[0].id
        }
      }
    } catch {
      // ignore storage errors
    }
  }

  function init() {
    loadFromStorage()
    if (sessions.value.length === 0) {
      createSession()
    }
  }

  return {
    sessions,
    currentSessionId,
    messages,
    currentMessages,
    currentSession,
    isLoading,
    streamingContent,
    createSession,
    selectSession,
    deleteSession,
    addUserMessage,
    sendMessage,
    cancelRequest,
    clearMessages,
    init,
    saveToStorage,
    loadFromStorage
  }
})
