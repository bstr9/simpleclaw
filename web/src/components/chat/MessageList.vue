<script setup lang="ts">
import { nextTick, watch, ref } from 'vue'
import MessageItem from './MessageItem.vue'
import type { Message } from '@/types'

const props = defineProps<{
  messages: Message[]
  loading: boolean
}>()

const containerRef = ref<HTMLElement | null>(null)

const scrollToBottom = async () => {
  await nextTick()
  if (containerRef.value) {
    containerRef.value.scrollTop = containerRef.value.scrollHeight
  }
}

watch(
  () => props.messages.length,
  () => scrollToBottom()
)

watch(
  () => props.messages[props.messages.length - 1]?.content,
  () => scrollToBottom()
)
</script>

<template>
  <div ref="containerRef" class="message-list">
    <div v-if="messages.length === 0" class="empty-state">
      <div class="welcome">
        <el-icon :size="64" color="var(--color-primary-400)"><ChatDotRound /></el-icon>
        <h2>开始对话</h2>
        <p>输入消息开始与 AI 助手对话</p>
      </div>
    </div>

    <MessageItem
      v-for="message in messages"
      :key="message.id"
      :message="message"
    />

    <div v-if="loading" class="message-item assistant">
      <div class="message-avatar assistant-avatar">
        <el-icon><ChatDotRound /></el-icon>
      </div>
      <div class="message-content">
        <div class="typing-indicator">
          <span></span>
          <span></span>
          <span></span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.message-list {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-4);
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}

.welcome {
  text-align: center;
  color: var(--color-text-secondary);
}

.welcome h2 {
  font-size: var(--text-xl);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  margin: var(--space-4) 0 var(--space-2);
}

.welcome p {
  font-size: var(--text-base);
}

.message-item {
  display: flex;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}

.message-avatar {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-lg);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.assistant-avatar {
  background: var(--color-primary-100);
  color: var(--color-primary-600);
}

.message-content {
  flex: 1;
  min-width: 0;
}

.typing-indicator {
  display: flex;
  gap: 4px;
  padding: var(--space-3);
  background: var(--color-bg-secondary);
  border-radius: var(--radius-lg);
  width: fit-content;
}

.typing-indicator span {
  width: 8px;
  height: 8px;
  background: var(--color-text-tertiary);
  border-radius: 50%;
  animation: bounce 1.4s infinite ease-in-out both;
}

.typing-indicator span:nth-child(1) { animation-delay: -0.32s; }
.typing-indicator span:nth-child(2) { animation-delay: -0.16s; }

@keyframes bounce {
  0%, 80%, 100% { transform: scale(0); }
  40% { transform: scale(1); }
}
</style>
