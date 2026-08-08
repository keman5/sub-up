import { computed, ref } from 'vue'

export type ThemePreference = 'system' | 'light' | 'dark'
export type EffectiveTheme = 'light' | 'dark'

const THEME_STORAGE_KEY = 'theme'
const SYSTEM_DARK_QUERY = '(prefers-color-scheme: dark)'
const THEME_PREFERENCES: ThemePreference[] = ['system', 'light', 'dark']

const preference = ref<ThemePreference>('system')
const systemPrefersDark = ref(false)

let mediaQuery: MediaQueryList | null = null
let mediaQueryListening = false

function isThemePreference(value: string | null): value is ThemePreference {
  return THEME_PREFERENCES.includes(value as ThemePreference)
}

function canUseBrowserApis(): boolean {
  return typeof window !== 'undefined' && typeof document !== 'undefined'
}

function readStoredPreference(): ThemePreference {
  if (!canUseBrowserApis()) return 'system'

  const storedPreference = localStorage.getItem(THEME_STORAGE_KEY)
  if (isThemePreference(storedPreference)) {
    return storedPreference
  }

  if (storedPreference !== null) {
    localStorage.setItem(THEME_STORAGE_KEY, 'system')
  }
  return 'system'
}

function getSystemPrefersDark(): boolean {
  if (!canUseBrowserApis() || typeof window.matchMedia !== 'function') {
    return false
  }
  return window.matchMedia(SYSTEM_DARK_QUERY).matches
}

function getEffectiveTheme(themePreference: ThemePreference): EffectiveTheme {
  if (themePreference === 'dark') return 'dark'
  if (themePreference === 'light') return 'light'
  return systemPrefersDark.value ? 'dark' : 'light'
}

function persistPreference(themePreference: ThemePreference): void {
  if (!canUseBrowserApis()) return
  localStorage.setItem(THEME_STORAGE_KEY, themePreference)
}

function applyThemeClass(): void {
  if (!canUseBrowserApis()) return

  const effectiveTheme = getEffectiveTheme(preference.value)
  document.documentElement.classList.toggle('dark', effectiveTheme === 'dark')
  document.documentElement.style.colorScheme = effectiveTheme
}

function handleSystemThemeChange(event: MediaQueryListEvent): void {
  systemPrefersDark.value = event.matches
  if (preference.value === 'system') {
    applyThemeClass()
  }
}

function ensureSystemThemeListener(): void {
  if (!canUseBrowserApis() || typeof window.matchMedia !== 'function') return

  mediaQuery = mediaQuery ?? window.matchMedia(SYSTEM_DARK_QUERY)
  systemPrefersDark.value = mediaQuery.matches

  if (mediaQueryListening) return

  if (typeof mediaQuery.addEventListener === 'function') {
    mediaQuery.addEventListener('change', handleSystemThemeChange)
  } else if (typeof mediaQuery.addListener === 'function') {
    mediaQuery.addListener(handleSystemThemeChange)
  }
  mediaQueryListening = true
}

export function initThemeClass(): void {
  preference.value = readStoredPreference()
  systemPrefersDark.value = getSystemPrefersDark()
  ensureSystemThemeListener()
  applyThemeClass()
}

export function useTheme() {
  initThemeClass()

  const effectiveTheme = computed<EffectiveTheme>(() => getEffectiveTheme(preference.value))

  function setThemePreference(nextPreference: ThemePreference): void {
    preference.value = isThemePreference(nextPreference) ? nextPreference : 'system'
    persistPreference(preference.value)
    applyThemeClass()
  }

  return {
    preference,
    effectiveTheme,
    preferences: THEME_PREFERENCES,
    setThemePreference
  }
}
