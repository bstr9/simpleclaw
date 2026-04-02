import axios from 'axios'
import { ElMessage } from 'element-plus'
import type {
  LoginRequest,
  LoginResponse,
  SetupRequest,
  SystemStatus,
  Channel,
  LLMProvider,
  ChatConfig,
  UploadResponse,
  ApiResponse
} from '@/types'

const api = axios.create({
  baseURL: '/admin/api',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' }
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('admin_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (response) => response.data,
  (error) => {
    const { response } = error
    if (response) {
      const { status, data } = response
      if (status === 401) {
        localStorage.removeItem('admin_token')
        localStorage.removeItem('admin_user')
        window.location.href = '/login'
      } else if (data?.message) {
        ElMessage.error(data.message)
      } else {
        ElMessage.error(`请求失败: ${status}`)
      }
    } else {
      ElMessage.error('网络错误，请检查网络连接')
    }
    return Promise.reject(error)
  }
)

export const authApi = {
  login: (data: LoginRequest): Promise<LoginResponse> =>
    api.post('/auth/login', data),
  logout: (): Promise<ApiResponse> =>
    api.post('/auth/logout')
}

export const configApi = {
  setup: (data: SetupRequest): Promise<ApiResponse> =>
    api.post('/setup', data),
  getStatus: (): Promise<SystemStatus> =>
    api.get('/status'),
  getConfig: (): Promise<ChatConfig> =>
    api.get('/config'),
  updateConfig: (config: Partial<ChatConfig>): Promise<ApiResponse> =>
    api.put('/config', config),
  getChannels: (): Promise<{ channels: Channel[] }> =>
    api.get('/channels'),
  getProviders: (): Promise<{ providers: LLMProvider[] }> =>
    api.get('/providers'),
  testLlm: (data: { provider?: string; api_key?: string; model?: string }): Promise<ApiResponse> =>
    api.post('/test/llm', data)
}

export const chatApi = {
  sendMessage: (data: {
    session_id: string
    message: string
    stream?: boolean
    attachments?: Array<{ file_path: string; file_name: string; file_type: string }>
  }): Promise<{ request_id: string; stream: boolean }> =>
    axios.post('/message', data).then(r => r.data),

  upload: async (file: File, sessionId?: string): Promise<UploadResponse> => {
    const formData = new FormData()
    formData.append('file', file)
    if (sessionId) {
      formData.append('session_id', sessionId)
    }
    const response = await axios.post('/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 60000
    })
    return response.data
  },

  getConfig: (): Promise<ChatConfig> =>
    axios.get('/config').then(r => r.data),

  getProviders: (): Promise<{ providers: LLMProvider[] }> =>
    axios.get('/api/providers').then(r => r.data)
}

export const createSSEConnection = (requestId: string): EventSource => {
  return new EventSource(`/stream?request_id=${requestId}`)
}

export default api
