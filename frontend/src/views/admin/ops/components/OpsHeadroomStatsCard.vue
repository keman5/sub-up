<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { opsAPI, type OpsHeadroomStatsSnapshot } from '@/api/admin/ops'
import { formatNumber } from '@/utils/format'

interface Props {
  refreshToken: number
}

const props = defineProps<Props>()
const { t } = useI18n()

const loading = ref(false)
const errorMessage = ref('')
const disabled = ref(false)
const stats = ref<OpsHeadroomStatsSnapshot | null>(null)

const topModels = computed(() => sortBreakdown(stats.value?.by_model).slice(0, 6))
const topProviders = computed(() => sortBreakdown(stats.value?.by_provider).slice(0, 4))

const compressionRate = computed(() => {
  const total = stats.value?.requests_total ?? 0
  if (total <= 0) return 0
  return ((stats.value?.requests_compressed ?? 0) / total) * 100
})

const fetchedAtLabel = computed(() => {
  const value = stats.value?.fetched_at
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
})

function sortBreakdown(value?: Record<string, number>) {
  return Object.entries(value ?? {})
    .filter(([, count]) => Number.isFinite(count))
    .sort((a, b) => b[1] - a[1])
}

function formatInt(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return formatNumber(Math.round(value))
}

function formatPercent(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `${value.toFixed(2)}%`
}

function formatUSD(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `$${value.toFixed(4)}`
}

function isDisabledError(err: any): boolean {
  const status = err?.status ?? err?.response?.status
  const message = String(err?.message ?? err?.response?.data?.message ?? '').toLowerCase()
  return status === 503 && message.includes('headroom')
}

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  disabled.value = false
  try {
    stats.value = await opsAPI.getHeadroomStats()
  } catch (err: any) {
    stats.value = null
    if (isDisabledError(err)) {
      disabled.value = true
    } else {
      console.error('[OpsHeadroomStatsCard] Failed to load data', err)
      errorMessage.value = err?.message || t('admin.ops.headroomStats.failedToLoad')
    }
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
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h3 class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white">
          <span>{{ t('admin.ops.headroomStats.title') }}</span>
          <button
            type="button"
            class="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-gray-100 text-gray-600 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
            :disabled="loading"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            data-testid="headroom-stats-refresh"
            @click="loadData"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </h3>
        <p v-if="stats" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.headroomStats.lastFetched', { time: fetchedAtLabel }) }}
        </p>
      </div>
      <span
        v-if="stats?.mode"
        class="rounded bg-gray-100 px-2 py-1 text-xs font-semibold text-gray-600 dark:bg-dark-700 dark:text-gray-300"
      >
        {{ stats.mode }}
      </span>
    </div>

    <div v-if="errorMessage" class="mb-4 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-600 dark:bg-red-900/20 dark:text-red-400">
      {{ errorMessage }}
    </div>

    <div v-if="loading" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.ops.loadingText') }}
    </div>

    <EmptyState
      v-else-if="disabled"
      :title="t('admin.ops.headroomStats.disabled')"
      :description="t('admin.ops.headroomStats.disabledHint')"
    />

    <EmptyState
      v-else-if="!stats"
      :title="t('common.noData')"
      :description="t('admin.ops.headroomStats.empty')"
    />

    <div v-else class="space-y-4">
      <div class="grid grid-cols-2 gap-3 md:grid-cols-4">
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.headroomStats.savedTokens') }}</div>
          <div class="mt-1 text-xl font-bold text-emerald-600 dark:text-emerald-400">{{ formatInt(stats.tokens_saved) }}</div>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.headroomStats.savingsPercent') }}</div>
          <div class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{{ formatPercent(stats.savings_percent) }}</div>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.headroomStats.compressedRequests') }}</div>
          <div class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{{ formatInt(stats.requests_compressed) }}</div>
          <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ formatPercent(compressionRate) }}</div>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.headroomStats.estimatedSavings') }}</div>
          <div class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{{ formatUSD(stats.total_saved_usd) }}</div>
          <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ formatPercent(stats.cost_savings_percent) }}</div>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <h4 class="mb-2 text-xs font-semibold text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.headroomStats.tokenDetails') }}
          </h4>
          <div class="grid grid-cols-2 gap-2 text-xs text-gray-600 dark:text-gray-300">
            <span>{{ t('admin.ops.headroomStats.inputTokens') }}</span>
            <span class="text-right font-medium">{{ formatInt(stats.input_tokens) }}</span>
            <span>{{ t('admin.ops.headroomStats.outputTokens') }}</span>
            <span class="text-right font-medium">{{ formatInt(stats.output_tokens) }}</span>
            <span>{{ t('admin.ops.headroomStats.beforeCompression') }}</span>
            <span class="text-right font-medium">{{ formatInt(stats.total_before_compression) }}</span>
            <span>{{ t('admin.ops.headroomStats.averageCompression') }}</span>
            <span class="text-right font-medium">{{ formatPercent(stats.average_compression_percent) }}</span>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <h4 class="mb-2 text-xs font-semibold text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.headroomStats.requestDetails') }}
          </h4>
          <div class="grid grid-cols-2 gap-2 text-xs text-gray-600 dark:text-gray-300">
            <span>{{ t('admin.ops.headroomStats.totalRequests') }}</span>
            <span class="text-right font-medium">{{ formatInt(stats.requests_total) }}</span>
            <span>{{ t('admin.ops.headroomStats.apiRequests') }}</span>
            <span class="text-right font-medium">{{ formatInt(stats.api_requests) }}</span>
            <span>{{ t('admin.ops.headroomStats.failedRequests') }}</span>
            <span class="text-right font-medium">{{ formatInt(stats.requests_failed) }}</span>
            <span>{{ t('admin.ops.headroomStats.proxyCompressionSaved') }}</span>
            <span class="text-right font-medium">{{ formatInt(stats.proxy_compression_saved) }}</span>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <h4 class="mb-2 text-xs font-semibold text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.headroomStats.topModels') }}
          </h4>
          <div v-if="topModels.length" class="space-y-2">
            <div v-for="[model, count] in topModels" :key="model" class="flex items-center justify-between gap-3 text-xs">
              <span class="min-w-0 truncate font-medium text-gray-700 dark:text-gray-200">{{ model }}</span>
              <span class="shrink-0 text-gray-500 dark:text-gray-400">{{ formatInt(count) }}</span>
            </div>
          </div>
          <div v-else class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.noData') }}</div>
        </div>

        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <h4 class="mb-2 text-xs font-semibold text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.headroomStats.providers') }}
          </h4>
          <div v-if="topProviders.length" class="space-y-2">
            <div v-for="[provider, count] in topProviders" :key="provider" class="flex items-center justify-between gap-3 text-xs">
              <span class="min-w-0 truncate font-medium text-gray-700 dark:text-gray-200">{{ provider }}</span>
              <span class="shrink-0 text-gray-500 dark:text-gray-400">{{ formatInt(count) }}</span>
            </div>
          </div>
          <div v-else class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.noData') }}</div>
        </div>
      </div>
    </div>
  </section>
</template>
