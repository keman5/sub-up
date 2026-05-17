<template>
  <div class="home-page relative min-h-screen overflow-x-clip bg-background text-foreground">
    <PublicHeader />
    <HomeHero />
    <HomeStats />
    <HomePricing />
    <HomeFeatures />
    <HomeIntegrations />
    <HomeFaq />
    <HomeFooter />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import PublicHeader from './home/components/PublicHeader.vue'
import HomeHero from './home/components/HomeHero.vue'
import HomeStats from './home/components/HomeStats.vue'
import HomeFeatures from './home/components/HomeFeatures.vue'
import HomeIntegrations from './home/components/HomeIntegrations.vue'
import HomePricing from './home/components/HomePricing.vue'
import HomeFaq from './home/components/HomeFaq.vue'
import HomeFooter from './home/components/HomeFooter.vue'
import { useHomeScrollRestoration } from './home/components/useHomeScrollRestoration'

useHomeScrollRestoration(true)

onMounted(() => {
  document.title = '51token 算力'
})
</script>
