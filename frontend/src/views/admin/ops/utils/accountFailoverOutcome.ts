export interface AccountFailoverOutcome {
  result: 'success' | 'failed' | 'canceled'
  final_account_id?: number
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
      return {
        result: outcome.result,
        final_account_id: outcome.result === 'success' && Number.isSafeInteger(outcome.final_account_id) && outcome.final_account_id > 0
          ? outcome.final_account_id : undefined
      }
    }
  } catch {
    // Old diagnostics can contain plain text or truncated JSON.
  }
  return null
}
