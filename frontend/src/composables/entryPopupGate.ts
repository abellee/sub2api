import { ref } from 'vue'

/** True while the entry tours are still running. Announcement, lottery, and task popups stay queued. */
export const entryPopupsHeld = ref(false)

const waiters: Array<() => void> = []

export function holdEntryPopups() {
  entryPopupsHeld.value = true
}

export function releaseEntryPopups() {
  if (!entryPopupsHeld.value && waiters.length === 0) return
  entryPopupsHeld.value = false
  const pending = waiters.splice(0, waiters.length)
  for (const waiter of pending) waiter()
}

/** Runs immediately when popups are already allowed. */
export function whenEntryPopupsReleased(fn: () => void) {
  if (!entryPopupsHeld.value) {
    fn()
    return
  }
  waiters.push(fn)
}
