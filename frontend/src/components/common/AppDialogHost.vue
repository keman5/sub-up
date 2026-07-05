<template>
  <BaseDialog
    :show="!!dialog"
    :title="dialogTitle"
    width="narrow"
    :close-on-click-outside="false"
    :z-index="100"
    @close="handleCancel"
  >
    <div v-if="dialog" class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-gray-400">{{ dialog.message }}</p>
      <input
        v-if="dialog.kind === 'prompt'"
        ref="inputRef"
        v-model="inputValue"
        :type="dialog.inputType || 'text'"
        class="input w-full"
        :placeholder="dialog.placeholder || ''"
        @keydown.enter.prevent="handleConfirm"
      />
    </div>

    <template #footer>
      <div v-if="dialog" class="flex justify-end space-x-3">
        <button
          v-if="dialog.kind !== 'alert'"
          type="button"
          class="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600 dark:focus:ring-offset-dark-800"
          @click="handleCancel"
        >
          {{ dialog.cancelText || t('common.cancel') }}
        </button>
        <button
          type="button"
          :class="[
            'rounded-md px-4 py-2 text-sm font-medium text-white focus:outline-none focus:ring-2 focus:ring-offset-2 dark:focus:ring-offset-dark-800',
            dialog.danger
              ? 'bg-red-600 hover:bg-red-700 focus:ring-red-500'
              : 'bg-primary-600 hover:bg-primary-700 focus:ring-primary-500',
          ]"
          @click="handleConfirm"
        >
          {{ dialog.confirmText || t('common.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useAppDialog } from '@/composables/useAppDialog'

const { t } = useI18n()
const { currentDialog, settleCurrent } = useAppDialog()
const inputRef = ref<HTMLInputElement | null>(null)
const inputValue = ref('')

const dialog = currentDialog
const dialogTitle = computed(() => dialog.value?.title || t('common.confirm'))

watch(
  () => dialog.value?.id,
  async () => {
    inputValue.value = dialog.value?.defaultValue || ''
    if (dialog.value?.kind === 'prompt') {
      await nextTick()
      inputRef.value?.focus()
      inputRef.value?.select()
    }
  },
)

function handleConfirm() {
  if (!dialog.value) return
  if (dialog.value.kind === 'prompt') {
    settleCurrent(inputValue.value)
    return
  }
  settleCurrent(true)
}

function handleCancel() {
  if (!dialog.value) return
  if (dialog.value.kind === 'alert') {
    settleCurrent(true)
    return
  }
  if (dialog.value.kind === 'prompt') {
    settleCurrent(null)
    return
  }
  settleCurrent(false)
}
</script>
