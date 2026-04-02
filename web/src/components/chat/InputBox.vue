<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { chatApi } from '@/api'

const props = defineProps<{
  loading: boolean
}>()

const emit = defineEmits<{
  send: [content: string, attachments?: Array<{ file_path: string; file_name: string; file_type: string }>]
}>()

const inputText = ref('')
const attachments = ref<Array<{ file_path: string; file_name: string; file_type: string }>>([])
const uploading = ref(false)
const textareaRef = ref<HTMLTextAreaElement | null>(null)

function handleInput(event: Event) {
  const target = event.target as HTMLTextAreaElement
  inputText.value = target.value
  autoResize()
}

function autoResize() {
  if (textareaRef.value) {
    textareaRef.value.style.height = 'auto'
    textareaRef.value.style.height = Math.min(textareaRef.value.scrollHeight, 200) + 'px'
  }
}

async function handleSend() {
  const content = inputText.value.trim()
  if (!content && attachments.value.length === 0) return
  if (props.loading) return

  emit('send', content, attachments.value.length > 0 ? attachments.value : undefined)
  inputText.value = ''
  attachments.value = []
  
  if (textareaRef.value) {
    textareaRef.value.style.height = 'auto'
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    handleSend()
  }
}

async function handleFileUpload(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  uploading.value = true
  try {
    const result = await chatApi.upload(file)
    attachments.value.push({
      file_path: result.file_path,
      file_name: result.file_name,
      file_type: result.file_type as 'image' | 'video' | 'file'
    })
    ElMessage.success(`已添加: ${result.file_name}`)
  } catch {
    ElMessage.error('文件上传失败')
  } finally {
    uploading.value = false
    target.value = ''
  }
}

function removeAttachment(index: number) {
  attachments.value.splice(index, 1)
}

function getFileIcon(type: string): string {
  switch (type) {
    case 'image': return 'Picture'
    case 'video': return 'VideoCamera'
    default: return 'Document'
  }
}
</script>

<template>
  <div class="input-box">
    <div v-if="attachments.length > 0" class="attachments">
      <div 
        v-for="(att, index) in attachments" 
        :key="att.file_path" 
        class="attachment-item"
      >
        <el-icon><component :is="getFileIcon(att.file_type)" /></el-icon>
        <span class="attachment-name">{{ att.file_name }}</span>
        <el-button 
          :icon="Close" 
          circle 
          size="small" 
          text 
          @click="removeAttachment(index)"
        />
      </div>
    </div>

    <div class="input-wrapper">
      <el-button 
        class="upload-btn"
        :icon="Paperclip" 
        circle 
        text
        :loading="uploading"
        @click="$refs.fileInput?.click()"
      />
      <input
        ref="fileInput"
        type="file"
        accept="image/*,video/*,.pdf,.doc,.docx,.txt,.md"
        style="display: none"
        @change="handleFileUpload"
      />

      <textarea
        ref="textareaRef"
        :value="inputText"
        placeholder="输入消息... (Shift+Enter 换行)"
        class="input-textarea"
        rows="1"
        @input="handleInput"
        @keydown="handleKeydown"
      />

      <el-button
        type="primary"
        class="send-btn"
        :icon="loading ? Loading : Position"
        circle
        :disabled="(!inputText.trim() && attachments.length === 0) || loading"
        @click="handleSend"
      />
    </div>

    <div class="input-hint">
      <span>按 Enter 发送，Shift+Enter 换行</span>
    </div>
  </div>
</template>

<style scoped>
.input-box {
  background: var(--color-bg-primary);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-sm);
  padding: var(--space-3);
}

.attachments {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin-bottom: var(--space-2);
}

.attachment-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-1) var(--space-2);
  background: var(--color-bg-secondary);
  border-radius: var(--radius-md);
  font-size: var(--text-sm);
}

.attachment-name {
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.input-wrapper {
  display: flex;
  align-items: flex-end;
  gap: var(--space-2);
}

.upload-btn {
  flex-shrink: 0;
  color: var(--color-text-tertiary);
}

.input-textarea {
  flex: 1;
  border: none;
  outline: none;
  resize: none;
  font-family: inherit;
  font-size: var(--text-base);
  line-height: 1.5;
  background: transparent;
  color: var(--color-text-primary);
  max-height: 200px;
  padding: var(--space-2) 0;
}

.input-textarea::placeholder {
  color: var(--color-text-tertiary);
}

.send-btn {
  flex-shrink: 0;
  width: 40px;
  height: 40px;
}

.input-hint {
  text-align: center;
  margin-top: var(--space-2);
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

[data-theme='dark'] .input-box {
  background: var(--color-bg-secondary);
}
</style>
