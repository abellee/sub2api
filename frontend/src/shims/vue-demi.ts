// vue-demi's Vue 3 surface. The local dependency install can omit the
// package's generated bridge, while Pinia still imports its compatibility
// exports (`set`, `del`, and version flags).
export * from 'vue'

export const isVue2 = false
export const isVue3 = true

export function set<T extends object, K extends keyof T>(target: T, key: K, value: T[K]): T[K] {
  target[key] = value
  return value
}

export function del<T extends object, K extends keyof T>(target: T, key: K): void {
  delete target[key]
}
