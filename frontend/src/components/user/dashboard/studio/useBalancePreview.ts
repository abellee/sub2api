import { computed, onMounted, onScopeDispose, ref } from 'vue'

const simulatedBalance = ref<number | null>(null)
const samples = [12.5, 8.76, 4.32, 9.99, 10, 102.45, 7.65]
let subscribers = 0
let timer: ReturnType<typeof setInterval> | undefined

export function useBalancePreview(source: () => number) {
  const enabled = import.meta.env.DEV && new URLSearchParams(window.location.search).get('balanceDemo') === '1'
  let subscribed = false
  onMounted(() => {
    if (!enabled) return
    subscribed = true
    subscribers++
    if (timer !== undefined) return
    let index = 0
    simulatedBalance.value = samples[index]!
    timer = setInterval(() => {
      index = (index + 1) % samples.length
      simulatedBalance.value = samples[index]!
    }, 3000)
  })
  onScopeDispose(() => {
    if (!subscribed) return
    subscribers--
    if (subscribers === 0) {
      clearInterval(timer)
      timer = undefined
      simulatedBalance.value = null
    }
  })
  return { balanceDemo: enabled, balance: computed(() => enabled && simulatedBalance.value !== null ? simulatedBalance.value : source()) }
}
