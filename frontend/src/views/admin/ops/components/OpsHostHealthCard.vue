<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, Filler, Legend, LineElement, LinearScale, PointElement, Tooltip } from 'chart.js'
import { Line } from 'vue-chartjs'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { opsAPI, type OpsHostHealthSnapshot } from '@/api/admin/ops'

ChartJS.register(Tooltip, Legend, LineElement, LinearScale, PointElement, CategoryScale, Filler)

interface Props {
  refreshToken: number
}

const MAX_TREND_POINTS = 30

const props = defineProps<Props>()
const { t } = useI18n()

const loading = ref(false)
const errorMessage = ref('')
const health = ref<OpsHostHealthSnapshot | null>(null)
const trendSnapshots = ref<OpsHostHealthSnapshot[]>([])

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const chartColors = computed(() => ({
  cpu: '#ef4444',
  cpuFill: '#ef444420',
  load: '#2563eb',
  loadFill: '#2563eb20',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280',
}))

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

const trendChartData = computed(() => {
  if (trendSnapshots.value.length < 2) return null
  const colors = chartColors.value
  return {
    labels: trendSnapshots.value.map((snapshot) => formatTrendLabel(snapshot.collected_at)),
    datasets: [
      {
        label: t('admin.ops.hostHealth.cpuUsageTrend'),
        data: trendSnapshots.value.map((snapshot) => normalizeNumber(snapshot.cpu?.usage_percent)),
        borderColor: colors.cpu,
        backgroundColor: colors.cpuFill,
        fill: true,
        tension: 0.35,
        pointRadius: 2,
        pointHitRadius: 8,
        spanGaps: true,
      },
      {
        label: t('admin.ops.hostHealth.loadOneMinute'),
        data: trendSnapshots.value.map((snapshot) => normalizeNumber(snapshot.load_average?.one)),
        borderColor: colors.load,
        backgroundColor: colors.loadFill,
        fill: false,
        tension: 0.35,
        pointRadius: 2,
        pointHitRadius: 8,
        yAxisID: 'y1',
        spanGaps: true,
      },
    ],
  }
})

const trendChartOptions = computed(() => {
  const colors = chartColors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: {
        position: 'top' as const,
        align: 'end' as const,
        labels: { color: colors.text, usePointStyle: true, boxWidth: 6, font: { size: 10 } },
      },
      tooltip: {
        backgroundColor: isDarkMode.value ? '#1f2937' : '#ffffff',
        titleColor: isDarkMode.value ? '#f3f4f6' : '#111827',
        bodyColor: isDarkMode.value ? '#d1d5db' : '#4b5563',
        borderColor: colors.grid,
        borderWidth: 1,
        padding: 10,
        displayColors: true,
        callbacks: {
          label: (context: any) => {
            const suffix = context.dataset.yAxisID === 'y1' ? '' : '%'
            const value = Number(context.parsed.y)
            const displayValue = Number.isFinite(value) ? value.toFixed(1) : '-'
            return `${context.dataset.label}: ${displayValue}${suffix}`
          },
        },
      },
    },
    scales: {
      x: {
        type: 'category' as const,
        grid: { display: false },
        ticks: { color: colors.text, font: { size: 10 }, maxTicksLimit: 8 },
      },
      y: {
        type: 'linear' as const,
        display: true,
        position: 'left' as const,
        min: 0,
        suggestedMax: 100,
        grid: { color: colors.grid, borderDash: [4, 4] },
        ticks: {
          color: colors.text,
          font: { size: 10 },
          callback: (value: string | number) => `${value}%`,
        },
      },
      y1: {
        type: 'linear' as const,
        display: true,
        position: 'right' as const,
        min: 0,
        grid: { display: false },
        ticks: { color: colors.load, font: { size: 10 } },
      },
    },
  }
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

function normalizeNumber(value?: number | null): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value)) return null
  return Number(value.toFixed(2))
}

function formatTrendLabel(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function recordTrendSnapshot(snapshot: OpsHostHealthSnapshot | null) {
  if (!snapshot?.available || !snapshot.collected_at) return
  trendSnapshots.value = [
    ...trendSnapshots.value.filter((item) => item.collected_at !== snapshot.collected_at),
    snapshot,
  ].slice(-MAX_TREND_POINTS)
}

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    const snapshot = await opsAPI.getHostHealth()
    health.value = snapshot
    recordTrendSnapshot(snapshot)
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

      <div v-if="trendChartData" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
        <h4 class="mb-2 text-xs font-semibold text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.hostHealth.cpuTrend') }}
        </h4>
        <div class="h-48 min-h-0">
          <Line :data="trendChartData" :options="trendChartOptions" />
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
