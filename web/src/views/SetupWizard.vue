<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { configApi } from '@/api'
import type { LLMProvider } from '@/types'

const router = useRouter()

const currentStep = ref(0)
const loading = ref(false)
const providers = ref<LLMProvider[]>([])

const form = ref({
  provider: '',
  apiKey: '',
  apiBase: '',
  modelName: '',
  adminPassword: '',
  confirmPassword: ''
})

const steps = [
  { title: '欢迎', icon: 'Compass' },
  { title: '选择模型', icon: 'Cpu' },
  { title: '设置密码', icon: 'Lock' },
  { title: '完成', icon: 'CircleCheck' }
]

const selectedProvider = computed(() => 
  providers.value.find(p => p.name === form.value.provider)
)

const availableModels = computed(() => 
  selectedProvider.value?.models || []
)

const canProceed = computed(() => {
  switch (currentStep.value) {
    case 0: return true
    case 1: return form.value.provider && form.value.apiKey && form.value.modelName
    case 2: return form.value.adminPassword.length >= 6 && 
                  form.value.adminPassword === form.value.confirmPassword
    case 3: return true
    default: return false
  }
})

async function loadProviders() {
  try {
    const result = await configApi.getProviders()
    providers.value = result.providers || []
    if (providers.value.length > 0) {
      form.value.provider = providers.value[0].name
      form.value.modelName = providers.value[0].models[0] || ''
    }
  } catch {
    providers.value = [
      { name: 'openai', label: 'OpenAI', models: ['gpt-4', 'gpt-3.5-turbo'] },
      { name: 'anthropic', label: 'Anthropic', models: ['claude-3-opus', 'claude-3-sonnet'] },
      { name: 'zhipu', label: '智谱AI', models: ['glm-4', 'glm-5'] },
      { name: 'deepseek', label: 'DeepSeek', models: ['deepseek-chat'] },
      { name: 'qwen', label: '通义千问', models: ['qwen-max'] }
    ]
    form.value.provider = 'openai'
    form.value.modelName = 'gpt-4'
  }
}

function onProviderChange() {
  form.value.modelName = selectedProvider.value?.models[0] || ''
}

async function nextStep() {
  if (currentStep.value === 1) {
    loading.value = true
    try {
      await configApi.testLlm({
        provider: form.value.provider,
        api_key: form.value.apiKey,
        model: form.value.modelName
      })
      ElMessage.success('连接测试成功')
    } catch {
      ElMessage.error('连接测试失败，请检查配置')
      loading.value = false
      return
    } finally {
      loading.value = false
    }
  }
  
  if (currentStep.value === 2) {
    await submitSetup()
    return
  }
  
  currentStep.value++
}

async function submitSetup() {
  loading.value = true
  try {
    await configApi.setup({
      provider: form.value.provider,
      apiKey: form.value.apiKey,
      apiBase: form.value.apiBase || undefined,
      modelName: form.value.modelName,
      adminPassword: form.value.adminPassword
    })
    currentStep.value = 3
  } catch (error) {
    ElMessage.error('设置失败，请重试')
  } finally {
    loading.value = false
  }
}

function goToLogin() {
  router.push('/login')
}

loadProviders()
</script>

<template>
  <div class="setup-container">
    <div class="setup-card">
      <div class="setup-header">
        <h1 class="setup-title">SimpleClaw</h1>
        <p class="setup-subtitle">AI Agent 平台初始化设置</p>
      </div>

      <el-steps :active="currentStep" align-center class="setup-steps">
        <el-step v-for="step in steps" :key="step.title" :title="step.title" :icon="step.icon" />
      </el-steps>

      <div class="setup-content">
        <transition name="fade" mode="out-in">
          <div v-if="currentStep === 0" key="welcome" class="step-content">
            <div class="welcome-icon">
              <el-icon :size="80" color="var(--color-primary-500)"><Promotion /></el-icon>
            </div>
            <h2>欢迎使用 SimpleClaw</h2>
            <p>接下来将引导您完成初始化设置，整个过程大约需要 2 分钟。</p>
            <ul class="feature-list">
              <li><el-icon><Check /></el-icon> 配置 AI 模型</li>
              <li><el-icon><Check /></el-icon> 设置管理员密码</li>
              <li><el-icon><Check /></el-icon> 开始使用</li>
            </ul>
          </div>

          <div v-else-if="currentStep === 1" key="model" class="step-content">
            <h2>配置 AI 模型</h2>
            <el-form label-position="top" class="setup-form">
              <el-form-item label="模型提供商" required>
                <el-select v-model="form.provider" @change="onProviderChange" style="width: 100%">
                  <el-option 
                    v-for="p in providers" 
                    :key="p.name" 
                    :label="p.label" 
                    :value="p.name" 
                  />
                </el-select>
              </el-form-item>
              
              <el-form-item label="API Key" required>
                <el-input 
                  v-model="form.apiKey" 
                  type="password" 
                  show-password 
                  placeholder="请输入 API Key"
                />
              </el-form-item>

              <el-form-item label="API Base URL（可选）">
                <el-input 
                  v-model="form.apiBase" 
                  placeholder="自定义 API 地址，留空使用默认"
                />
              </el-form-item>

              <el-form-item label="模型" required>
                <el-select v-model="form.modelName" style="width: 100%">
                  <el-option 
                    v-for="m in availableModels" 
                    :key="m" 
                    :label="m" 
                    :value="m" 
                  />
                </el-select>
              </el-form-item>
            </el-form>
          </div>

          <div v-else-if="currentStep === 2" key="password" class="step-content">
            <h2>设置管理员密码</h2>
            <el-form label-position="top" class="setup-form">
              <el-form-item label="管理员密码" required>
                <el-input 
                  v-model="form.adminPassword" 
                  type="password" 
                  show-password 
                  placeholder="至少 6 位字符"
                />
              </el-form-item>
              
              <el-form-item label="确认密码" required>
                <el-input 
                  v-model="form.confirmPassword" 
                  type="password" 
                  show-password 
                  placeholder="再次输入密码"
                />
              </el-form-item>
            </el-form>
            <p v-if="form.adminPassword && form.confirmPassword && form.adminPassword !== form.confirmPassword" class="error-text">
              两次输入的密码不一致
            </p>
          </div>

          <div v-else-if="currentStep === 3" key="complete" class="step-content">
            <div class="success-icon">
              <el-icon :size="80" color="var(--color-success)"><CircleCheckFilled /></el-icon>
            </div>
            <h2>设置完成！</h2>
            <p>恭喜，SimpleClaw 已准备就绪。</p>
            <p>您可以开始使用聊天功能，或登录管理后台进行更多配置。</p>
          </div>
        </transition>
      </div>

      <div class="setup-actions">
        <el-button v-if="currentStep > 0 && currentStep < 3" @click="currentStep--">
          上一步
        </el-button>
        <el-button 
          v-if="currentStep < 3" 
          type="primary" 
          :disabled="!canProceed" 
          :loading="loading"
          @click="nextStep"
        >
          {{ currentStep === 2 ? '完成设置' : '下一步' }}
        </el-button>
        <el-button v-if="currentStep === 3" type="primary" @click="goToLogin">
          进入登录
        </el-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.setup-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--color-primary-50) 0%, var(--color-neutral-100) 100%);
  padding: var(--space-4);
}

.setup-card {
  width: 100%;
  max-width: 560px;
  background: var(--color-bg-primary);
  border-radius: var(--radius-2xl);
  box-shadow: var(--shadow-xl);
  padding: var(--space-8);
}

.setup-header {
  text-align: center;
  margin-bottom: var(--space-8);
}

.setup-title {
  font-size: var(--text-3xl);
  font-weight: var(--font-bold);
  color: var(--color-primary-600);
  margin: 0 0 var(--space-2);
}

.setup-subtitle {
  color: var(--color-text-secondary);
  margin: 0;
}

.setup-steps {
  margin-bottom: var(--space-8);
}

.setup-content {
  min-height: 280px;
}

.step-content {
  text-align: center;
}

.step-content h2 {
  font-size: var(--text-xl);
  font-weight: var(--font-semibold);
  margin: 0 0 var(--space-4);
  color: var(--color-text-primary);
}

.step-content p {
  color: var(--color-text-secondary);
  margin: 0 0 var(--space-4);
}

.welcome-icon, .success-icon {
  margin-bottom: var(--space-6);
}

.feature-list {
  list-style: none;
  padding: 0;
  margin: var(--space-6) 0;
  text-align: left;
  display: inline-block;
}

.feature-list li {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) 0;
  color: var(--color-text-primary);
}

.feature-list .el-icon {
  color: var(--color-success);
}

.setup-form {
  text-align: left;
  margin-top: var(--space-6);
}

.error-text {
  color: var(--color-error);
  font-size: var(--text-sm);
}

.setup-actions {
  display: flex;
  justify-content: center;
  gap: var(--space-4);
  margin-top: var(--space-8);
}

.setup-actions .el-button {
  min-width: 120px;
}

.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from, .fade-leave-to {
  opacity: 0;
}
</style>
