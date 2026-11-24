<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DashboardStudioLayout from '@/components/user/dashboard/studio/DashboardStudioLayout.vue'

const { t } = useI18n()
</script>

<template>
  <DashboardStudioLayout>
    <section class="resource-coming-soon">
      <p>{{ t('common.resourceComingSoon') }}</p>
    </section>
  </DashboardStudioLayout>
</template>

<style scoped>
.resource-coming-soon{display:grid;place-items:center;min-height:60dvh;padding:32px;border-radius:18px;background:var(--studio-card-surface,var(--tf-console-surface));color:var(--studio-ink);box-shadow:var(--tf-console-card-shadow);text-align:center}
.resource-coming-soon p{font-size:clamp(20px,2.4vw,32px);font-weight:500;line-height:1.8;overflow-wrap:anywhere}
:global(.dark .resource-coming-soon){background:var(--studio-menu-surface)}
</style>
