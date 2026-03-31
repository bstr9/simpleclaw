<template>
  <div class="config-container">
    <el-container>
      <el-header class="app-header">
        <div class="header-left">
          <el-button link @click="router.push('/')">
            <el-icon><ArrowLeft /></el-icon>
            返回
          </el-button>
          <h1>配置管理</h1>
        </div>
        <div class="header-right">
          <el-button type="primary" @click="saveConfig" :loading="saving">
            保存配置
          </el-button>
        </div>
      </el-header>

      <el-main>
        <el-tabs v-model="activeTab" type="border-card">
          <el-tab-pane label="LLM 配置" name="llm">
            <el-form :model="configData.llm" label-width="120px">
              <el-form-item label="默认提供商">
                <el-select v-model="configData.llm.default_provider" style="width: 200px;">
                  <el-option v-for="p in llmProviders" :key="p.value" :label="p.label" :value="p.value" />
                </el-select>
              </el-form-item>

              <el-divider content-position="left">提供商配置</el-divider>

              <div v-for="(provider, name) in configData.llm.providers" :key="name" class="provider-config">
                <h4>{{ getProviderLabel(name) }}</h4>
                <el-form-item label="API Key">
                  <el-input v-model="provider.api_key" type="password" show-password />
                </el-form-item>
                <el-form-item label="Base URL">
                  <el-input v-model="provider.base_url" placeholder="可选" />
                </el-form-item>
                <el-form-item label="默认模型">
                  <el-input v-model="provider.model" placeholder="默认模型名称" />
                </el-form-item>
                <el-form-item label="启用">
                  <el-switch v-model="provider.enabled" />
                </el-form-item>
              </div>
            </el-form>
          </el-tab-pane>

          <el-tab-pane label="渠道配置" name="channels">
            <el-form :model="configData.channels" label-width="120px">
              <div v-for="(channel, name) in configData.channels" :key="name" class="channel-config">
                <el-divider content-position="left">{{ getChannelLabel(name) }}</el-divider>
                <el-form-item label="启用">
                  <el-switch v-model="channel.enabled" />
                </el-form-item>
                <template v-if="channel.enabled">
                  <el-form-item v-if="channel.webhook_url" label="Webhook URL">
                    <el-input v-model="channel.webhook_url" />
                  </el-form-item>
                  <el-form-item v-if="channel.app_id" label="App ID">
                    <el-input v-model="channel.app_id" />
                  </el-form-item>
                  <el-form-item v-if="channel.app_secret" label="App Secret">
                    <el-input v-model="channel.app_secret" type="password" show-password />
                  </el-form-item>
                  <el-form-item v-if="channel.token" label="Token">
                    <el-input v-model="channel.token" type="password" show-password />
                  </el-form-item>
                  <el-form-item v-if="channel.encoding_aes_key" label="Encoding AES Key">
                    <el-input v-model="channel.encoding_aes_key" type="password" show-password />
                  </el-form-item>
                </template>
              </div>
            </el-form>
          </el-tab-pane>

          <el-tab-pane label="Agent 配置" name="agent">
            <el-form :model="configData.agent" label-width="120px">
              <el-form-item label="Agent 名称">
                <el-input v-model="configData.agent.name" />
              </el-form-item>
              <el-form-item label="系统提示词">
                <el-input
                  v-model="configData.agent.system_prompt"
                  type="textarea"
                  :rows="5"
                  placeholder="Agent 的系统提示词"
                />
              </el-form-item>
              <el-form-item label="最大上下文">
                <el-input-number v-model="configData.agent.max_context" :min="1" :max="100" />
              </el-form-item>
              <el-form-item label="温度">
                <el-slider v-model="configData.agent.temperature" :min="0" :max="2" :step="0.1" show-input />
              </el-form-item>
              <el-form-item label="最大 Token">
                <el-input-number v-model="configData.agent.max_tokens" :min="100" :max="32000" />
              </el-form-item>
              <el-form-item label="启用记忆">
                <el-switch v-model="configData.agent.enable_memory" />
              </el-form-item>
              <el-form-item v-if="configData.agent.enable_memory" label="记忆类型">
                <el-select v-model="configData.agent.memory_type" style="width: 200px;">
                  <el-option label="短期记忆" value="short_term" />
                  <el-option label="长期记忆" value="long_term" />
                  <el-option label="向量记忆" value="vector" />
                </el-select>
              </el-form-item>
            </el-form>
          </el-tab-pane>

          <el-tab-pane label="工具配置" name="tools">
            <el-form :model="configData.tools" label-width="120px">
              <div v-for="(tool, name) in configData.tools" :key="name" class="tool-config">
                <el-divider content-position="left">{{ getToolLabel(name) }}</el-divider>
                <el-form-item label="启用">
                  <el-switch v-model="tool.enabled" />
                </el-form-item>
                <template v-if="tool.enabled">
                  <el-form-item v-if="tool.timeout !== undefined" label="超时时间">
                    <el-input-number v-model="tool.timeout" :min="1000" :max="300000" />
                    <span style="margin-left: 10px; color: #909399;">毫秒</span>
                  </el-form-item>
                  <el-form-item v-if="tool.max_results !== undefined" label="最大结果数">
                    <el-input-number v-model="tool.max_results" :min="1" :max="50" />
                  </el-form-item>
                  <el-form-item v-if="tool.allowed_commands !== undefined" label="允许的命令">
                    <el-select v-model="tool.allowed_commands" multiple style="width: 100%;">
                      <el-option v-for="cmd in commonCommands" :key="cmd" :label="cmd" :value="cmd" />
                    </el-select>
                  </el-form-item>
                </template>
              </div>
            </el-form>
          </el-tab-pane>
        </el-tabs>
      </el-main>
    </el-container>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import { useConfigStore } from '@/stores/config'

const router = useRouter()
const configStore = useConfigStore()

const activeTab = ref('llm')
const saving = ref(false)

const configData = reactive({
  llm: {
    default_provider: 'openai',
    providers: {
      openai: { api_key: '', base_url: '', model: 'gpt-4', enabled: true },
      claude: { api_key: '', base_url: '', model: 'claude-3-opus', enabled: false },
      gemini: { api_key: '', base_url: '', model: 'gemini-pro', enabled: false },
      azure: { api_key: '', base_url: '', model: '', enabled: false },
      ollama: { api_key: '', base_url: 'http://localhost:11434', model: '', enabled: false },
      deepseek: { api_key: '', base_url: '', model: 'deepseek-chat', enabled: false }
    }
  },
  channels: {
    terminal: { enabled: true },
    web: { enabled: false, port: 8080 },
    feishu: { enabled: false, app_id: '', app_secret: '' },
    dingtalk: { enabled: false, app_key: '', app_secret: '' },
    weixin: { enabled: false },
    wechatmp: { enabled: false, app_id: '', app_secret: '', token: '', encoding_aes_key: '' },
    qq: { enabled: false }
  },
  agent: {
    name: 'SimpleClaw',
    system_prompt: '',
    max_context: 10,
    temperature: 0.7,
    max_tokens: 4096,
    enable_memory: true,
    memory_type: 'short_term'
  },
  tools: {
    read: { enabled: true },
    write: { enabled: true },
    edit: { enabled: true },
    bash: { enabled: true, timeout: 30000, allowed_commands: [] },
    web_search: { enabled: true, max_results: 5 },
    web_fetch: { enabled: true, timeout: 30000 },
    memory: { enabled: true },
    vision: { enabled: true }
  }
})

const llmProviders = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'claude', label: 'Claude' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'azure', label: 'Azure OpenAI' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'deepseek', label: 'DeepSeek' }
]

const commonCommands = ['ls', 'cat', 'grep', 'find', 'mkdir', 'rm', 'cp', 'mv', 'git', 'npm', 'go', 'python']

function getProviderLabel(name) {
  const provider = llmProviders.find(p => p.value === name)
  return provider?.label || name
}

const channelLabels = {
  terminal: '终端',
  web: 'Web',
  feishu: '飞书',
  dingtalk: '钉钉',
  weixin: '微信',
  wechatmp: '微信公众号',
  qq: 'QQ'
}

function getChannelLabel(name) {
  return channelLabels[name] || name
}

const toolLabels = {
  read: '文件读取',
  write: '文件写入',
  edit: '文件编辑',
  bash: 'Shell 命令',
  web_search: '网络搜索',
  web_fetch: '网页获取',
  memory: '记忆系统',
  vision: '视觉识别'
}

function getToolLabel(name) {
  return toolLabels[name] || name
}

async function saveConfig() {
  try {
    await ElMessageBox.confirm('确定要保存配置吗？', '提示', {
      confirmButtonText: '保存',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }

  saving.value = true
  try {
    await configStore.updateConfig(configData)
    ElMessage.success('配置已保存')
  } catch (error) {
    void error
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  try {
    const config = await configStore.fetchConfig()
    if (config) {
      Object.assign(configData, config)
    }
  } catch (error) {
    void error
  }
})
</script>

<style scoped>
.config-container {
  min-height: 100vh;
  background: #f5f7fa;
}

.app-header {
  background: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 15px;
}

.header-left h1 {
  margin: 0;
  font-size: 20px;
  color: #303133;
}

.provider-config,
.channel-config,
.tool-config {
  padding: 15px;
  margin-bottom: 15px;
  background: #fafafa;
  border-radius: 4px;
}

.provider-config h4 {
  margin: 0 0 15px 0;
  color: #303133;
}
</style>
