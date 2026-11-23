<template>
  <div class="key-channel-overlay" @click.self="$emit('close')">
    <section
      ref="panel"
      class="key-channel-panel"
      role="dialog"
      :aria-label="label"
      tabindex="-1"
      @keydown.esc.stop="$emit('close')"
      @keydown.tab="trapFocus"
    >
      <slot />
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
defineProps<{ label: string }>()
defineEmits<{ close: [] }>()
const panel = ref<HTMLElement | null>(null)
let previousFocus: HTMLElement | null = null

onMounted(() => {
  previousFocus = document.activeElement as HTMLElement | null
  panel.value?.focus({ preventScroll: true })
})
onBeforeUnmount(() => previousFocus?.focus({ preventScroll: true }))

function trapFocus(event: KeyboardEvent) {
  const buttons = panel.value?.querySelectorAll<HTMLButtonElement>('button')
  if (!buttons?.length) {
    event.preventDefault()
    return
  }
  const first = buttons[0]!
  const last = buttons[buttons.length - 1]!
  if (event.shiftKey && (document.activeElement === first || document.activeElement === panel.value)) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}
</script>

<style scoped>
.key-channel-overlay { position: absolute; inset: 0; z-index: 10; display: flex; align-items: flex-end; overflow: hidden; border-radius: inherit; background: rgb(0 0 0 / .6); }
.key-channel-panel { width: 100%; max-height: 100%; overflow-y: auto; border-radius: 18px; background: var(--keys-card-surface, var(--tf-console-surface)); color: var(--studio-ink); box-shadow: 0 -8px 24px rgb(0 0 0 / .15); }
</style>
