<template>
  <div
    class="troubleshooting-assistant"
    :style="containerStyle"
    data-testid="troubleshooting-assistant"
  >
    <button
      v-if="collapsed"
      type="button"
      class="assistant-restore-handle"
      aria-label="恢复故障排查助手"
      @click="restoreFromEdge"
    >
      <Icon name="chevronLeft" size="xs" />
    </button>

    <div
      v-else-if="!open"
      class="assistant-fab-shell"
    >
      <button
        type="button"
        class="assistant-fab"
        aria-label="打开故障排查助手"
        @mousedown="event => startDrag(event, { allowButton: true })"
        @click="handleFabClick"
      >
        <Icon name="chatBubble" size="sm" />
        <span>故障排查</span>
      </button>
      <button
        type="button"
        class="assistant-fab-close"
        aria-label="收起故障排查入口"
        @mousedown.stop
        @click.stop="collapseToEdge"
      >
        <Icon name="x" size="xs" />
      </button>
    </div>

    <section
      v-else
      class="assistant-panel"
      aria-label="故障排查助手"
    >
      <header class="assistant-header" @mousedown="startDrag">
        <div class="assistant-title">
          <Icon name="chatBubble" size="sm" />
          <span>故障排查助手</span>
        </div>
        <button type="button" class="icon-button" aria-label="关闭故障排查助手" @click="closePanel">
          <Icon name="x" size="sm" />
        </button>
      </header>

      <div ref="messagesRef" class="assistant-messages">
        <div
          v-for="message in messages"
          :key="message.id"
          class="assistant-message"
          :class="[
            `assistant-message-${message.role}`,
            { 'assistant-message-loading': message.loading },
          ]"
        >
          <div class="assistant-message-body">
            <div v-if="message.loading" class="assistant-waiting">
              <span>{{ message.content }}</span>
              <span class="assistant-typing-dots" aria-hidden="true">
                <span></span>
                <span></span>
                <span></span>
              </span>
            </div>
            <pre v-else>{{ message.content }}</pre>
            <div
              v-if="message.role === 'assistant' && message.needsAdmin && !message.loading"
              class="assistant-message-actions"
            >
              <button
                type="button"
                class="notify-admin-button"
                data-testid="troubleshooting-notify-admin"
                :disabled="message.notifying || message.notified"
                @click="notifyAdmin(message)"
              >
                {{ message.notifying ? '通知中' : '通知管理员' }}
              </button>
              <span v-if="message.notifyStatus" class="notify-admin-status">{{ message.notifyStatus }}</span>
            </div>
          </div>
        </div>
      </div>

      <form class="assistant-form" @submit.prevent="submit">
        <textarea
          v-model="draft"
          class="assistant-input"
          rows="4"
          maxlength="4000"
          placeholder="粘贴报错信息、状态码、接口 URL 或 request id"
          :disabled="loading"
        />
        <div class="assistant-actions">
          <span class="assistant-limit">{{ limitText }}</span>
          <button type="submit" class="send-button" :disabled="loading || !draft.trim()">
            {{ loading ? '排查中' : '发送' }}
          </button>
        </div>
      </form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { troubleshootingAPI, type TroubleshootingAnalysis } from '@/api/troubleshooting'

const STORAGE_KEY = 'sub2api.troubleshootingAssistant.position'
const DEFAULT_POSITION = { x: 24, y: 24 }

interface AssistantMessage {
  id: number
  role: 'user' | 'assistant'
  content: string
  loading?: boolean
  report?: string
  needsAdmin?: boolean
  notifying?: boolean
  notified?: boolean
  notifyStatus?: string
}

const open = ref(false)
const collapsed = ref(false)
const loading = ref(false)
const draft = ref('')
const messages = ref<AssistantMessage[]>([])
const messagesRef = ref<HTMLElement | null>(null)
const position = ref(loadPosition())
const latestLimit = ref<TroubleshootingAnalysis['limit'] | null>(null)
let messageID = 0
let dragStart: { mouseX: number; mouseY: number; x: number; y: number } | null = null
let draggedDuringGesture = false

const containerStyle = computed(() => (
  collapsed.value
    ? { right: '0px', bottom: `${position.value.y}px` }
    : { right: `${position.value.x}px`, bottom: `${position.value.y}px` }
))

const limitText = computed(() => {
  const limit = latestLimit.value
  if (!limit) return ''
  return `剩余 ${limit.short_window_remaining}/5分钟 · ${limit.daily_remaining}/今日`
})

function loadPosition() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return { ...DEFAULT_POSITION }
    const parsed = JSON.parse(raw) as Partial<typeof DEFAULT_POSITION>
    return clampPosition({
      x: Number.isFinite(parsed.x) ? Number(parsed.x) : DEFAULT_POSITION.x,
      y: Number.isFinite(parsed.y) ? Number(parsed.y) : DEFAULT_POSITION.y,
    })
  } catch {
    return { ...DEFAULT_POSITION }
  }
}

function savePosition() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(position.value))
  } catch {
    // ignore storage failures
  }
}

function clampPosition(next: { x: number; y: number }) {
  const maxX = Math.max(12, window.innerWidth - 360)
  const maxY = Math.max(12, window.innerHeight - 220)
  return {
    x: Math.min(Math.max(12, next.x), maxX),
    y: Math.min(Math.max(12, next.y), maxY),
  }
}

function handleFabClick() {
  if (draggedDuringGesture) {
    draggedDuringGesture = false
    return
  }
  collapsed.value = false
  open.value = true
}

function collapseToEdge() {
  open.value = false
  collapsed.value = true
}

function closePanel() {
  open.value = false
  collapsed.value = false
}

function restoreFromEdge() {
  collapsed.value = false
  open.value = true
}

function startDrag(event: MouseEvent, options: { allowButton?: boolean } = {}) {
  const target = event.target as HTMLElement | null
  if (!options.allowButton && target?.closest('button')) return
  dragStart = {
    mouseX: event.clientX,
    mouseY: event.clientY,
    x: position.value.x,
    y: position.value.y,
  }
  draggedDuringGesture = false
  window.addEventListener('mousemove', onDrag)
  window.addEventListener('mouseup', stopDrag)
}

function onDrag(event: MouseEvent) {
  if (!dragStart) return
  const deltaX = event.clientX - dragStart.mouseX
  const deltaY = event.clientY - dragStart.mouseY
  if (Math.abs(deltaX) > 3 || Math.abs(deltaY) > 3) {
    draggedDuringGesture = true
  }
  position.value = clampPosition({
    x: dragStart.x - deltaX,
    y: dragStart.y - deltaY,
  })
}

function stopDrag() {
  if (!dragStart) return
  dragStart = null
  window.removeEventListener('mousemove', onDrag)
  window.removeEventListener('mouseup', stopDrag)
  savePosition()
}

async function submit() {
  const text = draft.value.trim()
  if (!text || loading.value) return

  messages.value.push({ id: ++messageID, role: 'user', content: text })
  const loadingMessageID = ++messageID
  messages.value.push({
    id: loadingMessageID,
    role: 'assistant',
    content: '正在分析错误原因',
    loading: true,
  })
  draft.value = ''
  loading.value = true
  await scrollToBottom()

  try {
    const result = await troubleshootingAPI.analyze(text)
    latestLimit.value = result.limit ?? null
    replaceMessage(loadingMessageID, {
      role: 'assistant',
      content: result.answer,
      report: text,
      needsAdmin: result.needs_admin,
    })
  } catch (error) {
    replaceMessage(loadingMessageID, { role: 'assistant', content: extractErrorMessage(error) })
  } finally {
    loading.value = false
    await scrollToBottom()
  }
}

function replaceMessage(id: number, patch: Omit<AssistantMessage, 'id'>) {
  const index = messages.value.findIndex(message => message.id === id)
  const next = { id, ...patch }
  if (index >= 0) {
    messages.value.splice(index, 1, next)
    return
  }
  messages.value.push(next)
}

function extractErrorMessage(error: unknown): string {
  if (error && typeof error === 'object' && 'message' in error) {
    const message = String((error as { message?: unknown }).message || '').trim()
    if (message) return message
  }
  return '排查失败，请稍后重试或联系管理员。'
}

async function notifyAdmin(message: AssistantMessage) {
  if (message.notifying || message.notified || !message.report) return
  patchMessage(message.id, { notifying: true, notifyStatus: '' })
  try {
    const result = await troubleshootingAPI.notifyAdmin({
      message: message.report,
      diagnosis: message.content,
    })
    patchMessage(message.id, {
      notifying: false,
      notified: true,
      notifyStatus: result.message || '已通知管理员，请等待 5 分钟后重试。',
    })
  } catch (error) {
    patchMessage(message.id, {
      notifying: false,
      notifyStatus: extractErrorMessage(error),
    })
  } finally {
    await scrollToBottom()
  }
}

function patchMessage(id: number, patch: Partial<AssistantMessage>) {
  const index = messages.value.findIndex(message => message.id === id)
  if (index < 0) return
  messages.value.splice(index, 1, { ...messages.value[index], ...patch })
}

async function scrollToBottom() {
  await nextTick()
  if (messagesRef.value) {
    messagesRef.value.scrollTop = messagesRef.value.scrollHeight
  }
}

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', onDrag)
  window.removeEventListener('mouseup', stopDrag)
})
</script>

<style scoped>
.troubleshooting-assistant {
  position: fixed;
  z-index: 60;
}

.assistant-fab-shell {
  position: relative;
  display: inline-flex;
}

.assistant-fab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 38px;
  padding: 0 12px;
  border: 1px solid rgba(17, 24, 39, 0.12);
  border-radius: 8px;
  background: #111827;
  color: #ffffff;
  font-size: 13px;
  font-weight: 600;
  box-shadow: 0 10px 24px rgba(17, 24, 39, 0.2);
}

.assistant-fab-close {
  position: absolute;
  top: -6px;
  right: -6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border: 1px solid rgba(17, 24, 39, 0.12);
  border-radius: 999px;
  background: #ffffff;
  color: #4b5563;
  box-shadow: 0 5px 14px rgba(17, 24, 39, 0.16);
}

.assistant-fab-close:hover {
  background: #f3f4f6;
  color: #111827;
}

.assistant-restore-handle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 34px;
  border: 1px solid rgba(17, 24, 39, 0.14);
  border-right: 0;
  border-radius: 6px 0 0 6px;
  background: #ffffff;
  color: #111827;
  box-shadow: 0 8px 18px rgba(17, 24, 39, 0.16);
}

.assistant-restore-handle:hover {
  background: #eff6ff;
  color: #1d4ed8;
}

.assistant-panel {
  width: min(360px, calc(100vw - 24px));
  height: min(520px, calc(100vh - 24px));
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgba(17, 24, 39, 0.12);
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 16px 40px rgba(17, 24, 39, 0.18);
}

.assistant-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 44px;
  padding: 0 10px 0 14px;
  border-bottom: 1px solid #e5e7eb;
  cursor: move;
  user-select: none;
}

.assistant-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 700;
  color: #111827;
}

.icon-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 6px;
  color: #4b5563;
}

.icon-button:hover {
  background: #f3f4f6;
  color: #111827;
}

.assistant-messages {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 14px;
  background: #f9fafb;
}

.assistant-message {
  margin-bottom: 10px;
  display: flex;
}

.assistant-message-body {
  display: flex;
  max-width: 100%;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
}

.assistant-message-user .assistant-message-body {
  align-items: flex-end;
}

.assistant-message pre,
.assistant-waiting {
  max-width: 100%;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  border-radius: 8px;
  padding: 10px 12px;
  font-family: inherit;
  font-size: 13px;
  line-height: 1.55;
}

.assistant-message-user {
  justify-content: flex-end;
}

.assistant-message-user pre {
  background: #2563eb;
  color: #ffffff;
}

.assistant-message-assistant pre {
  background: #ffffff;
  color: #111827;
  border: 1px solid #e5e7eb;
}

.assistant-message-loading .assistant-waiting {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: #ffffff;
  color: #111827;
  border: 1px solid #e5e7eb;
}

.assistant-message-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.notify-admin-button {
  height: 30px;
  padding: 0 10px;
  border-radius: 6px;
  background: #2563eb;
  color: #ffffff;
  font-size: 12px;
  font-weight: 600;
}

.notify-admin-button:disabled {
  cursor: not-allowed;
  background: #93c5fd;
}

.notify-admin-status {
  color: #2563eb;
  font-size: 12px;
}

.assistant-typing-dots {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

.assistant-typing-dots span {
  width: 5px;
  height: 5px;
  border-radius: 999px;
  background: #6b7280;
  animation: assistantTypingPulse 1.1s infinite ease-in-out;
}

.assistant-typing-dots span:nth-child(2) {
  animation-delay: 0.15s;
}

.assistant-typing-dots span:nth-child(3) {
  animation-delay: 0.3s;
}

@keyframes assistantTypingPulse {
  0%,
  80%,
  100% {
    opacity: 0.35;
    transform: translateY(0);
  }

  40% {
    opacity: 1;
    transform: translateY(-2px);
  }
}

.assistant-form {
  border-top: 1px solid #e5e7eb;
  padding: 10px;
  background: #ffffff;
}

.assistant-input {
  width: 100%;
  min-height: 84px;
  resize: vertical;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  padding: 9px 10px;
  font-size: 13px;
  line-height: 1.45;
  color: #111827;
}

.assistant-input:focus {
  outline: 2px solid rgba(37, 99, 235, 0.22);
  border-color: #2563eb;
}

.assistant-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-top: 8px;
}

.assistant-limit {
  min-width: 0;
  color: #6b7280;
  font-size: 12px;
}

.send-button {
  height: 32px;
  min-width: 68px;
  padding: 0 12px;
  border-radius: 6px;
  background: #111827;
  color: #ffffff;
  font-size: 13px;
  font-weight: 600;
}

.send-button:disabled {
  cursor: not-allowed;
  background: #9ca3af;
}

:global(.dark) .assistant-fab {
  border-color: rgba(96, 165, 250, 0.55);
  background: #2563eb;
  color: #ffffff;
  box-shadow: 0 12px 26px rgba(37, 99, 235, 0.34);
}

:global(.dark) .assistant-fab-close {
  border-color: rgba(96, 165, 250, 0.48);
  background: #111827;
  color: #bfdbfe;
  box-shadow: 0 7px 16px rgba(0, 0, 0, 0.34);
}

:global(.dark) .assistant-fab-close:hover {
  background: #1f2937;
  color: #ffffff;
}

:global(.dark) .assistant-restore-handle {
  border-color: rgba(96, 165, 250, 0.62);
  background: #1d4ed8;
  color: #ffffff;
  box-shadow: 0 9px 20px rgba(37, 99, 235, 0.32);
}

:global(.dark) .assistant-restore-handle:hover {
  background: #2563eb;
  color: #ffffff;
}

:global(.dark) .assistant-panel {
  border-color: #374151;
  background: #111827;
}

:global(.dark) .assistant-header,
:global(.dark) .assistant-form {
  border-color: #374151;
  background: #111827;
}

:global(.dark) .assistant-title,
:global(.dark) .assistant-input {
  color: #f9fafb;
}

:global(.dark) .assistant-messages {
  background: #0f172a;
}

:global(.dark) .assistant-message-assistant pre,
:global(.dark) .assistant-message-loading .assistant-waiting {
  border-color: #374151;
  background: #1f2937;
  color: #f9fafb;
}

:global(.dark) .assistant-input {
  border-color: #4b5563;
  background: #0f172a;
}

:global(.dark) .assistant-limit {
  color: #9ca3af;
}

:global(.dark) .notify-admin-status {
  color: #93c5fd;
}
</style>
