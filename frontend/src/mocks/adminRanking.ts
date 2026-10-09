import type { UserBreakdownItem } from '@/types'
import type { UserBreakdownParams, UserBreakdownResponse } from '@/api/admin/dashboard'

export const isAdminRankingMockEnabled = import.meta.env.DEV &&
  import.meta.env.VITE_ENABLE_RANKING_MOCK === 'true'

const parseDate = (value?: string): Date | null => {
  if (!value) return null
  const date = new Date(`${value}T00:00:00`)
  return Number.isNaN(date.getTime()) ? null : date
}

const getRangeDays = (params: UserBreakdownParams): number => {
  const start = parseDate(params.start_date)
  const end = parseDate(params.end_date)
  if (!start || !end) return 1
  return Math.max(1, Math.floor((end.getTime() - start.getTime()) / 86400000) + 1)
}

const baseUsers: Array<Pick<UserBreakdownItem, 'user_id' | 'email' | 'requests' | 'input_tokens' | 'output_tokens' | 'cache_tokens' | 'actual_cost'>> = [
  { user_id: 101, email: 'alice@demo.local', requests: 42, input_tokens: 12800, output_tokens: 6400, cache_tokens: 2100, actual_cost: 12.48 },
  { user_id: 102, email: 'bob@demo.local', requests: 31, input_tokens: 9200, output_tokens: 5100, cache_tokens: 1700, actual_cost: 9.16 },
  { user_id: 103, email: 'carol@demo.local', requests: 27, input_tokens: 7400, output_tokens: 3900, cache_tokens: 1200, actual_cost: 6.73 },
  { user_id: 104, email: 'david@demo.local', requests: 18, input_tokens: 5100, output_tokens: 2500, cache_tokens: 800, actual_cost: 4.29 },
  { user_id: 105, email: 'erin@demo.local', requests: 11, input_tokens: 2800, output_tokens: 1300, cache_tokens: 400, actual_cost: 2.18 },
]

export function getMockUserBreakdown(params: UserBreakdownParams): UserBreakdownResponse {
  const rangeDays = getRangeDays(params)
  const scale = Math.max(1, Math.min(rangeDays, 30) / 1.5)
  const users = baseUsers.map((user) => {
    const requests = Math.max(1, Math.round(user.requests * scale))
    const inputTokens = Math.round(user.input_tokens * scale)
    const outputTokens = Math.round(user.output_tokens * scale)
    const cacheTokens = Math.round(user.cache_tokens * scale)
    return {
      ...user,
      requests,
      input_tokens: inputTokens,
      output_tokens: outputTokens,
      cache_tokens: cacheTokens,
      total_tokens: inputTokens + outputTokens + cacheTokens,
      actual_cost: Number((user.actual_cost * scale).toFixed(4)),
      cost: Number((user.actual_cost * scale * 1.12).toFixed(4)),
      account_cost: Number((user.actual_cost * scale * 0.68).toFixed(4)),
    }
  })

  const sortBy = params.sort_by === 'actual_cost' || params.sort_by === 'cost'
    ? 'actual_cost'
    : params.sort_by || 'total_tokens'
  users.sort((left, right) => Number(right[sortBy]) - Number(left[sortBy]))

  return {
    users: params.limit === 0 ? users : users.slice(0, params.limit || 50),
    start_date: params.start_date || '',
    end_date: params.end_date || '',
  }
}
