import { describe, expect, it } from 'vitest'
import { createPieGeometry, modelSlices } from '../modelPie'
import type { ModelStat } from '@/types'

const model = (name: string, tokens: number): ModelStat => ({ model: name, total_tokens: tokens, requests: 1, input_tokens: tokens, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, cost: 0, actual_cost: 0 })

describe('model pie', () => {
  it('uses native total_tokens without changing order or source data', () => {
    const models = [model('a', 25), model('b', 0), model('c', 75)]
    const original = JSON.stringify(models)
    const slices = modelSlices(models)
    expect(slices.map(s => s.share)).toEqual([0.25, 0, 0.75])
    expect(slices.map(s => s.model.model)).toEqual(['a', 'b', 'c'])
    expect(slices[2].end).toBeCloseTo(Math.PI * 2)
    expect(JSON.stringify(models)).toBe(original)
  })

  it('does not fabricate slices for an empty or zero-token account', () => {
    expect(modelSlices([])).toEqual([])
    expect(modelSlices([model('a', 0), model('b', 0)]).map(s => s.share)).toEqual([0, 0])
  })

  it('ignores invalid token weights instead of generating invalid vertices', () => {
    const slices = modelSlices([model('a', -1), model('b', NaN), model('c', 10)])
    expect(slices.map(s => s.share)).toEqual([0, 0, 1])
  })

  it.each([[0, Math.PI * 2], [Math.PI / 2, Math.PI], [0, 0.0001]])('builds a closed finite 3D sector from %s to %s', (start, end) => {
    const geometry = createPieGeometry(start, end)
    geometry.computeBoundingBox()
    const box = geometry.boundingBox!
    expect(Array.from(geometry.getAttribute('position').array).every(Number.isFinite)).toBe(true)
    expect(box.max.y - box.min.y).toBeCloseTo(0.8)
    expect(geometry.getAttribute('position').count).toBeGreaterThan(0)
    const colors = Array.from(geometry.getAttribute('color').array)
    expect(colors.every(Number.isFinite)).toBe(true)
    expect(Math.max(...colors) - Math.min(...colors)).toBeGreaterThan(0.1)
    if (end === Math.PI * 2) {
      expect(box.max.x - box.min.x).toBeCloseTo(4, 2)
      expect(box.max.z - box.min.z).toBeCloseTo(4, 2)
    }
    geometry.dispose()
  })
})
