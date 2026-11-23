import { afterEach, describe, expect, it, vi } from 'vitest'
import { inflationRate, probeAccountInflation, type InflationEvent } from '../admin/accountInflation'

vi.mock('@/api/client', () => ({ buildApiUrl: (path: string) => `/api/v1${path}`, apiClient: {} }))
afterEach(() => vi.unstubAllGlobals())

describe('account inflation', () => {
  it('calculates extra output as a percentage and keeps missing usage distinct from zero', () => {
    const base = { token_count: 2, output_tokens: 4, reasoning_tokens: null, encoding: 'o200k_base' }
    expect(inflationRate(base)).toBe(100)
    expect(inflationRate({ ...base, output_tokens: 2 })).toBe(0)
    expect(inflationRate({ ...base, output_tokens: 0 })).toBe(-100)
    expect(inflationRate({ ...base, output_tokens: null })).toBeNull()
    expect(inflationRate({ ...base, token_count: 0 })).toBeNull()
    expect(inflationRate({ ...base, error: 'Tokenizer unavailable' })).toBeNull()
  })

  it('reads fragmented UTF-8 SSE including CRLF and the last unterminated event', async () => {
    const encoded = new TextEncoder().encode('data:{"type":"content","text":"中文"}\r\n\r\ndata: {"type":"test_complete","success":true}')
    const fetchMock = vi.fn().mockResolvedValue(new Response(new ReadableStream({
      start(controller) {
        for (let i = 0; i < encoded.length; i += 2) controller.enqueue(encoded.slice(i, i + 2))
        controller.close()
      }
    }), { headers: { 'content-type': 'text/event-stream' } }))
    vi.stubGlobal('fetch', fetchMock)
    const events: InflationEvent[] = []
    await probeAccountInflation(2, { model: 'test-model', prompt: 'shared prompt', encoding: 'o200k_base' }, e => events.push(e), new AbortController().signal)
    expect(events).toEqual([{ type: 'content', text: '中文' }, { type: 'test_complete', success: true }])
    expect(JSON.parse(fetchMock.mock.calls[0]![1].body)).toMatchObject({ measure_tokens: true, prompt: 'shared prompt', model_id: 'test-model' })
  })
})
