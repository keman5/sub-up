<template>
  <header class="glass-nav">
    <div class="container-main">
      <nav class="flex h-16 items-center justify-between">
        <router-link to="/" class="group flex shrink-0 items-center gap-2">
          <SiteLogo class-name="size-8" />
          <span class="font-display text-xl font-bold tracking-tight">
            51token
          </span>
        </router-link>

        <div class="hidden items-center gap-0.5 sm:flex">
          <router-link to="/" class="home-nav-link home-nav-link-active">主页</router-link>
          <a :href="externalAppUrls.console" class="home-nav-link">控制台</a>
          <div class="mx-2 h-4 w-px bg-[var(--border)]"></div>
          <ThemeSwitcher icon-only />
        </div>

        <div class="flex items-center gap-2 sm:hidden">
          <ThemeSwitcher icon-only />
          <button class="home-icon-button" aria-label="打开菜单" @click="mobileOpen = true">
            <Icon name="menu" size="md" />
          </button>
        </div>
      </nav>
    </div>

    <div v-if="mobileOpen" class="fixed inset-0 z-[60] bg-background/95 backdrop-blur-xl sm:hidden">
      <div class="container-main flex h-16 items-center justify-between">
        <router-link to="/" class="flex items-center gap-2" @click="mobileOpen = false">
          <SiteLogo class-name="size-8" />
          <span class="font-display text-xl font-bold tracking-tight">51token</span>
        </router-link>
        <button class="home-icon-button" aria-label="关闭菜单" @click="mobileOpen = false">
          <Icon name="x" size="md" />
        </button>
      </div>
      <div class="container-main flex flex-col gap-2 py-8">
        <router-link to="/" class="home-mobile-link" @click="mobileOpen = false">主页</router-link>
        <a :href="externalAppUrls.console" class="home-mobile-link" @click="mobileOpen = false">控制台</a>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import ThemeSwitcher from '@/components/common/ThemeSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import SiteLogo from './SiteLogo.vue'
import { externalAppUrls } from './homeData'

const mobileOpen = ref(false)

watch(mobileOpen, (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
})

onUnmounted(() => {
  document.body.style.overflow = ''
})
</script>
