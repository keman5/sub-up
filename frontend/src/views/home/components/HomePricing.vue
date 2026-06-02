<template>
  <section id="pricing" class="relative scroll-mt-20 border-[var(--border-light)] bg-background py-10 md:py-20">
    <div class="container-main relative z-10">
      <div class="mb-8 text-center md:mb-16">
        <h2 class="font-display mb-3 text-2xl font-bold tracking-tight break-words text-foreground md:mb-4 md:text-4xl">
          低成本解决算力瓶颈
        </h2>
        <p class="mx-auto max-w-2xl text-base leading-relaxed break-words text-muted-foreground md:text-lg">
          无需每个人都订阅昂贵的 Pro 账号。极大降低研发测试与项目演示成本。
        </p>
      </div>

      <div class="mx-auto grid max-w-6xl min-w-0 gap-4 md:grid-cols-2 md:gap-8 lg:grid-cols-3">
        <article
          v-for="(plan, index) in pricingPlans"
          :key="plan.name"
          class="relative flex min-w-0 flex-col rounded-xl border p-4 landing-animate-fade-up sm:p-8 md:rounded-3xl"
          :class="plan.highlighted ? 'border-primary bg-muted shadow-[0_0_40px_color-mix(in_oklch,var(--primary)_15%,transparent)]' : 'border-[var(--border)] bg-background md:my-4'"
          :style="{ animationDelay: `${index * 80}ms` }"
        >
          <div
            v-if="plan.highlighted"
            class="absolute top-0 left-1/2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary px-2.5 py-0.5 text-[10px] font-bold tracking-wider text-primary-foreground uppercase shadow-lg md:px-3 md:py-1 md:text-xs"
          >
            最受欢迎
          </div>

          <h3 class="mb-1 text-base font-bold break-words text-foreground md:mb-2 md:text-xl">{{ plan.name }}</h3>
          <p class="mb-3 text-xs leading-relaxed break-words text-muted-foreground md:mb-6 md:min-h-10 md:text-sm">{{ plan.description }}</p>

          <div class="mb-3 flex flex-wrap items-end gap-1.5 md:mb-8 md:gap-2">
            <span class="text-lg font-bold text-muted-foreground md:text-2xl">￥</span>
            <span class="font-display text-3xl font-bold tracking-tight text-foreground md:text-5xl">{{ plan.price }}</span>
            <span class="mb-0.5 text-sm text-muted-foreground md:mb-1 md:text-base">{{ plan.frequency }}</span>
          </div>

          <ul class="mb-4 flex-1 space-y-1.5 md:mb-8 md:space-y-4">
            <li v-for="feature in plan.features" :key="feature" class="flex min-w-0 items-start gap-2 text-xs leading-snug text-foreground/80 md:gap-3 md:text-sm md:leading-relaxed">
              <Icon name="check" size="md" class="mt-0.5 size-3.5 shrink-0 text-primary md:size-5" />
              <span class="min-w-0 break-words">{{ feature }}</span>
            </li>
          </ul>

          <a
            :href="targetHref"
            class="flex w-full items-center justify-center rounded-lg px-4 py-2 text-sm font-medium transition-colors md:rounded-xl md:px-6 md:py-3 md:text-base"
            :class="plan.highlighted ? 'bg-primary text-primary-foreground shadow-lg shadow-primary/25 hover:bg-primary/90' : 'bg-secondary text-secondary-foreground hover:bg-secondary/80'"
          >
            立即订阅
          </a>
        </article>
      </div>

      <div class="mt-10 border-t border-[var(--border)] pt-7 landing-animate-fade-up md:mt-16 md:pt-12">
        <div class="mb-5 text-center md:mb-10">
          <h3 class="font-display text-xl font-bold tracking-tight text-foreground md:text-2xl">
            灵活的短期用量包
          </h3>
          <p class="mt-2 text-sm leading-relaxed break-words text-muted-foreground md:text-base">
            专为短期应急或小规模测试提供，即买即用，不绑定长期订阅。
          </p>
        </div>

        <div class="mx-auto grid max-w-5xl min-w-0 grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-6">
          <article
            v-for="(plan, index) in shortTermPlans"
            :key="plan.name"
            class="panel-card group flex min-h-36 min-w-0 flex-col items-center justify-center p-3.5 text-center transition-colors hover:border-primary/50 landing-animate-fade-up sm:min-h-56 sm:p-5 lg:min-h-64 lg:p-6"
            :style="{ animationDelay: `${index * 40}ms` }"
          >
            <span class="max-w-full text-sm font-medium break-words text-muted-foreground transition-colors group-hover:text-foreground">
              {{ plan.name }}
            </span>
            <div class="mt-1.5 mb-1 flex items-baseline gap-1 md:mt-3 md:mb-2">
              <span class="text-sm text-muted-foreground">￥</span>
              <span class="font-display text-xl font-bold text-foreground md:text-3xl">{{ plan.price }}</span>
            </div>
            <div class="mb-2.5 min-w-0 text-xs md:mb-4">
              <div class="mb-1 font-semibold text-primary">{{ plan.tokens }}</div>
              <div class="leading-relaxed break-words text-muted-foreground">{{ plan.equivalent }}</div>
            </div>
            <a
              :href="targetHref"
              class="mt-auto inline-flex items-center justify-center rounded-lg bg-secondary px-4 py-1.5 text-sm font-medium text-secondary-foreground transition-colors group-hover:bg-primary group-hover:text-primary-foreground md:px-3 md:py-2"
            >
              快速购买
            </a>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { externalAppUrls, pricingPlans, shortTermPlans } from './homeData'

const cachedAuthenticated = ref(false)
const targetHref = computed(() => (cachedAuthenticated.value ? externalAppUrls.console : externalAppUrls.register))

if (typeof window !== 'undefined') {
  cachedAuthenticated.value = Boolean(localStorage.getItem('auth_token') && localStorage.getItem('auth_user'))
}
</script>
