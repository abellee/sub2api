import type { LatencyEvent } from '@/api/admin/latencyCollections'

const starts: Record<string, string> = {
  body_read_complete: 'body_read_start',
  connection_ready: 'connection_wait_start',
  dns_done: 'dns_start', connect_done: 'connect_start', tls_done: 'tls_start',
  upload_complete: 'connection_ready', first_response_byte: 'upload_complete',
  first_body_byte: 'upstream_headers', first_semantic_output: 'upload_complete',
  downstream_first_write: 'downstream_write_start',
  user_slot_done: 'user_slot_start', account_slot_done: 'account_slot_start'
}
export function latencyStageDuration(events: LatencyEvent[], event: LatencyEvent): number | undefined {
  if (event.duration_ms !== undefined) return event.duration_ms
  const startName = starts[event.name]
  if (!startName) return undefined
  const start = events.find(item => item.name === startName && (item.attempt || 0) === (event.attempt || 0))
  if (!start || event.at_ms < start.at_ms) return undefined
  return event.at_ms - start.at_ms
}
