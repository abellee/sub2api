<template>
  <span
    class="studio-brand-icon inline-flex shrink-0 items-center justify-center"
    :class="ink ? 'studio-brand-icon--ink' : ''"
    :style="ink ? undefined : { color: fill }"
    aria-hidden="true"
  >
    <PlatformIcon :platform="platform" :size="size" />
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { GroupPlatform } from '@/types'
import { studioBrandFill, studioBrandIsInk } from './studioBrand'

const props = withDefaults(
  defineProps<{
    platform?: GroupPlatform
    size?: 'xs' | 'sm' | 'md' | 'lg'
  }>(),
  { size: 'md' },
)

const fill = computed(() => studioBrandFill(props.platform))
const ink = computed(() => studioBrandIsInk(props.platform))
</script>

<style scoped>
.studio-brand-icon--ink {
  color: #111111;
}
:global(.dark) .studio-brand-icon--ink {
  color: #f4f4f5;
}
</style>
