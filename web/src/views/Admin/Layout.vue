<script setup lang="ts">
import { inject, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()
const theme = inject<{ current: Ref<'light' | 'dark'>; toggle: () => void }>('theme')

function goToChat() {
  router.push('/')
}

async function handleLogout() {
  await authStore.logout()
  router.push('/login')
}
</script>

<template>
  <div class="admin-layout">
    <aside class="admin-sidebar">
      <div class="sidebar-header">
        <el-icon :size="24" color="var(--color-primary-500)"><Setting /></el-icon>
        <span class="sidebar-title">SimpleClaw</span>
      </div>

      <el-menu
        :default-active="$route.path"
        router
        class="sidebar-menu"
      >
        <el-menu-item index="/admin">
          <el-icon><DataBoard /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>
        <el-menu-item index="/admin/config">
          <el-icon><Tools /></el-icon>
          <span>配置管理</span>
        </el-menu-item>
      </el-menu>

      <div class="sidebar-footer">
        <el-button 
          :icon="theme?.current === 'dark' ? Sunny : Moon" 
          circle 
          size="small"
          @click="theme?.toggle"
        />
        <el-button :icon="ChatDotRound" circle size="small" @click="goToChat" />
        <el-button :icon="SwitchButton" circle size="small" @click="handleLogout" />
      </div>
    </aside>

    <main class="admin-main">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.admin-layout {
  display: flex;
  min-height: 100vh;
  background: var(--color-bg-secondary);
}

.admin-sidebar {
  width: 240px;
  background: var(--color-bg-primary);
  border-right: 1px solid var(--color-border);
  display: flex;
  flex-direction: column;
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
}

.sidebar-header {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-4);
  border-bottom: 1px solid var(--color-border);
}

.sidebar-title {
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
}

.sidebar-menu {
  flex: 1;
  border-right: none;
  padding: var(--space-2);
}

.sidebar-menu .el-menu-item {
  border-radius: var(--radius-lg);
  margin-bottom: var(--space-1);
}

.sidebar-menu .el-menu-item.is-active {
  background: var(--color-primary-50);
}

[data-theme='dark'] .sidebar-menu .el-menu-item.is-active {
  background: var(--color-primary-900);
}

.sidebar-footer {
  padding: var(--space-4);
  border-top: 1px solid var(--color-border);
  display: flex;
  justify-content: center;
  gap: var(--space-2);
}

.admin-main {
  flex: 1;
  margin-left: 240px;
  padding: var(--space-6);
  min-height: 100vh;
}
</style>
