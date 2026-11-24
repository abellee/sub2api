import { computed, readonly, ref } from 'vue'
import { fetchMyPoints, type PointsSummary } from '@/api/points'

const summary = ref<PointsSummary | null>(null)
let userId: number | null = null
let generation = 0
let pending: Promise<PointsSummary | null> | null = null
let loadedAt = 0

export function setUserPointsIdentity(id: number | null) {
  if (id === userId) return
  userId = id
  generation++
  pending = null
  summary.value = null
  loadedAt = 0
  if (id) void refreshUserPoints()
}

export function updateUserPoints(value: PointsSummary, id: number) {
  if (id !== userId) return
  generation++
  pending = null
  summary.value = value
  loadedAt = Date.now()
}

export function refreshUserPoints(force = false): Promise<PointsSummary | null> {
  if (!userId) return Promise.resolve(null)
  if (pending) return pending
  if (!force && summary.value && Date.now() - loadedAt < 5000) return Promise.resolve(summary.value)
  const version = generation
  const request = fetchMyPoints().then(value => {
    if (version !== generation) return null
    summary.value = value
    loadedAt = Date.now()
    return value
  }).catch(() => {
    if (version === generation) summary.value = null
    return null
  }).finally(() => {
    if (pending === request) pending = null
  })
  pending = request
  return request
}

export function useUserPoints() {
  return { summary: readonly(summary), balance: computed(() => summary.value?.balance ?? '—') }
}
