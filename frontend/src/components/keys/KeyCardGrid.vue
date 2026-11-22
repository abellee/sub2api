<template>
  <div class="key-card-list" :aria-busy="loading">
    <div v-if="data.length" class="key-selection">
      <label><input type="checkbox" :checked="allSelected" :indeterminate="someSelected" :aria-label="t('common.selectAll')" @change="toggleAll" />{{ t('common.selectAll') }}</label>
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
            <dt><button v-if="column.sortable" type="button" class="key-field-sort" :title="column.label" @click="sort(column.key)">{{ column.label }}<span aria-hidden="true">{{ sortBy === column.key ? (sortOrder === 'asc' ? '↑' : '↓') : '↕' }}</span></button><span v-else>{{ column.label }}</span></dt>
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
  sortBy: string
  sortOrder: 'asc' | 'desc'
}>()
const emit = defineEmits<{
  'update:selectedKeys': [ids: number[]]
  sort: [key: string, order: 'asc' | 'desc']
}>()
const { t } = useI18n()
const fields = computed(() => props.columns.filter(column => column.key !== 'name' && column.key !== 'actions'))
const allSelected = computed(() => props.data.length > 0 && props.data.every(row => props.selectedKeys.includes(row.id)))
const someSelected = computed(() => !allSelected.value && props.data.some(row => props.selectedKeys.includes(row.id)))
function toggleAll() { emit('update:selectedKeys', allSelected.value ? [] : props.data.map(row => row.id)) }
function toggleRow(id: number) { emit('update:selectedKeys', props.selectedKeys.includes(id) ? props.selectedKeys.filter(key => key !== id) : [...props.selectedKeys, id]) }
function sort(key: string) { emit('sort', key, props.sortBy === key && props.sortOrder === 'asc' ? 'desc' : 'asc') }
</script>

<style scoped>
.key-card-list{min-width:0}.key-selection{display:flex;justify-content:flex-end;margin-bottom:12px;color:var(--studio-muted);font-size:12px}.key-selection label{display:flex;align-items:center;gap:8px}
.key-card-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(min(100%,320px),1fr));gap:20px;align-items:start}
.key-card{width:100%;min-width:0;max-width:440px;padding:22px;border-radius:18px;background:var(--keys-card-surface,var(--tf-console-surface));box-shadow:0 6px 28px #26385d05;color:var(--studio-ink)}
.key-card-selected{outline:2px solid var(--tf-console-active-ink);outline-offset:2px}.key-card-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:12px;margin-bottom:20px}.key-card-heading h2{min-width:0;font-size:15px;font-weight:600;overflow-wrap:anywhere}.key-card-heading :deep(.flex){flex-wrap:wrap}
input[type=checkbox]{width:15px;height:15px;flex-shrink:0;accent-color:var(--tf-console-active-ink);cursor:pointer}.key-card-heading input{margin-top:3px}
.key-card-fields{display:flex;flex-direction:column;gap:15px}.key-card-field{display:grid;grid-template-columns:76px minmax(0,1fr);gap:10px;align-items:start}.key-card-field dt{font-size:12px;color:var(--studio-muted);padding-top:2px}.key-card-field dd{min-width:0;font-size:13px;overflow-wrap:anywhere}.key-field-sort{display:flex;align-items:center;gap:4px;text-align:left}.key-field-sort span{opacity:.5}.key-field-sort:hover{color:var(--tf-console-active-ink)}.key-field-sort:focus-visible{outline:2px solid var(--tf-console-active-ink);outline-offset:3px;border-radius:4px}
.key-card-field dd :deep(.code){display:block;min-width:0;overflow-wrap:anywhere}.key-card-field dd :deep(button){max-width:100%;white-space:normal;flex-wrap:wrap;text-align:left}.key-card-field dd :deep(.flex){flex-wrap:wrap}
.key-card-actions{margin-top:20px;padding-top:14px;border-top:1px solid var(--studio-haze)}.key-card-actions :deep(>div){flex-wrap:wrap;justify-content:space-between;gap:6px}
.key-card-skeleton{display:flex;flex-direction:column;gap:20px;min-height:320px}.key-card-empty{padding:32px 16px;border-radius:18px;background:var(--keys-card-surface,var(--tf-console-surface))}
.dark .key-card{box-shadow:0 8px 28px #0000001f}
@media(max-width:640px){.key-card-grid{gap:16px}.key-card{padding:18px}}
</style>
