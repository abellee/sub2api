<template>
  <span class="rolling-balance" :aria-label="formatted">
    <span class="sr-only">{{ formatted }}</span>
    <span class="rolling-balance-visual" aria-hidden="true">
      <template v-for="(column, index) in columns" :key="`${revision}-${index}`">
        <span v-if="column.digits" class="balance-digit">
          <span class="balance-digit-track" :class="{ 'is-rolling': column.digits.length > 1 }" :style="{ '--roll-offset': `${-(column.digits.length - 1) * 1.25}em` }">
            <span v-for="(digit, step) in column.digits" :key="step" class="balance-digit-step" :data-digit="digit"></span>
          </span>
        </span>
        <span v-else class="balance-punctuation" :data-character="column.character"></span>
      </template>
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = defineProps<{ value: number }>()
const formatter = new Intl.NumberFormat('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const formatted = computed(() => `$${formatter.format(Number.isFinite(props.value) ? props.value : 0)}`)
type Column = { character: string; digits?: number[] }
const columns = ref<Column[]>([])
const revision = ref(0)
watch(formatted, (next, previous) => {
  const oldDigits = (previous ?? '').replace(/\D/g, '').split('').reverse()
  let position = next.replace(/\D/g, '').length
  columns.value = Array.from(next, character => {
    if (!/\d/.test(character)) return { character }
    const target = Number(character)
    const start = Number(oldDigits[--position] ?? target)
    // A full turn also animates digits whose final value stays the same.
    const steps = previous === undefined ? 0 : 10 + (target - start + 10) % 10
    return { character, digits: Array.from({ length: steps + 1 }, (_, step) => (start + step) % 10) }
  })
  revision.value++
}, { immediate: true })
</script>

<style scoped>
.rolling-balance{display:inline-block;vertical-align:middle;font-variant-numeric:tabular-nums;letter-spacing:0;white-space:nowrap}
.rolling-balance-visual{display:inline-flex;align-items:center;vertical-align:middle}
.balance-digit{display:inline-block;width:.62em;height:1.25em;overflow:hidden}
.balance-digit-track{display:block;transform:translateY(var(--roll-offset))}
.balance-digit-track.is-rolling{animation:balance-digit-roll 1.25s cubic-bezier(.16,.75,.22,1) both}
.balance-digit-step{display:block;height:1.25em;line-height:1.25em;text-align:center}
.balance-digit-step::before{content:attr(data-digit)}
.balance-punctuation::before{content:attr(data-character)}
@keyframes balance-digit-roll{from{transform:translateY(0)}to{transform:translateY(var(--roll-offset))}}
@media(prefers-reduced-motion:reduce){.balance-digit-track.is-rolling{animation:none}}
</style>
