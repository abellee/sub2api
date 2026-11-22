<template>
  <div class="model-distribution">
    <div class="model-stage">
      <StudioSceneBackdrop />
      <div class="stage-toolbar"><button v-if="total && !unavailable" type="button" class="reset-view" @click="resetView">{{ t('dashboard.studio.resetView') }}</button></div>
      <div ref="host" class="three-host" role="img" tabindex="0"
        :aria-label="t('dashboard.studio.sceneLabel')" @keydown="onKeydown" />
      <div v-if="!total || unavailable" class="scene-empty">
        <span class="empty-orbit" aria-hidden="true"></span>
        <p>{{ unavailable ? t('dashboard.studio.noWebgl') : t('dashboard.noDataAvailable') }}</p>
      </div>
      <div v-else class="scene-caption" aria-live="polite">
        <span>{{ active ? usageModelLabel(active.model.model, t) : t('dashboard.tokens') }}</span>
        <strong>{{ formatTokensK(active ? active.model.total_tokens : total) }}</strong>
        <small>{{ active ? `${(active.share * 100).toFixed(1)}%` : t('dashboard.studio.selectedRange') }}</small>
      </div>
      <p class="scene-hint">{{ t('dashboard.studio.sceneHint') }}</p>
    </div>
    <div class="model-details">
      <div class="legend-heading"><span>{{ t('dashboard.model') }}</span><span>Token / %</span></div>
      <div class="model-legend">
        <button v-for="(slice, index) in slices" :key="slice.model.model" type="button"
          class="model-row" :class="{ 'is-selected': selected === index }"
          :aria-pressed="selected === index" @click="select(index)" @mouseenter="hovered = index" @mouseleave="hovered = null"
          @focus="hovered = index" @blur="hovered = null">
          <i :style="{ background: slice.color }" aria-hidden="true"></i>
          <span class="model-name" :title="usageModelLabel(slice.model.model, t)">{{ usageModelLabel(slice.model.model, t) }}</span>
          <span class="model-amount">{{ formatTokensK(slice.model.total_tokens) }}<small>{{ (slice.share * 100).toFixed(1) }}%</small></span>
        </button>
        <p v-if="!slices.length" class="legend-empty">{{ t('dashboard.studio.modelsEmpty') }}</p>
      </div>
      <div class="model-facts" aria-live="polite">
        <span>{{ t('dashboard.requests') }}<b>{{ active ? formatNumberLocaleString(active.model.requests) : '-' }}</b></span>
        <span>{{ t('dashboard.actual') }}<b>{{ active ? `$${formatCostFixed(active.model.actual_cost)}` : '-' }}</b></span>
        <span>{{ t('dashboard.standard') }}<b>{{ active ? `$${formatCostFixed(active.model.cost)}` : '-' }}</b></span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { AmbientLight, Color, DirectionalLight, Group, Mesh, MeshPhysicalMaterial, PerspectiveCamera, Raycaster, Scene, Vector2, Vector3, BufferGeometry, WebGLRenderer, MOUSE, Spherical, PlaneGeometry, ShadowMaterial, PCFSoftShadowMap } from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'
import { useTheme } from '@/composables/useTheme'
import { usageModelLabel } from '@/utils/billingMode'
import { formatTokensK, formatCostFixed, formatNumberLocaleString } from '@/utils/format'
import type { ModelStat } from '@/types'
import { createPieGeometry, modelSlices } from './modelPie'
import StudioSceneBackdrop from './StudioSceneBackdrop.vue'

const props = defineProps<{ models: ModelStat[] }>()
const { t } = useI18n()
const { isDark } = useTheme()
const host = ref<HTMLDivElement>()
const unavailable = ref(false)
const selected = ref<number | null>(null)
const hovered = ref<number | null>(null)
const slices = computed(() => modelSlices(props.models))
const total = computed(() => slices.value.reduce((sum, slice) => sum + Math.max(0, slice.model.total_tokens || 0), 0))
const activeIndex = computed(() => hovered.value ?? selected.value)
const active = computed(() => activeIndex.value === null ? null : slices.value[activeIndex.value])
let renderer: WebGLRenderer | undefined
let controls: OrbitControls | undefined
let observer: ResizeObserver | undefined
let floor: Mesh<PlaneGeometry, ShadowMaterial> | undefined
let scene: Scene
let camera: PerspectiveCamera
const pie = new Group()
const raycaster = new Raycaster()
const pointer = new Vector2()
let press: { x: number; y: number } | null = null
let dragging = false
let pointerInside = false

function render() { if (renderer && scene && camera) renderer.render(scene, camera) }
function select(index: number) { selected.value = selected.value === index ? null : index }
function resetView() {
  if (!controls) return
  camera.position.set(4.8, 5.4, 6.8)
  controls.target.set(0, 0, 0)
  controls.update()
  render()
}
function disposeGroup(group: Group) {
  for (const object of [...group.children]) {
    if (object instanceof Mesh) {
      object.geometry.dispose()
      const materials = Array.isArray(object.material) ? object.material : [object.material]
      materials.forEach(material => material.dispose())
    }
    group.remove(object)
  }
}
function rebuild() {
  disposeGroup(pie)
  slices.value.forEach((slice, index) => {
    if (!slice.share) return
    const mesh = new Mesh(createPieGeometry(slice.start, slice.end, slice.color), new MeshPhysicalMaterial({
      color: '#ffffff', vertexColors: true, roughness: 0.35, metalness: 0, clearcoat: 0.32, clearcoatRoughness: 0.4,
      emissive: new Color(slice.color), emissiveIntensity: isDark.value ? 0.07 : 0.015
    }))
    mesh.castShadow = true
    mesh.userData.index = index
    pie.add(mesh)
  })
  highlight()
}
function highlight() {
  pie.children.forEach(object => {
    const mesh = object as Mesh<BufferGeometry, MeshPhysicalMaterial>
    const slice = slices.value[mesh.userData.index]
    const focused = mesh.userData.index === activeIndex.value
    const middle = (slice.start + slice.end) / 2
    const offset = focused && slices.value.filter(s => s.share > 0).length > 1 ? 0.13 : 0
    mesh.position.set(Math.cos(middle) * offset, focused ? 0.12 : 0, -Math.sin(middle) * offset)
    mesh.material.emissiveIntensity = focused ? 0.13 : isDark.value ? 0.07 : 0.015
  })
  render()
}
function hitTest(event: PointerEvent) {
  if (!host.value || !camera || dragging) return
  const box = host.value.getBoundingClientRect()
  pointer.set(((event.clientX - box.left) / box.width) * 2 - 1, -((event.clientY - box.top) / box.height) * 2 + 1)
  raycaster.setFromCamera(pointer, camera)
  const hit = raycaster.intersectObjects(pie.children)[0]
  hovered.value = hit ? hit.object.userData.index : null
}
function onMove(event: PointerEvent) {
  if (press && Math.hypot(event.clientX - press.x, event.clientY - press.y) > 4) { dragging = true; hovered.value = null }
  hitTest(event)
}
function onDown(event: PointerEvent) { if (event.button === 0) { press = { x: event.clientX, y: event.clientY }; dragging = false } }
function onUp(event: PointerEvent) {
  if (press && !dragging && pointerInside) { hitTest(event); if (hovered.value !== null) select(hovered.value) }
  press = null
  dragging = false
}
function onEnter() { pointerInside = true }
function onLeave() { pointerInside = false; hovered.value = null }
function onContextLost(event: Event) { event.preventDefault(); unavailable.value = true }
function onContextRestored() { unavailable.value = false; render() }
function onKeydown(event: KeyboardEvent) {
  if (!controls || !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', '+', '=', '-', 'Home'].includes(event.key)) return
  event.preventDefault()
  if (event.key === 'Home') { resetView(); return }
  const spherical = new Spherical().setFromVector3(camera.position.clone().sub(controls.target))
  if (event.key === 'ArrowLeft') spherical.theta -= 0.15
  if (event.key === 'ArrowRight') spherical.theta += 0.15
  if (event.key === 'ArrowUp') spherical.phi -= 0.15
  if (event.key === 'ArrowDown') spherical.phi += 0.15
  if (event.key === '+' || event.key === '=') spherical.radius *= 0.9
  if (event.key === '-') spherical.radius *= 1.1
  spherical.phi = Math.max(0.15, Math.min(Math.PI / 2 - 0.08, spherical.phi))
  spherical.radius = Math.max(4.8, Math.min(14, spherical.radius))
  camera.position.copy(new Vector3().setFromSpherical(spherical).add(controls.target))
  controls.update()
}

watch(slices, () => { selected.value = null; hovered.value = null; if (renderer) rebuild() })
watch(activeIndex, highlight)
watch(isDark, () => { if (renderer) { highlight() } })
onMounted(() => {
  if (!host.value) return
  try { renderer = new WebGLRenderer({ alpha: true, antialias: true }) } catch { unavailable.value = true; return }
  scene = new Scene()
  camera = new PerspectiveCamera(38, 1, 0.1, 100)
  renderer.shadowMap.enabled = true
  renderer.shadowMap.type = PCFSoftShadowMap
  floor = new Mesh(new PlaneGeometry(8, 8), new ShadowMaterial({ opacity: 0.12 }))
  floor.rotation.x = -Math.PI / 2
  floor.position.y = -0.095
  floor.receiveShadow = true
  scene.add(floor)
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
  host.value.appendChild(renderer.domElement)
  scene.add(new AmbientLight('#ffffff', 1.15), pie)
  const keyLight = new DirectionalLight('#ffffff', 1.7)
  keyLight.position.set(-3, 7, 5)
  keyLight.castShadow = true
  keyLight.shadow.mapSize.set(1024, 1024)
  keyLight.shadow.camera.left = -4
  keyLight.shadow.camera.right = 4
  keyLight.shadow.camera.top = 4
  keyLight.shadow.camera.bottom = -4
  keyLight.shadow.normalBias = 0.025
  keyLight.shadow.radius = 4
  scene.add(keyLight)
  const fillLight = new DirectionalLight('#c2d1ff', 0.6)
  fillLight.position.set(-4, 2, -2)
  scene.add(fillLight)
  controls = new OrbitControls(camera, renderer.domElement)
  controls.enablePan = false
  controls.enableDamping = false
  controls.minDistance = 4.8
  controls.maxDistance = 14
  controls.minPolarAngle = 0.15
  controls.maxPolarAngle = Math.PI / 2 - 0.08
  controls.mouseButtons = { LEFT: MOUSE.ROTATE, MIDDLE: MOUSE.DOLLY, RIGHT: null as unknown as MOUSE }
  controls.addEventListener('change', render)
  resetView()
  rebuild()
  observer = new ResizeObserver(() => {
    if (!host.value || !renderer) return
    const { width, height } = host.value.getBoundingClientRect()
    if (!width || !height) return
    camera.aspect = width / height
    camera.updateProjectionMatrix()
    renderer.setSize(width, height)
    render()
  })
  observer.observe(host.value)
  host.value.addEventListener('pointermove', onMove)
  host.value.addEventListener('pointerdown', onDown)
  host.value.addEventListener('pointerenter', onEnter)
  host.value.addEventListener('pointerleave', onLeave)
  window.addEventListener('pointerup', onUp)
  renderer.domElement.addEventListener('webglcontextlost', onContextLost)
  renderer.domElement.addEventListener('webglcontextrestored', onContextRestored)
})
onBeforeUnmount(() => {
  observer?.disconnect()
  controls?.dispose()
  host.value?.removeEventListener('pointermove', onMove)
  host.value?.removeEventListener('pointerdown', onDown)
  host.value?.removeEventListener('pointerenter', onEnter)
  host.value?.removeEventListener('pointerleave', onLeave)
  window.removeEventListener('pointerup', onUp)
  renderer?.domElement.removeEventListener('webglcontextlost', onContextLost)
  renderer?.domElement.removeEventListener('webglcontextrestored', onContextRestored)
  disposeGroup(pie)
  floor?.geometry.dispose()
  floor?.material.dispose()
  scene?.traverse(object => { if (object instanceof DirectionalLight) object.shadow.dispose() })
  renderer?.dispose()
  renderer?.domElement.remove()
})
</script>

<style scoped>
.model-distribution{display:grid;grid-template-columns:minmax(0,1.35fr) minmax(185px,1fr);align-items:start;gap:12px;min-height:374px}
.model-stage{position:relative;min-width:0;height:374px;background:transparent}
.three-host{position:absolute;inset:36px 0 0;cursor:grab;touch-action:none;outline-offset:-4px;border-radius:20px}
.three-host:active{cursor:grabbing}.three-host :deep(canvas){display:block;width:100%;height:100%}
.scene-caption{position:absolute;bottom:41px;left:20px;pointer-events:none;display:flex;flex-direction:column;gap:2px;max-width:80%}
.scene-caption span{font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--studio-muted)}
.scene-caption strong{font-size:22px;font-weight:600;font-variant-numeric:tabular-nums;letter-spacing:-.7px}.scene-caption small{font-size:12px;color:var(--studio-muted)}
.scene-hint{position:absolute;bottom:9px;left:0;right:0;text-align:center;font-size:12px;color:var(--studio-muted);pointer-events:none}
.stage-toolbar{height:36px;display:flex;align-items:center;justify-content:flex-end;padding-right:6px;position:relative;z-index:2}
.reset-view{border:0;border-radius:9px;padding:5px 9px;font-size:12px;color:var(--studio-muted);background:transparent}
.reset-view:hover{color:var(--studio-ink);background:var(--tf-console-hover)}
.scene-empty{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:17px;pointer-events:none;color:var(--studio-muted);font-size:12px}
.empty-orbit{width:148px;height:80px;border:1px dashed var(--studio-grid);border-radius:50%;box-shadow:0 14px 0 -1px var(--studio-haze);transform:rotate(-18deg)}
.model-details{min-width:0;padding:0}.legend-heading{height:36px;display:flex;align-items:center;justify-content:space-between;font-size:12px;color:var(--studio-muted);padding:0 9px}
.model-legend{height:245px;overflow-y:auto;scrollbar-width:thin;scrollbar-color:#a7b3d1 transparent;scrollbar-gutter:stable}
@supports selector(::-webkit-scrollbar){.model-legend{scrollbar-width:auto;scrollbar-color:auto}.model-legend::-webkit-scrollbar{width:5px}.model-legend::-webkit-scrollbar-track{background:transparent}.model-legend::-webkit-scrollbar-thumb{background:#a7b3d188;border-radius:8px}.model-legend::-webkit-scrollbar-thumb:hover{background:#899bc6}.model-legend::-webkit-scrollbar-button{display:none;width:0;height:0}.model-legend::-webkit-scrollbar-corner{background:transparent}}
.model-row{display:flex;align-items:center;gap:9px;width:100%;padding:10px 9px;border-radius:10px;text-align:left;transition:background .15s}
.model-row:hover,.model-row.is-selected{background:var(--tf-console-hover)}.model-row i{width:7px;height:7px;border-radius:50%;flex-shrink:0}
.model-name{min-width:0;flex:1;font-size:12px;white-space:nowrap;text-overflow:ellipsis;overflow:hidden}
.model-amount{text-align:right;font-size:12px;font-variant-numeric:tabular-nums;line-height:1.4}.model-amount small{display:block;color:var(--studio-muted);font-size:12px}
.model-facts{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:6px;height:72px;min-height:72px;margin-top:12px;padding:12px 9px 0;border-top:1px solid var(--tf-console-line)}
.model-facts span{display:flex;flex-direction:column;color:var(--studio-muted);font-size:12px;gap:4px}.model-facts b{color:var(--studio-ink);font-weight:500;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;min-height:18px}
.legend-empty{font-size:12px;line-height:1.6;color:var(--studio-muted);padding:15px 9px}
@media(max-width:640px){.model-distribution{grid-template-columns:1fr;gap:0}.model-stage{height:290px}.model-legend{height:auto;max-height:188px}}
</style>
