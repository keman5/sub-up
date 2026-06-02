<template>
  <div
    ref="rootRef"
    class="fixed right-4 bottom-4 z-40 flex max-w-[calc(100vw-2rem)] flex-col items-end gap-3 md:right-6 md:bottom-6"
  >
    <transition
      enter-active-class="transition duration-180 ease-out"
      enter-from-class="translate-y-2 opacity-0"
      enter-to-class="translate-y-0 opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="translate-y-0 opacity-100"
      leave-to-class="translate-y-2 opacity-0"
    >
      <div
        v-if="isOpen"
        id="home-support-panel"
        class="w-[min(15rem,calc(100vw-2rem))] overflow-hidden rounded-2xl bg-[var(--card)] shadow-[0_18px_45px_rgba(15,23,42,0.16)] dark:shadow-[0_18px_45px_rgba(0,0,0,0.38)]"
      >
        <div class="border-b border-[var(--border)] px-3 py-2.5">
          <p class="text-sm font-semibold text-foreground">{{ homeSupportEntry.panelTitle }}</p>
        </div>

        <div class="px-3 py-3">
          <div class="mx-auto w-fit overflow-hidden rounded-xl bg-white p-2">
            <img
              :src="homeSupportEntry.qrImagePath"
              alt="QQ群客服二维码"
              class="h-auto w-[192px] max-w-full rounded-lg object-cover"
            />
            <div class="mt-2 flex items-center justify-center gap-1.5 rounded-lg bg-slate-50 px-2 py-1.5 text-slate-900">
              <p class="text-sm font-semibold tracking-[0.04em]">{{ homeSupportEntry.groupNumber }}</p>
              <button
                type="button"
                class="inline-flex size-8 shrink-0 items-center justify-center rounded-md text-slate-500 transition hover:text-slate-900"
                :aria-label="copied ? '已复制群号' : '复制群号'"
                :title="copied ? '已复制群号' : '复制群号'"
                @click="copyGroupNumber"
              >
                <Icon :name="copied ? 'check' : 'copy'" size="sm" :stroke-width="2" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </transition>

    <button
      type="button"
      class="inline-flex items-center gap-2 rounded-full bg-primary px-4 py-3 text-sm font-semibold text-primary-foreground shadow-[0_16px_40px_rgba(79,70,229,0.35)] transition hover:-translate-y-0.5 hover:shadow-[0_20px_48px_rgba(79,70,229,0.42)] focus:outline-none focus:ring-2 focus:ring-primary/40"
      :aria-expanded="isOpen"
      aria-controls="home-support-panel"
      @click="toggleOpen"
    >
      <Icon name="chat" size="sm" :stroke-width="2" />
      <span>{{ homeSupportEntry.buttonLabel }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { homeSupportEntry } from './homeData'

const rootRef = ref<HTMLElement | null>(null)
const isOpen = ref(false)
const { copied, copyToClipboard } = useClipboard()

function toggleOpen() {
  isOpen.value = !isOpen.value
}

async function copyGroupNumber() {
  await copyToClipboard(homeSupportEntry.groupNumber, 'QQ群号已复制')
}

function handleDocumentClick(event: MouseEvent) {
  const target = event.target as Node | null
  if (!target || !rootRef.value?.contains(target)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleDocumentClick)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleDocumentClick)
})
</script>
