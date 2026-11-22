<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { unwrap, type LotteryClient } from '@/api/lotteryClient'

const props = defineProps<{ client: LotteryClient }>()
const version = ref('加载中…')

onMounted(async () => {
  try {
    const health = await unwrap<{ version?: string }>(props.client.http.get('/health'))
    version.value = health?.version?.trim() || '暂不可用'
  } catch {
    version.value = '暂不可用'
  }
})
</script>

<template>
  <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">服务版本：{{ version }}</p>
</template>
