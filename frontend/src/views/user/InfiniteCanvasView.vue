<template>
  <AppLayout>
    <div class="-m-3 min-h-[calc(100vh-6rem)] overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:-m-4">
      <iframe
        class="block h-[calc(100vh-6rem)] min-h-[640px] w-full border-0"
        src="/infinite-canvas/index.html?baseUrl=%2Finfinite-api"
        title="无限画布"
        allow="clipboard-read; clipboard-write"
      ></iframe>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
</script>
