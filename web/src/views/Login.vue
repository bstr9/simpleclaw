<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const form = ref({
  username: '',
  password: ''
})
const loading = ref(false)
const showPassword = ref(false)

async function handleLogin() {
  if (!form.value.username || !form.value.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }

  loading.value = true
  try {
    await authStore.login({
      username: form.value.username,
      password: form.value.password
    })
    ElMessage.success('登录成功')
    router.push('/admin')
  } catch (error) {
    ElMessage.error('登录失败，请检查用户名和密码')
  } finally {
    loading.value = false
  }
}

function goToChat() {
  router.push('/')
}
</script>

<template>
  <div class="login-container">
    <div class="login-background">
      <div class="gradient-orb orb-1"></div>
      <div class="gradient-orb orb-2"></div>
      <div class="gradient-orb orb-3"></div>
    </div>

    <div class="login-card">
      <div class="login-header">
        <div class="logo">
          <el-icon :size="40" color="var(--color-primary-500)"><ChatDotRound /></el-icon>
        </div>
        <h1>SimpleClaw</h1>
        <p>AI Agent 平台管理后台</p>
      </div>

      <el-form @submit.prevent="handleLogin" class="login-form">
        <el-form-item>
          <el-input
            v-model="form.username"
            placeholder="用户名"
            size="large"
            :prefix-icon="User"
          />
        </el-form-item>

        <el-form-item>
          <el-input
            v-model="form.password"
            :type="showPassword ? 'text' : 'password'"
            placeholder="密码"
            size="large"
            :prefix-icon="Lock"
            show-password
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            class="login-button"
            @click="handleLogin"
          >
            登录
          </el-button>
        </el-form-item>
      </el-form>

      <div class="login-footer">
        <el-button link type="primary" @click="goToChat">
          直接进入聊天
        </el-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
  background: var(--color-bg-primary);
}

.login-background {
  position: absolute;
  inset: 0;
  overflow: hidden;
}

.gradient-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.4;
}

.orb-1 {
  width: 400px;
  height: 400px;
  background: var(--color-primary-300);
  top: -100px;
  right: -100px;
}

.orb-2 {
  width: 300px;
  height: 300px;
  background: var(--color-primary-200);
  bottom: -50px;
  left: -50px;
}

.orb-3 {
  width: 200px;
  height: 200px;
  background: var(--color-primary-100);
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
}

.login-card {
  position: relative;
  width: 100%;
  max-width: 400px;
  background: var(--color-bg-elevated);
  border-radius: var(--radius-2xl);
  box-shadow: var(--shadow-xl);
  padding: var(--space-10);
  margin: var(--space-4);
}

.login-header {
  text-align: center;
  margin-bottom: var(--space-8);
}

.logo {
  width: 72px;
  height: 72px;
  margin: 0 auto var(--space-4);
  background: var(--color-primary-50);
  border-radius: var(--radius-xl);
  display: flex;
  align-items: center;
  justify-content: center;
}

.login-header h1 {
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  margin: 0 0 var(--space-2);
}

.login-header p {
  color: var(--color-text-secondary);
  margin: 0;
}

.login-form {
  margin-top: var(--space-6);
}

.login-form :deep(.el-input__wrapper) {
  padding: 0 var(--space-4);
}

.login-button {
  width: 100%;
  height: 48px;
  font-size: var(--text-base);
  font-weight: var(--font-medium);
}

.login-footer {
  text-align: center;
  margin-top: var(--space-6);
  padding-top: var(--space-6);
  border-top: 1px solid var(--color-border);
}

[data-theme='dark'] .gradient-orb {
  opacity: 0.2;
}

[data-theme='dark'] .logo {
  background: var(--color-primary-900);
}
</style>
