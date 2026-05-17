<template>
  <section id="pricing" class="relative scroll-mt-20 border-t border-[var(--border-light)] bg-background py-16 md:py-20">
    <div class="container-main relative z-10">
      <div class="mb-16 text-center">
        <h2 class="font-display mb-4 text-3xl font-bold tracking-tight text-foreground md:text-4xl">
          低成本解决算力瓶颈
        </h2>
        <p class="mx-auto max-w-2xl text-lg text-muted-foreground">
          无需每个人都订阅昂贵的 Pro 账号。极大降低研发测试与项目演示成本。
        </p>
      </div>

      <div class="mx-auto grid max-w-6xl gap-8 md:grid-cols-2 lg:grid-cols-3">
        <article
          v-for="(plan, index) in pricingPlans"
          :key="plan.name"
          class="relative flex flex-col rounded-3xl border p-8 landing-animate-fade-up"
          :class="plan.highlighted ? 'border-primary bg-muted shadow-[0_0_40px_color-mix(in_oklch,var(--primary)_15%,transparent)]' : 'border-[var(--border)] bg-background md:my-4'"
          :style="{ animationDelay: `${index * 80}ms` }"
        >
          <div
            v-if="plan.highlighted"
            class="absolute top-0 left-1/2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary px-3 py-1 text-xs font-bold tracking-wider text-primary-foreground uppercase shadow-lg"
          >
            最受欢迎
          </div>

          <h3 class="mb-2 text-xl font-bold text-foreground">{{ plan.name }}</h3>
          <p class="mb-6 min-h-10 text-sm text-muted-foreground">{{ plan.description }}</p>

          <div class="mb-8 flex items-end gap-2">
            <span class="text-2xl font-bold text-muted-foreground">￥</span>
            <span class="font-display text-5xl font-bold tracking-tight text-foreground">{{ plan.price }}</span>
            <span class="mb-1 text-muted-foreground">{{ plan.frequency }}</span>
          </div>

          <ul class="mb-8 flex-1 space-y-4">
            <li v-for="feature in plan.features" :key="feature" class="flex items-start gap-3 text-sm text-foreground/80">
              <Icon name="check" size="md" class="size-5 shrink-0 text-primary" />
              <span>{{ feature }}</span>
            </li>
          </ul>

          <a
            :href="targetHref"
            class="flex w-full items-center justify-center rounded-xl px-6 py-3 font-medium transition-colors"
            :class="plan.highlighted ? 'bg-primary text-primary-foreground shadow-lg shadow-primary/25 hover:bg-primary/90' : 'bg-secondary text-secondary-foreground hover:bg-secondary/80'"
          >
            {{ cachedAuthenticated ? 'Go to Dashboard' : '立即订阅' }}
          </a>
        </article>
      </div>

      <div class="mt-16 border-t border-[var(--border)] pt-12 landing-animate-fade-up">
        <div class="mb-10 text-center">
          <h3 class="font-display text-2xl font-bold tracking-tight text-foreground">
            灵活的短期用量包
          </h3>
          <p class="mt-2 text-muted-foreground">
            专为短期应急或小规模测试提供，即买即用，不绑定长期订阅。
          </p>
        </div>

        <div class="mx-auto grid max-w-5xl grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-6">
          <article
            v-for="(plan, index) in shortTermPlans"
            :key="plan.name"
            class="panel-card group flex min-h-64 flex-col items-center justify-center p-6 text-center transition-colors hover:border-primary/50 landing-animate-fade-up"
            :style="{ animationDelay: `${index * 40}ms` }"
          >
            <span class="text-sm font-medium text-muted-foreground transition-colors group-hover:text-foreground">
              {{ plan.name }}
            </span>
            <div class="mt-3 mb-2 flex items-baseline gap-1">
              <span class="text-sm text-muted-foreground">￥</span>
              <span class="font-display text-3xl font-bold text-foreground">{{ plan.price }}</span>
            </div>
            <div class="mb-4 text-xs">
              <div class="mb-1 font-semibold text-primary">{{ plan.tokens }}</div>
              <div class="leading-relaxed text-muted-foreground">{{ plan.equivalent }}</div>
            </div>
            <a
              :href="targetHref"
              class="mt-auto flex w-full items-center justify-center rounded-lg bg-secondary px-3 py-2 text-sm font-medium text-secondary-foreground transition-colors group-hover:bg-primary group-hover:text-primary-foreground"
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
