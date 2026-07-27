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

export interface HeaderWidgetStatus {
  id: string
  name: string
  indicator: string
  description: string
  checkedAt: string
}

export const useRemoteWidgetsStore = defineStore('remoteWidgets', () => {
  const headerConfig = ref<HeaderWidgetConfig | null>(null)
  const headerStatuses = ref<HeaderWidgetStatus[]>([])
  const headerCountdown = ref(0)
  const sideVisible = ref(false)
  const sideHasIcon = ref(false)

  function setHeaderData(
    config: HeaderWidgetConfig,
    statuses: HeaderWidgetStatus[]
  ): void {
    headerConfig.value = config
    headerStatuses.value = statuses
    headerCountdown.value = Math.max(1, Math.ceil(config.refreshInterval / 1000))
  }

  function tickHeaderCountdown(): void {
    if (headerCountdown.value > 0) headerCountdown.value -= 1
  }

  function setSideState(visible: boolean, hasIcon: boolean): void {
    sideVisible.value = visible
    sideHasIcon.value = hasIcon
  }

  return {
    headerConfig,
    headerStatuses,
    headerCountdown,
    sideVisible,
    sideHasIcon,
    setHeaderData,
    tickHeaderCountdown,
    setSideState
  }
})
