import { readonly, ref } from 'vue'
import { claimDailyCheckIn, fetchCheckInStatus, type CheckInStatus } from '@/api/points'
import { updateUserPoints } from './useUserPoints'

const status = ref<CheckInStatus | null>(null)
const loading = ref(false)
const claiming = ref(false)
const error = ref(false)
let userId: number | null = null
let generation = 0
let requestVersion = 0
let loadedAt = 0
let pending: Promise<CheckInStatus | null> | null = null
let resetTimer: ReturnType<typeof setTimeout> | undefined

function scheduleReset() {
  clearTimeout(resetTimer)
  if (!status.value || !userId) return
  const wait = Date.parse(status.value.next_reset_at) - Date.now()
  if (wait > 0) resetTimer = setTimeout(() => { status.value = null; void refreshDailyCheckIn(true) }, wait + 100)
}

export function setDailyCheckInIdentity(id: number | null) {
  if (userId === id) return
  generation++
  requestVersion++
  userId = id
  status.value = null
  pending = null
  loadedAt = 0
  loading.value = claiming.value = error.value = false
  clearTimeout(resetTimer)
  if (id) void refreshDailyCheckIn()
}

export function refreshDailyCheckIn(force = false): Promise<CheckInStatus | null> {
  if (!userId) return Promise.resolve(null)
  if (pending) return pending
  if (status.value && Date.parse(status.value.next_reset_at) <= Date.now()) status.value = null
  if (!force && status.value && Date.now() - loadedAt < 5000) return Promise.resolve(status.value)
  const version = ++requestVersion
  loading.value = true
  error.value = false
  const request = fetchCheckInStatus().then(value => {
    if (version !== requestVersion) return null
    status.value = value
    loadedAt = Date.now()
    scheduleReset()
    return value
  }).catch(() => {
    if (version === requestVersion) { status.value = null; error.value = true }
    return null
  }).finally(() => {
    if (version === requestVersion) { pending = null; loading.value = false }
  })
  pending = request
  return request
}

async function claim() {
  if (!userId || claiming.value) return null
  const identity = userId
  const version = generation
  // A status read started before a claim must not overwrite the claimed result.
  requestVersion++
  pending = null
  loading.value = false
  claiming.value = true
  try {
    const result = await claimDailyCheckIn()
    if (version !== generation) return null
    requestVersion++
    pending = null
    loading.value = false
    status.value = result.status
    error.value = false
    loadedAt = Date.now()
    updateUserPoints(result.points, identity)
    scheduleReset()
    return result
  } finally {
    if (version === generation) claiming.value = false
  }
}

export function useDailyCheckIn() {
  return { status: readonly(status), loading: readonly(loading), claiming: readonly(claiming), error: readonly(error), refresh: refreshDailyCheckIn, claim }
}
