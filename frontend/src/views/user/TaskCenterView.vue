<script setup lang="ts">
/** 用户侧任务中心页。 */
import StudioLayout from '@/components/user/dashboard/studio/DashboardStudioLayout.vue'
import { createDefaultLotteryClient } from '@/api/lotteryClient'
import TaskCenterPanel from '@/components/task/TaskCenterPanel.vue'

const client = createDefaultLotteryClient()
</script>

<template>
  <StudioLayout>
    <TaskCenterPanel :client="client" />
  </StudioLayout>
</template>
