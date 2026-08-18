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
