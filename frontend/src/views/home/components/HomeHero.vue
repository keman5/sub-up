<template>
  <section class="relative z-10 overflow-hidden bg-background pt-28 pb-14 md:pt-40 md:pb-20">
    <div
      aria-hidden="true"
      class="pointer-events-none absolute top-1/2 left-1/2 -z-10 h-[400px] w-[800px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/10 blur-[120px]"
    ></div>

    <div class="container-main relative z-10">
      <div class="grid items-center gap-12 lg:grid-cols-2 lg:gap-8">
        <div class="landing-animate-fade-up flex flex-col items-start text-left">
          <div class="mb-6 inline-flex items-center gap-2 rounded-full border border-[var(--border)] bg-background px-3 py-1 text-sm font-medium">
            <span class="live-indicator">
              <span class="live-indicator-ping"></span>
              <span class="live-indicator-dot"></span>
            </span>
            Codex Pro 资源共享现已就绪
          </div>

          <h1 class="font-display mb-6 text-4xl leading-tight font-bold tracking-tight md:text-5xl lg:text-6xl">
            极致高效的
            <br />
            <span class="text-gradient-main">AI 接口分发网关</span>
          </h1>

          <p class="mb-8 max-w-xl text-lg leading-relaxed text-muted-foreground">
            将昂贵的 Codex Pro 账号聚合为统一的接口。两行代码无缝接入，提供安全鉴权、并发控制与详尽的用量统计。释放创造力并且成倍降低研发成本。
          </p>

          <div class="flex flex-wrap gap-4">
            <a :href="primaryActionHref" class="home-btn-primary" @click="handlePrimaryActionClick">
              立即获取密钥
              <Icon name="arrowRight" size="sm" />
            </a>
            <a href="#pricing" class="home-btn-secondary" @click="scrollToPricing">
              <Icon name="terminal" size="sm" />
              查看套餐
            </a>
          </div>
        </div>

        <div class="landing-animate-scale-in relative w-full">
          <div
            aria-hidden="true"
            class="pointer-events-none absolute inset-0 -z-10 bg-gradient-to-tr from-primary/10 via-transparent to-cyan-500/10 blur-3xl"
          ></div>
          <div class="panel-card overflow-hidden bg-background/80 shadow-2xl backdrop-blur-xl">
            <div class="flex items-center justify-between border-b border-[var(--border)] bg-muted/30 px-4">
              <div class="flex gap-2 py-4">
                <span class="size-3 rounded-full border border-red-500/50 bg-red-500/20"></span>
                <span class="size-3 rounded-full border border-yellow-500/50 bg-yellow-500/20"></span>
                <span class="size-3 rounded-full border border-emerald-500/50 bg-emerald-500/20"></span>
              </div>
              <div class="no-scrollbar flex overflow-x-auto">
                <button
                  v-for="tab in heroTabs"
                  :key="tab.id"
                  class="border-b-2 px-3 py-3 font-mono text-xs whitespace-nowrap transition-colors sm:px-4"
                  :class="activeTab === tab.id ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'"
                  @click="activeTab = tab.id"
                >
                  {{ tab.label }}
                </button>
              </div>
            </div>
            <div class="flex min-h-[18rem] w-full items-center overflow-x-auto p-5 sm:p-6">
              <pre class="font-mono text-sm leading-relaxed break-all whitespace-pre-wrap"><code class="text-foreground/90">{{ activeSnippet }}</code></pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { externalAppUrls, heroSnippets, heroTabs, type HeroCodeTab } from './homeData'

const activeTab = ref<HeroCodeTab>('mac')
const cachedAuthenticated = ref(false)

const activeSnippet = computed(() => heroSnippets[activeTab.value])
const primaryActionHref = computed(() => (cachedAuthenticated.value ? '#pricing' : externalAppUrls.login))

function scrollToPricing(event: MouseEvent) {
  const pricing = document.getElementById('pricing')
  if (!pricing) return

  event.preventDefault()
  pricing.scrollIntoView({ behavior: 'smooth', block: 'start' })
  window.history.pushState(null, '', '#pricing')
}

function handlePrimaryActionClick(event: MouseEvent) {
  if (!cachedAuthenticated.value) return
  scrollToPricing(event)
}

if (typeof window !== 'undefined') {
  cachedAuthenticated.value = Boolean(localStorage.getItem('auth_token') && localStorage.getItem('auth_user'))
}
</script>
