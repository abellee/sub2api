<script setup lang="ts">
/** 管理端任务管理页：仅负责创建客户端并挂载面板。 */
import { onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { createDefaultLotteryClient } from '@/api/lotteryClient'
import TaskAdminPanel from '@/components/task/TaskAdminPanel.vue'

const client = createDefaultLotteryClient()
onMounted(() => {
  // 预留：页面级初始化
})
</script>

<template>
  <AppLayout>
    <TaskAdminPanel :client="client" />
  </AppLayout>
</template>
