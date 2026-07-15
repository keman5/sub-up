import { onMounted, onUnmounted } from 'vue'

const HOME_SCROLL_KEY_PREFIX = '51token:home-scroll:'

function getCurrentScrollKey() {
  return `${HOME_SCROLL_KEY_PREFIX}${window.location.pathname}${window.location.search}`
}

function shouldRestoreScroll() {
  const navigation = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined
  return navigation?.type === 'reload' || navigation?.type === 'back_forward'
}

function readStoredScrollTop(key: string) {
  try {
    const value = Number(window.sessionStorage.getItem(key))
    if (Number.isFinite(value) && value > 0) return value
  } catch (_) {
    // Storage can be unavailable in private browsing or restricted frames.
  }

  try {
    const value = Number(window.localStorage.getItem(key))
    if (Number.isFinite(value) && value > 0) return value
  } catch (_) {
    // Storage can be unavailable in private browsing or restricted frames.
  }

  const homeScroll = window.history.state?.homeScroll
  const stateValue = Number(homeScroll?.[key])
  return Number.isFinite(stateValue) && stateValue > 0 ? stateValue : null
}

function writeStoredScrollTop(key: string, top: number) {
  try {
    window.sessionStorage.setItem(key, String(top))
  } catch (_) {
    // Storage can be unavailable in private browsing or restricted frames.
  }

  try {
    window.localStorage.setItem(key, String(top))
  } catch (_) {
    // History state may be unavailable in restricted browsing contexts.
  }

  try {
    const currentState = window.history.state && typeof window.history.state === 'object' ? window.history.state : {}
    const currentHomeScroll =
      currentState.homeScroll && typeof currentState.homeScroll === 'object' ? currentState.homeScroll : {}

    window.history.replaceState(
      {
        ...currentState,
        homeScroll: {
          ...currentHomeScroll,
          [key]: top
        }
      },
      document.title,
      window.location.href
    )
  } catch (_) {
    // History state may be unavailable in restricted browsing contexts.
  }
}

function withInstantScroll(callback: () => void) {
  const root = document.documentElement
  const previousScrollBehavior = root.style.scrollBehavior

  root.style.scrollBehavior = 'auto'
  callback()
  root.style.scrollBehavior = previousScrollBehavior
}

function getHashTarget() {
  if (!window.location.hash) return null

  try {
    return decodeURIComponent(window.location.hash.slice(1))
  } catch (_) {
    return window.location.hash.slice(1)
  }
}

export function useHomeScrollRestoration(enabled: boolean) {
  let saveFrameId = 0
  let restoreFrameId = 0
  let restoreTimeoutId = 0
  let key = ''
  let unloading = false

  const saveScrollPosition = () => {
    if (!enabled || !key || unloading) return
    writeStoredScrollTop(key, Math.round(window.scrollY))
  }

  const saveScrollPositionNow = () => {
    if (!enabled || !key) return
    writeStoredScrollTop(key, Math.round(window.scrollY))
  }

  const handleScroll = () => {
    if (saveFrameId) return

    saveFrameId = window.requestAnimationFrame(() => {
      saveFrameId = 0
      saveScrollPosition()
    })
  }

  const revealHome = () => {
    document.documentElement.classList.remove('home-scroll-restoring')
  }

  const restoreHashPosition = () => {
    const targetId = getHashTarget()
    if (!targetId) return false

    let attempts = 0

    const restore = () => {
      attempts += 1
      const target = document.getElementById(targetId)

      if (target) {
        withInstantScroll(() => {
          target.scrollIntoView({ behavior: 'auto', block: 'start' })
        })
        restoreFrameId = window.requestAnimationFrame(revealHome)
        return
      }

      if (attempts >= 30) {
        revealHome()
        return
      }

      restoreTimeoutId = window.setTimeout(restore, 50)
    }

    restoreFrameId = window.requestAnimationFrame(restore)
    return true
  }

  const restoreScrollPosition = () => {
    if ('scrollRestoration' in window.history) {
      window.history.scrollRestoration = 'manual'
    }

    if (enabled && restoreHashPosition()) {
      return
    }

    const isProtectedRestore = document.documentElement.classList.contains('home-scroll-restoring')

    if (!enabled || (!isProtectedRestore && !shouldRestoreScroll())) {
      revealHome()
      return
    }

    const targetTop = readStoredScrollTop(getCurrentScrollKey())
    if (targetTop === null) {
      revealHome()
      return
    }

    let attempts = 0

    const restore = () => {
      attempts += 1
      const maxTop = Math.max(0, document.documentElement.scrollHeight - window.innerHeight)

      if (maxTop >= targetTop || attempts >= 30) {
        const restoredTop = Math.min(targetTop, maxTop)

        withInstantScroll(() => {
          window.scrollTo({
            top: restoredTop,
            left: 0,
            behavior: 'auto'
          })
        })
        restoreFrameId = window.requestAnimationFrame(() => {
          withInstantScroll(() => {
            window.scrollTo(0, restoredTop)
          })
          restoreFrameId = window.requestAnimationFrame(revealHome)
        })
        return
      }

      restoreTimeoutId = window.setTimeout(restore, 50)
    }

    restoreFrameId = window.requestAnimationFrame(restore)
  }

  const handlePageExit = () => {
    if (!enabled || !key) return

    saveScrollPositionNow()
    unloading = true

    if ('scrollRestoration' in window.history) {
      window.history.scrollRestoration = 'manual'
    }

    window.removeEventListener('scroll', handleScroll)
    document.documentElement.classList.add('home-scroll-restoring')
    withInstantScroll(() => {
      window.scrollTo(0, 0)
    })
  }

  onMounted(() => {
    if (!enabled || typeof window === 'undefined') return

    key = getCurrentScrollKey()
    restoreScrollPosition()
    window.addEventListener('scroll', handleScroll, { passive: true })
    window.addEventListener('hashchange', restoreScrollPosition)
    window.addEventListener('pagehide', handlePageExit)
    window.addEventListener('beforeunload', handlePageExit)
  })

  onUnmounted(() => {
    revealHome()
    if (saveFrameId) window.cancelAnimationFrame(saveFrameId)
    if (restoreFrameId) window.cancelAnimationFrame(restoreFrameId)
    if (restoreTimeoutId) window.clearTimeout(restoreTimeoutId)
    window.removeEventListener('scroll', handleScroll)
    window.removeEventListener('hashchange', restoreScrollPosition)
    window.removeEventListener('pagehide', handlePageExit)
    window.removeEventListener('beforeunload', handlePageExit)
  })
}
