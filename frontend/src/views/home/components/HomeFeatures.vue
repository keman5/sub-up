<template>
  <section class="relative overflow-hidden border-t border-[var(--border-light)] bg-background py-16 md:py-20">
    <div
      aria-hidden="true"
      class="pointer-events-none absolute top-0 right-0 size-[36rem] translate-x-1/3 -translate-y-1/3 rounded-full bg-primary/5 blur-3xl"
    ></div>

    <div class="container-main relative z-10">
      <div class="mb-16 text-center">
        <h2 class="font-display mb-4 text-3xl font-bold tracking-tight break-words text-foreground md:text-4xl">
          高可用 AI 服务的核心基础设施
        </h2>
        <p class="mx-auto max-w-2xl text-lg break-words text-muted-foreground">
          不止解决接口转发，更解决国内团队获取官方 AI 额度、稳定使用、权限隔离与成本核算的一整套问题。
        </p>
      </div>

      <div class="grid min-w-0 gap-6 md:grid-cols-2 lg:grid-cols-4">
        <article
          v-for="(feature, index) in featureCards"
          :key="feature.title"
          class="panel-card group min-w-0 p-6 transition-all duration-300 hover:-translate-y-1 hover:shadow-lg"
          :class="'landing-animate-fade-up'"
          :style="{ animationDelay: `${index * 50}ms` }"
        >
          <div class="mb-6 flex size-12 shrink-0 items-center justify-center rounded-xl border border-[var(--border)] bg-background shadow-inner transition-all duration-300 group-hover:border-primary/50 group-hover:shadow-[0_0_15px_color-mix(in_oklch,var(--primary)_20%,transparent)]">
            <Icon :name="feature.icon" size="lg" class="text-muted-foreground transition-colors group-hover:text-primary" />
          </div>
          <h3 class="font-display mb-2 text-lg font-bold break-words text-foreground">
            {{ feature.title }}
          </h3>
          <p class="text-sm leading-relaxed font-medium break-words text-muted-foreground">
            {{ feature.description }}
          </p>
        </article>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import { featureCards } from './homeData'
</script>
