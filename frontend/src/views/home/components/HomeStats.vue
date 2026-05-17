<template>
  <section class="relative z-10 overflow-hidden border-y border-[var(--border-light)] bg-background py-10 md:py-12">
    <div class="pointer-events-none absolute inset-0 home-cube-pattern opacity-40"></div>
    <div class="container-main relative z-10">
      <div class="grid w-full grid-cols-2 gap-y-8 md:grid-cols-5 md:gap-y-0">
        <div
          v-for="(stat, index) in stats"
          :key="stat.label"
          class="landing-animate-fade-up flex w-full min-w-0 flex-col items-center justify-center overflow-hidden px-4 text-center md:border-l md:border-[var(--border)] md:px-6 md:first:border-l-0"
          :style="{ animationDelay: `${index * 100}ms` }"
        >
          <span class="font-display inline-flex max-w-full items-baseline justify-center overflow-hidden text-4xl font-bold tracking-tight whitespace-nowrap text-foreground md:text-5xl">
            <span v-if="stat.value">{{ stat.value }}</span>
            <AnimatedNumber
              v-else
              :end="stat.end ?? 0"
              :decimals="stat.decimals"
              :formatter="stat.formatter"
            />
            <span v-if="stat.unit" class="ml-1 text-[0.55em] font-medium tracking-normal text-muted-foreground">
              {{ stat.unit }}
            </span>
          </span>
          <span class="mt-2 block text-sm font-medium text-muted-foreground md:text-base">
            {{ stat.label }}
          </span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import AnimatedNumber from './AnimatedNumber.vue'

interface StatItem {
  end?: number
  unit?: string
  label: string
  decimals?: number
  formatter?: (value: number) => string
  value?: string
}

const stats: StatItem[] = [
  { end: 99.9, unit: '%', label: '服务可用性', decimals: 1 },
  {
    end: 120_000_000,
    unit: '亿次',
    label: '累计处理请求',
    formatter: (value) => (value / 100_000_000).toFixed(1)
  },
  { value: 'ChatGPT', label: '专业稳定 Codex' },
  { end: 24, unit: '/7', label: '全天候资源调度' },
  { end: 365, unit: '天+', label: '已稳定运行时长' }
]
</script>
