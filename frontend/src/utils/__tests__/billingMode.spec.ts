import { describe, expect, it } from 'vitest'
import {
  BILLING_MODE_CONTENT_SAFETY,
  BILLING_MODE_IMAGE,
  BILLING_MODE_TOKEN,
  BILLING_MODE_VIDEO,
  getDisplayBillingMode,
  isContentSafetyUsage,
  isImageUsage
} from '../billingMode'

describe('billingMode helpers', () => {
  it('prefers explicit video mode over image_count', () => {
    expect(
      getDisplayBillingMode({ image_count: 1, billing_mode: BILLING_MODE_VIDEO })
    ).toBe(BILLING_MODE_VIDEO)
    expect(isImageUsage({ image_count: 1, billing_mode: BILLING_MODE_VIDEO })).toBe(false)
  })

  it('infers image when image_count set and mode missing', () => {
    expect(getDisplayBillingMode({ image_count: 2, billing_mode: null })).toBe(BILLING_MODE_IMAGE)
  })

  it('keeps token mode even with image_count', () => {
    expect(
      getDisplayBillingMode({ image_count: 1, billing_mode: BILLING_MODE_TOKEN })
    ).toBe(BILLING_MODE_TOKEN)
  })

  it('keeps a content safety charge out of image billing', () => {
    expect(getDisplayBillingMode({ image_count: 0, billing_mode: BILLING_MODE_CONTENT_SAFETY })).toBe(BILLING_MODE_CONTENT_SAFETY)
    expect(isImageUsage({ image_count: 0, billing_mode: BILLING_MODE_CONTENT_SAFETY })).toBe(false)
    expect(isContentSafetyUsage({ model: 'content-safety', billing_mode: BILLING_MODE_CONTENT_SAFETY })).toBe(true)
  })
})
