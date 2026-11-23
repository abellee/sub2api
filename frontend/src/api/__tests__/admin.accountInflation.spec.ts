import { afterEach, describe, expect, it, vi } from 'vitest'
import { inflationRate, probeAccountInflation, probeDirectInflation, supportsAccountInflation, type InflationEvent } from '../admin/accountInflation'
import { apiClient } from '@/api/client'

vi.mock('@/api/client', () => ({ buildApiUrl: (path: string) => `/api/v1${path}`, apiClient: { get: vi.fn() } }))
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

  it('sends direct credentials in the POST body with the configured timeout', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('data: {"type":"test_complete","success":true}\n\n', { headers: { 'content-type': 'text/event-stream' } }))
    vi.stubGlobal('fetch', fetchMock)
    const target = { base_url: 'https://example.test/v1', api_key: 'test-secret', protocol: 'anthropic' as const }
    await probeDirectInflation(target, { model: 'model', prompt: 'same prompt', encoding: 'cl100k_base', timeoutSeconds: 35 }, () => {}, new AbortController().signal)
    expect(fetchMock.mock.calls[0]![0]).toBe('/api/v1/admin/accounts/inflation-probe')
    expect(JSON.parse(fetchMock.mock.calls[0]![1].body)).toMatchObject({ ...target, timeout_seconds: 35, model_id: 'model', prompt: 'same prompt' })
    expect(fetchMock.mock.calls[0]![0]).not.toContain('test-secret')
  })

  it('does not treat older account-only backends as supporting direct mode', async () => {
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: { version: 1 } })
    expect(await supportsAccountInflation(true)).toBe(false)
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: { version: 1, direct_probe: true } })
    expect(await supportsAccountInflation(true)).toBe(true)
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
