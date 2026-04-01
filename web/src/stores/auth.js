import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('admin_token') || '')
  const user = ref(JSON.parse(localStorage.getItem('admin_user') || 'null'))

  const isAuthenticated = computed(() => !!token.value)

  async function login(username, password) {
    const res = await authApi.login(username, password)
    const loginData = res.data || res
    token.value = loginData.token
    user.value = loginData.user || { username }
    localStorage.setItem('admin_token', loginData.token)
    localStorage.setItem('admin_user', JSON.stringify(loginData.user || { username }))
    return res
  }

  async function logout() {
    try {
      await authApi.logout()
    } catch (e) {
      void e
    }
    token.value = ''
    user.value = null
    localStorage.removeItem('admin_token')
    localStorage.removeItem('admin_user')
  }

  function clearAuth() {
    token.value = ''
    user.value = null
    localStorage.removeItem('admin_token')
    localStorage.removeItem('admin_user')
  }

  return {
    token,
    user,
    isAuthenticated,
    login,
    logout,
    clearAuth
  }
})
