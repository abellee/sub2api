/** Official brand fills for studio logos / names. Do not edit PlatformIcon. */

const STUDIO_BRAND_FILL: Record<string, string> = {
  anthropic: '#D97757',
  openai: '#10A37F',
  gemini: '#3186FF',
  grok: '#111111',
  kimi: '#027AFF',
  zhipu: '#3859FF',
  deepseek: '#4D6BFE',
  minimax: '#F23F5D',
  antigravity: '#A855F7',
  composite: '#06B6D4',
}

export function studioBrandFill(platform?: string | null): string {
  if (platform && STUDIO_BRAND_FILL[platform]) return STUDIO_BRAND_FILL[platform]
  return '#14B8A6'
}

export function studioBrandIsInk(platform?: string | null): boolean {
  return platform === 'grok'
}
