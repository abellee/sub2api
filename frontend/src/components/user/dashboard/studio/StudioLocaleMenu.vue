<template>
  <div ref="root" class="studio-locale" @mouseenter="cancelClose" @mouseleave="scheduleClose" @keydown.esc.stop="open = false">
    <button ref="trigger" type="button" class="locale-trigger" :aria-label="currentLocale?.name" :title="currentLocale?.name" :aria-expanded="open" :disabled="switching" @mouseenter="openOnHover" @click="toggle"><img v-if="currentLocale" :src="localeFlags[currentLocale.code]" alt="" class="locale-flag" /></button>
    <StudioPopover v-if="open" :anchor="trigger" side="right" class="locale-options" @mouseenter="cancelClose">
      <button v-for="item in availableLocales" :key="item.code" type="button" :disabled="switching" :aria-pressed="locale === item.code" @click="selectLocale(item.code)"><img :src="localeFlags[item.code]" alt="" class="locale-flag" /><span>{{ item.name }}</span><Icon v-if="locale === item.code" name="check" size="sm" /></button>
    </StudioPopover>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { availableLocales, setLocale } from '@/i18n'
import Icon from '@/components/icons/Icon.vue'
import englishFlag from '@/assets/icons/flag-en.svg'
import chineseFlag from '@/assets/icons/flag-zh.svg'
import StudioPopover from './StudioPopover.vue'
const props = defineProps<{ expanded?: boolean }>()
const localeFlags = { en: englishFlag, zh: chineseFlag }
const trigger = ref<HTMLElement | null>(null)
const { locale } = useI18n()
const emit = defineEmits<{ opened: [] }>()
const currentLocale = computed(() => availableLocales.find(item => item.code === locale.value))
const root = ref<HTMLElement | null>(null)
const open = ref(false)
const switching = ref(false)
let closeTimer: ReturnType<typeof setTimeout> | undefined
function cancelClose() { clearTimeout(closeTimer) }
watch(() => props.expanded, () => { cancelClose(); open.value = false }, { flush: 'sync' })
function scheduleClose() { cancelClose(); closeTimer = setTimeout(() => { open.value = false }, 180) }
function toggle() { cancelClose(); open.value = !open.value; if (open.value) emit('opened') }
function openOnHover() {
  if (!window.matchMedia('(hover: hover) and (pointer: fine)').matches) return
  cancelClose(); open.value = true; emit('opened')
}
async function selectLocale(code: string) {
  if (switching.value || code === locale.value) { open.value = false; return }
  switching.value = true
  try { await setLocale(code); open.value = false; window.location.reload() }
  finally { switching.value = false }
}
function outside(event: MouseEvent) { if (!root.value?.contains(event.target as Node)) open.value = false }
onMounted(() => document.addEventListener('click', outside))
onBeforeUnmount(() => { cancelClose(); document.removeEventListener('click', outside) })
</script>
<style scoped>
.studio-locale{position:relative}.locale-trigger{display:grid;place-items:center;width:36px;height:34px;border:0;border-radius:10px;background:transparent;color:var(--studio-muted);cursor:pointer}.locale-trigger:hover,.locale-trigger[aria-expanded=true]{background:var(--tf-console-hover);color:var(--studio-ink)}.locale-trigger:focus-visible{outline:2px solid #8796db;outline-offset:2px}
.locale-flag{display:block;width:20px;height:14px;flex:none;object-fit:contain}
.locale-options{width:164px;padding:6px;border-radius:12px;background:var(--tf-console-surface);color:var(--studio-ink)}.locale-options button{display:flex;align-items:center;gap:9px;width:100%;padding:9px 10px;border:0;border-radius:8px;background:transparent;text-align:left;font-size:13px}.locale-options button>span{flex:1}.locale-options button:hover,.locale-options button[aria-pressed=true]{background:var(--tf-console-hover)}
</style>
