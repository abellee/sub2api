<script setup lang="ts">
/**
 * 用户抽奖页（并入 Sub2API）：挂 LotteryDrawPanel + 全局资格引导弹窗。
 * 进入页面自动检查资格；lotteryd 地址经 VITE_LOTTERYD_BASE_URL 配置，
 * token 取自主站登录态（开发环境经 /lotteryd 代理到本地 lotteryd）。
 */
import AppLayout from '@/components/layout/AppLayout.vue'
import LotteryDrawPanel from '@/components/lottery/LotteryDrawPanel.vue'
import { createDefaultLotteryClient } from '@/api/lotteryClient'
// 资格引导弹窗由 App.vue 里的 LotteryPromptHost 负责，菜单切换不断开连接

const client = createDefaultLotteryClient()
</script>

<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-3xl">
      <LotteryDrawPanel :client="client" />
    </div>
  </AppLayout>
</template>
