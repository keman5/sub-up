<template>
  <div class="relative w-full">
    <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
      <Icon name="search" size="md" class="text-gray-400" />
    </div>
    <input
      :value="modelValue"
      type="text"
      class="input pl-10 pr-9"
      :placeholder="placeholder"
      @input="handleInput"
      @focus="emit('focus')"
      @blur="emit('blur')"
    />
    <button
      v-if="modelValue"
      data-test="search-suggest-clear"
      type="button"
      class="absolute inset-y-0 right-2 flex items-center text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-gray-300"
      :aria-label="t('common.clear')"
      @click="clearValue"
    >
      <Icon name="x" size="sm" :stroke-width="2" />
    </button>

    <div
      v-if="open && (loading || suggestions.length > 0 || emptyText)"
      class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
    >
      <div
        v-if="loading"
        class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('common.loading') }}
      </div>
      <div
        v-else-if="suggestions.length === 0 && emptyText"
        class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400"
      >
        {{ emptyText }}
      </div>
      <button
        v-for="suggestion in suggestions"
        :key="suggestion.id"
        data-test="search-suggest-option"
        type="button"
        class="w-full px-4 py-2 text-left hover:bg-gray-100 dark:hover:bg-gray-700"
        @mousedown.prevent="selectSuggestion(suggestion)"
      >
        <div class="truncate text-sm font-medium text-gray-900 dark:text-white">
          {{ suggestion.primaryText }}
        </div>
        <div
          v-if="suggestion.secondaryText"
          class="truncate text-xs text-gray-500 dark:text-gray-400"
        >
          {{ suggestion.secondaryText }}
        </div>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useDebounceFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'

import Icon from '@/components/icons/Icon.vue'

export interface SearchSuggestOption<T = any> {
  id: string | number
  primaryText: string
  secondaryText?: string
  value: T
}

const props = withDefaults(defineProps<{
  modelValue: string
  suggestions: SearchSuggestOption<any>[]
  placeholder?: string
  debounceMs?: number
  open?: boolean
  loading?: boolean
  emptyText?: string
}>(), {
  placeholder: 'Search...',
  debounceMs: 300,
  open: false,
  loading: false,
  emptyText: '',
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'search', value: string): void
  (e: 'select', option: SearchSuggestOption<any>): void
  (e: 'clear'): void
  (e: 'focus'): void
  (e: 'blur'): void
}>()

const { t } = useI18n()

const debouncedEmitSearch = useDebounceFn((value: string) => {
  emit('search', value)
}, props.debounceMs)

const handleInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  emit('update:modelValue', value)
  debouncedEmitSearch(value)
}

const selectSuggestion = (option: SearchSuggestOption<any>) => {
  emit('update:modelValue', option.primaryText)
  emit('select', option)
}

const clearValue = () => {
  emit('update:modelValue', '')
  emit('clear')
  emit('search', '')
}
</script>
