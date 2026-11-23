export interface UsageTier {
  daily_cost: number
  consecutive_days: number
  max_missed_days: number
  rate_multiplier: number
}

export interface GroupUsageTierConfig {
  enabled: boolean
  tiers: UsageTier[]
  linked_group_ids: number[]
  excluded_user_ids: number[]
}

export interface GroupUsageTierProgress {
  source_group_id: number
  source_group_name: string
  tiers: UsageTier[]
  excluded: boolean
  stage: number
  streak: number
  missed: number
  today_cost: number
  today_qualified: boolean
  promoted_today: boolean
  rate_multiplier?: number
}
