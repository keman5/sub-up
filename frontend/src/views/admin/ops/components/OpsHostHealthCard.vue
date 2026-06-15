<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { opsAPI, type OpsHostHealthSnapshot } from '@/api/admin/ops'

interface Props {
  refreshToken: number
}

const props = defineProps<Props>()
const { t } = useI18n()

const loading = ref(false)
const errorMessage = ref('')
const health = ref<OpsHostHealthSnapshot | null>(null)

const statusLabel = computed(() => {
  if (!health.value?.available) return t('admin.ops.hostHealth.unavailable')
  if (health.value.stale) return t('admin.ops.hostHealth.stale')
  if (health.value.cpu?.high) return t('admin.ops.hostHealth.highCpu')
  return t('admin.ops.hostHealth.normal')
})

const statusClass = computed(() => {
  if (!health.value?.available) return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  if (health.value.stale) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (health.value.cpu?.high) return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
})

const collectedAtLabel = computed(() => {
  const value = health.value?.collected_at
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
})

function formatPercent(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `${value.toFixed(1)}%`
}

function formatNumber(value?: number | null, digits = 2): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return value.toFixed(digits)
}

function formatMB(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `${Math.round(value)} MB`
}

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    health.value = await opsAPI.getHostHealth()
  } catch (err: any) {
    health.value = null
    console.error('[OpsHostHealthCard] Failed to load host health', err)
    errorMessage.value = err?.message || t('admin.ops.hostHealth.failedToLoad')
  } finally {
    loading.value = false
  }
}

watch(
  () => props.refreshToken,
  () => {
    void loadData()
  },
  { immediate: true }
)
</script>

<template>
  <section class="card p-4 md:p-5">
    <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
      <div>
        <h3 class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white">
          <span>{{ t('admin.ops.hostHealth.title') }}</span>
          <button
            type="button"
            class="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-gray-100 text-gray-600 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
            :disabled="loading"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            data-testid="host-health-refresh"
            @click="loadData"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </h3>
        <p v-if="health?.available" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.hostHealth.lastCollected', { time: collectedAtLabel }) }}
          <span v-if="typeof health.age_seconds === 'number'"> · {{ t('admin.ops.hostHealth.age', { seconds: health.age_seconds }) }}</span>
        </p>
      </div>
      <span class="rounded px-2 py-1 text-xs font-semibold" :class="statusClass">
        {{ statusLabel }}
      </span>
    </div>

    <div v-if="errorMessage" class="mb-4 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-600 dark:bg-red-900/20 dark:text-red-400">
      {{ errorMessage }}
    </div>

    <div v-if="loading" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.ops.loadingText') }}
    </div>

    <EmptyState
      v-else-if="health && !health.available"
      :title="t('admin.ops.hostHealth.unavailable')"
      :description="t('admin.ops.hostHealth.unavailableHint')"
    />

    <EmptyState
      v-else-if="!health"
      :title="t('common.noData')"
      :description="t('admin.ops.hostHealth.empty')"
    />

    <div v-else class="space-y-4">
      <div v-if="health.diagnosis" class="rounded-lg bg-amber-50 px-3 py-2 text-xs font-medium text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">
        {{ health.diagnosis }}
      </div>

      <div class="grid grid-cols-2 gap-3 md:grid-cols-4">
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.hostHealth.cpu') }}</div>
          <div class="mt-1 text-xl font-bold" :class="health.cpu?.high ? 'text-red-600 dark:text-red-400' : 'text-gray-900 dark:text-white'">
            {{ formatPercent(health.cpu?.usage_percent) }}
          </div>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.hostHealth.loadAverage') }}</div>
          <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
            {{ formatNumber(health.load_average?.one) }} / {{ formatNumber(health.load_average?.five) }} / {{ formatNumber(health.load_average?.fifteen) }}
          </div>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.hostHealth.availableMemory') }}</div>
          <div class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{{ formatMB(health.memory?.available_mb) }}</div>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.hostHealth.swapUsed') }}</div>
          <div class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{{ formatMB(health.memory?.swap_used_mb) }}</div>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <h4 class="mb-2 text-xs font-semibold text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.hostHealth.topContainers') }}
          </h4>
          <div v-if="health.top_containers?.length" class="space-y-2">
            <div v-for="item in health.top_containers.slice(0, 5)" :key="item.name" class="grid grid-cols-[1fr_auto] gap-3 text-xs">
              <span class="min-w-0 truncate font-medium text-gray-700 dark:text-gray-200">{{ item.name }}</span>
              <span class="font-semibold text-gray-900 dark:text-white">{{ formatPercent(item.cpu_percent) }}</span>
              <span class="min-w-0 truncate text-gray-500 dark:text-gray-400">{{ item.memory || '-' }}</span>
              <span class="text-gray-500 dark:text-gray-400">{{ item.pids ?? '-' }} PID</span>
            </div>
          </div>
          <div v-else class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.noData') }}</div>
        </div>

        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <h4 class="mb-2 text-xs font-semibold text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.hostHealth.topProcesses') }}
          </h4>
          <div v-if="health.top_processes?.length" class="space-y-2">
            <div v-for="item in health.top_processes.slice(0, 5)" :key="`${item.pid}-${item.command}`" class="grid grid-cols-[auto_1fr_auto] gap-3 text-xs">
              <span class="text-gray-500 dark:text-gray-400">{{ item.pid }}</span>
              <span class="min-w-0 truncate font-medium text-gray-700 dark:text-gray-200">{{ item.command }}</span>
              <span class="font-semibold text-gray-900 dark:text-white">{{ formatPercent(item.cpu_percent) }}</span>
              <span></span>
              <span class="text-gray-500 dark:text-gray-400">{{ formatMB(item.rss_mb) }}</span>
              <span></span>
            </div>
          </div>
          <div v-else class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.noData') }}</div>
        </div>
      </div>
    </div>
  </section>
</template>
