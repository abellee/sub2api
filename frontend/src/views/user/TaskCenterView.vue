<script setup lang="ts">
/** 用户侧任务中心页。 */
import AppLayout from '@/components/layout/AppLayout.vue'
import { createDefaultLotteryClient } from '@/api/lotteryClient'
import TaskCenterPanel from '@/components/task/TaskCenterPanel.vue'

const client = createDefaultLotteryClient()
</script>

<template>
  <AppLayout>
    <TaskCenterPanel :client="client" />
  </AppLayout>
</template>
