<script setup lang="ts">
import { RouterView, useRouter, useRoute } from 'vue-router'
import { onBeforeUnmount, watch } from 'vue'
import Toast from '@/components/common/Toast.vue'
import NavigationProgress from '@/components/common/NavigationProgress.vue'
import { resolveDocumentTitle } from '@/router/title'
import AnnouncementPopup from '@/components/common/AnnouncementPopup.vue'
import { useAppStore, useAuthStore, useSubscriptionStore, useAnnouncementStore } from '@/stores'
import { getSetupStatus } from '@/api/setup'
import { applySiteIcons } from '@/utils/siteIcons'

const router = useRouter()
const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const subscriptionStore = useSubscriptionStore()
const announcementStore = useAnnouncementStore()
let routeSetupCheckSeq = 0

// Watch for site settings changes and update favicon/title
watch(
  () => appStore.siteLogo,
  (newLogo) => {
    if (newLogo) {
      applySiteIcons(newLogo)
    }
  },
  { immediate: true }
)

// Watch for authentication state and manage subscription data + announcements
function onVisibilityChange() {
  if (document.visibilityState === 'visible' && authStore.isAuthenticated) {
    announcementStore.fetchAnnouncements()
  }
}

watch(
  () => authStore.isAuthenticated,
  (isAuthenticated, oldValue) => {
    if (isAuthenticated) {
      // User logged in: preload subscriptions and start polling
      subscriptionStore.fetchActiveSubscriptions().catch((error) => {
        console.error('Failed to preload subscriptions:', error)
      })
      subscriptionStore.startPolling()

      // Announcements: new login vs page refresh restore
      if (oldValue === false) {
        // New login: delay 3s then force fetch
        setTimeout(() => announcementStore.fetchAnnouncements(true), 3000)
      } else {
        // Page refresh restore (oldValue was undefined)
        announcementStore.fetchAnnouncements()
      }

      // Register visibility change listener
      document.addEventListener('visibilitychange', onVisibilityChange)
    } else {
      // User logged out: clear data and stop polling
      subscriptionStore.clear()
      announcementStore.reset()
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
  },
  { immediate: true }
)

// Route change trigger (throttled by store)
router.afterEach(() => {
  if (authStore.isAuthenticated) {
    announcementStore.fetchAnnouncements()
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onVisibilityChange)
})

async function initializeRouteEnvironment() {
  const seq = ++routeSetupCheckSeq
  const isStaticHome = route.path === '/' || route.path === '/home'
  if (isStaticHome) {
    await appStore.fetchPublicSettings()
    if (seq !== routeSetupCheckSeq) return
    document.title = '51token 算力'
    applySiteIcons(appStore.siteLogo || '/logo.png')
    return
  }

  // Check if setup is needed
  try {
    const status = await getSetupStatus()
    if (seq !== routeSetupCheckSeq) return
    if (status.needs_setup && route.path !== '/setup') {
      router.replace('/setup')
      return
    }
    if (status.needs_setup) {
      return
    }
  } catch {
    // If setup endpoint fails, assume normal mode and continue
  }

  // Load public settings into appStore (will be cached for other components)
  await appStore.fetchPublicSettings()
  if (seq !== routeSetupCheckSeq) return

  // Re-resolve document title now that siteName is available
  document.title = resolveDocumentTitle(route.meta.title, appStore.siteName, route.meta.titleKey as string)
}

watch(
  () => route.fullPath,
  () => {
    void initializeRouteEnvironment()
  },
  { immediate: true }
)
</script>

<template>
  <NavigationProgress />
  <RouterView />
  <Toast />
  <AnnouncementPopup />
</template>
