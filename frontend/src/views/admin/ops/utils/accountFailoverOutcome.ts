export interface AccountFailoverOutcome {
  result: 'success' | 'failed' | 'canceled'
  final_account_id?: number
  attempts?: AccountFailoverAttempt[]
}

export interface AccountFailoverAttempt {
  account_id: number
  account_name?: string
  result: 'success' | 'failed' | 'canceled'
  status_code?: number
  message?: string
  started_at_unix_ms?: number
  finished_at_unix_ms?: number
}

// Historical rows have no confirmed final outcome; never infer it from HTTP 200.
export function accountFailoverOutcome(raw?: string): AccountFailoverOutcome | null {
  if (!raw) return null
  try {
    const events: unknown = JSON.parse(raw)
    if (!Array.isArray(events)) return null
    for (let i = events.length - 1; i >= 0; i--) {
      const outcome = events[i]?.failover_result
      if (!outcome || !['success', 'failed', 'canceled'].includes(outcome.result)) continue
      const parsed: AccountFailoverOutcome = {
        result: outcome.result,
        final_account_id: Number.isSafeInteger(outcome.final_account_id) && outcome.final_account_id > 0
          ? outcome.final_account_id : undefined
      }
      if (Array.isArray(outcome.attempts)) {
        parsed.attempts = outcome.attempts.filter((attempt: AccountFailoverAttempt | null) =>
          attempt && Number.isSafeInteger(attempt.account_id) && attempt.account_id > 0 &&
          ['success', 'failed', 'canceled'].includes(attempt.result)
        )
      }
      const last = parsed.attempts?.[parsed.attempts.length - 1]
      const event = events[i]
      if (parsed.result !== 'success' && last?.result === 'success' &&
          last.account_id === parsed.final_account_id && event.account_id === last.account_id &&
          Number.isInteger(event.upstream_status_code) && event.upstream_status_code >= 400) {
        last.result = parsed.result
        last.status_code = parsed.result === 'canceled' ? 499 : event.upstream_status_code
        last.message = typeof event.message === 'string' ? event.message : undefined
      }
      return parsed
    }
  } catch {
    // Old diagnostics can contain plain text or truncated JSON.
  }
  return null
}
