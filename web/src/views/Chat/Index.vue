<script setup lang="ts">
import { onMounted, inject, type Ref } from 'vue'
import { useChatStore } from '@/stores/chat'
import Sidebar from '@/components/chat/Sidebar.vue'
import MessageList from '@/components/chat/MessageList.vue'
import InputBox from '@/components/chat/InputBox.vue'

const chatStore = useChatStore()
const theme = inject<{ current: Ref<'light' | 'dark'>; toggle: () => void }>('theme')

onMounted(() => {
  chatStore.init()
})
</script>

<template>
  <div class="chat-layout">
    <Sidebar 
      :collapsed="false"
      @toggle-theme="theme?.toggle"
    />
    
    <main class="chat-main">
      <div class="chat-container">
        <MessageList :messages="chatStore.currentMessages" :loading="chatStore.isLoading" />
        <InputBox 
          :loading="chatStore.isLoading" 
          @send="chatStore.sendMessage"
        />
      </div>
    </main>
  </div>
</template>

<style scoped>
.chat-layout {
  display: flex;
  height: 100vh;
  background: var(--color-bg-primary);
}

.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  margin-left: var(--sidebar-width);
}

@media (max-width: 768px) {
  .chat-main {
    margin-left: 0;
  }
}

.chat-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  max-width: var(--max-content-width);
  width: 100%;
  margin: 0 auto;
  padding: var(--space-4);
}

[data-theme='dark'] .chat-layout {
  background: var(--color-bg-primary);
}
</style>
