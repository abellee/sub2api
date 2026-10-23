import type { BillingMode as BaseBillingMode } from '@/constants/channel'

export { BILLING_MODE_TOKEN, BILLING_MODE_IMAGE } from '@/constants/channel'
export const BILLING_MODE_VIDEO = 'video' as const
export type BillingMode = BaseBillingMode | typeof BILLING_MODE_VIDEO
