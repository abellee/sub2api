<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main class="relative min-h-[calc(100vh-4rem)] p-4 md:p-6 lg:p-8">
        <slot />
      </main>
    </div>

  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
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
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')
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
