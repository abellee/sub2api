import Decimal from 'decimal.js'

export type PricingValue = Decimal.Value

/** Format a decimal without scientific notation, preserving every meaningful digit. */
export function formatDecimal(
  value: PricingValue | null | undefined,
  minFractionDigits = 0,
  groupThousands = false,
): string {
  if (value == null) return '-'

  let decimal: Decimal
  try {
    decimal = new Decimal(value)
  } catch {
    return '-'
  }
  if (!decimal.isFinite()) return '-'

  const [integerPart, fractionPart = ''] = decimal.toFixed().split('.')
  const paddedFraction = fractionPart.length < minFractionDigits
    ? fractionPart.padEnd(minFractionDigits, '0')
    : fractionPart
  const groupedInteger = groupThousands
    ? integerPart.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
    : integerPart
  return paddedFraction ? `${groupedInteger}.${paddedFraction}` : groupedInteger
}

/** Scale and multiply a price entirely in decimal arithmetic before formatting. */
export function formatScaled(
  value: PricingValue | null | undefined,
  scale: PricingValue,
  minFractionDigits = 0,
  multiplier: PricingValue = 1,
): string {
  if (value == null) return '-'
  try {
    const scaled = new Decimal(value).mul(scale).mul(multiplier)
    return `$${formatDecimal(scaled, minFractionDigits)}`
  } catch {
    return '-'
  }
}

import type { UserPricingInterval } from '@/api/channels'

type TokenPrices = Pick<UserPricingInterval, 'input_price' | 'output_price' | 'cache_write_price' | 'cache_write_1h_price' | 'cache_read_price'>

export function resolveIntervalPrices(iv: UserPricingInterval, base: TokenPrices): UserPricingInterval {
  const price = (absolute: number | null | undefined, multiplier: number | null | undefined, fallback: number | null | undefined) =>
    absolute ?? (fallback == null ? null : fallback * (multiplier ?? 1))
  return {
    ...iv,
    input_price: price(iv.input_price, iv.input_multiplier, base.input_price),
    output_price: price(iv.output_price, iv.output_multiplier, base.output_price),
    cache_write_price: price(iv.cache_write_price, iv.cache_write_multiplier, base.cache_write_price),
    // Resolver uses an explicit cache-write price for both durations unless 1h is overridden.
    cache_write_1h_price: iv.cache_write_1h_price ?? iv.cache_write_price ?? price(null, iv.cache_write_multiplier, base.cache_write_1h_price),
    cache_read_price: price(iv.cache_read_price, iv.cache_read_multiplier, base.cache_read_price)
  }
}
