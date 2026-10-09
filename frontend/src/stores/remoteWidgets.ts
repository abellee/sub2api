import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface HeaderWidgetConfig {
  qq: {
    groupName: string
    groupNumber: string
    groupLink: string
    logoUrl: string
    qrUrl: string
  }
  telegram: {
    groupName: string
    link: string
    logoUrl: string
    qrUrl: string
  }
  refreshInterval: number
}

export const useRemoteWidgetsStore = defineStore('remoteWidgets', () => {
  const headerConfig = ref<HeaderWidgetConfig | null>(null)
  const sideVisible = ref(false)
  const sideHasIcon = ref(false)

  function setHeaderData(config: HeaderWidgetConfig): void {
    headerConfig.value = config
  }

  function setSideState(visible: boolean, hasIcon: boolean): void {
    sideVisible.value = visible
    sideHasIcon.value = hasIcon
  }

  return {
    headerConfig,
    sideVisible,
    sideHasIcon,
    setHeaderData,
    setSideState
  }
})
