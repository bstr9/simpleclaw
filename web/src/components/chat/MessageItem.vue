<script setup lang="ts">
import { computed } from 'vue'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import DOMPurify from 'dompurify'
import type { Message } from '@/types'

const props = defineProps<{
  message: Message
}>()

const md = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: true,
  highlight: (str, lang) => {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return `<pre class="hljs"><code>${hljs.highlight(str, { language: lang, ignoreIllegals: true }).value}</code></pre>`
      } catch {
        void 0
      }
    }
    return `<pre class="hljs"><code>${md.utils.escapeHtml(str)}</code></pre>`
  }
})

const renderedContent = computed(() => {
  if (!props.message.content) return ''
  const html = md.render(props.message.content)
  return DOMPurify.sanitize(html)
})

const isUser = computed(() => props.message.role === 'user')
const isStreaming = computed(() => props.message.status === 'streaming')
</script>

<template>
  <div :class="['message-item', message.role]">
    <div v-if="isUser" class="message-avatar user-avatar">
      <el-icon><User /></el-icon>
    </div>
    <div v-else class="message-avatar assistant-avatar">
      <el-icon><ChatDotRound /></el-icon>
    </div>

    <div class="message-body">
      <div :class="['message-content', { streaming: isStreaming }]">
        <div v-if="isUser" class="user-message">{{ message.content }}</div>
        <div v-else class="markdown-body" v-html="renderedContent"></div>
      </div>
      
      <div v-if="message.status === 'error'" class="error-badge">
        <el-icon><Warning /></el-icon>
        发生错误
      </div>
    </div>
  </div>
</template>

<style scoped>
.message-item {
  display: flex;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}

.message-item.user {
  flex-direction: row-reverse;
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

.user-avatar {
  background: var(--color-primary-500);
  color: white;
}

.assistant-avatar {
  background: var(--color-primary-100);
  color: var(--color-primary-600);
}

[data-theme='dark'] .assistant-avatar {
  background: var(--color-primary-900);
  color: var(--color-primary-300);
}

.message-body {
  flex: 1;
  min-width: 0;
  max-width: 85%;
}

.message-content {
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-xl);
  word-wrap: break-word;
}

.message-item.user .message-content {
  background: var(--color-primary-500);
  color: white;
  border-bottom-right-radius: var(--radius-sm);
}

.message-item.assistant .message-content {
  background: var(--color-bg-secondary);
  border-bottom-left-radius: var(--radius-sm);
}

.streaming {
  position: relative;
}

.streaming::after {
  content: '';
  display: inline-block;
  width: 8px;
  height: 16px;
  background: var(--color-primary-500);
  margin-left: 2px;
  animation: blink 1s infinite;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

.user-message {
  white-space: pre-wrap;
}

.error-badge {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  color: var(--color-error);
  font-size: var(--text-sm);
  margin-top: var(--space-2);
}
</style>

<style>
.markdown-body {
  line-height: 1.6;
}

.markdown-body p {
  margin: 0 0 var(--space-3);
}

.markdown-body p:last-child {
  margin-bottom: 0;
}

.markdown-body h1, .markdown-body h2, .markdown-body h3, .markdown-body h4 {
  margin: var(--space-4) 0 var(--space-2);
  font-weight: var(--font-semibold);
}

.markdown-body h1 { font-size: 1.5em; }
.markdown-body h2 { font-size: 1.3em; }
.markdown-body h3 { font-size: 1.1em; }

.markdown-body ul, .markdown-body ol {
  margin: var(--space-2) 0;
  padding-left: var(--space-6);
}

.markdown-body li {
  margin: var(--space-1) 0;
}

.markdown-body code {
  font-family: var(--font-mono);
  font-size: 0.9em;
  padding: 0.125rem 0.375rem;
  background: var(--color-code-bg);
  border-radius: var(--radius-sm);
}

.markdown-body pre {
  margin: var(--space-3) 0;
  padding: var(--space-4);
  background: var(--color-code-bg);
  border-radius: var(--radius-lg);
  overflow-x: auto;
}

.markdown-body pre code {
  padding: 0;
  background: transparent;
}

.markdown-body blockquote {
  margin: var(--space-3) 0;
  padding: var(--space-2) var(--space-4);
  border-left: 4px solid var(--color-primary-400);
  background: var(--color-bg-tertiary);
  border-radius: 0 var(--radius-md) var(--radius-md) 0;
}

.markdown-body a {
  color: var(--color-primary-500);
  text-decoration: none;
}

.markdown-body a:hover {
  text-decoration: underline;
}

.markdown-body table {
  width: 100%;
  margin: var(--space-3) 0;
  border-collapse: collapse;
}

.markdown-body th, .markdown-body td {
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  text-align: left;
}

.markdown-body th {
  background: var(--color-bg-secondary);
  font-weight: var(--font-semibold);
}

.hljs {
  background: var(--color-code-bg) !important;
}
</style>
