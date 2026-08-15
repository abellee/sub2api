import type { ModelPlazaGroup } from '@/api/modelPlaza'

export interface ModelPlazaEntry {
  id: string
  provider: string
  input: number | null
  output: number | null
  cache: number | null
  groupId: number
  groupName: string
  rateMultiplier: number
}

export function buildModelPlazaEntries(
  groups: ModelPlazaGroup[],
): ModelPlazaEntry[] {
  return groups.flatMap((group) => {
    const seen = new Set<string>()
    return group.models.flatMap((model) => {
      const id = model.name.trim()
      if (seen.has(id)) return []
      seen.add(id)
      const official = model.official_pricing
      return [{
        id,
        provider: normalizeProvider(model.platform || group.platform),
        input: perMillion(official?.input_price),
        output: perMillion(official?.output_price),
        cache: perMillion(official?.cache_read_price ?? official?.cache_write_price),
        groupId: group.id,
        groupName: group.name,
        rateMultiplier: group.user_rate_multiplier ?? group.rate_multiplier,
      }]
    })
  })
}

export function formatActualModelPrice(value: number | null, rateMultiplier: number): string {
  if (value == null || !Number.isFinite(value) || !Number.isFinite(rateMultiplier)) return '-'

  const left = toScaledInteger(value)
  const right = toScaledInteger(rateMultiplier)
  const product = left.integer * right.integer
  const scale = left.scale + right.scale
  const negative = product < 0n
  const digits = (negative ? -product : product).toString().padStart(scale + 1, '0')
  const integerPart = scale === 0 ? digits : digits.slice(0, -scale)
  const rawFraction = scale === 0 ? '' : digits.slice(-scale)
  const fraction = rawFraction.replace(/0+$/, '')
  const groupedInteger = integerPart.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  const paddedFraction = fraction.length < 2 ? fraction.padEnd(2, '0') : fraction

  return `${negative ? '-' : ''}${groupedInteger}.${paddedFraction}`
}

function toScaledInteger(value: number): { integer: bigint; scale: number } {
  const negative = value < 0
  const [coefficient, exponentText = '0'] = Math.abs(value).toString().toLowerCase().split('e')
  const [integerPart, fractionPart = ''] = coefficient.split('.')
  let digits = `${integerPart}${fractionPart}`.replace(/^0+(?=\d)/, '')
  let scale = fractionPart.length - Number(exponentText)

  if (scale < 0) {
    digits += '0'.repeat(-scale)
    scale = 0
  }

  const integer = BigInt(digits || '0')
  return { integer: negative ? -integer : integer, scale }
}

function perMillion(value: number | null | undefined) {
  return typeof value === 'number' ? value * 1_000_000 : null
}

function normalizeProvider(provider: string) {
  const value = provider.toLowerCase()
  return value === 'google' ? 'gemini' : value
}
