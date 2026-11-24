import { createDefaultLotteryClient, unwrap } from './lotteryClient'

export interface PointsSummary {
  balance: string
  total_earned: string
  total_spent: string
}

export interface PointsTransaction {
  id: string
  delta: string
  balance_after: string
  kind: string
  note: string
  created_at: string
}

export type PointsFilter = 'all' | 'grant' | 'spend'

export interface PointsHistory {
  items: PointsTransaction[]
  total: number
  page: number
  page_size: number
}

const client = createDefaultLotteryClient()

export function fetchMyPoints(): Promise<PointsSummary> {
  return unwrap(client.http.get('/v1/me/points'))
}

export function fetchMyPointsHistory(page: number, kind: PointsFilter): Promise<PointsHistory> {
  return unwrap(client.http.get('/v1/me/points/transactions', { params: { page, page_size: 10, kind } }))
}
