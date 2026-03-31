import { defineStore } from 'pinia'
import { ref } from 'vue'
import { configApi } from '@/api'

export const useConfigStore = defineStore('config', () => {
  const config = ref(null)
  const status = ref(null)
  const channels = ref([])
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
    channels.value = await configApi.getChannels()
    return channels.value
  }

  async function updateConfig(newConfig) {
    loading.value = true
    try {
      config.value = await configApi.updateConfig(newConfig)
      return config.value
    } finally {
      loading.value = false
    }
  }

  async function validateConfig(configData) {
    return configApi.validateConfig(configData)
  }

  async function testLlm(llmConfig) {
    return configApi.testLlm(llmConfig)
  }

  async function setup(setupData) {
    return configApi.setup(setupData)
  }

  return {
    config,
    status,
    channels,
    loading,
    fetchConfig,
    fetchStatus,
    fetchChannels,
    updateConfig,
    validateConfig,
    testLlm,
    setup
  }
})
