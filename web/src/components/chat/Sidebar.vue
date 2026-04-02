<script setup lang="ts">
import { computed, inject, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { useChatStore } from '@/stores/chat'
import { useAuthStore } from '@/stores/auth'
import type { Session } from '@/types'

const router = useRouter()
const chatStore = useChatStore()
const authStore = useAuthStore()
const theme = inject<{ current: Ref<'light' | 'dark'>; toggle: () => void }>('theme')

defineProps<{
  collapsed: boolean
}>()

const emit = defineEmits<{
  toggleTheme: []
}>()

const sessions = computed(() => chatStore.sessions)
const currentSessionId = computed(() => chatStore.currentSessionId)

function createNewChat() {
  chatStore.createSession()
}

function selectSession(id: string) {
  chatStore.selectSession(id)
}

function deleteSession(id: string) {
  chatStore.deleteSession(id)
}

function goToAdmin() {
  router.push('/admin')
}

function goToLogin() {
  router.push('/login')
}

function formatTime(timestamp: number): string {
  const date = new Date(timestamp)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`
  if (diff < 604800000) return `${Math.floor(diff / 86400000)} 天前`
  
  return date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-header">
      <div class="brand">
        <el-icon :size="24" color="var(--color-primary-500)"><ChatDotRound /></el-icon>
        <span class="brand-text">SimpleClaw</span>
      </div>
      <el-button 
        :icon="theme?.current === 'dark' ? Sunny : Moon" 
        circle 
        size="small"
        @click="theme?.toggle"
      />
    </div>

    <div class="sidebar-actions">
      <el-button type="primary" class="new-chat-btn" @click="createNewChat">
        <el-icon><Plus /></el-icon>
        新建对话
      </el-button>
    </div>

    <div class="sessions-list">
      <div class="sessions-header">
        <span>历史对话</span>
        <span class="sessions-count">{{ sessions.length }}</span>
      </div>

      <div class="sessions-scroll">
        <div
          v-for="session in sessions"
          :key="session.id"
          :class="['session-item', { active: session.id === currentSessionId }]"
          @click="selectSession(session.id)"
        >
          <el-icon class="session-icon"><ChatLineRound /></el-icon>
          <div class="session-info">
            <div class="session-title">{{ session.title }}</div>
            <div class="session-meta">{{ formatTime(session.updatedAt) }}</div>
          </div>
          <el-button
            class="delete-btn"
            :icon="Delete"
            circle
            size="small"
            text
            @click.stop="deleteSession(session.id)"
          />
        </div>

        <div v-if="sessions.length === 0" class="empty-state">
          <el-icon :size="32" color="var(--color-text-tertiary)"><ChatLineRound /></el-icon>
          <p>暂无对话记录</p>
        </div>
      </div>
    </div>

    <div class="sidebar-footer">
      <template v-if="authStore.isAuthenticated">
        <el-button class="footer-btn" @click="goToAdmin">
          <el-icon><Setting /></el-icon>
          管理后台
        </el-button>
      </template>
      <template v-else>
        <el-button class="footer-btn" @click="goToLogin">
          <el-icon><User /></el-icon>
          登录
        </el-button>
      </template>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  position: fixed;
  left: 0;
  top: 0;
  bottom: 0;
  width: var(--sidebar-width);
  background: var(--color-bg-secondary);
  border-right: 1px solid var(--color-border);
  display: flex;
  flex-direction: column;
  z-index: var(--z-sticky);
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4);
  border-bottom: 1px solid var(--color-border);
}

.brand {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.brand-text {
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
}

.sidebar-actions {
  padding: var(--space-3) var(--space-4);
}

.new-chat-btn {
  width: 100%;
  justify-content: flex-start;
  height: 44px;
  font-weight: var(--font-medium);
}

.sessions-list {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.sessions-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-2) var(--space-4);
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
}

.sessions-count {
  background: var(--color-bg-tertiary);
  padding: 0 var(--space-2);
  border-radius: var(--radius-full);
  font-size: var(--text-xs);
}

.sessions-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 0 var(--space-2);
}

.session-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3);
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: background-color var(--transition-fast);
  position: relative;
}

.session-item:hover {
  background: var(--color-surface-hover);
}

.session-item.active {
  background: var(--color-primary-50);
}

[data-theme='dark'] .session-item.active {
  background: var(--color-primary-900);
}

.session-icon {
  color: var(--color-text-tertiary);
  flex-shrink: 0;
}

.session-info {
  flex: 1;
  min-width: 0;
}

.session-title {
  font-size: var(--text-sm);
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-meta {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
  margin-top: 2px;
}

.delete-btn {
  opacity: 0;
  transition: opacity var(--transition-fast);
}

.session-item:hover .delete-btn {
  opacity: 1;
}

.empty-state {
  text-align: center;
  padding: var(--space-8);
  color: var(--color-text-tertiary);
}

.empty-state p {
  margin: var(--space-2) 0 0;
  font-size: var(--text-sm);
}

.sidebar-footer {
  padding: var(--space-3) var(--space-4);
  border-top: 1px solid var(--color-border);
}

.footer-btn {
  width: 100%;
  justify-content: flex-start;
}

@media (max-width: 768px) {
  .sidebar {
    transform: translateX(-100%);
  }
}
</style>
