import { Color, ExtrudeGeometry, Float32BufferAttribute, Shape } from 'three'
import type { ModelStat } from '@/types'

export const modelColors = ['#4d7bfa', '#ff9657', '#35bfd0', '#8297f5', '#f5b94f', '#ef88aa', '#56b8ee', '#66c8a2']

export function modelSlices(models: ModelStat[]) {
  const values = models.map(model => Number.isFinite(model.total_tokens) ? Math.max(0, model.total_tokens) : 0)
  const total = values.reduce((sum, value) => sum + value, 0)
  let angle = 0
  return models.map((model, index) => {
    const start = angle
    const share = total > 0 ? values[index] / total : 0
    angle += share * Math.PI * 2
    return { model, share, start, end: angle, color: modelColors[index % modelColors.length] }
  })
}

export function createPieGeometry(start: number, end: number, color = modelColors[0]) {
  const span = end - start
  const fullCircle = span >= Math.PI * 2 - 0.000001
  const bevel = Math.min(0.055, fullCircle ? 0.055 : span * 0.12)
  const gap = fullCircle ? 0 : Math.min(0.018, span * 0.08)
  const radius = 2 - bevel
  const shape = new Shape()
  if (fullCircle) {
    shape.absarc(0, 0, radius, 0, Math.PI * 2, false)
  } else {
    shape.moveTo(0, 0)
    shape.lineTo(Math.cos(start + gap) * radius, Math.sin(start + gap) * radius)
    shape.absarc(0, 0, radius, start + gap, end - gap, false)
    shape.lineTo(0, 0)
  }
  const geometry = new ExtrudeGeometry(shape, {
    depth: 0.66, bevelEnabled: true, bevelThickness: 0.07, bevelSize: bevel, bevelSegments: 4,
    curveSegments: Math.max(3, Math.ceil(span * 32))
  })
  geometry.rotateX(-Math.PI / 2)
  const positions = geometry.getAttribute('position')
  const base = new Color(color)
  const highlight = base.clone().lerp(new Color('#ffffff'), 0.23)
  const shade = base.clone().multiplyScalar(0.72)
  const colors: number[] = []
  for (let i = 0; i < positions.count; i++) {
    const height = Math.max(0, Math.min(1, (positions.getY(i) + 0.07) / 0.8))
    const center = 1 - Math.min(1, Math.hypot(positions.getX(i), positions.getZ(i)) / 2)
    const tint = shade.clone().lerp(base, height).lerp(highlight, height * (0.16 + center * 0.42))
    colors.push(tint.r, tint.g, tint.b)
  }
  geometry.setAttribute('color', new Float32BufferAttribute(colors, 3))
  return geometry
}
