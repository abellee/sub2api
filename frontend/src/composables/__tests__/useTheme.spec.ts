import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, type EffectScope } from 'vue'
import { initThemeClass, useTheme } from '../useTheme'

const scopes: EffectScope[] = []
const systemListeners = new Set<(event: MediaQueryListEvent) => void>()
let systemLight = true

function consumer() {
  const scope = effectScope()
  scopes.push(scope)
  return scope.run(useTheme)!
}

beforeEach(() => {
  localStorage.clear()
  systemLight = true
  vi.spyOn(window, 'matchMedia').mockImplementation(() => ({
    matches: systemLight,
    addEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) => systemListeners.add(listener),
    removeEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) => systemListeners.delete(listener),
  }) as unknown as MediaQueryList)
})

afterEach(() => {
  scopes.splice(0).forEach(scope => scope.stop())
  systemListeners.clear()
  vi.restoreAllMocks()
})

describe('shared public and console theme', () => {
  it('defaults to dark and gives the console preference priority over a stale homepage preference', () => {
    initThemeClass()
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    localStorage.setItem('tokenfree-home-theme', 'dark')
    localStorage.setItem('theme', 'light')
    expect(consumer().resolvedTheme.value).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('shares changes immediately and preserves them across route remounts', () => {
    localStorage.setItem('tokenfree-home-theme', 'light')
    const home = consumer()
    const consoleTheme = consumer()
    consoleTheme.setTheme('dark')
    expect(home.resolvedTheme.value).toBe('dark')
    expect(localStorage.getItem('tokenfree-home-theme')).toBeNull()
    expect(consumer().resolvedTheme.value).toBe('dark')
    home.setTheme('light')
    expect(consoleTheme.isDark.value).toBe(false)
    expect(localStorage.getItem('theme')).toBe('light')
  })

  it('follows system changes only in system mode and cleans up subscribers', () => {
    const theme = consumer()
    theme.setTheme('system')
    expect(localStorage.getItem('theme')).toBe('system')
    expect(theme.resolvedTheme.value).toBe('light')
    systemListeners.forEach(listener => listener({ matches: false } as MediaQueryListEvent))
    expect(theme.resolvedTheme.value).toBe('dark')
    theme.setTheme('light')
    systemListeners.forEach(listener => listener({ matches: false } as MediaQueryListEvent))
    expect(theme.resolvedTheme.value).toBe('light')
    scopes.forEach(scope => scope.stop())
    expect(systemListeners.size).toBe(0)
  })

  it('synchronizes changes from another tab', () => {
    const theme = consumer()
    localStorage.setItem('theme', 'light')
    window.dispatchEvent(new StorageEvent('storage', { key: 'theme', newValue: 'light' }))
    expect(theme.resolvedTheme.value).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })
})
