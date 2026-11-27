import { createDefaultLotteryClient, unwrap } from './lotteryClient'

export interface EventDefinition {
  code: string
  name: string
  group: string
  status: 'active' | 'reserved' | 'internal'
  fields: string[]
  description: string
}
export interface EventCondition {
  field: string
  operator: 'eq' | 'ne' | 'in' | 'contains' | 'gt' | 'gte' | 'lt' | 'lte'
  value: string
}
export interface EventAction {
  kind: 'grant_points' | 'grant_item' | 'item_effect'
  points: number
  item_id: number
  quantity: number
  consume_item: boolean
}
export interface EventRule {
  id: number
  version: number
  name: string
  event_type: string
  enabled: boolean
  probability: number
  recipient: 'user' | 'inviter' | 'invitee' | 'actor'
  conditions: EventCondition[]
  action: EventAction
  user_limit: number
  daily_limit: number
  total_limit: number
  cooldown_seconds: number
  every_n: number
  starts_at: string
  ends_at: string
  total_triggered: number
}
export interface EventSettings {
  enabled: boolean
  settlement_disabled: boolean
  pending: number
  failed: number
  processed: number
}
export interface EventLog {
  id: string | number
  event_id?: string
  event_type: string
  user_id: number
  status: string
  rule_name?: string
  rule_version?: number
  action?: string
  reason?: string
  amount?: string
  item_id?: number
  attempts?: number
  last_error?: string
  created_at: string
  payload?: { reference?: string; source?: string; occurred_at?: string; data?: Record<string, string> }
}
export interface EventLogPage { items: EventLog[]; total: number; page: number; page_size: number }

const client = createDefaultLotteryClient()
export async function fetchEventCatalog(): Promise<EventDefinition[]> {
  return (await unwrap<{ items: EventDefinition[] }>(client.http.get('/v1/admin/events/catalog'))).items
}
export async function fetchEventRules(): Promise<EventRule[]> {
  return (await unwrap<{ items: EventRule[] }>(client.http.get('/v1/admin/events/rules'))).items
}
export function saveEventRule(rule: EventRule): Promise<EventRule> {
  return unwrap(rule.id ? client.http.put(`/v1/admin/events/rules/${rule.id}`, rule) : client.http.post('/v1/admin/events/rules', rule))
}
export function disableEventRule(id: number): Promise<{ disabled: boolean }> {
  return unwrap(client.http.delete(`/v1/admin/events/rules/${id}`))
}
export function fetchEventSettings(): Promise<EventSettings> {
  return unwrap(client.http.get('/v1/admin/events/settings'))
}
export function saveEventSettings(enabled: boolean): Promise<EventSettings> {
  return unwrap(client.http.put('/v1/admin/events/settings', { enabled }))
}
export function fetchEventLogs(executions: boolean, eventType: string, status: string, page: number): Promise<EventLogPage> {
  return unwrap(client.http.get(`/v1/admin/events/${executions ? 'executions' : 'logs'}`, {
    params: { event_type: eventType, status, page, page_size: 20 }
  }))
}

export function blankEventRule(eventType = ''): EventRule {
  return {
    id: 0, version: 0, name: '', event_type: eventType, enabled: false, probability: 1000000,
    recipient: 'user', conditions: [], action: { kind: 'grant_points', points: 10, item_id: 0, quantity: 1, consume_item: false },
    user_limit: 0, daily_limit: 0, total_limit: 0, cooldown_seconds: 0, every_n: 1,
    starts_at: '', ends_at: '', total_triggered: 0
  }
}
