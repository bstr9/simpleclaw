import { defineStore } from 'pinia'
import { ref } from 'vue'
import { configApi } from '@/api'
import type { SystemStatus, Channel, LLMProvider, ChatConfig, SetupRequest } from '@/types'

export const useConfigStore = defineStore('config', () => {
  const config = ref<ChatConfig | null>(null)
  const status = ref<SystemStatus | null>(null)
  const channels = ref<Channel[]>([])
  const providers = ref<LLMProvider[]>([])
  const loading = ref(false)

  async function fetchConfig() {
    loading.value = true
    try {
      config.value = await configApi.getConfig()
    } finally {
      loading.value = false
    }
  }

  async function fetchStatus() {
    status.value = await configApi.getStatus()
    return status.value
  }

  async function fetchChannels() {
    const result = await configApi.getChannels()
    channels.value = result.channels || []
    return channels.value
  }

  async function fetchProviders() {
    const result = await configApi.getProviders()
    providers.value = result.providers || []
    return providers.value
  }

  async function updateConfig(newConfig: Partial<ChatConfig>) {
    loading.value = true
    try {
      await configApi.updateConfig(newConfig)
      if (config.value) {
        Object.assign(config.value, newConfig)
      }
    } finally {
      loading.value = false
    }
  }

  async function testLlm(data: { provider?: string; api_key?: string; model?: string }) {
    return configApi.testLlm(data)
  }

  async function setup(data: SetupRequest) {
    return configApi.setup(data)
  }

  return {
    config,
    status,
    channels,
    providers,
    loading,
    fetchConfig,
    fetchStatus,
    fetchChannels,
    fetchProviders,
    updateConfig,
    testLlm,
    setup
  }
})
