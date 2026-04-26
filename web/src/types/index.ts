export interface User {
  id: string
  username: string
  role: 'admin' | 'user'
  createdAt: string
}

export interface AuthState {
  token: string | null
  user: User | null
  isAuthenticated: boolean
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  user: User
}

export interface SetupRequest {
  provider: string
  apiKey: string
  apiBase?: string
  modelName: string
  adminPassword: string
}

export interface SystemStatus {
  version: string
  go_version?: string
  os?: string
  uptime: string
  start_time?: string
  memory_usage?: string
  cpu_cores?: number
  total_sessions?: number
  llm_connected?: boolean
  has_llm_config?: boolean
  has_password: boolean
  is_configured: boolean
}

export interface Channel {
  name: string
  label: Record<string, string>
  active: boolean
  type?: string
  connections?: number
}

export interface LLMProvider {
  name: string
  label: string
  models: string[]
}

export interface Message {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  timestamp: number
  status?: 'pending' | 'streaming' | 'done' | 'error'
  toolCalls?: ToolCall[]
}

export interface ToolCall {
  name: string
  arguments: Record<string, unknown>
  status: 'pending' | 'running' | 'done' | 'error'
  result?: string
  executionTime?: number
}

export interface Session {
  id: string
  title: string
  createdAt: number
  updatedAt: number
  messageCount: number
}

export interface ChatConfig {
  use_agent: boolean
  title: string
  model: string
  bot_type: string
  port: number
  agent_max_context_tokens: number
  agent_max_context_turns: number
  agent_max_steps: number
  debug: boolean
}

export interface SSEEvent {
  type: 'delta' | 'done' | 'error' | 'tool_start' | 'tool_end'
  content?: string
  tool?: string
  arguments?: Record<string, unknown>
  status?: string
  result?: string
  execution_time?: number
  request_id?: string
  timestamp?: number
}

export interface UploadResponse {
  status: string
  file_path: string
  file_name: string
  file_type: 'image' | 'video' | 'file'
  preview_url: string
}

export interface ApiResponse<T = unknown> {
  status: 'success' | 'error'
  data?: T
  message?: string
}
