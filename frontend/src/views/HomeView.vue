<template>
  <div class="home-page relative min-h-screen overflow-x-clip bg-background text-foreground">
    <PublicHeader />
    <HomeHero :api-base-url="apiBaseUrl" />
    <HomeStats />
    <HomePricing />
    <HomeFeatures />
    <HomeIntegrations :api-base-url="apiBaseUrl" />
    <HomeFaq />
    <HomeFooter />
    <HomeSupportWidget />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import PublicHeader from './home/components/PublicHeader.vue'
import HomeHero from './home/components/HomeHero.vue'
import HomeStats from './home/components/HomeStats.vue'
import HomeFeatures from './home/components/HomeFeatures.vue'
import HomeIntegrations from './home/components/HomeIntegrations.vue'
import HomePricing from './home/components/HomePricing.vue'
import HomeFaq from './home/components/HomeFaq.vue'
import HomeFooter from './home/components/HomeFooter.vue'
import HomeSupportWidget from './home/components/HomeSupportWidget.vue'
import { useHomeScrollRestoration } from './home/components/useHomeScrollRestoration'
import { useAppStore } from '@/stores/app'

useHomeScrollRestoration(true)

const appStore = useAppStore()
const apiBaseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || appStore.apiBaseUrl || '')

onMounted(() => {
  document.title = '51token 算力'
  void appStore.fetchPublicSettings()
})
</script>
