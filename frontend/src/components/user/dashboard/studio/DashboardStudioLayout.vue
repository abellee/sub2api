<template>
  <div class="tokenfree-console dashboard-studio-shell">
    <div class="studio-page-glow pointer-events-none fixed inset-0 bg-mesh-gradient" aria-hidden="true"></div>
    <StudioSidebar />
    <div class="studio-workspace">
      <header class="studio-navigation">
        <div class="studio-page-title"><button type="button" class="studio-menu-toggle" :aria-label="t('common.toggleMenu')" @click="appStore.toggleMobileSidebar()"><Icon name="menu" size="md" /></button><h1>{{ pageTitle }}</h1></div>
        <slot name="filters" />
      </header>
      <main><slot /></main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useRoute } from 'vue-router'
import { resolveRouteMetaKeys } from '@/router/title'
import { resolveSiteBillingMode } from '@/utils/siteBillingMode'
import { onboardingTourStorageKey, useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import { whenAnnouncementEntrySettled } from '@/stores/announcements'
import { entryPopupsHeld, holdEntryPopups, releaseEntryPopups } from '@/composables/entryPopupGate'
import {
  hasSeenPushNotifyTour,
  markPushNotifyTourSeen,
  pushTourMessages,
  startPushNotificationTour,
} from '@/composables/usePushNotificationTour'
import { isPushSupported } from '@/services/pushNotifications'
import StudioSidebar from './StudioSidebar.vue'
import Icon from '@/components/icons/Icon.vue'
import { useI18n } from 'vue-i18n'

const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()
const route = useRoute()
const { t } = useI18n()
const isAdmin = computed(() => authStore.user?.role === 'admin')
const pageTitle = computed(() => {
  if (route.name === 'CustomPage') {
    const id = route.params.id as string
    const menuItem = appStore.cachedPublicSettings?.custom_menu_items?.find(item => item.id === id)
      ?? (isAdmin.value ? adminSettingsStore.customMenuItems.find(item => item.id === id) : undefined)
    if (menuItem?.label) return menuItem.label
  }
  const { titleKey } = resolveRouteMetaKeys(route, {
    billingMode: resolveSiteBillingMode(appStore.cachedPublicSettings),
  })
  return titleKey ? t(titleKey) : (route.meta.title as string) || t('nav.dashboard')
})
const onboardingBaseKey = isAdmin.value ? 'admin_guide' : 'user_guide'

const { replayTour } = useOnboardingTour({
  storageKey: onboardingBaseKey,
  autoStart: true
})

const onboardingStore = useOnboardingStore()

let guideGeneration = 0
let cancelAnnouncementWait = () => {}

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function onboardingAutoStartPending(userId: number, role: string) {
  if (role !== 'admin' || authStore.isSimpleMode) return false
  return localStorage.getItem(onboardingTourStorageKey(onboardingBaseKey, userId, role)) !== 'true'
}

async function runEntryGuides(userId: number) {
  const generation = ++guideGeneration
  // 上一轮引导留下的压住状态会让公告一直停在队列里，入口判断就会永远等下去。
  if (entryPopupsHeld.value) releaseEntryPopups()
  const user = authStore.user
  if (!user || user.id !== userId) {
    if (generation === guideGeneration) releaseEntryPopups()
    return
  }

  const onboardingPending = onboardingAutoStartPending(user.id, user.role)
  const pushPending = !hasSeenPushNotifyTour(userId)
  if (!onboardingPending && !pushPending) {
    releaseEntryPopups()
    return
  }

  // 有公告时先等用户关掉，再决定要不要开始首次引导。
  const stillCurrent = await new Promise<boolean>((resolve) => {
    let settled = false
    const cancel = whenAnnouncementEntrySettled(() => {
      settled = true
      resolve(generation === guideGeneration)
    })
    cancelAnnouncementWait = () => {
      cancel()
      if (!settled) resolve(false)
    }
  })
  if (!stillCurrent || generation !== guideGeneration) return

  // 引导期间先压住抽奖和任务弹窗，避免盖住漫游。
  holdEntryPopups()
  try {
    if (onboardingPending) {
      const startDeadline = Date.now() + 12_000
      while (generation === guideGeneration && !onboardingStore.isDriverActive() && Date.now() < startDeadline) {
        await sleep(200)
      }
      while (generation === guideGeneration && onboardingStore.isDriverActive()) {
        await sleep(200)
      }
    }

    if (generation !== guideGeneration || hasSeenPushNotifyTour(userId)) return

    await nextTick()
    for (let attempt = 0; attempt < 60 && generation === guideGeneration; attempt += 1) {
      if (hasSeenPushNotifyTour(userId)) break
      while (generation === guideGeneration && onboardingStore.isDriverActive()) {
        await sleep(200)
      }
      if (generation !== guideGeneration || hasSeenPushNotifyTour(userId)) break
      if (attempt > 0) await sleep(1000)
      if (generation !== guideGeneration || onboardingStore.isDriverActive()) continue
      const copy = await pushTourMessages()
      const started = await startPushNotificationTour({
        notification: isPushSupported()
          ? {
              title: copy.title,
              description: copy.enter,
            }
          : undefined,
        qq: {
          title: copy.qqTitle,
          description: copy.qq,
        },
        buttons: {
          next: copy.next,
          previous: copy.previous,
          done: copy.done,
        },
      })
      if (generation !== guideGeneration) return
      if (started && !onboardingStore.isDriverActive()) {
        markPushNotifyTourSeen(userId)
        break
      }
    }
  } finally {
    if (generation === guideGeneration) releaseEntryPopups()
  }
}

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

onUnmounted(() => {
  guideGeneration += 1
  cancelAnnouncementWait()
  releaseEntryPopups()
})

watch(() => authStore.user?.id, (userId) => {
  cancelAnnouncementWait()
  if (!userId) {
    guideGeneration += 1
    releaseEntryPopups()
    return
  }
  void runEntryGuides(userId)
}, { immediate: true })

defineExpose({ replayTour })
</script>

<style scoped>
.dashboard-studio-shell{--studio-sidebar-width:248px;--studio-motion:240ms;--studio-ease:cubic-bezier(.22,.68,.24,1);--studio-nav-bg:#f7f8fcf5;--studio-ink:#20273e;--studio-muted:#69748d;--studio-haze:#536de512;--studio-grid:#dce2f0;min-height:100vh;background:#f7f8fc;color:var(--studio-ink);font-size:14px}
:global(.dark .dashboard-studio-shell){--studio-nav-bg:#0c101bf5;--studio-ink:#e9edfa;--studio-muted:#a0abc4;--studio-haze:#6f84ed16;--studio-grid:#303c56;--studio-menu-surface:#1b1e32;background:#0c101b}
.studio-workspace{position:relative;margin-left:var(--studio-sidebar-width);min-height:100vh}
.studio-page-glow{pointer-events:none}
.studio-navigation{position:sticky;top:0;z-index:30;display:flex;align-items:center;justify-content:space-between;gap:20px;min-height:86px;padding:18px 32px;background:var(--studio-nav-bg);backdrop-filter:blur(18px)}
.studio-page-title{display:flex;align-items:center;gap:12px;flex-shrink:0}.studio-page-title h1{font-size:22px;font-weight:600;letter-spacing:-.7px}.studio-menu-toggle{display:none;border:0;background:transparent;color:inherit;padding:8px}
main{padding:10px 32px 32px}
.dashboard-studio-shell :deep(.sidebar){width:88px;left:0;top:0;bottom:0;height:100dvh;border:0;border-radius:0;box-shadow:none;background:var(--tf-console-surface);z-index:50;overflow:visible;transition:width var(--studio-motion) var(--studio-ease),transform .18s,box-shadow var(--studio-motion)}
.dashboard-studio-shell :deep(.sidebar-header){height:86px;padding:24px 26px;border:0;gap:0;justify-content:flex-start;overflow:visible}
.dashboard-studio-shell :deep(.sidebar-nav){padding:6px 18px;min-height:0;overflow-x:hidden}
.dashboard-studio-shell :deep(.sidebar-link){height:38px;min-height:38px;padding:8px 16px;gap:18px;border-radius:10px;white-space:nowrap;margin-bottom:4px;font-size:13px}
.dashboard-studio-shell :deep(.sidebar-link-active){box-shadow:none;background:var(--tf-console-hover)}
.dashboard-studio-shell :deep(.sidebar>.mt-auto){position:relative;z-index:2;padding:14px 18px 18px;border:0;background:var(--tf-console-surface);transition:box-shadow var(--studio-motion)}
.dashboard-studio-shell :deep(.sidebar.studio-sidebar-expanded>.mt-auto){box-shadow:0 -12px 28px -12px #17234b26}
.dashboard-studio-shell :deep(.sidebar .sidebar-section-title){height:26px;min-height:26px;margin:6px 0 4px;padding-left:16px;color:var(--studio-muted);font-size:12px;font-weight:500}.dashboard-studio-shell :deep(.sidebar .sidebar-section-title::after){background:color-mix(in srgb,var(--studio-muted) 14%,transparent);transition:opacity .15s}
.dashboard-studio-shell :deep(.sidebar .sidebar-home-lockup img){width:34px;height:34px;margin-right:0}
.dashboard-studio-shell :deep(.sidebar .sidebar-home-brand){flex:none;width:160px;max-width:34px;overflow:hidden;transition:max-width var(--studio-motion) var(--studio-ease)}
.dashboard-studio-shell :deep(.sidebar.studio-sidebar-expanded .sidebar-home-brand){max-width:160px}
.dashboard-studio-shell :deep(.sidebar .sidebar-home-lockup){width:160px}
.dashboard-studio-shell :deep(.sidebar .sidebar-home-word){margin-left:-6px}
.dashboard-studio-shell :deep(.sidebar .sidebar-version){display:block;position:absolute;left:calc(var(--studio-sidebar-width) - 58px);top:29px;margin:0}
.dashboard-studio-shell :deep(.sidebar-version .version-info-trigger){color:var(--studio-muted);border-radius:8px}
.dashboard-studio-shell :deep(.sidebar-version .version-info-trigger:hover){background:var(--tf-console-hover);color:var(--studio-ink)}
.dashboard-studio-shell :deep(.sidebar-version .version-dropdown-icon){left:calc(84px - var(--studio-sidebar-width));max-width:calc(100vw - 52px);max-height:calc(100dvh - 78px);overflow-y:auto;overscroll-behavior:contain}
.dashboard-studio-shell :deep(.sidebar .sidebar-brand){display:none}
.dashboard-studio-shell :deep(.sidebar-header){transition:none}
/* Keep text at its final width; only the outer edge reveals it. */
.dashboard-studio-shell :deep(.sidebar-label){flex:none;width:142px;max-width:142px;transform:none;transition:opacity .15s}
.dashboard-studio-shell :deep(.sidebar-link-status .sidebar-label){width:96px;max-width:96px}
.dashboard-studio-shell :deep(.sidebar-section-title-text){flex:none;width:180px;transform:none;transition:opacity .15s}
.dashboard-studio-shell :deep(.sidebar-home-word){flex:none;transition:opacity .15s}
.dashboard-studio-shell :deep(.sidebar-link){transition:background-color .15s,color .15s}
@media(min-width:1024px){
.dashboard-studio-shell :deep(.sidebar.studio-sidebar-expanded){width:var(--studio-sidebar-width);box-shadow:16px 0 40px #17234b0a}
.dashboard-studio-shell :deep(.sidebar:not(.studio-sidebar-expanded) .sidebar-label),.dashboard-studio-shell :deep(.sidebar:not(.studio-sidebar-expanded) .sidebar-home-word){opacity:0;pointer-events:none}
.dashboard-studio-shell :deep(.sidebar.studio-sidebar-expanded .sidebar-label),.dashboard-studio-shell :deep(.sidebar.studio-sidebar-expanded .sidebar-home-word){opacity:1;transition-delay:60ms}
}
@media(max-width:1023px){.dashboard-studio-shell{--studio-sidebar-width:256px}.studio-workspace{margin-left:0}.studio-menu-toggle{display:grid}.dashboard-studio-shell :deep(.sidebar){width:var(--studio-sidebar-width)}.studio-navigation{padding:14px 20px;min-height:76px}.studio-page-title h1{font-size:20px}main{padding:10px 20px 24px}}
@media(max-width:640px){.studio-navigation{flex-wrap:wrap;gap:8px;padding:10px 16px}.studio-page-title h1{font-size:19px}main{padding:8px 16px 24px}}
@media(prefers-reduced-motion:reduce){.dashboard-studio-shell{--studio-motion:0ms}.dashboard-studio-shell :deep(.sidebar),.dashboard-studio-shell :deep(.sidebar *){transition:none!important}}
</style>
