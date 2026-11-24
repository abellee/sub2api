<template>
  <div class="reaction-bar relative" @keydown.esc="pickerOpen = false">
    <div class="flex flex-wrap items-center gap-2">
      <button
        v-for="reaction in visibleReactions" :key="reaction.emoji_id" type="button"
        class="reaction-pill" :class="{ 'reaction-pill-selected': reaction.reacted }"
        :aria-label="`${label(reaction.emoji_id)} · ${reaction.count}`" :aria-pressed="reaction.reacted"
        :disabled="busy || disabled" @click="select(reaction.emoji_id)"
        @mouseenter="hovered = reaction.emoji_id" @mouseleave="hovered = null"
        @focus="hovered = reaction.emoji_id" @blur="hovered = null"
      >
        <LottieEmoji :name="assetName(reaction.emoji_id)" :emoji="glyph(reaction.emoji_id)" :playing="hovered === reaction.emoji_id || celebrating === reaction.emoji_id" class="h-7 w-7 text-[28px]" />
        <span class="min-w-3 text-xs font-semibold tabular-nums">{{ reaction.count }}</span>
      </button>
      <div ref="pickerRoot">
        <button type="button" class="reaction-add" :disabled="disabled" :aria-label="t('announcements.reactions.add')"
          :aria-expanded="pickerOpen" aria-haspopup="dialog" @click="pickerOpen = !pickerOpen">
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
            <path stroke-linecap="round" d="M20 11a8 8 0 1 1-7-7M8 14s1 2 4 2 4-2 4-2M8.5 9h.01M15.5 9h.01M19 2v6M16 5h6" />
          </svg>
          <span v-if="!visibleReactions.length" class="text-xs font-medium">{{ t('announcements.reactions.add') }}</span>
        </button>
        <Transition name="reaction-picker">
          <div v-if="pickerOpen" class="reaction-picker" role="dialog" :aria-label="t('announcements.reactions.choose')">
            <button v-for="item in announcementEmojis" :key="item.id" type="button" class="reaction-choice"
              :class="{ 'reaction-choice-selected': reactions.some(r => r.emoji_id === item.id && r.reacted) }"
              :aria-label="label(item.id)" :title="label(item.id)" :disabled="busy"
              @click="select(item.id)" @mouseenter="hovered = item.id" @mouseleave="hovered = null"
              @focus="hovered = item.id" @blur="hovered = null">
              <LottieEmoji :name="item.name" :emoji="item.emoji" :playing="hovered === item.id" class="h-10 w-10 text-[40px]" />
            </button>
          </div>
        </Transition>
      </div>
      <span v-if="busy" class="h-3 w-3 animate-spin rounded-full border-2 border-gray-200 border-t-primary-500" />
    </div>
    <p v-if="error" class="mt-2 text-xs text-red-500" role="alert">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import announcementsAPI from '@/api/announcements'
import LottieEmoji from './LottieEmoji.vue'
import { announcementEmojis, type AnnouncementEmoji, type AnnouncementReaction } from './announcementReactions'

const props = withDefaults(defineProps<{ announcementId: number; disabled?: boolean }>(), { disabled: false })
const { t } = useI18n()
const reactions = ref<AnnouncementReaction[]>([])
const pickerOpen = ref(false)
const pickerRoot = ref<HTMLElement>()
const busy = ref(false)
const error = ref('')
const hovered = ref<AnnouncementEmoji | null>(null)
const celebrating = ref<AnnouncementEmoji | null>(null)
let generation = 0
let celebrationTimer: ReturnType<typeof setTimeout> | undefined
const visibleReactions = computed(() => reactions.value.filter(r => r.count > 0 && announcementEmojis.some(e => e.id === r.emoji_id)))
const glyph = (id: AnnouncementEmoji) => announcementEmojis.find(e => e.id === id)!.emoji
const assetName = (id: AnnouncementEmoji) => announcementEmojis.find(e => e.id === id)!.name
const label = (id: AnnouncementEmoji) => t(`announcements.reactions.${assetName(id)}`)
onClickOutside(pickerRoot, () => { pickerOpen.value = false })

watch(() => props.announcementId, async id => {
  const request = ++generation
  reactions.value = []; error.value = ''; pickerOpen.value = false; busy.value = false
  if (!id || props.disabled) return
  try {
    const result = await announcementsAPI.reactions(id)
    if (request === generation) reactions.value = result
  } catch {
    if (request === generation) error.value = t('announcements.reactions.loadFailed')
  }
}, { immediate: true })

async function select(emoji: AnnouncementEmoji) {
  if (busy.value || props.disabled) return
  const request = ++generation
  const before = reactions.value.map(reaction => ({ ...reaction }))
  const remove = reactions.value.some(r => r.emoji_id === emoji && r.reacted)
  const next = before.map(reaction => ({
    ...reaction,
    count: reaction.count - (reaction.reacted ? 1 : 0),
    reacted: false,
  }))
  if (!remove) {
    const selected = next.find(reaction => reaction.emoji_id === emoji)
    if (selected) { selected.count++; selected.reacted = true }
    else next.push({ emoji_id: emoji, count: 1, reacted: true })
  }
  reactions.value = next.filter(reaction => reaction.count > 0)
  clearTimeout(celebrationTimer)
  celebrating.value = remove ? null : emoji
  if (!remove) celebrationTimer = setTimeout(() => { celebrating.value = null }, 2500)
  busy.value = true; error.value = ''; pickerOpen.value = false
  try {
    const result = await announcementsAPI.setReaction(props.announcementId, remove ? null : emoji)
    if (request !== generation) return
    reactions.value = result
  } catch {
    if (request === generation) {
      reactions.value = before
      celebrating.value = null
      clearTimeout(celebrationTimer)
      error.value = t('announcements.reactions.saveFailed')
    }
  } finally {
    if (request === generation) busy.value = false
  }
}
onBeforeUnmount(() => { generation++; clearTimeout(celebrationTimer) })
</script>

<style scoped>
.reaction-pill { display: inline-flex; height: 36px; align-items: center; gap: 5px; padding: 2px 11px 2px 5px; border: 1px solid transparent; border-radius: 999px; color: #64748b; background: #f1f5f9; transition: background .18s, transform .18s; }
.reaction-pill:hover { background: #e2e8f0; transform: translateY(-1px); }
.reaction-pill-selected { color: #4f46e5; background: #eef2ff; border-color: #c7d2fe; }
.reaction-add { display: inline-flex; min-height: 36px; align-items: center; justify-content: center; gap: 7px; border: 1px dashed #cbd5e1; border-radius: 999px; padding: 0 10px; color: #94a3b8; transition: color .18s, border-color .18s; }
.reaction-add:hover { color: #6366f1; border-color: #a5b4fc; }
.reaction-picker { position: absolute; bottom: calc(100% + 12px); left: 0; z-index: 10; display: grid; grid-template-columns: repeat(5, 44px); gap: 3px; max-height: min(260px, 50vh); overflow-y: auto; padding: 9px; border: 1px solid #e2e8f0; border-radius: 22px; background: #fff; box-shadow: 0 12px 45px #0f172a26, 0 2px 8px #0f172a0a; transform-origin: bottom left; }
.reaction-choice { display: flex; width: 44px; height: 44px; align-items: center; justify-content: center; border-radius: 14px; transition: background .15s, transform .2s; }
.reaction-choice:hover, .reaction-choice:focus-visible { background: #f1f5f9; transform: scale(1.15); }
.reaction-choice-selected { background: #eef2ff; }
button:focus-visible { outline: 2px solid #818cf8; outline-offset: 3px; }
button:disabled { cursor: default; opacity: .55; }
:global(.dark) .reaction-pill { background: #ffffff0c; color: #cbd5e1; }
:global(.dark) .reaction-pill-selected { background: #818cf826; border-color: #818cf84d; color: #c7d2fe; }
:global(.dark) .reaction-picker { background: #1e293b; border-color: #334155; }
:global(.dark) .reaction-choice:hover { background: #ffffff0c; }
.reaction-picker-enter-active, .reaction-picker-leave-active { transition: opacity .15s, transform .18s; }
.reaction-picker-enter-from, .reaction-picker-leave-to { opacity: 0; transform: translateY(6px) scale(.94); }
@media (prefers-reduced-motion: reduce) { *, *::before, *::after { transition: none !important; transform: none !important; } }
</style>
