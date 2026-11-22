// Group API stores USD per 1000 searches; Gemini's editor displays USD per query.
// Round at the boundary to avoid showing binary conversion artifacts.
export function searchPricePerCall(price: number | string | null | undefined): string {
  if (price === '' || price == null) return ''
  return (Number(price) / 1000).toFixed(6)
}

export function searchPricePer1k(value: string): number | null {
  if (value.trim() === '') return null
  const price = Number(value)
  return Number.isFinite(price) && price >= 0 ? Number((price * 1000).toFixed(6)) : null
}

export function searchPricePreview(price: number | string | null | undefined, rate: number): string {
  if (price === '' || price == null) return '—'
  return '$' + ((Number(price) / 1000) * rate).toFixed(6)
}
