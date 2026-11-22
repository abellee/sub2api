import { computed, onScopeDispose, readonly, ref, watch } from 'vue'

export type ThemeChoice = 'dark' | 'light' | 'system'
const themeEvent = 'tokenfree:theme-change'

function isTheme(value: unknown): value is ThemeChoice {
  return value === 'dark' || value === 'light' || value === 'system'
}

export function readTheme(): ThemeChoice {
  try {
    const saved = localStorage.getItem('theme')
    if (isTheme(saved)) return saved
    const legacy = localStorage.getItem('tokenfree-home-theme')
    if (isTheme(legacy)) return legacy
  } catch { /* Storage can be unavailable in private contexts. */ }
  return 'dark'
}

export function initThemeClass() {
  const theme = readTheme()
  const dark = theme === 'system'
    ? !window.matchMedia('(prefers-color-scheme: light)').matches
    : theme === 'dark'
  document.documentElement.classList.toggle('dark', dark)
}

export function useTheme() {
  const theme = ref<ThemeChoice>(readTheme())
  const media = window.matchMedia('(prefers-color-scheme: light)')
  const systemLight = ref(media.matches)
  const resolvedTheme = computed(() => theme.value === 'system'
    ? (systemLight.value ? 'light' : 'dark')
    : theme.value)
  const isDark = computed(() => resolvedTheme.value === 'dark')

  watch(isDark, dark => document.documentElement.classList.toggle('dark', dark), { immediate: true, flush: 'sync' })

  function setTheme(value: ThemeChoice) {
    theme.value = value
    try {
      localStorage.setItem('theme', value)
      localStorage.removeItem('tokenfree-home-theme')
    } catch { /* Keep the current selection even when storage is unavailable. */ }
    window.dispatchEvent(new CustomEvent(themeEvent, { detail: value }))
  }

  function syncTheme(event: Event) {
    const value = (event as CustomEvent).detail
    if (isTheme(value)) theme.value = value
  }
  function syncStorage(event: StorageEvent) {
    if (event.key === 'theme' || event.key === null) theme.value = readTheme()
  }
  function syncSystem(event: MediaQueryListEvent) {
    systemLight.value = event.matches
  }

  window.addEventListener(themeEvent, syncTheme)
  window.addEventListener('storage', syncStorage)
  media.addEventListener('change', syncSystem)
  onScopeDispose(() => {
    window.removeEventListener(themeEvent, syncTheme)
    window.removeEventListener('storage', syncStorage)
    media.removeEventListener('change', syncSystem)
  })

  return { theme: readonly(theme), resolvedTheme, isDark, setTheme }
}
