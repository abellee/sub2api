import { describe, expect, it } from 'vitest'
import { accountFailoverOutcome } from '../accountFailoverOutcome'

describe('account failover outcome', () => {
  it('shows the actual successful account instead of the last failed account', () => {
    expect(accountFailoverOutcome(JSON.stringify([{ account_id: 333, failover_result: { result: 'success', final_account_id: 12 } }])))
      .toEqual({ result: 'success', final_account_id: 12 })
  })
  it('does not infer success from old or malformed records', () => {
    for (const raw of [undefined, '', 'bad json', '{}', 'null', '[null]', '[{"upstream_status_code":200}]']) {
      expect(accountFailoverOutcome(raw)).toBeNull()
    }
  })
  it('preserves the final account separately from the outcome', () => {
    for (const result of ['failed', 'canceled']) {
      expect(accountFailoverOutcome(JSON.stringify([{ failover_result: { result, final_account_id: 404 } }])))
        .toEqual({ result, final_account_id: 404 })
    }
  })
  it('preserves ordered attempts, including the successful final account', () => {
    const attempts = [
      { account_id: 31, result: 'failed', status_code: 502 },
      { account_id: 37, result: 'success', status_code: 200 }
    ]
    expect(accountFailoverOutcome(JSON.stringify([{ failover_result: { result: 'success', final_account_id: 37, attempts: [null, ...attempts] } }]))?.attempts)
      .toEqual(attempts)
  })
  it('uses the confirmed upstream failure for a contradictory old final attempt', () => {
    const raw = JSON.stringify([{ account_id: 403, upstream_status_code: 502, message: 'upstream unavailable', failover_result: {
      result: 'failed', final_account_id: 403, attempts: [{ account_id: 403, result: 'success', status_code: 200 }]
    } }])
    expect(accountFailoverOutcome(raw)?.attempts).toEqual([
      { account_id: 403, result: 'failed', status_code: 502, message: 'upstream unavailable' }
    ])
  })
  it('does not rewrite a recovered request or borrow another account error', () => {
    for (const [result, account] of [['success', 403], ['failed', 305]]) {
      const raw = JSON.stringify([{ account_id: account, upstream_status_code: 502, failover_result: {
        result, final_account_id: 403, attempts: [{ account_id: 403, result: 'success', status_code: 200 }]
      } }])
      expect(accountFailoverOutcome(raw)?.attempts?.[0].result).toBe('success')
    }
  })

})
