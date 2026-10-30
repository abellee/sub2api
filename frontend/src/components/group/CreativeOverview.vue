<template>
  <section id="top" class="creative-overview" aria-label="创作概览">
    <div class="creative-docs-header">
      <div class="creative-docs-breadcrumb">开发者文档 / 创作指南</div>
      <h1>图片与视频 <span>CREATIVE API</span></h1>
    </div>
    <div class="overview-copy">
      <div class="overview-eyebrow">✳ CREATIVE API / 灵感，不止于想象</div>
      <h2>让灵感成像。<br><span>让画面生动。</span></h2>
      <p>从一句描述，到一张画面，再到一段故事。<br>用熟悉的 API，把图片与视频创作接入你的应用。</p>
      <div class="overview-actions"><button type="button" class="overview-start" @click="$emit('start')">开始接入 <span>→</span></button></div>
      <div class="overview-tags"><span>✓ 图片生成与编辑</span><span>✓ 多素材视频创作</span><span>✓ 异步任务查询</span></div>
      <div class="overview-index">01 — IDEA TO REALITY <span>向下探索 ↙</span></div>
    </div>
    <div ref="canvas" class="creative-flow" aria-label="图片到视频的创作流程示意">
      <div class="flow-caption"><span>● THE CREATIVE FLOW</span><span class="canvas-demo">视觉示意</span></div>
      <svg class="flow-wires" :viewBox="`0 0 ${wires.width} ${wires.height}`" aria-hidden="true">
        <defs><linearGradient id="creative-flow-gradient"><stop stop-color="#9299ff"/><stop offset="1" stop-color="#899bff"/></linearGradient></defs>
        <path v-for="(path, index) in wires.paths" :key="index" :d="path" :class="{ 'wire-color': index === 1 }"/>
        <circle v-for="(point, index) in wires.ends" :key="index" :cx="point.x" :cy="point.y" r="4"/>
        <circle v-for="(path, index) in wires.paths" :key="`particle-${index}`" class="wire-particle" r="3">
          <animateMotion :path="path" dur="3.2s" :begin="`${index * -1.6}s`" repeatCount="indefinite" calcMode="linear"/>
        </circle>
      </svg>
      <article ref="sourceNode" class="flow-node flow-source"><div class="node-heading"><svg viewBox="0 0 20 20" aria-hidden="true"><rect x="3" y="3" width="14" height="14" rx="3"/><circle cx="7" cy="7" r="1"/><path d="m4 14 4-4 3 3 3-4 3 5"/></svg><span>01 / 参考画面</span><span class="node-dots">···</span></div><div class="flow-portrait"><img src="/creative-docs/assets/portrait.webp" alt="卷发与鲜花装饰的人像，创作流程示意"><span>A STUDY IN BLOOM</span></div><div class="node-footer"><span>人物与花，光影与想象。</span><span class="round-icon" aria-hidden="true">✳</span></div><div ref="port" class="output-port" aria-hidden="true">+</div></article>
      <article ref="promptNode" class="flow-node flow-prompt"><div class="node-heading"><span class="violet-dot" aria-hidden="true"></span><span>02 / 赋予灵感</span><span class="node-dots">···</span></div><div class="prompt-content"><span>PROMPT</span><p>保持 <b>@Image1</b> 人物与花朵。微风轻拂，镜头缓慢推进，电影感自然光。</p><div class="prompt-options"><span>sd2.5-720p</span><span>6s ↗</span></div></div></article>
      <article ref="resultNode" class="flow-node flow-result" :class="{ 'is-playing': playing }"><div class="node-heading"><svg viewBox="0 0 20 20" aria-hidden="true"><rect x="2" y="5" width="16" height="11" rx="3"/><path d="m8 8 4 3-4 2z"/></svg><span>03 / 让想象发生</span><span class="node-dots">···</span></div><div class="flow-video"><img src="/creative-docs/assets/portrait.webp" alt="人像创作流程演示"><div class="film-title">Make it<br><em>move.</em></div><button type="button" class="flow-play" :aria-label="playing ? '暂停流程演示' : '播放流程演示'" :aria-pressed="playing" @click="playing = !playing">{{ playing ? 'Ⅱ' : '▶' }}</button><span class="video-caption">流程演示 · 视觉示意</span><div class="video-progress" aria-hidden="true"><span></span></div></div><div class="node-footer result-footer"><span>● IMAGE TO VIDEO</span><span>00:06</span></div><span class="sr-only" role="status">{{ playing ? '正在播放流程演示' : '流程演示已暂停' }}</span></article>
      <div class="flow-bottom">✥ 灵感在此连接 <span>YOUR NEXT CREATION</span></div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

defineEmits<{ start: [] }>()
const playing = ref(false)
const canvas = ref<HTMLElement | null>(null)
const port = ref<HTMLElement | null>(null)
const sourceNode = ref<HTMLElement | null>(null)
const promptNode = ref<HTMLElement | null>(null)
const resultNode = ref<HTMLElement | null>(null)
const wires = ref({ width: 580, height: 520, paths: [] as string[], ends: [] as { x: number; y: number }[] })
let observer: ResizeObserver | null = null

function connectNodes() {
  if (!canvas.value || !port.value || !promptNode.value || !resultNode.value) return
  // Measure before perspective so wires stay attached throughout the sway.
  canvas.value.classList.add('is-measuring')
  const bounds = canvas.value.getBoundingClientRect()
  const output = port.value.getBoundingClientRect()
  const start = { x: output.x + output.width / 2 - bounds.x, y: output.y + output.height / 2 - bounds.y }
  const ends = [promptNode.value, resultNode.value].map(node => {
    const target = node.getBoundingClientRect()
    return { x: target.x - bounds.x + 2, y: target.y + target.height / 2 - bounds.y }
  })
  wires.value = {
    width: bounds.width || 580, height: bounds.height || 520, ends,
    paths: ends.map(end => {
      const middle = start.x + (end.x - start.x) / 2
      return `M${start.x} ${start.y} C${middle} ${start.y} ${middle} ${end.y} ${end.x} ${end.y}`
    }),
  }
  canvas.value.classList.remove('is-measuring')
}

onMounted(() => {
  observer = new ResizeObserver(connectNodes)
  for (const element of [canvas.value, sourceNode.value, port.value, promptNode.value, resultNode.value]) if (element) observer.observe(element)
  connectNodes()
})
onBeforeUnmount(() => observer?.disconnect())
</script>

<style scoped>
.creative-overview { display: grid; grid-template-columns: .92fr 1.08fr; grid-template-rows: auto 1fr; grid-template-areas: 'header flow' 'copy flow'; align-items: stretch; gap: 56px 40px; width: 100%; padding: 14px 24px 42px; border-bottom: 1px solid var(--tf-console-line); }
.creative-docs-header { grid-area: header; }
.creative-docs-breadcrumb { opacity: .6; margin-bottom: 12px; }
.creative-docs-header h1 { display: flex; align-items: center; gap: 20px; font-size: 26px; font-weight: 400; margin: 0; }
.creative-docs-header h1 span { font-size: 12px; letter-spacing: 1px; border-left: 1px solid var(--tf-console-line); padding-left: 20px; opacity: .6; }
.overview-copy { grid-area: copy; align-self: end; }
.overview-eyebrow { font-size: 12px; letter-spacing: 1.3px; opacity: .65; }
h2 { margin: 26px 0 24px; font-size: clamp(34px,3.25vw,44px); font-weight: 400; line-height: 1.45; letter-spacing: -1px; }
h2 > span { background: var(--tf-console-action-gradient); color: transparent; background-clip: text; }
:global(.dark .creative-overview .overview-copy h2 > span) { background-image: linear-gradient(115deg,#899bff,#c0a6ff); }
.overview-copy > p { font-size: 13px; line-height: 2; opacity: .65; margin-bottom: 28px; }
.overview-actions { display: flex; flex-wrap: wrap; gap: 12px; }
.overview-start { display: inline-flex; align-items: center; justify-content: space-between; gap: 30px; border: 1px solid var(--tf-console-line); border-radius: 7px; padding: 12px 18px; font-size: 12px; transition: transform .2s ease; }
.overview-start { color: white; background: var(--tf-console-action-gradient); border-color: transparent; }
.overview-start:hover { transform: translateY(-2px); }
.overview-tags { display: flex; flex-wrap: wrap; gap: 16px; margin-top: 26px; font-size: 12px; opacity: .6; }
.overview-index { display: flex; justify-content: space-between; max-width: 360px; margin-top: 44px; font-size: 12px; opacity: .5; }
.creative-flow { grid-area: flow; position: relative; min-width: 0; min-height: 520px; border: 1px solid var(--tf-console-line); border-radius: 14px; overflow: hidden; background: radial-gradient(rgb(var(--tf-console-primary-500) / .18) .7px,transparent .7px) 0 0 / 13px 13px,linear-gradient(135deg,rgb(var(--tf-console-primary-500) / .04),transparent); }
.creative-flow { transform-origin: 50% 50%; animation: creative-flow-sway 10s ease-in-out infinite; will-change: transform; }
.creative-flow.is-measuring { transform: none !important; }
@keyframes creative-flow-sway {
  0%,100% { transform: perspective(1200px) rotateX(3deg) rotateY(-8deg) rotateZ(-.7deg); }
  33% { transform: perspective(1200px) rotateX(-4deg) rotateY(2deg) rotateZ(.5deg); }
  66% { transform: perspective(1200px) rotateX(2deg) rotateY(8deg) rotateZ(.7deg); }
}
.flow-caption,.flow-bottom { position: absolute; left: 20px; right: 20px; display: flex; justify-content: space-between; gap: 12px; font-size: 12px; opacity: .6; }
.flow-caption { top: 18px; align-items: center; letter-spacing: 1.2px; }
.flow-caption span { letter-spacing: 0; }
.canvas-demo { border: 1px solid var(--tf-console-line); border-radius: 3px; padding: 3px 7px; }
.flow-bottom { bottom: 18px; }
.flow-wires { position: absolute; inset: 0; width: 100%; height: 100%; fill: none; stroke: #b7c2ff; stroke-width: 1.3; }
.wire-color { stroke: url(#creative-flow-gradient); stroke-width: 1.7; }
.flow-wires circle { fill: #b7c2ff; stroke: var(--tf-console-active-ink); }
.flow-wires .wire-particle { fill: #fff; stroke: #a9b4ff; stroke-width: 1; filter: drop-shadow(0 0 4px #8a91ff) drop-shadow(0 0 8px #8a91ff); }
.flow-node { position: absolute; z-index: 1; border: 1px solid var(--tf-console-line); border-radius: 10px; background: var(--tf-console-surface); box-shadow: 0 12px 35px rgb(24 43 83 / .10); }
.flow-source { left: 5%; top: 20%; width: 35%; transform: rotate(-3deg); }
.node-heading { display: flex; align-items: center; gap: 6px; padding: 12px 11px; font-size: 12px; line-height: 1.6; }
.node-heading svg { width: 13px; height: 13px; fill: none; stroke: currentColor; stroke-width: 1.2; flex-shrink: 0; opacity: .7; }
.node-dots { margin-left: auto; letter-spacing: 2px; opacity: .65; }
.violet-dot { width: 5px; height: 5px; flex-shrink: 0; border-radius: 50%; background: #b39cff; }
.flow-portrait { position: relative; height: 218px; margin: 0 8px; border-radius: 4px; overflow: hidden; }
.flow-portrait img,.flow-video img { width: 100%; height: 100%; object-fit: cover; }
.flow-portrait > span { position: absolute; bottom: 10px; left: 10px; color: white; font-size: 12px; letter-spacing: 1px; text-shadow: 0 1px 8px #000; }
.node-footer { display: flex; justify-content: space-between; align-items: center; gap: 5px; padding: 12px 10px; font-size: 12px; line-height: 1.6; }
.round-icon { display: grid; place-items: center; flex-shrink: 0; width: 23px; height: 23px; border-radius: 50%; color: white; background: var(--tf-console-action-gradient); font-size: 17px; }
.output-port { position: absolute; right: -10px; top: 51%; display: grid; place-items: center; width: 19px; height: 19px; border: 1px solid var(--tf-console-line); border-radius: 50%; background: var(--tf-console-surface); box-shadow: 0 0 0 5px var(--tf-console-table-head); font-size: 13px; }
.flow-prompt { right: 4%; top: 11%; width: 41%; transform: rotate(1deg); }
.prompt-content { margin: 0 8px 8px; padding: 10px; border-radius: 5px; font-size: 12px; }
.prompt-content > span { opacity: .5; letter-spacing: 1px; }
.prompt-content p { line-height: 1.8; margin: 8px 0 10px; }
.prompt-content b { font-weight: 400; color: var(--tf-console-active-ink); }
.prompt-options { display: flex; justify-content: space-between; flex-wrap: wrap; gap: 6px; color: var(--tf-console-active-ink); }
.prompt-options span { padding: 4px; border: 1px solid var(--tf-console-line); border-radius: 3px; }
.flow-result { right: 3%; bottom: 10%; width: 43%; transform: rotate(2deg); }
.flow-video { position: relative; height: 129px; margin: 0 8px; border-radius: 4px; overflow: hidden; background: #222; }
.flow-video img { object-position: center 30%; filter: brightness(.65) saturate(.65); transform: scale(1.1); }
.flow-video::after { content: ''; position: absolute; inset: 0; background: linear-gradient(90deg,#07091288,transparent 80%); pointer-events: none; }
.film-title { position: absolute; z-index: 1; top: 18px; left: 16px; font-size: 28px; letter-spacing: -1px; font-weight: 600; color: white; line-height: 1.05; }
.film-title em { font-weight: 400; }
.flow-play { position: absolute; z-index: 3; right: 17px; top: 50px; display: grid; place-items: center; width: 34px; height: 34px; border: 1px solid #fff6; border-radius: 50%; background: #fff2; backdrop-filter: blur(5px); color: white; font-size: 12px; cursor: pointer; }
.flow-play:hover { background: #fff4; }
.video-caption { position: absolute; z-index: 2; bottom: 16px; left: 13px; font-size: 12px; color: white; }
.video-progress { position: absolute; z-index: 2; left: 12px; right: 12px; bottom: 8px; height: 2px; background: #fff4; }
.video-progress > span { display: block; width: 27%; height: 100%; background: #b7c2ff; }
.is-playing .flow-video img { animation: creative-film 6s ease-in-out infinite alternate; }
.is-playing .video-progress > span { animation: creative-progress 6s linear infinite; }
@keyframes creative-film { to { transform: scale(1.26) translate(-3%,-1%); } }
@keyframes creative-progress { from { width: 0; } to { width: 100%; } }
@media(max-width:1150px) { .creative-overview { gap: 24px; grid-template-columns: 1fr; grid-template-rows: auto; grid-template-areas: 'header' 'copy' 'flow'; } .creative-flow { width: 100%; max-width: 760px; justify-self: center; } .overview-copy { padding: 10px 0; } .overview-index { margin-top: 26px; } }
@media(max-width:700px) { .creative-overview { padding-inline: 0; } .creative-docs-header h1 { font-size: 22px; } .creative-docs-header h1 span { display: none; } .creative-flow { min-height: 610px; } .flow-source { width: 36%; left: 5%; top: 21%; } .flow-prompt { width: 39%; right: 5%; top: 9%; } .flow-result { width: 45%; right: 3%; bottom: 7%; } .flow-caption { letter-spacing: 0; } .flow-bottom > span { display: none; } .flow-portrait { height: 180px; } .node-heading,.node-footer { padding: 10px 8px; } .node-dots { display: none; } .prompt-content { padding: 8px; } .flow-video { height: 115px; } .film-title { font-size: 22px; left: 10px; top: 12px; } .flow-play { right: 12px; top: 34px; width: 28px; height: 28px; } .video-caption { left: 10px; } .result-footer { flex-wrap: wrap; } }
@media(prefers-reduced-motion:reduce) { .overview-start { transition: none; } .creative-flow { animation: none; transform: none; will-change: auto; } .wire-particle { display: none; } .is-playing .flow-video img { animation: none; } }
</style>
