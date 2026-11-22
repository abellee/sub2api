<script setup lang="ts">
/**
 * 右下角页面通知。从底部移入，新的一条把旧的往上顶。
 */
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { dismissPageNotification, showPageNotification, usePageNotifications, type PageNotification } from '@/composables/usePageNotifications'
import { listPageNotifications } from '@/api/pushNotifications'

const { items } = usePageNotifications()
const router = useRouter()
const hiddenImages = ref<Record<number, true>>({})
let pageCursor = ''
let pageTimer = 0

function onServiceWorkerMessage(event: MessageEvent) {
  const data = event.data
  if (!data || data.type !== 'sub2api-push') return
  if (typeof data.title !== 'string') return
  showPageNotification({
    key: typeof data.id === 'string' ? data.id : '',
    title: data.title,
    body: typeof data.body === 'string' ? data.body : '',
    image: typeof data.image === 'string' ? data.image : '',
    url: typeof data.url === 'string' ? data.url : '',
  })
}

function hideImage(id: number) {
  hiddenImages.value = { ...hiddenImages.value, [id]: true }
}

function openNotice(item: PageNotification) {
  if (!item.url) return
  if (item.url.startsWith('/') && !item.url.startsWith('//')) {
    void router.push(item.url)
    return
  }
  window.location.assign(item.url)
}

async function pullPageNotices() {
  if (!localStorage.getItem('auth_token')) return
  try {
    const result = await listPageNotifications(pageCursor)
    pageCursor = result.now
    for (const notice of result.notices) {
      showPageNotification({
        key: notice.id,
        title: notice.title,
        body: notice.body,
        image: notice.image,
        url: notice.url,
      })
    }
  } catch {
    // 推送服务暂时不可用时，下一轮再拉。
  }
}

onMounted(() => {
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.addEventListener('message', onServiceWorkerMessage)
  }
  if (import.meta.env.MODE === 'test') return
  void pullPageNotices()
  pageTimer = window.setInterval(() => {
    if (document.visibilityState === 'visible') void pullPageNotices()
  }, 5000)
  document.addEventListener('visibilitychange', pullPageNotices)
})

onBeforeUnmount(() => {
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.removeEventListener('message', onServiceWorkerMessage)
  }
  window.clearInterval(pageTimer)
  document.removeEventListener('visibilitychange', pullPageNotices)
})
</script>

<template>
  <Teleport to="body">
  <TransitionGroup
    tag="div"
    class="pointer-events-none fixed bottom-5 right-5 z-[90] flex w-[min(22rem,calc(100vw-2.5rem))] flex-col gap-3"
    data-testid="page-notification-stack"
    enter-active-class="transition duration-300 ease-out"
    enter-from-class="translate-y-8 opacity-0"
    enter-to-class="translate-y-0 opacity-100"
    leave-active-class="transition duration-200 ease-in absolute"
    leave-from-class="opacity-100"
    leave-to-class="translate-y-6 opacity-0"
    move-class="transition-transform duration-300"
  >
    <article
      v-for="item in items"
      :key="item.id"
      class="pointer-events-auto relative overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-800"
      :class="item.url ? 'cursor-pointer' : ''"
      role="status"
      data-testid="page-notification"
      @click="openNotice(item)"
    >
      <button
        type="button"
        class="absolute right-2 top-2 z-10 flex h-6 w-6 items-center justify-center rounded text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200"
        aria-label="关闭"
        data-testid="page-notification-close"
        @click.stop="dismissPageNotification(item.id)"
      >
        ×
      </button>
      <div class="px-4 py-3 pr-10">
        <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ item.title }}</p>
        <p v-if="item.body" class="mt-1 whitespace-pre-wrap text-sm leading-relaxed text-gray-600 dark:text-gray-300">{{ item.body }}</p>
      </div>
      <img
        v-if="item.image && !hiddenImages[item.id]"
        :src="item.image"
        alt=""
        class="aspect-[2/1] max-h-40 w-full object-cover"
        data-testid="page-notification-image"
        referrerpolicy="no-referrer"
        @error="hideImage(item.id)"
      />
    </article>
  </TransitionGroup>
  </Teleport>
</template>
