import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import StudioFaceSvg from '../StudioFaceSvg.vue'

describe('StudioFaceSvg', () => {
  it('paints face color from status tone, not mood', () => {
    const healthy = mount(StudioFaceSvg, { props: { mood: 'sad', tone: 'healthy' } })
    expect(healthy.find('circle').attributes('fill')).toBe('#6EE7B7')

    const warning = mount(StudioFaceSvg, { props: { mood: 'joy', tone: 'warning' } })
    expect(warning.find('circle').attributes('fill')).toBe('#FDBA74')

    const critical = mount(StudioFaceSvg, { props: { mood: 'smile', tone: 'critical' } })
    expect(critical.find('circle').attributes('fill')).toBe('#FCA5A5')

    const unknown = mount(StudioFaceSvg, { props: { mood: 'happy', tone: 'unknown' } })
    expect(unknown.find('circle').attributes('fill')).toBe('#E5E7EB')
  })

  it('uses a darker fill when the score is higher', () => {
    const low = mount(StudioFaceSvg, { props: { mood: 'smile', tone: 'healthy', score: 20 } })
    const high = mount(StudioFaceSvg, { props: { mood: 'smile', tone: 'healthy', score: 96 } })
    const lum = (hex: string) => {
      const h = hex.replace('#', '')
      const r = parseInt(h.slice(0, 2), 16)
      const g = parseInt(h.slice(2, 4), 16)
      const b = parseInt(h.slice(4, 6), 16)
      return 0.2126 * r + 0.7152 * g + 0.0722 * b
    }
    expect(lum(high.find('circle').attributes('fill') || '')).toBeLessThan(
      lum(low.find('circle').attributes('fill') || ''),
    )
  })
})
