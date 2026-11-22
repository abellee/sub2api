<template>
  <div class="key-card-list" :aria-busy="loading">
    <div v-if="data.length" class="key-selection">
      <label><input type="checkbox" :checked="allSelected" :indeterminate="someSelected" :aria-label="t('common.selectAll')" @change="toggleAll" />{{ t('common.selectAll') }}</label>
      <slot name="selection-actions" />
    </div>
    <div v-if="loading && !data.length" class="key-card-grid" role="status" :aria-label="t('common.loading')">
      <article v-for="index in 6" :key="index" class="key-card key-card-skeleton" aria-hidden="true">
        <Skeleton width="60%" :height="22" /><Skeleton :height="36" /><Skeleton width="75%" :height="80" /><Skeleton :height="32" />
      </article>
    </div>
    <div v-else-if="!data.length" class="key-card-empty"><slot name="empty" /></div>
    <div v-else class="key-card-grid">
      <article v-for="row in data" :key="row.id" class="key-card" :class="{ 'key-card-selected': selectedKeys.includes(row.id) }" :data-key-id="row.id">
        <header class="key-card-heading">
          <h2><slot name="cell-name" :row="row" :value="row.name">{{ row.name }}</slot></h2>
          <input type="checkbox" :checked="selectedKeys.includes(row.id)" :aria-label="t('keys.bulkEdit.selectKey', { name: row.name })" @change="toggleRow(row.id)" />
        </header>
        <dl class="key-card-fields">
          <div v-for="column in fields" :key="column.key" class="key-card-field" :data-field="column.key">
            <dt>{{ column.label }}</dt>
            <dd><slot :name="`cell-${column.key}`" :row="row" :value="row[column.key as keyof ApiKey]">{{ row[column.key as keyof ApiKey] }}</slot></dd>
          </div>
        </dl>
        <footer class="key-card-actions"><slot name="cell-actions" :row="row" /></footer>
      </article>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Skeleton from '@/components/common/Skeleton.vue'
import type { Column } from '@/components/common/types'
import type { ApiKey } from '@/types'

const props = defineProps<{
  columns: Column[]
  data: ApiKey[]
  loading: boolean
  selectedKeys: number[]
}>()
const emit = defineEmits<{
  'update:selectedKeys': [ids: number[]]
}>()
const { t } = useI18n()
const fields = computed(() => props.columns.filter(column => column.key !== 'name' && column.key !== 'actions'))
const allSelected = computed(() => props.data.length > 0 && props.data.every(row => props.selectedKeys.includes(row.id)))
const someSelected = computed(() => !allSelected.value && props.data.some(row => props.selectedKeys.includes(row.id)))
function toggleAll() { emit('update:selectedKeys', allSelected.value ? [] : props.data.map(row => row.id)) }
function toggleRow(id: number) { emit('update:selectedKeys', props.selectedKeys.includes(id) ? props.selectedKeys.filter(key => key !== id) : [...props.selectedKeys, id]) }
</script>

<style scoped>
.key-card-list{min-width:0}.key-selection{display:flex;align-items:center;gap:16px;min-height:36px;margin-bottom:12px;overflow-x:auto;color:var(--studio-muted);font-size:12px;white-space:nowrap}.key-selection label{display:flex;flex-shrink:0;align-items:center;gap:8px;cursor:pointer}
.key-card-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(min(100%,320px),1fr));grid-auto-rows:1fr;gap:20px;align-items:stretch}
.key-card{display:flex;flex-direction:column;width:100%;height:100%;min-width:0;max-width:440px;padding:22px;border-radius:18px;background:var(--keys-card-surface,var(--tf-console-surface));box-shadow:0 6px 28px #26385d05;color:var(--studio-ink)}
.key-card-selected{outline:2px solid var(--tf-console-active-ink);outline-offset:2px}.key-card-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:12px;margin-bottom:20px}.key-card-heading h2{min-width:0;font-size:15px;font-weight:600;overflow-wrap:anywhere}.key-card-heading :deep(.flex){flex-wrap:wrap}
input[type=checkbox]{appearance:none;width:16px;height:16px;flex-shrink:0;margin:0;border:1px solid color-mix(in srgb,var(--studio-muted) 45%,transparent);border-radius:5px;background:transparent;cursor:pointer;transition:background-color .15s,border-color .15s}input[type=checkbox]:hover{border-color:var(--tf-console-active-ink)}input[type=checkbox]:checked,input[type=checkbox]:indeterminate{border-color:var(--tf-console-active-ink);background-color:var(--tf-console-active-ink);background-position:center;background-repeat:no-repeat;background-size:12px}input[type=checkbox]:checked{background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16' fill='none' stroke='white' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m3 8 3 3 7-7'/%3E%3C/svg%3E")}input[type=checkbox]:indeterminate{background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16' stroke='white' stroke-width='2' stroke-linecap='round'%3E%3Cpath d='M4 8h8'/%3E%3C/svg%3E")}input[type=checkbox]:focus-visible{outline:2px solid var(--tf-console-active-ink);outline-offset:3px}.key-card-heading input{margin-top:3px}
.key-card-fields{display:flex;flex-direction:column;gap:15px}.key-card-field{display:grid;grid-template-columns:76px minmax(0,1fr);gap:10px;align-items:start}.key-card-field dt{font-size:12px;color:var(--studio-muted);padding-top:2px}.key-card-field dd{min-width:0;font-size:13px;overflow-wrap:anywhere}
.key-card-field dd :deep(.code){display:block;min-width:0;overflow-wrap:anywhere}.key-card-field dd :deep(button){max-width:100%;text-align:left}
.key-card-fields{margin-bottom:20px}.key-card-actions{margin-top:auto;padding-top:14px;border-top:1px solid var(--studio-haze)}.key-card-actions :deep(>div){flex-wrap:wrap;justify-content:space-between;gap:6px}
.key-card-skeleton{display:flex;flex-direction:column;gap:20px;min-height:320px}.key-card-empty{padding:32px 16px;border-radius:18px;background:var(--keys-card-surface,var(--tf-console-surface))}
.dark .key-card{box-shadow:0 8px 28px #0000001f}
@media(max-width:640px){.key-card-grid{gap:16px}.key-card{padding:18px}}
</style>
