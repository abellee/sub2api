<template>
  <div v-for="section in sections" :key="section.id" class="sidebar-section">
    <div
      class="sidebar-section-title"
      :class="{ 'sidebar-section-title-collapsed': collapsed }"
      :aria-hidden="collapsed ? 'true' : 'false'"
    >
      <span class="sidebar-section-title-text" :class="{ 'sidebar-section-title-text-collapsed': collapsed }">
        {{ section.title }}
      </span>
    </div>
    <slot v-for="item in section.items" :key="item.path" :item="item" />
  </div>
</template>

<script setup lang="ts" generic="T extends { path: string }">
import type { UserNavSection } from '@/utils/userMenu'

defineProps<{ sections: UserNavSection<T>[]; collapsed: boolean }>()
</script>

<style scoped>
.sidebar-section-title {
  position: relative;
  display: flex;
  align-items: center;
  min-height: 1.25rem;
  overflow: hidden;
  white-space: nowrap;
}

.sidebar-section-title-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition:
    opacity 0.16s ease,
    transform 0.16s ease;
}

.sidebar-section-title::after {
  content: '';
  position: absolute;
  left: 0.75rem;
  right: 0.75rem;
  top: 50%;
  height: 1px;
  background: rgb(229 231 235);
  opacity: 0;
  transform: translateY(-50%);
  transition: opacity 0.18s ease;
}

.dark .sidebar-section-title::after {
  background: rgb(55 65 81);
}

.sidebar-section-title-text-collapsed {
  opacity: 0;
  transform: translateX(-4px);
}

.sidebar-section-title-collapsed::after {
  opacity: 1;
  transition-delay: 0.08s;
}
</style>
