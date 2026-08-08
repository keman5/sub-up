<template>
  <footer class="relative z-10 border-t border-[var(--border-light)] bg-background py-10 md:py-12">
    <div class="container-main">
      <div class="mb-12 grid min-w-0 grid-cols-1 gap-8 sm:grid-cols-2 md:grid-cols-4">
        <div class="sm:col-span-2">
          <router-link to="/" class="font-display mb-4 block text-lg font-bold tracking-tight text-foreground">
            51token Pro
          </router-link>
          <p class="max-w-xs text-sm break-words text-muted-foreground">
            高性能 AI 接口分发基础设施。无缝兼容 OpenAI 接口规范，专注于 Codex Pro 账号额度的高效、安全、稳定分发。
          </p>
        </div>

        <div>
          <h4 class="mb-4 font-medium text-foreground">资源</h4>
          <ul class="space-y-2 text-sm text-muted-foreground">
            <li><a class="footer-link" href="#pricing">套餐价格</a></li>
            <li><a class="footer-link" href="#integrations">接入示例</a></li>
            <li><a class="footer-link" :href="externalAppUrls.console">系统状态</a></li>
          </ul>
        </div>

        <div>
          <h4 class="mb-4 font-medium text-foreground">入口</h4>
          <ul class="space-y-2 text-sm text-muted-foreground">
            <li><a class="footer-link" :href="externalAppUrls.login">登录</a></li>
            <li><a class="footer-link" :href="externalAppUrls.register">注册</a></li>
            <li><a class="footer-link" :href="externalAppUrls.console">控制台</a></li>
          </ul>
        </div>
      </div>

      <div class="flex flex-col items-center justify-between gap-4 border-t border-[var(--border)] pt-8 text-xs text-muted-foreground md:flex-row">
        <div class="hidden md:block" aria-hidden="true"></div>
        <div class="flex flex-col items-center gap-2 md:items-end">
          <div class="flex items-center gap-1.5 text-[11px] text-muted-foreground/55">
            <span class="live-indicator-dot scale-75 shadow-[0_0_6px_color-mix(in_oklch,var(--success)_35%,transparent)]"></span>
            所有系统运行正常
          </div>
          <div class="text-center text-xs text-muted-foreground/45 sm:text-right">
            <span>&copy; {{ currentYear }} <span class="font-medium text-foreground/70">51token 算力</span></span>
          </div>
        </div>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { externalAppUrls } from './homeData'

const currentYear = computed(() => new Date().getFullYear())
</script>
