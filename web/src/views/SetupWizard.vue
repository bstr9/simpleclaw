<template>
  <div class="setup-container">
    <el-card class="setup-card">
      <template #header>
        <div class="card-header">
          <h1>SimpleClaw 初始配置向导</h1>
          <p>欢迎使用 SimpleClaw，请完成以下配置步骤</p>
        </div>
      </template>

      <el-steps :active="currentStep" align-center finish-status="success">
        <el-step title="选择 LLM" />
        <el-step title="配置 API" />
        <el-step title="选择渠道" />
        <el-step title="设置账号" />
      </el-steps>

      <div class="step-content">
        <div v-show="currentStep === 0" class="step-panel">
          <h3>选择大语言模型提供商</h3>
          <el-radio-group v-model="setupData.llmProvider" class="provider-group">
            <el-radio-button v-for="provider in llmProviders" :key="provider.value" :value="provider.value">
              {{ provider.label }}
            </el-radio-button>
          </el-radio-group>
          <div class="provider-desc">
            {{ currentProviderDesc }}
          </div>
        </div>

        <div v-show="currentStep === 1" class="step-panel">
          <h3>配置 API 连接</h3>
          <el-form :model="setupData" label-width="100px">
            <el-form-item label="API Key" required>
              <el-input
                v-model="setupData.apiKey"
                type="password"
                show-password
                placeholder="请输入 API Key"
              />
            </el-form-item>
            <el-form-item label="Base URL">
              <el-input
                v-model="setupData.baseUrl"
                placeholder="可选，自定义 API 端点"
              />
            </el-form-item>
            <el-form-item label="模型名称">
              <el-input
                v-model="setupData.model"
                placeholder="默认模型名称"
              />
            </el-form-item>
            <el-form-item>
              <el-button @click="testConnection" :loading="testing">
                测试连接
              </el-button>
              <span v-if="testResult" :class="['test-result', testResult.success ? 'success' : 'error']">
                {{ testResult.message }}
              </span>
            </el-form-item>
          </el-form>
        </div>

        <div v-show="currentStep === 2" class="step-panel">
          <h3>选择默认渠道</h3>
          <el-checkbox-group v-model="setupData.channels" class="channel-group">
            <el-checkbox v-for="channel in availableChannels" :key="channel.value" :value="channel.value">
              <div class="channel-item">
                <span class="channel-name">{{ channel.label }}</span>
                <span class="channel-desc">{{ channel.desc }}</span>
              </div>
            </el-checkbox>
          </el-checkbox-group>
        </div>

        <div v-show="currentStep === 3" class="step-panel">
          <h3>设置管理员账号</h3>
          <el-form :model="setupData" label-width="100px" :rules="adminRules" ref="adminFormRef">
            <el-form-item label="用户名" prop="adminUsername">
              <el-input v-model="setupData.adminUsername" placeholder="管理员用户名" />
            </el-form-item>
            <el-form-item label="密码" prop="adminPassword">
              <el-input
                v-model="setupData.adminPassword"
                type="password"
                show-password
                placeholder="管理员密码"
              />
            </el-form-item>
            <el-form-item label="确认密码" prop="adminPasswordConfirm">
              <el-input
                v-model="setupData.adminPasswordConfirm"
                type="password"
                show-password
                placeholder="再次输入密码"
              />
            </el-form-item>
          </el-form>
        </div>
      </div>

      <div class="step-actions">
        <el-button v-if="currentStep > 0" @click="prevStep">上一步</el-button>
        <el-button v-if="currentStep < 3" type="primary" @click="nextStep">下一步</el-button>
        <el-button v-else type="primary" @click="submitSetup" :loading="submitting">
          完成配置
        </el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useConfigStore } from '@/stores/config'

const router = useRouter()
const configStore = useConfigStore()

const currentStep = ref(0)
const testing = ref(false)
const submitting = ref(false)
const testResult = ref(null)
const adminFormRef = ref(null)

const setupData = ref({
  llmProvider: 'openai',
  apiKey: '',
  baseUrl: '',
  model: '',
  channels: ['terminal'],
  adminUsername: '',
  adminPassword: '',
  adminPasswordConfirm: ''
})

const llmProviders = [
  { value: 'openai', label: 'OpenAI', desc: 'GPT-4, GPT-3.5 等模型' },
  { value: 'claude', label: 'Claude', desc: 'Anthropic Claude 系列模型' },
  { value: 'gemini', label: 'Gemini', desc: 'Google Gemini 系列模型' },
  { value: 'azure', label: 'Azure OpenAI', desc: '微软 Azure OpenAI 服务' },
  { value: 'ollama', label: 'Ollama', desc: '本地部署的 Ollama 服务' },
  { value: 'deepseek', label: 'DeepSeek', desc: 'DeepSeek 大模型' }
]

const availableChannels = [
  { value: 'terminal', label: 'Terminal', desc: '命令行终端' },
  { value: 'web', label: 'Web', desc: 'Web 网页界面' },
  { value: 'feishu', label: '飞书', desc: '飞书机器人' },
  { value: 'dingtalk', label: '钉钉', desc: '钉钉机器人' },
  { value: 'weixin', label: '微信', desc: '微信个人号' },
  { value: 'wechatmp', label: '微信公众号', desc: '微信公众号' },
  { value: 'qq', label: 'QQ', desc: 'QQ 机器人' }
]

const currentProviderDesc = computed(() => {
  const provider = llmProviders.find(p => p.value === setupData.value.llmProvider)
  return provider?.desc || ''
})

const adminRules = {
  adminUsername: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 20, message: '用户名长度 3-20 个字符', trigger: 'blur' }
  ],
  adminPassword: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少 6 个字符', trigger: 'blur' }
  ],
  adminPasswordConfirm: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== setupData.value.adminPassword) {
          callback(new Error('两次输入的密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

async function testConnection() {
  if (!setupData.value.apiKey) {
    ElMessage.warning('请先输入 API Key')
    return
  }
  
  testing.value = true
  testResult.value = null
  
  try {
    const result = await configStore.testLlm({
      provider: setupData.value.llmProvider,
      api_key: setupData.value.apiKey,
      base_url: setupData.value.baseUrl,
      model: setupData.value.model
    })
    testResult.value = { success: true, message: '连接成功！' }
  } catch (error) {
    testResult.value = { success: false, message: '连接失败，请检查配置' }
  } finally {
    testing.value = false
  }
}

function prevStep() {
  if (currentStep.value > 0) {
    currentStep.value--
  }
}

async function nextStep() {
  if (currentStep.value === 0) {
    currentStep.value++
  } else if (currentStep.value === 1) {
    if (!setupData.value.apiKey) {
      ElMessage.warning('请输入 API Key')
      return
    }
    currentStep.value++
  } else if (currentStep.value === 2) {
    if (setupData.value.channels.length === 0) {
      ElMessage.warning('请至少选择一个渠道')
      return
    }
    currentStep.value++
  }
}

async function submitSetup() {
  try {
    await adminFormRef.value.validate()
  } catch {
    return
  }
  
  submitting.value = true
  
  try {
    await configStore.setup({
      llm: {
        provider: setupData.value.llmProvider,
        api_key: setupData.value.apiKey,
        base_url: setupData.value.baseUrl,
        model: setupData.value.model
      },
      channels: setupData.value.channels,
      admin: {
        username: setupData.value.adminUsername,
        password: setupData.value.adminPassword
      }
    })
    
    ElMessage.success('配置完成！')
    router.push('/login')
  } catch (error) {
    ElMessage.error('配置失败，请重试')
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  try {
    const status = await configStore.fetchStatus()
    if (status.configured) {
      router.replace('/login')
    }
  } catch (error) {
    void error
  }
})
</script>

<style scoped>
.setup-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 20px;
}

.setup-card {
  width: 100%;
  max-width: 600px;
}

.card-header {
  text-align: center;
}

.card-header h1 {
  margin: 0 0 8px 0;
  font-size: 24px;
}

.card-header p {
  margin: 0;
  color: #909399;
}

.step-content {
  margin: 30px 0;
  min-height: 250px;
}

.step-panel {
  padding: 20px 0;
}

.step-panel h3 {
  margin-bottom: 20px;
  color: #303133;
}

.provider-group {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.provider-desc {
  margin-top: 15px;
  color: #909399;
  font-size: 14px;
}

.channel-group {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.channel-item {
  display: flex;
  flex-direction: column;
}

.channel-name {
  font-weight: 500;
}

.channel-desc {
  font-size: 12px;
  color: #909399;
}

.test-result {
  margin-left: 15px;
  font-size: 14px;
}

.test-result.success {
  color: #67c23a;
}

.test-result.error {
  color: #f56c6c;
}

.step-actions {
  display: flex;
  justify-content: center;
  gap: 15px;
}
</style>
