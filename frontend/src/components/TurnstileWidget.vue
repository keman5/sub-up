<template>
  <div v-if="siteKey" class="turnstile-wrapper">
    <button
      v-if="loadFailed"
      type="button"
      class="turnstile-reload-prompt"
      data-testid="turnstile-reload-prompt"
      @click="reloadPage"
    >
      <span class="turnstile-reload-title">验证码加载失败</span>
      <span class="turnstile-reload-desc">点击刷新页面</span>
    </button>
    <div v-else ref="containerRef" class="turnstile-container"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'

interface TurnstileRenderOptions {
  sitekey: string
  callback: (token: string) => void
  'expired-callback'?: () => void
  'error-callback'?: () => void
  theme?: 'light' | 'dark' | 'auto'
  size?: 'normal' | 'compact' | 'flexible'
}

interface TurnstileAPI {
  render: (container: HTMLElement, options: TurnstileRenderOptions) => string
  reset: (widgetId?: string) => void
  remove: (widgetId?: string) => void
}

declare global {
  interface Window {
    turnstile?: TurnstileAPI
    onTurnstileLoad?: () => void
  }
}

const props = withDefaults(
  defineProps<{
    siteKey: string
    theme?: 'light' | 'dark' | 'auto'
    size?: 'normal' | 'compact' | 'flexible'
  }>(),
  {
    theme: 'auto',
    size: 'flexible'
  }
)

const emit = defineEmits<{
  (e: 'verify', token: string): void
  (e: 'expire'): void
  (e: 'error'): void
}>()

const containerRef = ref<HTMLElement | null>(null)
const widgetId = ref<string | null>(null)
const scriptLoaded = ref(false)
const loadFailed = ref(false)

const loadScript = (): Promise<void> => {
  return new Promise((resolve, reject) => {
    if (window.turnstile) {
      scriptLoaded.value = true
      loadFailed.value = false
      resolve()
      return
    }

    // Check if script is already loading
    const existingScript = document.querySelector('script[src*="turnstile"]')
    if (existingScript) {
      window.onTurnstileLoad = () => {
        scriptLoaded.value = true
        loadFailed.value = false
        resolve()
      }
      return
    }

    const script = document.createElement('script')
    script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad'
    script.async = true
    script.defer = true

    window.onTurnstileLoad = () => {
      scriptLoaded.value = true
      loadFailed.value = false
      resolve()
    }

    script.onerror = () => {
      reject(new Error('Failed to load Turnstile script'))
    }

    document.head.appendChild(script)
  })
}

const reloadPage = () => {
  window.location.reload()
}

const renderWidget = () => {
  if (!window.turnstile || !containerRef.value || !props.siteKey) {
    return
  }

  // Remove existing widget if any
  if (widgetId.value) {
    try {
      window.turnstile.remove(widgetId.value)
    } catch {
      // Ignore errors when removing
    }
    widgetId.value = null
  }

  // Clear container
  containerRef.value.innerHTML = ''

  widgetId.value = window.turnstile.render(containerRef.value, {
    sitekey: props.siteKey,
    callback: (token: string) => {
      emit('verify', token)
    },
    'expired-callback': () => {
      emit('expire')
    },
    'error-callback': () => {
      emit('error')
    },
    theme: props.theme,
    size: props.size
  })
}

const reset = () => {
  if (window.turnstile && widgetId.value) {
    window.turnstile.reset(widgetId.value)
  }
}

// Expose reset method to parent
defineExpose({ reset })

onMounted(async () => {
  if (!props.siteKey) {
    return
  }

  try {
    await loadScript()
    renderWidget()
  } catch (error) {
    console.error('Failed to initialize Turnstile:', error)
    loadFailed.value = true
    emit('error')
  }
})

onUnmounted(() => {
  if (window.turnstile && widgetId.value) {
    try {
      window.turnstile.remove(widgetId.value)
    } catch {
      // Ignore errors when removing
    }
  }
})

// Re-render when siteKey changes
watch(
  () => props.siteKey,
  (newKey) => {
    if (newKey && scriptLoaded.value) {
      loadFailed.value = false
      renderWidget()
    }
  }
)
</script>

<style scoped>
.turnstile-wrapper {
  width: 100%;
}

.turnstile-container {
  width: 100%;
  min-height: 65px;
}

.turnstile-reload-prompt {
  display: flex;
  min-height: 65px;
  width: 100%;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.25rem;
  border-radius: 0.75rem;
  border: 1px dashed rgb(239 68 68 / 0.45);
  background: rgb(254 242 242 / 0.75);
  padding: 0.875rem 1rem;
  text-align: center;
  transition:
    background-color 150ms ease,
    border-color 150ms ease;
}

.turnstile-reload-prompt:hover {
  border-color: rgb(239 68 68 / 0.7);
  background: rgb(254 226 226 / 0.9);
}

.turnstile-reload-title {
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(185 28 28);
}

.turnstile-reload-desc {
  font-size: 0.75rem;
  color: rgb(220 38 38);
}

:global(.dark) .turnstile-reload-prompt {
  border-color: rgb(248 113 113 / 0.45);
  background: rgb(127 29 29 / 0.22);
}

:global(.dark) .turnstile-reload-prompt:hover {
  border-color: rgb(248 113 113 / 0.7);
  background: rgb(127 29 29 / 0.32);
}

:global(.dark) .turnstile-reload-title {
  color: rgb(254 202 202);
}

:global(.dark) .turnstile-reload-desc {
  color: rgb(252 165 165);
}

/* Make the Turnstile iframe fill the container width */
.turnstile-container :deep(iframe) {
  width: 100% !important;
}
</style>
