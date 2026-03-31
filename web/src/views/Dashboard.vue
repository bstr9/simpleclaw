<template>
  <div class="dashboard-container">
    <el-container>
      <el-header class="app-header">
        <div class="header-left">
          <h1>SimpleClaw 管理后台</h1>
        </div>
        <div class="header-right">
          <span class="username">{{ authStore.user?.username }}</span>
          <el-dropdown @command="handleCommand">
            <el-button type="primary" link>
              <el-icon><Setting /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="config">配置管理</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main>
        <el-row :gutter="20">
          <el-col :span="6">
            <el-card class="stat-card" shadow="hover">
              <div class="stat-content">
                <div class="stat-icon" style="background: #409eff;">
                  <el-icon size="24"><Connection /></el-icon>
                </div>
                <div class="stat-info">
                  <div class="stat-value">{{ channels.length }}</div>
                  <div class="stat-label">活跃渠道</div>
                </div>
              </div>
            </el-card>
          </el-col>

          <el-col :span="6">
            <el-card class="stat-card" shadow="hover">
              <div class="stat-content">
                <div class="stat-icon" style="background: #67c23a;">
                  <el-icon size="24"><ChatDotRound /></el-icon>
                </div>
                <div class="stat-info">
                  <div class="stat-value">{{ status?.total_sessions || 0 }}</div>
                  <div class="stat-label">总会话数</div>
                </div>
              </div>
            </el-card>
          </el-col>

          <el-col :span="6">
            <el-card class="stat-card" shadow="hover">
              <div class="stat-content">
                <div class="stat-icon" style="background: #e6a23c;">
                  <el-icon size="24"><Timer /></el-icon>
                </div>
                <div class="stat-info">
                  <div class="stat-value">{{ status?.uptime || '0s' }}</div>
                  <div class="stat-label">运行时间</div>
                </div>
              </div>
            </el-card>
          </el-col>

          <el-col :span="6">
            <el-card class="stat-card" shadow="hover">
              <div class="stat-content">
                <div class="stat-icon" :style="{ background: status?.llm_connected ? '#67c23a' : '#f56c6c' }">
                  <el-icon size="24"><Cpu /></el-icon>
                </div>
                <div class="stat-info">
                  <div class="stat-value">{{ status?.llm_connected ? '正常' : '异常' }}</div>
                  <div class="stat-label">LLM 状态</div>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <el-row :gutter="20" style="margin-top: 20px;">
          <el-col :span="16">
            <el-card>
              <template #header>
                <span>渠道状态</span>
              </template>
              <el-table :data="channels" stripe>
                <el-table-column prop="name" label="渠道名称" />
                <el-table-column prop="type" label="类型" />
                <el-table-column label="状态">
                  <template #default="{ row }">
                    <el-tag :type="row.active ? 'success' : 'info'">
                      {{ row.active ? '运行中' : '已停止' }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="connections" label="连接数" />
              </el-table>
            </el-card>
          </el-col>

          <el-col :span="8">
            <el-card>
              <template #header>
                <span>快捷操作</span>
              </template>
              <div class="quick-actions">
                <el-button type="primary" @click="goToConfig" class="action-btn">
                  <el-icon><Setting /></el-icon>
                  配置管理
                </el-button>
                <el-button type="success" @click="testLlm" class="action-btn" :loading="testingLlm">
                  <el-icon><Connection /></el-icon>
                  测试 LLM
                </el-button>
                <el-button type="warning" @click="refreshStatus" class="action-btn" :loading="refreshing">
                  <el-icon><Refresh /></el-icon>
                  刷新状态
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
                <el-descriptions-item label="CPU 核心数">{{ status?.cpu_cores || '-' }}</el-descriptions-item>
                <el-descriptions-item label="启动时间">{{ status?.start_time || '-' }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
        </el-row>
      </el-main>
    </el-container>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Setting, Connection, ChatDotRound, Timer, Cpu, Refresh } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useConfigStore } from '@/stores/config'

const router = useRouter()
const authStore = useAuthStore()
const configStore = useConfigStore()

const status = ref(null)
const channels = ref([])
const testingLlm = ref(false)
const refreshing = ref(false)

function handleCommand(command) {
  if (command === 'config') {
    router.push('/config')
  } else if (command === 'logout') {
    authStore.logout()
    router.push('/login')
  }
}

function goToConfig() {
  router.push('/config')
}

async function testLlm() {
  testingLlm.value = true
  try {
    await configStore.testLlm({})
    ElMessage.success('LLM 连接正常')
  } catch (error) {
    ElMessage.error('LLM 连接失败')
  } finally {
    testingLlm.value = false
  }
}

async function refreshStatus() {
  refreshing.value = true
  try {
    status.value = await configStore.fetchStatus()
    channels.value = await configStore.fetchChannels()
    ElMessage.success('状态已刷新')
  } catch (error) {
    void error
  } finally {
    refreshing.value = false
  }
}

onMounted(async () => {
  try {
    status.value = await configStore.fetchStatus()
    channels.value = await configStore.fetchChannels()
  } catch (error) {
    void error
  }
})
</script>

<style scoped>
.dashboard-container {
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

.header-left h1 {
  margin: 0;
  font-size: 20px;
  color: #303133;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 15px;
}

.username {
  color: #606266;
  font-size: 14px;
}

.stat-card {
  height: 100px;
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 15px;
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: 600;
  color: #303133;
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 5px;
}

.quick-actions {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.action-btn {
  width: 100%;
  justify-content: flex-start;
}
</style>
