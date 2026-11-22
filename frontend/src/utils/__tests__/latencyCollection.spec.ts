import { describe, expect, it } from 'vitest'
import { latencyStageDuration } from '../latencyCollection'

describe('latency stage durations', () => {
  it('keeps retries separate and leaves missing or overlapping boundaries unknown', () => {
    const events = [
      { name: 'upload_complete', at_ms: 100, attempt: 1 },
      { name: 'upload_complete', at_ms: 500, attempt: 2 },
      { name: 'first_response_byte', at_ms: 800, attempt: 2 }
    ]
    expect(latencyStageDuration(events, events[2]!)).toBe(300)
    expect(latencyStageDuration(events, { name: 'first_body_byte', at_ms: 900, attempt: 2 })).toBeUndefined()
    expect(latencyStageDuration(events, { name: 'first_response_byte', at_ms: 400, attempt: 2 })).toBeUndefined()
    expect(latencyStageDuration(events, { name: 'ops_auth_latency_ms', at_ms: 100, duration_ms: 0 })).toBe(0)
  })
})
