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

export interface CheckInStatus {
  date: string
  day: number
  completed_days: number
  claimed: boolean
  rewards: number[]
  today_reward: number
  next_reset_at: string
}

export interface CheckInResult {
  status: CheckInStatus
  points: PointsSummary
  granted: boolean
}

export interface PointsSettings { daily_checkin_rewards: number[] }

export function fetchCheckInStatus(): Promise<CheckInStatus> {
  return unwrap(client.http.get('/v1/me/points/check-in'))
}

export function claimDailyCheckIn(): Promise<CheckInResult> {
  return unwrap(client.http.post('/v1/me/points/check-in'))
}

export function fetchPointsSettings(): Promise<PointsSettings> {
  return unwrap(client.http.get('/v1/admin/points/settings'))
}

export function savePointsSettings(settings: PointsSettings): Promise<PointsSettings> {
  return unwrap(client.http.put('/v1/admin/points/settings', settings))
}

export function fetchAdminUserPoints(userId: number): Promise<PointsSummary> {
  return unwrap(client.http.get(`/v1/admin/users/${userId}/points`))
}

export function rechargeUserPoints(userId: number, amount: string, notes: string, requestId: string): Promise<{ points: PointsSummary; granted: boolean }> {
  return unwrap(client.http.post(`/v1/admin/users/${userId}/points`, { amount, notes, request_id: requestId }))
}
