import { driver, type DriveStep, type PopoverDOM } from 'driver.js'
import 'driver.js/dist/driver.css'
import { getLocale, i18n, loadLocaleMessages } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingStore } from '@/stores/onboarding'
import { getPushPermissionState, isPushSupported, type PushPermissionState } from '@/services/pushNotifications'

export interface PushTourCopy {
  title: string
  description: string
}

export interface PushTourButtons {
  next: string
  previous: string
  done: string
}

export interface PushTourMute {
  label: string
  storageKey: string
}

export interface PushTourOptions {
  notification?: PushTourCopy
  qq?: PushTourCopy
  buttons: PushTourButtons
  mute?: PushTourMute
}

export const OPEN_USER_MENU_EVENT = 'open-user-menu'
export const PUSH_TOUR_UNLOCK_EVENT = 'push-tour-unlock'
export const OPEN_QQ_GROUP_EVENT = 'open-qq-group'
export const CLOSE_QQ_GROUP_EVENT = 'close-qq-group'
const TOUR_SELECTOR = '[data-tour="push-notification-toggle"]'
const QQ_SELECTOR = '[data-tour="join-qq-group"]'

export function pushNotifyTourStorageKey(userId: number | string) {
  return `push_notify_tour_v3_${userId}`
}

export function hasSeenPushNotifyTour(userId: number | string) {
  return localStorage.getItem(pushNotifyTourStorageKey(userId)) === '1'
}

export function markPushNotifyTourSeen(userId: number | string) {
  localStorage.setItem(pushNotifyTourStorageKey(userId), '1')
}

export function joinPushGuideMuteKey(userId: number | string) {
  return `push_notify_join_mute_${userId}`
}

export function isJoinPushGuideMuted(userId: number | string) {
  return localStorage.getItem(joinPushGuideMuteKey(userId)) === '1'
}

const TOUR_FALLBACK = {
  zh: {
    title: '开启浏览器通知',
    enter: '打开右上角菜单里的通知开关，即可接收抽奖开始、开奖和任务奖励提醒。',
    join: '你还没有开启通知。打开这里的开关后，活动开始、开奖和中奖时会提醒你。',
    mute: '不再提醒',
    qqTitle: '加入 QQ 群',
    qq: '加入 QQ 群可以及时获取最新消息。',
    next: '下一步',
    previous: '返回',
    done: '确认',
  },
  en: {
    title: 'Turn on browser notifications',
    enter: 'Use the switch in the account menu to get reminders when a lottery starts, when it draws, and when a task reward arrives.',
    join: 'Notifications are off. Turn on this switch to get reminders when a lottery starts, draws, or you win.',
    mute: "Don't remind me again",
    qqTitle: 'Join the QQ group',
    qq: 'Join the QQ group to get the latest news in time.',
    next: 'Next',
    previous: 'Back',
    done: 'Confirm',
  },
} as const

function activeTourLocale(): 'zh' | 'en' {
  const saved = localStorage.getItem('sub2api_locale')
  if (saved === 'zh' || saved === 'en') return saved
  return getLocale() === 'zh' ? 'zh' : 'en'
}

function tourPhrase(key: string, fallback: string) {
  if (!i18n.global.te(key)) return fallback
  const translated = String(i18n.global.t(key))
  return translated && translated !== key ? translated : fallback
}

/** Loads the active language, then returns sentences instead of raw message keys. */
export async function pushTourMessages() {
  const locale = activeTourLocale()
  await loadLocaleMessages(locale)
  if (locale !== 'en') await loadLocaleMessages('en')
  if (i18n.global.locale.value !== locale) i18n.global.locale.value = locale
  const pack = TOUR_FALLBACK[locale]
  return {
    title: tourPhrase('pushNotifications.tour.title', pack.title),
    enter: tourPhrase('pushNotifications.tour.enter', pack.enter),
    join: tourPhrase('pushNotifications.tour.join', pack.join),
    mute: tourPhrase('pushNotifications.tour.mute', pack.mute),
    qqTitle: tourPhrase('pushNotifications.tour.qqTitle', pack.qqTitle),
    qq: tourPhrase('pushNotifications.tour.qq', pack.qq),
    next: tourPhrase('common.next', pack.next),
    previous: tourPhrase('common.back', pack.previous),
    done: tourPhrase('common.confirm', pack.done),
  }
}

function currentUserId() {
  const id = useAuthStore().user?.id
  return typeof id === 'number' && id > 0 ? id : null
}

function mountMuteCheckbox(popover: PopoverDOM, mute: PushTourMute) {
  if (popover.description.querySelector('[data-push-tour-mute]')) return
  const label = document.createElement('label')
  label.className = 'push-tour-mute'
  const input = document.createElement('input')
  input.type = 'checkbox'
  input.dataset.pushTourMute = '1'
  input.checked = localStorage.getItem(mute.storageKey) === '1'
  input.addEventListener('change', () => {
    if (input.checked) localStorage.setItem(mute.storageKey, '1')
    else localStorage.removeItem(mute.storageKey)
  })
  label.appendChild(input)
  label.appendChild(document.createTextNode(mute.label))
  popover.description.appendChild(label)
}

function visibleTourElement(selector: string) {
  return [...document.querySelectorAll(selector)].find((element) => {
    const box = element.getBoundingClientRect()
    return box.width > 0 && box.height > 0
  }) ?? null
}

function waitForVisible(selector: string, timeout = 4000) {
  const start = Date.now()
  return new Promise<Element | null>((resolve) => {
    const tick = () => {
      const element = visibleTourElement(selector)
      if (element) {
        resolve(element)
        return
      }
      if (Date.now() - start >= timeout) {
        resolve(null)
        return
      }
      window.setTimeout(tick, 50)
    }
    tick()
  })
}

function prepareQqStep() {
  window.dispatchEvent(new CustomEvent(OPEN_QQ_GROUP_EVENT))
  return waitForVisible(QQ_SELECTOR)
}

function prepareNotificationStep() {
  window.dispatchEvent(new CustomEvent(CLOSE_QQ_GROUP_EVENT))
  window.dispatchEvent(new CustomEvent(OPEN_USER_MENU_EVENT))
  return waitForVisible(TOUR_SELECTOR)
}

/** Highlights the notification switch and, when requested, the QQ group entry in one tour. */
export async function startPushNotificationTour(options: PushTourOptions): Promise<boolean> {
  const onboardingStore = useOnboardingStore()
  if (onboardingStore.isDriverActive()) return false
  const wantsNotification = Boolean(options.notification) && isPushSupported()
  const wantsQq = Boolean(options.qq)
  if (wantsQq && !document.querySelector(QQ_SELECTOR)) return false
  if (!wantsNotification && !wantsQq) return false

  const steps: DriveStep[] = []
  if (wantsNotification && options.notification) {
    const notification = options.notification
    steps.push({
      element: TOUR_SELECTOR,
      popover: {
        title: notification.title,
        description: notification.description,
        side: 'left',
        align: 'start',
        ...(wantsQq
          ? {
              onNextClick: (_element, _step, { driver: tour }) => {
                void prepareQqStep().then((element) => {
                  if (element) tour.moveNext()
                  else tour.destroy()
                })
              },
            }
          : {}),
      },
    })
  }
  if (wantsQq && options.qq) {
    const qq = options.qq
    steps.push({
      element: () => visibleTourElement(QQ_SELECTOR) ?? document.querySelector(QQ_SELECTOR) ?? document.body,
      popover: {
        title: qq.title,
        description: qq.description,
        side: 'bottom',
        align: 'end',
        ...(wantsNotification
          ? {
              onPrevClick: (_element, _step, { driver: tour }) => {
                void prepareNotificationStep().then((element) => {
                  if (element) tour.movePrevious()
                })
              },
            }
          : {}),
      },
    })
  }

  if (wantsNotification) {
    const visible = await prepareNotificationStep()
    if (!visible || onboardingStore.isDriverActive()) return false
  } else {
    const visible = await prepareQqStep()
    if (!visible || onboardingStore.isDriverActive()) return false
  }

  return await new Promise<boolean>((resolve) => {
    let settled = false
    const finish = (started: boolean) => {
      if (settled) return
      settled = true
      resolve(started)
    }
    const instance = driver({
      showProgress: false,
      animate: true,
      allowClose: true,
      overlayClickBehavior: 'close',
      stagePadding: 6,
      popoverClass: 'theme-tour-popover',
      progressText: '',
      nextBtnText: options.buttons.next,
      prevBtnText: options.buttons.previous,
      doneBtnText: options.buttons.done,
      onPopoverRender: (popover) => {
        popover.progress.style.display = 'none'
        if (options.mute) mountMuteCheckbox(popover, options.mute)
      },
      steps,
      onDestroyed: () => {
        window.dispatchEvent(new CustomEvent(CLOSE_QQ_GROUP_EVENT))
        window.dispatchEvent(new CustomEvent(PUSH_TOUR_UNLOCK_EVENT))
        if (onboardingStore.getDriverInstance() === instance) onboardingStore.setDriverInstance(null)
        finish(true)
      },
    })
    onboardingStore.setDriverInstance(instance)
    try {
      instance.drive()
    } catch {
      if (onboardingStore.getDriverInstance() === instance) onboardingStore.setDriverInstance(null)
      finish(false)
    }
  })
}

export async function guidePushIfDisabled(): Promise<void> {
  const userId = currentUserId()
  if (userId && isJoinPushGuideMuted(userId)) return
  let state: PushPermissionState
  try {
    state = await getPushPermissionState(false)
  } catch {
    state = 'disabled'
  }
  if (state === 'enabled' || state === 'unsupported') return
  const copy = await pushTourMessages()
  await startPushNotificationTour({
    notification: {
      title: copy.title,
      description: copy.join,
    },
    buttons: {
      next: copy.next,
      previous: copy.previous,
      done: copy.done,
    },
    ...(userId
      ? { mute: { label: copy.mute, storageKey: joinPushGuideMuteKey(userId) } }
      : {}),
  })
}
