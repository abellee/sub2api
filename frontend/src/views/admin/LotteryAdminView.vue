<script setup lang="ts">
/**
 * 抽奖管理页（并入 Sub2API，管理员）：挂 LotteryAdminPanel。
 */
import AppLayout from '@/components/layout/AppLayout.vue'
import LotteryAdminPanel from '@/components/lottery/LotteryAdminPanel.vue'
import { createDefaultLotteryClient } from '@/api/lotteryClient'

const client = createDefaultLotteryClient()
</script>

<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-4xl">
      <LotteryAdminPanel :client="client" />
    </div>
  </AppLayout>
</template>
