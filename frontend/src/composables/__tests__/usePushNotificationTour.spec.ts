import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { Config, PopoverDOM } from 'driver.js'
import { useAuthStore } from '@/stores/auth'

const pushState = vi.hoisted(() => ({ value: 'disabled' as string }))
const driverMock = vi.hoisted(() => ({
  drive: vi.fn(),
  config: null as Config | null,
}))

vi.mock('@/services/pushNotifications', () => ({
  getPushPermissionState: vi.fn(async () => pushState.value),
  isPushSupported: () => true,
}))

vi.mock('driver.js', () => ({
  driver: (config: Config) => {
    driverMock.config = config
    return {
      drive: () => {
        driverMock.drive()
        config.onDestroyed?.()
      },
      isActive: () => false,
    }
  },
}))

import { guidePushIfDisabled } from '../usePushNotificationTour'

function mountToggle() {
  const toggle = document.createElement('div')
  toggle.setAttribute('data-tour', 'push-notification-toggle')
  toggle.getBoundingClientRect = () => ({
    width: 20,
    height: 20,
    top: 0,
    left: 0,
    right: 20,
    bottom: 20,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  }) as DOMRect
  document.body.appendChild(toggle)
  return toggle
}

function popoverStub() {
  const description = document.createElement('div')
  const progress = document.createElement('div')
  return { description, progress } as Pick<PopoverDOM, 'description' | 'progress'>
}

describe('guidePushIfDisabled', () => {
  let toggle: HTMLElement

  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    pushState.value = 'disabled'
    driverMock.drive.mockClear()
    driverMock.config = null
    toggle = mountToggle()
  })

  afterEach(() => {
    toggle.remove()
    localStorage.clear()
  })

  it('opens the notification tour with real sentences when notifications are off', async () => {
    useAuthStore().user = { id: 7 } as never
    await guidePushIfDisabled()
    expect(driverMock.drive).toHaveBeenCalledOnce()
    const title = driverMock.config?.steps?.[0]?.popover?.title
    expect(title === '开启浏览器通知' || title === 'Turn on browser notifications').toBe(true)
    const popover = popoverStub()
    driverMock.config?.onPopoverRender?.(popover as PopoverDOM, {} as never)
    expect(popover.description.textContent).toMatch(/不再提醒|Don't remind me again/)
  })

  it('skips the tour when notifications are already on', async () => {
    pushState.value = 'enabled'
    await guidePushIfDisabled()
    expect(driverMock.drive).not.toHaveBeenCalled()
  })

  it('does not open again after 不再提醒 is checked', async () => {
    useAuthStore().user = { id: 7 } as never
    await guidePushIfDisabled()
    const popover = popoverStub()
    driverMock.config?.onPopoverRender?.(popover as PopoverDOM, {} as never)
    const input = popover.description.querySelector('input') as HTMLInputElement
    input.checked = true
    input.dispatchEvent(new Event('change'))
    driverMock.drive.mockClear()
    await guidePushIfDisabled()
    expect(driverMock.drive).not.toHaveBeenCalled()
  })
})
