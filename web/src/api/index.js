import axios from 'axios'
import { ElMessage } from 'element-plus'

const api = axios.create({
  baseURL: '/admin/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('admin_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

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
  login: (username, password) => api.post('/auth/login', { username, password }),
  logout: () => api.post('/auth/logout')
}

export const configApi = {
  setup: (data) => api.post('/setup', data),
  getConfig: () => api.get('/config'),
  updateConfig: (config) => api.put('/config', config),
  validateConfig: (config) => api.post('/config/validate', config),
  testLlm: (llmConfig) => api.post('/test/llm', llmConfig),
  getStatus: () => api.get('/status'),
  getChannels: () => api.get('/channels')
}

export default api
