<template>
  <div class="relative" ref="dropdownRef">
    <button
      type="button"
      data-testid="theme-switcher-trigger"
      :class="
        iconOnly
          ? 'home-icon-button rounded-full'
          : 'flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'
      "
      :title="currentOption.label"
      :aria-label="t('common.theme.label')"
      @click="toggleDropdown"
    >
      <Icon :name="triggerIcon" size="sm" />
      <span v-if="!iconOnly" class="hidden sm:inline">{{ currentOption.shortLabel }}</span>
      <Icon
        v-if="!iconOnly"
        name="chevronDown"
        size="xs"
        class="text-gray-400 transition-transform duration-200"
        :class="{ 'rotate-180': isOpen }"
      />
    </button>

    <transition name="dropdown">
      <div
        v-if="isOpen"
        class="absolute right-0 z-50 mt-1 w-36 overflow-hidden rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-800"
      >
        <button
          v-for="option in themeOptions"
          :key="option.value"
          type="button"
          :data-testid="`theme-option-${option.value}`"
          class="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700"
          :class="{
            'bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-400':
              option.value === preference
          }"
          @click="selectTheme(option.value)"
        >
          <Icon :name="option.icon" size="sm" />
          <span>{{ option.label }}</span>
          <Icon v-if="option.value === preference" name="check" size="sm" class="ml-auto text-primary-500" />
        </button>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useTheme, type ThemePreference } from '@/composables/useTheme'

type ThemeOption = {
  value: ThemePreference
  label: string
  shortLabel: string
  icon: 'sun' | 'moon' | 'sparkles'
}

const { t } = useI18n()
const props = withDefaults(
  defineProps<{
    iconOnly?: boolean
  }>(),
  {
    iconOnly: false
  }
)

const { preference, effectiveTheme, setThemePreference } = useTheme()

const isOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

const themeOptions = computed<ThemeOption[]>(() => [
  {
    value: 'system',
    label: t('common.theme.system'),
    shortLabel: t('common.theme.systemShort'),
    icon: 'sparkles'
  },
  {
    value: 'light',
    label: t('common.theme.light'),
    shortLabel: t('common.theme.lightShort'),
    icon: 'sun'
  },
  {
    value: 'dark',
    label: t('common.theme.dark'),
    shortLabel: t('common.theme.darkShort'),
    icon: 'moon'
  }
])

const currentOption = computed(() => {
  return themeOptions.value.find((option) => option.value === preference.value) ?? themeOptions.value[0]
})

const iconOnly = computed(() => props.iconOnly)
const triggerIcon = computed<'sun' | 'moon' | 'sparkles'>(() => {
  if (iconOnly.value) {
    return effectiveTheme.value === 'dark' ? 'moon' : 'sun'
  }
  return currentOption.value.icon
})

function toggleDropdown() {
  isOpen.value = !isOpen.value
}

function selectTheme(nextPreference: ThemePreference) {
  setThemePreference(nextPreference)
  isOpen.value = false
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.15s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}
</style>
