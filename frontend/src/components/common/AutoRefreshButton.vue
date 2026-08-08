<template>
  <div class="relative" ref="dropdownRef">
    <button
      @click="toggleDropdown"
      class="inline-flex max-w-full items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
      :title="t('common.autoRefresh.title')"
    >
      <svg
        class="h-3.5 w-3.5"
        :class="enabled ? 'animate-spin' : ''"
        xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"
      >
        <path fill-rule="evenodd" d="M15.312 11.424a5.5 5.5 0 01-9.201 2.466l-.312-.311h2.433a.75.75 0 000-1.5H4.598a.75.75 0 00-.75.75v3.634a.75.75 0 001.5 0v-2.033l.312.312a7 7 0 0011.712-3.138.75.75 0 00-1.449-.39zm-10.624-2.848a5.5 5.5 0 019.201-2.466l.312.311H11.768a.75.75 0 000 1.5h3.634a.75.75 0 00.75-.75V3.537a.75.75 0 00-1.5 0v2.034l-.312-.312A7 7 0 002.628 8.397a.75.75 0 001.449.39z" clip-rule="evenodd" />
      </svg>
      <span>
        {{ enabled
          ? t('common.autoRefresh.countdown', { seconds: countdown })
          : t('common.autoRefresh.title')
        }}
      </span>
    </button>

  </div>

  <Teleport to="body">
    <div
      v-if="showDropdown"
      ref="menuRef"
      class="fixed z-50 max-w-[calc(100vw-1rem)] w-44 rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-800"
      :style="menuStyle"
      @click.stop
    >
      <div class="p-1.5">
        <button
          @click="$emit('update:enabled', !enabled)"
          class="flex w-full items-center justify-between rounded-md px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700"
        >
          <span>{{ t('common.autoRefresh.enable') }}</span>
          <svg v-if="enabled" class="h-4 w-4 text-primary-500" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M16.704 4.153a.75.75 0 01.143 1.052l-8 10.5a.75.75 0 01-1.127.075l-4.5-4.5a.75.75 0 011.06-1.06l3.894 3.893 7.48-9.817a.75.75 0 011.05-.143z" clip-rule="evenodd" />
          </svg>
        </button>
        <div class="my-1 border-t border-gray-100 dark:border-dark-700"></div>
        <button
          v-for="sec in intervals"
          :key="sec"
          @click="$emit('update:interval', sec)"
          class="flex w-full items-center justify-between rounded-md px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700"
        >
          <span>{{ t('common.autoRefresh.seconds', { n: sec }) }}</span>
          <svg v-if="intervalSeconds === sec" class="h-4 w-4 text-primary-500" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M16.704 4.153a.75.75 0 01.143 1.052l-8 10.5a.75.75 0 01-1.127.075l-4.5-4.5a.75.75 0 011.06-1.06l3.894 3.893 7.48-9.817a.75.75 0 011.05-.143z" clip-rule="evenodd" />
          </svg>
        </button>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, onMounted, onBeforeUnmount, type CSSProperties } from 'vue'
import { useI18n } from 'vue-i18n'
import { clampFloatingMenuPosition } from '@/utils/floatingMenuPosition'

defineProps<{
  enabled: boolean
  intervalSeconds: number
  countdown: number
  intervals: readonly number[]
}>()

defineEmits<{
  (e: 'update:enabled', value: boolean): void
  (e: 'update:interval', value: number): void
}>()

const { t } = useI18n()
const showDropdown = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const menuRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLElement | null>(null)
const menuPosition = ref<{ top: number; left: number; maxHeight: number } | null>(null)

const menuStyle = computed<CSSProperties>(() => {
  const position = menuPosition.value
  if (!position) return {}
  return {
    top: `${position.top}px`,
    left: `${position.left}px`,
    maxHeight: `${position.maxHeight}px`,
    overflowY: 'auto'
  }
})

function closeDropdown() {
  showDropdown.value = false
  menuPosition.value = null
  triggerRef.value = null
}

function adjustMenuPosition() {
  if (!showDropdown.value || !triggerRef.value) return

  nextTick(() => {
    const triggerRect = triggerRef.value?.getBoundingClientRect()
    const menuRect = menuRef.value?.getBoundingClientRect()
    if (!triggerRect || !menuRect) return

    const padding = 8
    let top = triggerRect.bottom + 4
    if (top + menuRect.height > window.innerHeight - padding) {
      top = triggerRect.top - menuRect.height - 4
    }
    menuPosition.value = clampFloatingMenuPosition(
      { top, left: triggerRect.right - menuRect.width },
      { width: menuRect.width, height: menuRect.height },
      { width: window.innerWidth, height: window.innerHeight },
      padding
    )
  })
}

function toggleDropdown(event: MouseEvent) {
  if (showDropdown.value) {
    closeDropdown()
    return
  }

  triggerRef.value = event.currentTarget as HTMLElement
  showDropdown.value = true
  adjustMenuPosition()
}

function handleClickOutside(event: MouseEvent) {
  const target = event.target as Node
  if (!dropdownRef.value?.contains(target) && !menuRef.value?.contains(target)) {
    closeDropdown()
  }
}

function handleViewportChange() {
  adjustMenuPosition()
}

onMounted(() => document.addEventListener('click', handleClickOutside))
onMounted(() => {
  window.addEventListener('resize', handleViewportChange)
  window.addEventListener('scroll', handleViewportChange, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
  window.removeEventListener('resize', handleViewportChange)
  window.removeEventListener('scroll', handleViewportChange, true)
})
</script>
