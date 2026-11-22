import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getUserMenu, saveUserMenu, type UserMenuConfig } from '@/api/userMenu'

export const useUserMenuStore = defineStore('userMenu', () => {
  const config = ref<UserMenuConfig>({ sections: [] })
  const loaded = ref(false)
  let pending: Promise<UserMenuConfig> | null = null

  async function load(force = false, admin = false): Promise<UserMenuConfig> {
    if (pending) return pending
    if (loaded.value && !force) return config.value
    pending = getUserMenu(admin).then(value => {
      config.value = value
      loaded.value = true
      return value
    }).finally(() => { pending = null })
    return pending
  }

  async function save(value: UserMenuConfig): Promise<UserMenuConfig> {
    const saved = await saveUserMenu(value)
    config.value = saved
    loaded.value = true
    return saved
  }

  return { config, loaded, load, save }
})
