import { apiClient, buildApiUrl } from '@/api/client'
import { ADMIN_UI_REQUEST_HEADER } from '@/api/adminUIRequest'

export async function supportsAccountInflation(): Promise<boolean> {
  try {
    const { data } = await apiClient.get<{ version: number }>('/admin/accounts/inflation-capabilities')
    return data.version === 1
  } catch { return false }
}

export interface InflationMeasurement {
  token_count: number
  output_tokens: number | null
  output_field?: string
  reasoning_tokens: number | null
  reasoning_text_tokens?: number
  reasoning_redacted?: boolean
  encoding: string
  error?: string
}

export interface InflationEvent {
  type: string
  text?: string
  model?: string
  error?: string
  success?: boolean
  data?: InflationMeasurement
}

export function inflationRate(measurement: InflationMeasurement): number | null {
  if (measurement.error || measurement.token_count <= 0 || measurement.output_tokens === null) return null
  return (measurement.output_tokens / measurement.token_count - 1) * 100
}

export async function probeAccountInflation(
  id: number,
  options: { model: string; prompt: string; encoding: string },
  onEvent: (event: InflationEvent) => void,
  signal: AbortSignal
): Promise<void> {
  const response = await fetch(buildApiUrl(`/admin/accounts/${id}/test`), {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${localStorage.getItem('auth_token') || ''}`,
      'Content-Type': 'application/json',
      [ADMIN_UI_REQUEST_HEADER]: '1'
    },
    body: JSON.stringify({
      model_id: options.model,
      prompt: options.prompt,
      mode: 'text',
      measure_tokens: true,
      tokenizer_encoding: options.encoding
    }),
    signal
  })
  if (!response.ok) throw new Error(`HTTP ${response.status}`)
  if (!response.headers.get('content-type')?.includes('text/event-stream')) {
    throw new Error('Expected an SSE response')
  }
  const reader = response.body?.getReader()
  if (!reader) throw new Error('Empty response body')
  const decoder = new TextDecoder()
  let buffer = ''
  let dataLines: string[] = []
  const dispatch = () => {
    if (dataLines.length) onEvent(JSON.parse(dataLines.join('\n')) as InflationEvent)
    dataLines = []
  }
  const line = (value: string) => {
    value = value.replace(/\r$/, '')
    if (!value) dispatch()
    else if (value.startsWith('data:')) dataLines.push(value.slice(5).replace(/^ /, ''))
  }
  try {
    while (true) {
      const { value, done } = await reader.read()
      buffer += done ? decoder.decode() : decoder.decode(value, { stream: true })
      let end: number
      while ((end = buffer.indexOf('\n')) >= 0) {
        line(buffer.slice(0, end))
        buffer = buffer.slice(end + 1)
      }
      if (done) break
    }
    if (buffer) line(buffer)
    dispatch()
  } finally {
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
}
