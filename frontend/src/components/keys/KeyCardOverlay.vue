<template>
  <div class="key-channel-overlay" @click.self="$emit('close')">
    <section
      ref="panel"
      class="key-channel-panel"
      :class="{ 'key-card-panel-closable': showCloseButton }"
      role="dialog"
      :aria-label="label"
      tabindex="-1"
      @keydown.esc.stop="$emit('close')"
      @keydown.tab="trapFocus"
    >
      <button v-if="showCloseButton" type="button" class="key-card-close" :aria-label="t('common.close')" @click="$emit('close')">
        <Icon name="x" size="sm" aria-hidden="true" />
      </button>
      <div class="key-card-scroll"><slot /></div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
defineProps<{ label: string; showCloseButton?: boolean }>()
defineEmits<{ close: [] }>()
const { t } = useI18n()
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
.key-channel-panel { position: relative; display: flex; flex-direction: column; width: 100%; max-height: 100%; overflow: hidden; border-radius: 18px; background: var(--keys-card-surface, var(--tf-console-surface)); color: var(--studio-ink); box-shadow: 0 -8px 24px rgb(0 0 0 / .15); }
.key-card-scroll { min-height: 0; overflow-y: auto; }
.key-card-panel-closable .key-card-scroll { padding-top: 28px; }
.key-card-close { position: absolute; top: 8px; right: 8px; z-index: 1; display: flex; align-items: center; justify-content: center; width: 28px; height: 28px; border-radius: 8px; background: var(--keys-card-surface, var(--tf-console-surface)); color: var(--studio-muted); }
.key-card-close:hover { color: var(--studio-ink); background: var(--studio-haze); }
.key-card-close:focus-visible { outline: 2px solid var(--tf-console-active-ink); outline-offset: 2px; }
</style>
