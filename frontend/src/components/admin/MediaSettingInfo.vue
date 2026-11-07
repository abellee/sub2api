<script setup lang="ts">
import HelpTooltip from '@/components/common/HelpTooltip.vue'
defineProps<{ title: string; content: string }>()
</script>

<template>
  <HelpTooltip :content="content" width-class="w-80 max-w-[calc(100vw-2rem)]" data-testid="setting-info">
    <template #trigger>
      <span role="img" tabindex="0" class="inline-flex cursor-help rounded text-gray-400 hover:text-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500" :aria-label="title" @click.stop.prevent>
        <svg aria-hidden="true" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="9" /><path stroke-linecap="round" d="M12 11v6m0-10v.1" /></svg>
      </span>
    </template>
  </HelpTooltip>
</template>
