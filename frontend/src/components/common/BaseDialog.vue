<template>
  <Teleport to="body">
    <Transition name="modal" @after-leave="handleAfterLeave">
      <div
        v-if="show"
        class="modal-overlay"
        :style="zIndexStyle"
        :aria-labelledby="dialogId"
        role="dialog"
        aria-modal="true"
        @click.self="handleClose"
      >
        <!-- Modal panel -->
        <div ref="dialogRef" :class="['modal-content', widthClasses]" @click.stop>
          <!-- Header -->
          <div class="modal-header">
            <h3 :id="dialogId" class="modal-title">
              {{ title }}
            </h3>
            <button
              v-if="showCloseButton"
              @click="emit('close')"
              class="-mr-2 rounded-xl p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 focus-visible:ring-offset-2 dark:text-dark-500 dark:hover:bg-dark-700 dark:hover:text-dark-300 dark:focus-visible:ring-offset-dark-900"
              aria-label="Close modal"
            >
              <Icon name="x" size="md" />
            </button>
          </div>

          <!-- Body -->
          <div class="modal-body">
            <slot></slot>
          </div>

          <!-- Footer -->
          <div v-if="$slots.footer" class="modal-footer">
            <slot name="footer"></slot>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script lang="ts">
let dialogIdCounter = 0
let openModalCount = 0
let lockedScrollY = 0
let previousBodyStyles: Partial<Record<'overflow' | 'position' | 'top' | 'width', string>> | null = null
</script>

<script setup lang="ts">
import { computed, watch, onMounted, onUnmounted, ref, nextTick } from 'vue'
import Icon from '@/components/icons/Icon.vue'

// 生成唯一ID以避免多个对话框时ID冲突
const dialogId = `modal-title-${++dialogIdCounter}`

// 焦点管理
const dialogRef = ref<HTMLElement | null>(null)
let previousActiveElement: HTMLElement | null = null
let hasLockedBodyScroll = false
let pendingUnlockAfterLeave = false

type DialogWidth = 'narrow' | 'normal' | 'wide' | 'extra-wide' | 'full'

interface Props {
  show: boolean
  title: string
  width?: DialogWidth
  closeOnEscape?: boolean
  closeOnClickOutside?: boolean
  showCloseButton?: boolean
  zIndex?: number
}

interface Emits {
  (e: 'close'): void
}

const props = withDefaults(defineProps<Props>(), {
  width: 'normal',
  closeOnEscape: true,
  closeOnClickOutside: false,
  showCloseButton: true,
  zIndex: 50
})

const emit = defineEmits<Emits>()

// Custom z-index style (overrides the default z-50 from CSS)
const zIndexStyle = computed(() => {
  return props.zIndex !== 50 ? { zIndex: props.zIndex } : undefined
})

const widthClasses = computed(() => {
  // Width guidance: narrow=confirm/short prompts, normal=standard forms,
  // wide=multi-section forms or rich content, extra-wide=analytics/tables,
  // full=full-screen or very dense layouts.
  const widths: Record<DialogWidth, string> = {
    narrow: 'max-w-md',
    normal: 'max-w-lg',
    wide: 'w-full sm:max-w-2xl md:max-w-3xl lg:max-w-4xl',
    'extra-wide': 'w-full sm:max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl',
    full: 'w-full sm:max-w-4xl md:max-w-5xl lg:max-w-6xl xl:max-w-7xl'
  }
  return widths[props.width]
})

const handleClose = () => {
  if (props.closeOnClickOutside) {
    emit('close')
  }
}

const handleEscape = (event: KeyboardEvent) => {
  if (props.show && props.closeOnEscape && event.key === 'Escape') {
    emit('close')
  }
}

const lockBodyScroll = () => {
  if (hasLockedBodyScroll) return
  if (openModalCount === 0) {
    lockedScrollY = window.scrollY
    previousBodyStyles = {
      overflow: document.body.style.overflow,
      position: document.body.style.position,
      top: document.body.style.top,
      width: document.body.style.width
    }
    document.documentElement.classList.add('modal-open')
    document.body.classList.add('modal-open')
    document.body.style.overflow = 'hidden'
    document.body.style.position = 'fixed'
    document.body.style.top = `-${lockedScrollY}px`
    document.body.style.width = '100%'
  }
  openModalCount += 1
  hasLockedBodyScroll = true
}

const unlockBodyScroll = () => {
  if (!hasLockedBodyScroll) return
  openModalCount = Math.max(0, openModalCount - 1)
  if (openModalCount === 0) {
    document.documentElement.classList.remove('modal-open')
    document.body.classList.remove('modal-open')
    document.body.style.overflow = previousBodyStyles?.overflow || ''
    document.body.style.position = previousBodyStyles?.position || ''
    document.body.style.top = previousBodyStyles?.top || ''
    document.body.style.width = previousBodyStyles?.width || ''
    window.scrollTo(0, lockedScrollY)
    previousBodyStyles = null
    lockedScrollY = 0
  }
  hasLockedBodyScroll = false
}

const restorePreviousFocus = () => {
  if (previousActiveElement && typeof previousActiveElement.focus === 'function') {
    previousActiveElement.focus()
  }
  previousActiveElement = null
}

const handleAfterLeave = () => {
  if (pendingUnlockAfterLeave) {
    unlockBodyScroll()
    pendingUnlockAfterLeave = false
  }
  restorePreviousFocus()
}

// Prevent body scroll when modal is open and manage focus
watch(
  () => props.show,
  async (isOpen) => {
    if (isOpen) {
      pendingUnlockAfterLeave = false
      // 保存当前焦点元素
      previousActiveElement = document.activeElement as HTMLElement
      lockBodyScroll()

      // 等待DOM更新后设置焦点到对话框
      await nextTick()
      if (dialogRef.value) {
        const firstFocusable = dialogRef.value.querySelector<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        )
        firstFocusable?.focus()
      }
    } else {
      if (hasLockedBodyScroll) {
        pendingUnlockAfterLeave = true
      } else {
        restorePreviousFocus()
      }
    }
  },
  { immediate: true }
)

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleEscape)
  unlockBodyScroll()
})
</script>
