<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useConfigStore } from '@/stores/config'
import { useAuthStore } from '@/stores/auth'
import type { SystemStatus, Channel } from '@/types'

const router = useRouter()
const configStore = useConfigStore()
const authStore = useAuthStore()

const status = ref<SystemStatus | null>(null)
const channels = ref<Channel[]>([])
const loading = ref(false)
const testingLlm = ref(false)

async function loadDashboard() {
  loading.value = true
  try {
    status.value = await configStore.fetchStatus()
    const channelsResult = await configStore.fetchChannels()
    channels.value = channelsResult
  } catch {
    ElMessage.error('加载数据失败')
  } finally {
    loading.value = false
  }
}

async function testLlm() {
  testingLlm.value = true
  try {
    await configStore.testLlm({})
    ElMessage.success('LLM 连接正常')
  } catch {
    ElMessage.error('LLM 连接失败')
  } finally {
    testingLlm.value = false
  }
}

function goToConfig() {
  router.push('/admin/config')
}

async function handleLogout() {
  await authStore.logout()
  router.push('/login')
}

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  
  if (days > 0) return `${days}天 ${hours}小时`
  if (hours > 0) return `${hours}小时 ${mins}分钟`
  return `${mins}分钟`
}

onMounted(loadDashboard)
</script>

<template>
  <div class="dashboard">
    <div class="page-header">
      <h1>仪表盘</h1>
      <el-button :icon="Refresh" @click="loadDashboard" :loading="loading">
        刷新
      </el-button>
    </div>

    <el-row :gutter="20" class="stat-cards">
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon primary">
              <el-icon :size="28"><Connection /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ channels.filter(c => c.active).length }}</div>
              <div class="stat-label">活跃渠道</div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon success">
              <el-icon :size="28"><ChatDotRound /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ status?.total_sessions ?? 0 }}</div>
              <div class="stat-label">总会话数</div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon warning">
              <el-icon :size="28"><Timer /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ status?.uptime || '-' }}</div>
              <div class="stat-label">运行时间</div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" :class="status?.has_llm_config ? 'success' : 'danger'">
              <el-icon :size="28"><Cpu /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ status?.has_llm_config ? '正常' : '异常' }}</div>
              <div class="stat-label">LLM 状态</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px;">
      <el-col :xs="24" :lg="16">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>渠道状态</span>
            </div>
          </template>
          <el-table :data="channels" stripe>
            <el-table-column prop="name" label="渠道名称" />
            <el-table-column label="状态">
              <template #default="{ row }">
                <el-tag :type="row.active ? 'success' : 'info'">
                  {{ row.active ? '运行中' : '已停止' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="connections" label="连接数" width="100" />
          </el-table>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="8">
        <el-card>
          <template #header>
            <span>快捷操作</span>
          </template>
          <div class="quick-actions">
            <el-button type="primary" @click="goToConfig" class="action-btn">
              <el-icon><Tools /></el-icon>
              配置管理
            </el-button>
            <el-button type="success" @click="testLlm" :loading="testingLlm" class="action-btn">
              <el-icon><Connection /></el-icon>
              测试 LLM 连接
            </el-button>
            <el-button type="info" @click="$router.push('/')" class="action-btn">
              <el-icon><ChatDotRound /></el-icon>
              进入聊天
            </el-button>
            <el-button type="warning" @click="handleLogout" class="action-btn">
              <el-icon><SwitchButton /></el-icon>
              退出登录
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px;">
      <el-col :span="24">
        <el-card>
          <template #header>
            <span>系统信息</span>
          </template>
          <el-descriptions :column="3" border>
            <el-descriptions-item label="版本">{{ status?.version || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Go 版本">{{ status?.go_version || '-' }}</el-descriptions-item>
            <el-descriptions-item label="操作系统">{{ status?.os || '-' }}</el-descriptions-item>
            <el-descriptions-item label="内存使用">{{ status?.memory_usage || '-' }}</el-descriptions-item>
            <el-descriptions-item label="CPU 核心数">{{ status?.cpu_cores ?? '-' }}</el-descriptions-item>
            <el-descriptions-item label="启动时间">{{ status?.start_time || '-' }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.dashboard {
  max-width: 1400px;
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

.stat-cards {
  margin-bottom: var(--space-4);
}

.stat-card {
  height: 100%;
}

.stat-content {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-lg);
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
}

.stat-icon.primary { background: var(--color-primary-500); }
.stat-icon.success { background: var(--color-success); }
.stat-icon.warning { background: var(--color-warning); }
.stat-icon.danger { background: var(--color-error); }

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
}

.stat-label {
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  margin-top: var(--space-1);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.quick-actions {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.action-btn {
  justify-content: flex-start;
}
</style>
