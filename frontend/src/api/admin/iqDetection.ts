import { apiClient } from '../client'

export interface IQDetectAgentHealth {
  enabled: boolean
  reason?: string
  base_url: string
  status?: string
  version?: string
  uptime_seconds?: number
}

// The IQ detection admin page is still being designed; for now the menu only
// surfaces the sidecar health. More proxied endpoints (models, runs, scores)
// will be added with the page itself.
export async function getHealth(): Promise<IQDetectAgentHealth> {
  const { data } = await apiClient.get<IQDetectAgentHealth>('/admin/iq-detection/agent/health')
  return data
}

const iqDetectionAPI = { getHealth }
export default iqDetectionAPI
