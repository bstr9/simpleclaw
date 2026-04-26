<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useConfigStore } from '@/stores/config'
import type { ChatConfig, LLMProvider } from '@/types'

const configStore = useConfigStore()

const config = ref<ChatConfig | null>(null)
const providers = ref<LLMProvider[]>([])
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)

const activeTab = ref('llm')

const llmForm = ref({
  provider: '',
  model: '',
  apiKey: '',
  apiBase: ''
})

const agentForm = ref({
  maxSteps: 15,
  maxContextTokens: 4000,
  maxContextTurns: 10
})

const selectedProvider = computed(() => 
  providers.value.find(p => p.name === llmForm.value.provider)
)

const availableModels = computed(() => 
  selectedProvider.value?.models || []
)

async function loadData() {
  loading.value = true
  try {
    config.value = await configStore.fetchConfig()
    const providersResult = await configStore.fetchProviders()
    providers.value = providersResult
    
    if (config.value) {
      llmForm.value.model = config.value.model || ''
      agentForm.value.maxSteps = config.value.agent_max_steps || 15
      agentForm.value.maxContextTokens = config.value.agent_max_context_tokens || 4000
      agentForm.value.maxContextTurns = config.value.agent_max_context_turns || 10
    }
  } catch {
    ElMessage.error('加载配置失败')
  } finally {
    loading.value = false
  }
}

function onProviderChange() {
  llmForm.value.model = selectedProvider.value?.models[0] || ''
}

async function testConnection() {
  if (!llmForm.value.provider || !llmForm.value.apiKey) {
    ElMessage.warning('请选择提供商并输入 API Key')
    return
  }

  testing.value = true
  try {
    await configStore.testLlm({
      provider: llmForm.value.provider,
      api_key: llmForm.value.apiKey,
      model: llmForm.value.model
    })
    ElMessage.success('连接测试成功')
  } catch {
    ElMessage.error('连接测试失败')
  } finally {
    testing.value = false
  }
}

async function saveLlmConfig() {
  saving.value = true
  try {
    await configStore.updateConfig({
      model: llmForm.value.model
    })
    ElMessage.success('配置已保存')
  } catch {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

async function saveAgentConfig() {
  saving.value = true
  try {
    await configStore.updateConfig({
      agent_max_steps: agentForm.value.maxSteps,
      agent_max_context_tokens: agentForm.value.maxContextTokens,
      agent_max_context_turns: agentForm.value.maxContextTurns
    })
    ElMessage.success('配置已保存')
  } catch {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadData)

// TODO: 渠道启停功能需要后端 API 支持（POST /admin/api/channels/:name/toggle）
// 当前后端仅提供 GET /admin/api/channels 查询渠道状态，尚未实现启停接口
async function toggleChannel(channel: { name: string; active: boolean }, enable: boolean) {
  try {
    // TODO: 调用后端启停 API，示例：
    // await configApi.toggleChannel(channel.name, enable)
    ElMessage.warning('渠道启停功能需要后端 API 支持')
  } catch {
    ElMessage.error('操作失败')
  }
}
</script>

<template>
  <div class="config-page">
    <div class="page-header">
      <h1>配置管理</h1>
      <el-button :icon="Refresh" @click="loadData" :loading="loading">
        刷新
      </el-button>
    </div>

    <el-tabs v-model="activeTab" class="config-tabs">
      <el-tab-pane label="LLM 配置" name="llm">
        <el-card shadow="never">
          <el-form label-position="top" class="config-form">
            <el-form-item label="模型提供商">
              <el-select 
                v-model="llmForm.provider" 
                @change="onProviderChange"
                style="width: 100%"
                placeholder="选择模型提供商"
              >
                <el-option
                  v-for="p in providers"
                  :key="p.name"
                  :label="p.label"
                  :value="p.name"
                />
              </el-select>
            </el-form-item>

            <el-form-item label="模型">
              <el-select 
                v-model="llmForm.model" 
                style="width: 100%"
                placeholder="选择模型"
              >
                <el-option
                  v-for="m in availableModels"
                  :key="m"
                  :label="m"
                  :value="m"
                />
              </el-select>
            </el-form-item>

            <el-form-item label="API Key">
              <el-input
                v-model="llmForm.apiKey"
                type="password"
                show-password
                placeholder="输入 API Key"
              />
            </el-form-item>

            <el-form-item label="API Base URL (可选)">
              <el-input
                v-model="llmForm.apiBase"
                placeholder="自定义 API 地址"
              />
            </el-form-item>

            <el-form-item>
              <el-button type="primary" @click="saveLlmConfig" :loading="saving">
                保存配置
              </el-button>
              <el-button @click="testConnection" :loading="testing">
                测试连接
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="Agent 配置" name="agent">
        <el-card shadow="never">
          <el-form label-position="top" class="config-form">
            <el-form-item label="最大执行步数">
              <el-input-number 
                v-model="agentForm.maxSteps" 
                :min="1" 
                :max="50"
              />
              <div class="form-hint">Agent 在单次请求中最多执行的步骤数</div>
            </el-form-item>

            <el-form-item label="最大上下文 Token 数">
              <el-input-number 
                v-model="agentForm.maxContextTokens" 
                :min="1000" 
                :max="128000"
                :step="1000"
              />
              <div class="form-hint">保留给上下文的最大 Token 数量</div>
            </el-form-item>

            <el-form-item label="最大上下文轮数">
              <el-input-number 
                v-model="agentForm.maxContextTurns" 
                :min="1" 
                :max="50"
              />
              <div class="form-hint">保留的对话历史轮数</div>
            </el-form-item>

            <el-form-item>
              <el-button type="primary" @click="saveAgentConfig" :loading="saving">
                保存配置
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="渠道配置" name="channels">
        <el-card shadow="never">
          <el-table :data="configStore.channels" stripe>
            <el-table-column prop="name" label="渠道名称" />
            <el-table-column label="状态">
              <template #default="{ row }">
                <el-tag :type="row.active ? 'success' : 'info'">
                  {{ row.active ? '运行中' : '已停止' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200">
              <template #default="{ row }">
                <el-button 
                  v-if="!row.active" 
                  type="primary" 
                  size="small"
                  text
                  @click="toggleChannel(row, true)"
                >
                  启动
                </el-button>
                <el-button 
                  v-else 
                  type="danger" 
                  size="small"
                  text
                  @click="toggleChannel(row, false)"
                >
                  停止
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped>
.config-page {
  max-width: 1000px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--space-6);
}

.page-header h1 {
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  margin: 0;
}

.config-tabs {
  background: var(--color-bg-primary);
  border-radius: var(--radius-xl);
  padding: var(--space-4);
}

.config-form {
  max-width: 500px;
}

.form-hint {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
  margin-top: var(--space-1);
}
</style>
