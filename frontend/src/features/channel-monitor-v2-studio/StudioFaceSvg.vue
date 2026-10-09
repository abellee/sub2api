<template>
  <svg
    class="studio-face-svg"
    viewBox="0 0 64 64"
    aria-hidden="true"
    focusable="false"
  >
    <circle cx="32" cy="32" r="28" :fill="palette.skin" />
    <ellipse
      v-if="mood !== 'blank'"
      cx="18"
      cy="40"
      rx="6"
      ry="3.2"
      :fill="palette.blush"
      opacity="0.7"
    />
    <ellipse
      v-if="mood !== 'blank'"
      cx="46"
      cy="40"
      rx="6"
      ry="3.2"
      :fill="palette.blush"
      opacity="0.7"
    />

    <g v-if="mood === 'joy'" fill="none" :stroke="palette.ink" stroke-linecap="round">
      <path d="M17 28c2.6-4 8.2-4 10.8 0" stroke-width="2.6" />
      <path d="M36.2 28c2.6-4 8.2-4 10.8 0" stroke-width="2.6" />
      <path d="M20 41c3.6 8.2 20.4 8.2 24 0" stroke-width="2.7" />
    </g>
    <g v-else-if="mood === 'happy'" :fill="palette.ink">
      <circle cx="23" cy="29" r="3.2" />
      <circle cx="41" cy="29" r="3.2" />
      <path
        d="M20 40.5c3.4 7.6 20.6 7.6 24 0"
        fill="none"
        :stroke="palette.ink"
        stroke-width="2.6"
        stroke-linecap="round"
      />
    </g>
    <g v-else-if="mood === 'smile'" :fill="palette.ink">
      <circle cx="23" cy="29.5" r="2.8" />
      <circle cx="41" cy="29.5" r="2.8" />
      <path
        d="M22 41c2.8 5.2 17.2 5.2 20 0"
        fill="none"
        :stroke="palette.ink"
        stroke-width="2.4"
        stroke-linecap="round"
      />
    </g>
    <g v-else-if="mood === 'neutral'" :fill="palette.ink">
      <circle cx="23" cy="29.5" r="2.7" />
      <circle cx="41" cy="29.5" r="2.7" />
      <path d="M23 42h18" :stroke="palette.ink" stroke-width="2.4" stroke-linecap="round" />
    </g>
    <g v-else-if="mood === 'worried'" fill="none" :stroke="palette.ink" stroke-linecap="round">
      <path d="M17.5 24.5l8.2 2.4" stroke-width="2.1" />
      <path d="M46.5 24.5l-8.2 2.4" stroke-width="2.1" />
      <circle cx="23" cy="30.5" r="2.7" :fill="palette.ink" stroke="none" />
      <circle cx="41" cy="30.5" r="2.7" :fill="palette.ink" stroke="none" />
      <path d="M24 44c2.4-3.8 13.6-3.8 16 0" stroke-width="2.3" />
    </g>
    <g v-else-if="mood === 'sad'" fill="none" :stroke="palette.ink" stroke-linecap="round">
      <path d="M17 26l8.4 3.2" stroke-width="2.2" />
      <path d="M47 26l-8.4 3.2" stroke-width="2.2" />
      <circle cx="23" cy="32" r="2.8" :fill="palette.ink" stroke="none" />
      <circle cx="41" cy="32" r="2.8" :fill="palette.ink" stroke="none" />
      <path d="M24 46c2.6-5.4 13.4-5.4 16 0" stroke-width="2.4" />
    </g>
    <g v-else :fill="palette.ink">
      <path d="M19 30h8" :stroke="palette.ink" stroke-width="2.4" stroke-linecap="round" />
      <path d="M37 30h8" :stroke="palette.ink" stroke-width="2.4" stroke-linecap="round" />
      <ellipse cx="32" cy="42" rx="3.1" ry="2.3" />
    </g>
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { studioFacePalette, type StudioFaceMood, type StudioTone } from './studioFormat'

const props = defineProps<{
  mood: StudioFaceMood
  tone?: StudioTone
  score?: number | null
}>()

const palette = computed(() => studioFacePalette(props.tone, props.score))
</script>
