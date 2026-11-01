<template>
  <div class="ac-root">
    <!-- 背景：点阵 + 极光 -->
    <div class="ac-stage" aria-hidden="true">
      <div class="ac-dots"></div>
      <div class="ac-aurora a1"></div>
      <div class="ac-aurora a2"></div>
      <div class="ac-aurora a3"></div>
    </div>

    <!-- PROMPT 卡：多模态输入 -->
    <div class="ac-float fc-prompt" aria-hidden="true">
      <div class="fc-head"><span>PROMPT · 多模态</span><span class="live">STREAM</span></div>
      <p>保持 <b>@Image1</b> 人物与花朵。微风轻拂，镜头缓慢推进，电影感自然光。</p>
      <div class="opts"><span>720p · 6s</span><span>4K ↗</span></div>
    </div>

    <!-- 生图卡：文生图进行中 -->
    <div class="ac-float fc-image" aria-hidden="true">
      <div class="fc-head"><span>IMAGE GEN · 文生图</span><span class="live">2.8s</span></div>
      <div class="imgwrap">
        <img :src="portraitUrl" alt="">
        <span class="gen">✳ 文生图 · 1024×1536</span>
        <div class="prog"><i></i></div>
      </div>
      <div class="foot"><span>质量 <b>4K</b> · 风格 <b>胶片</b></span><span>ETA <b>1.2s</b></span></div>
    </div>

    <!-- API 流量卡 -->
    <div class="ac-float fc-api" aria-hidden="true">
      <div class="fc-head"><span>GATEWAY · 实时网关</span><span class="live">LIVE</span></div>
      <pre>POST /v1/chat/completions
<span class="ok">✓ 200 OK</span> · 38ms · stream: true
<span class="ok">✓ 200 OK</span> · 41ms · /v1/images
<span class="ok">✓ 200 OK</span> · 36ms · /v1/video ▊<span class="cursor"></span></pre>
    </div>

    <!-- 速度/稳定卡 -->
    <div class="ac-float fc-speed" aria-hidden="true">
      <div class="fc-head"><span>LATENCY · 网关延迟</span><span class="live">P50</span></div>
      <div class="speed-row"><b><span>{{ latencyMs }}</span><small>ms</small></b><span>TOKEN FIRST BYTE</span></div>
      <div class="bars">
        <i v-for="n in 12" :key="n" :style="{ animationDelay: `${(n - 1) * 0.2}s` }"></i>
      </div>
      <div class="speed-tags">
        <span>SLA <b>99.98%</b></span><span>自动重试 <b>×3</b></span><span>智能路由 <b>✓</b></span>
      </div>
    </div>

    <!-- 顶部公告胶囊：文案取自 site_subtitle 动态配置 -->
    <div v-if="announceText" class="ac-announce">✳ {{ announceText }}</div>

    <main class="ac-center">
      <div class="ac-glass">
        <div class="ac-brand-row">
          <div v-if="settingsLoaded" class="ac-logo-chip">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" />
          </div>
          <div class="ac-brand-name">{{ brandText }}</div>
        </div>

        <div class="ac-tabs" :class="{ reg: isRegister && showRegisterEntry, single: !showRegisterEntry }">
          <div class="ink" aria-hidden="true"></div>
          <button type="button" :class="{ on: isLogin }" @click="goLogin">
            {{ t('auth.signIn') }}
          </button>
          <button v-if="showRegisterEntry" type="button" :class="{ on: isRegister }" @click="goRegister">
            {{ t('auth.signUp') }}
          </button>
        </div>

        <div v-if="isLogin" class="ac-pane">
          <LoginCanvasPanel />
        </div>
        <div v-else class="ac-pane">
          <RegisterCanvasPanel />
        </div>
      </div>

      <div class="ac-below">
        <p v-if="isLogin && showRegisterEntry">
          {{ t('auth.dontHaveAccount') }}
          <a href="/register" @click.prevent="goRegister">{{ t('auth.signUp') }}</a>
        </p>
        <p v-if="isRegister">
          {{ t('auth.alreadyHaveAccount') }}
          <a href="/login" @click.prevent="goLogin">{{ t('auth.signIn') }}</a>
        </p>
      </div>
    </main>

    <!-- 模型品牌跑马灯 -->
    <div class="ac-marquee-wrap" aria-hidden="true">
      <div class="ac-marquee-pill">
        <div class="ac-marquee">
          <div v-for="group in 2" :key="group" class="mq-group">
            <div v-for="brand in brands" :key="`${group}-${brand.name}`" class="mq-item">
              <img class="lg" :src="brand.logo" alt="">
              <span>{{ brand.name }}</span>
            </div>
          </div>
        </div>
        <div class="fade l"></div>
        <div class="fade r"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// 沉浸式全屏画布登录/注册页（B 方案）。只做视觉层；表单逻辑在 canvas 面板组件内，
// 与 LoginView.vue / RegisterView.vue 保持一致。原视图保留未动，路由处整体替换。
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import LoginCanvasPanel from '@/components/auth/canvas/LoginCanvasPanel.vue'
import RegisterCanvasPanel from '@/components/auth/canvas/RegisterCanvasPanel.vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import openaiLogo from '@/assets/auth-canvas/brands/openai.svg'
import geminiLogo from '@/assets/auth-canvas/brands/gemini.svg'
import claudeLogo from '@/assets/auth-canvas/brands/claude.svg'
import grokLogo from '@/assets/auth-canvas/brands/grok.svg'
import deepseekLogo from '@/assets/auth-canvas/brands/deepseek.svg'
import glmLogo from '@/assets/auth-canvas/brands/glm.svg'
import kimiLogo from '@/assets/auth-canvas/brands/kimi.svg'
import midjourneyLogo from '@/assets/auth-canvas/brands/midjourney.svg'
import bytedanceLogo from '@/assets/auth-canvas/brands/bytedance.svg'
import alibabaLogo from '@/assets/auth-canvas/brands/alibaba.svg'
import stablediffusionLogo from '@/assets/auth-canvas/brands/stablediffusion.svg'
import azureLogo from '@/assets/auth-canvas/brands/azure.svg'
import adobeLogo from '@/assets/auth-canvas/brands/adobe.svg'
import portraitUrl from '@/assets/auth-canvas/portrait.webp'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

// ==================== 品牌跑马灯（官方名称，不含分组/具体模型名） ====================

const brands = [
  { name: 'OpenAI', logo: openaiLogo },
  { name: 'Gemini', logo: geminiLogo },
  { name: 'Claude', logo: claudeLogo },
  { name: 'Grok', logo: grokLogo },
  { name: 'DeepSeek', logo: deepseekLogo },
  { name: 'GLM', logo: glmLogo },
  { name: 'Kimi', logo: kimiLogo },
  { name: 'Midjourney', logo: midjourneyLogo },
  { name: 'Seedream', logo: bytedanceLogo },
  { name: 'Wan', logo: alibabaLogo },
  { name: 'Stable Diffusion', logo: stablediffusionLogo },
  { name: 'Azure', logo: azureLogo },
  { name: 'Adobe', logo: adobeLogo }
]

// ==================== Tab（登录/注册由路由驱动，支持深链与前进后退） ====================

const isLogin = computed(() => route.path !== '/register')
const isRegister = computed(() => route.path === '/register')

function pushAuthPath(path: '/login' | '/register'): void {
  if (route.path === path) return
  void router.push({ path, query: route.query })
}

function goLogin(): void {
  pushAuthPath('/login')
}

function goRegister(): void {
  pushAuthPath('/register')
}

// ==================== 站点配置（公告 / Logo / 品牌文案） ====================

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const announceText = computed(() => String(appStore.cachedPublicSettings?.site_subtitle || '').trim())

const brandText = computed(() =>
  isRegister.value
    ? t('auth.signUpToStart', { siteName: siteName.value })
    : t('auth.signInToSite', { siteName: siteName.value })
)

// 顶部与页脚注册入口均遵守系统开关。
const showRegisterEntry = computed(() => {
  const settings = appStore.cachedPublicSettings
  return settingsLoaded.value && settings?.backend_mode_enabled !== true && settings?.registration_enabled === true
})

// ==================== LATENCY 卡数字跳动（纯装饰） ====================

const latencyMs = ref(38)
let latencyTimer: ReturnType<typeof setInterval> | null = null

// ==================== 挂载期间强制暗色（画布为暗色设计），离开时恢复 ====================

let prevDark: boolean | null = null

onMounted(() => {
  appStore.fetchPublicSettings()

  const root = document.documentElement
  prevDark = root.classList.contains('dark')
  root.classList.add('dark')

  latencyTimer = setInterval(() => {
    latencyMs.value = 34 + Math.floor(Math.random() * 9) // 34–42ms
  }, 1800)
})

onUnmounted(() => {
  if (prevDark !== null) {
    document.documentElement.classList.toggle('dark', prevDark)
    prevDark = null
  }
  if (latencyTimer) {
    clearInterval(latencyTimer)
    latencyTimer = null
  }
})
</script>

<style scoped>
@font-face {
  font-family: 'JetBrains Mono';
  src: url('@/assets/auth-canvas/fonts/JetBrainsMono-Variable.ttf') format('truetype');
  font-weight: 100 800;
  font-display: swap;
}

.ac-root {
  --tf-300: #abb9ff;
  --tf-400: #899bff;
  --tf-500: #536eff;
  --tf-ink: #070912;
  --tf-card: rgba(15, 20, 34, 0.72);
  --tf-line: rgba(137, 155, 255, 0.16);
  --tf-text: #eef1fc;
  --tf-dim: #99a3b9;
  --tf-mono: 'JetBrains Mono', ui-monospace, SFMono-Regular, Consolas, monospace;
  --tf-sans: -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif;

  position: relative;
  isolation: isolate;
  min-height: 100vh;
  background: var(--tf-ink);
  color: var(--tf-text);
  font-family: var(--tf-sans);
  overflow-x: hidden;
}
.ac-root button,
.ac-root input {
  font-family: inherit;
}

/* ── 背景：点阵 + 极光 ─────────────────── */
.ac-stage {
  position: fixed;
  inset: 0;
  z-index: 0;
  background: var(--tf-ink);
  pointer-events: none;
}
.ac-dots {
  position: absolute;
  inset: 0;
  background-image: radial-gradient(rgba(137, 155, 255, 0.14) 1px, transparent 1.4px);
  background-size: 26px 26px;
  mask-image: radial-gradient(ellipse 90% 80% at 50% 45%, #000 35%, transparent 80%);
  -webkit-mask-image: radial-gradient(ellipse 90% 80% at 50% 45%, #000 35%, transparent 80%);
}
.ac-aurora {
  position: absolute;
  border-radius: 50%;
  filter: blur(90px);
  opacity: 0.5;
}
.ac-aurora.a1 {
  width: 560px;
  height: 420px;
  left: -140px;
  top: -120px;
  background: radial-gradient(closest-side, rgba(83, 110, 255, 0.5), transparent);
  animation: ac-drift 22s ease-in-out infinite;
}
.ac-aurora.a2 {
  width: 640px;
  height: 460px;
  right: -180px;
  bottom: -140px;
  background: radial-gradient(closest-side, rgba(119, 85, 243, 0.55), transparent);
  animation: ac-drift 26s ease-in-out -8s infinite reverse;
}
.ac-aurora.a3 {
  width: 380px;
  height: 320px;
  left: 56%;
  top: 8%;
  background: radial-gradient(closest-side, rgba(137, 155, 255, 0.22), transparent);
  animation: ac-drift 30s ease-in-out -14s infinite;
}
@keyframes ac-drift {
  0%,
  100% {
    transform: translate(0, 0) scale(1);
  }
  33% {
    transform: translate(60px, 34px) scale(1.08);
  }
  66% {
    transform: translate(-40px, 22px) scale(0.95);
  }
}

/* ── 漂浮卡片通用 ──────────────────────── */
.ac-float {
  position: fixed;
  z-index: 1;
  border: 1px solid var(--tf-line);
  border-radius: 14px;
  background: var(--tf-card);
  backdrop-filter: blur(14px);
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.45);
  font-family: var(--tf-mono);
  animation: ac-float 9s ease-in-out infinite;
}
.fc-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 11px 14px 0;
  font-size: 10px;
  letter-spacing: 2px;
  color: var(--tf-400);
}
.fc-head .live {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--tf-dim);
  letter-spacing: 1px;
}
.fc-head .live::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--tf-400);
  box-shadow: 0 0 8px var(--tf-400);
  animation: ac-blink 1.6s ease infinite;
}
@keyframes ac-blink {
  50% {
    opacity: 0.35;
  }
}
@keyframes ac-float {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-13px);
  }
}

/* PROMPT 卡 */
.fc-prompt {
  left: 5%;
  top: 15%;
  width: 236px;
  padding-bottom: 14px;
  animation-delay: -2s;
}
.fc-prompt p {
  margin: 9px 14px 0;
  font-family: var(--tf-sans);
  font-size: 12px;
  line-height: 1.9;
  color: #b9c4dd;
}
.fc-prompt p b {
  color: var(--tf-300);
  font-weight: 500;
}
.fc-prompt .opts {
  display: flex;
  justify-content: space-between;
  margin: 10px 14px 0;
  font-size: 10.5px;
  color: var(--tf-300);
}
.fc-prompt .opts span {
  padding: 3px 7px;
  border: 1px solid var(--tf-line);
  border-radius: 4px;
}

/* 生图卡 */
.fc-image {
  right: 5%;
  top: 13%;
  width: 250px;
  animation-delay: -4.5s;
  animation-duration: 11s;
}
.fc-image .imgwrap {
  position: relative;
  margin: 11px 12px 0;
  border-radius: 9px;
  overflow: hidden;
}
.fc-image img {
  display: block;
  width: 100%;
  height: 120px;
  object-fit: cover;
  object-position: center 30%;
  filter: saturate(0.92);
  transform: scale(1.04);
  animation: ac-imgBreathe 7s ease-in-out infinite;
}
@keyframes ac-imgBreathe {
  0%,
  100% {
    transform: scale(1.02);
  }
  50% {
    transform: scale(1.07);
  }
}
.fc-image .imgwrap::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, transparent 55%, rgba(6, 12, 11, 0.55));
}
.fc-image .gen {
  position: absolute;
  left: 10px;
  bottom: 8px;
  z-index: 1;
  font-size: 9.5px;
  letter-spacing: 1.4px;
  color: #fff;
  opacity: 0.9;
}
.fc-image .prog {
  position: absolute;
  left: 10px;
  right: 10px;
  bottom: 5px;
  z-index: 1;
  height: 2px;
  border-radius: 1px;
  background: rgba(255, 255, 255, 0.25);
  overflow: hidden;
}
.fc-image .prog i {
  display: block;
  height: 100%;
  width: 72%;
  background: linear-gradient(90deg, var(--tf-400), var(--tf-300));
  box-shadow: 0 0 6px var(--tf-300);
  animation: ac-genFill 4.2s ease-in-out infinite;
}
@keyframes ac-genFill {
  0% {
    width: 8%;
  }
  62% {
    width: 86%;
  }
  100% {
    width: 96%;
  }
}
.fc-image .foot {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: 10px 12px 12px;
  font-size: 10px;
  color: var(--tf-dim);
  letter-spacing: 1.2px;
}
.fc-image .foot b {
  color: var(--tf-300);
  font-weight: 500;
}

/* API 流量卡 */
.fc-api {
  left: 7%;
  bottom: 15%;
  width: 280px;
  padding: 0 0 12px;
  animation-delay: -6s;
}
.fc-api pre {
  margin: 9px 14px 0;
  font-size: 11.5px;
  line-height: 2.05;
  color: #9aa6c4;
}
.fc-api .ok {
  color: var(--tf-300);
}
.fc-api .cursor {
  display: inline-block;
  width: 7px;
  height: 13px;
  background: var(--tf-300);
  vertical-align: -2px;
  animation: ac-caret 1.1s steps(1) infinite;
}
@keyframes ac-caret {
  50% {
    opacity: 0;
  }
}

/* 延迟/稳定卡 */
.fc-speed {
  right: 7%;
  bottom: 14%;
  width: 250px;
  padding-bottom: 14px;
  animation-delay: -8s;
  animation-duration: 10s;
}
.speed-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin: 9px 14px 0;
}
.speed-row b {
  font-size: 26px;
  font-weight: 600;
  color: var(--tf-300);
}
.speed-row b small {
  font-size: 12px;
  font-weight: 400;
  color: var(--tf-dim);
  margin-left: 2px;
}
.speed-row span {
  font-size: 10px;
  letter-spacing: 1.6px;
  color: var(--tf-dim);
}
.bars {
  display: flex;
  align-items: flex-end;
  gap: 4px;
  height: 34px;
  margin: 10px 14px 0;
}
.bars i {
  flex: 1;
  border-radius: 2px 2px 0 0;
  background: linear-gradient(180deg, var(--tf-300), rgba(83, 110, 255, 0.25));
  transform-origin: bottom;
  animation: ac-barPulse 2.8s ease-in-out infinite;
}
.bars i:nth-child(odd) {
  opacity: 0.55;
}
@keyframes ac-barPulse {
  0%,
  100% {
    transform: scaleY(0.55);
  }
  50% {
    transform: scaleY(1);
  }
}
.speed-tags {
  display: flex;
  gap: 6px;
  margin: 11px 14px 0;
  flex-wrap: wrap;
}
.speed-tags span {
  padding: 3px 8px;
  border: 1px solid var(--tf-line);
  border-radius: 99px;
  font-size: 9.5px;
  letter-spacing: 1px;
  color: #9aa8d6;
}
.speed-tags span b {
  color: var(--tf-300);
  font-weight: 500;
}

/* ── 模型品牌跑马灯：胶囊浮条 ──────────── */
.ac-marquee-wrap {
  position: fixed;
  bottom: 18px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 2;
  width: min(940px, calc(100vw - 28px));
}
.ac-marquee-pill {
  position: relative;
  overflow: hidden;
  border: 1px solid rgba(137, 155, 255, 0.13);
  border-radius: 999px;
  background: rgba(7, 9, 18, 0.86);
  padding: 12px 0;
  box-shadow: 0 18px 44px rgba(0, 0, 0, 0.35);
}
.ac-marquee {
  display: flex;
  width: max-content;
  animation: ac-scroll 48s linear infinite;
  will-change: transform;
  transform: translateZ(0);
  backface-visibility: hidden;
}
.ac-marquee-pill:hover .ac-marquee {
  animation-play-state: paused;
}
@keyframes ac-scroll {
  to {
    transform: translateX(-50%);
  }
}
.mq-group {
  display: flex;
  align-items: center;
}
.mq-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 17px;
  opacity: 0.72;
  transition: opacity 0.3s;
}
.ac-marquee-pill:hover .mq-item {
  opacity: 1;
}
.mq-item .lg {
  display: block;
  width: 17px;
  height: 17px;
  flex: none;
  filter: brightness(0) invert(1);
  opacity: 0.92;
}
.mq-item span {
  font: 400 11.5px/1 var(--tf-mono);
  letter-spacing: 0.8px;
  color: #9aa6c4;
  white-space: nowrap;
}
.fade {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 90px;
  pointer-events: none;
  z-index: 1;
}
.fade.l {
  left: 0;
  background: linear-gradient(90deg, rgba(10, 13, 24, 0.92), transparent);
}
.fade.r {
  right: 0;
  background: linear-gradient(270deg, rgba(10, 13, 24, 0.92), transparent);
}

/* ── 顶部公告 ─────────────────────────── */
.ac-announce {
  position: fixed;
  top: 18px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 30;
  padding: 8px 18px;
  border: 1px solid var(--tf-line);
  border-radius: 99px;
  white-space: nowrap;
  font: 500 12px/1.5 var(--tf-mono);
  letter-spacing: 0.4px;
  color: #a6b1cf;
  background: rgba(7, 9, 18, 0.7);
  backdrop-filter: blur(10px);
  animation: ac-fadeDown 0.8s both;
}
@keyframes ac-fadeDown {
  from {
    opacity: 0;
    transform: translate(-50%, -12px);
  }
}

/* ── 中央玻璃卡 ───────────────────────── */
.ac-center {
  position: relative;
  z-index: 10;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 96px 20px 110px;
}
.ac-glass {
  width: 100%;
  max-width: 408px;
  padding: 30px 34px 26px;
  border: 1px solid var(--tf-line);
  border-radius: 20px;
  background: var(--tf-card);
  backdrop-filter: blur(18px);
  box-shadow: 0 40px 90px rgba(0, 0, 0, 0.5);
  animation: ac-rise 0.8s 0.1s both;
}
@keyframes ac-rise {
  from {
    opacity: 0;
    transform: translateY(22px) scale(0.98);
  }
}
.ac-brand-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  margin-bottom: 22px;
  text-align: center;
}
.ac-logo-chip {
  display: grid;
  place-items: center;
  width: 56px;
  height: 56px;
  border-radius: 14px;
  overflow: hidden;
  background: #ffffff;
  box-shadow: 0 10px 28px rgba(83, 110, 255, 0.4);
}
.ac-logo-chip img {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: contain;
}
.ac-brand-name {
  font: 600 18px/1.5 var(--tf-sans);
  letter-spacing: 0.3px;
  color: #eef1fc;
}

.ac-tabs {
  position: relative;
  display: grid;
  grid-template-columns: 1fr 1fr;
  margin-bottom: 22px;
  background: rgba(9, 12, 24, 0.75);
  border: 1px solid var(--tf-line);
  border-radius: 11px;
  padding: 4px;
}
.ac-tabs .ink {
  position: absolute;
  top: 4px;
  bottom: 4px;
  left: 4px;
  width: calc(50% - 4px);
  border-radius: 8px;
  background: linear-gradient(120deg, rgba(83, 110, 255, 0.32), rgba(137, 155, 255, 0.18));
  border: 1px solid rgba(137, 155, 255, 0.4);
  transition: transform 0.28s cubic-bezier(0.5, 1.4, 0.4, 1);
}
.ac-tabs.reg .ink {
  transform: translateX(100%);
}
.ac-tabs.single {
  grid-template-columns: 1fr;
}
.ac-tabs.single .ink {
  width: calc(100% - 8px);
}
.ac-tabs button {
  position: relative;
  z-index: 1;
  padding: 9px 0;
  border: 0;
  background: none;
  cursor: pointer;
  font: 500 13px/1 var(--tf-sans);
  letter-spacing: 2px;
  color: var(--tf-dim);
  transition: color 0.2s;
}
.ac-tabs button.on {
  color: #eef2ff;
}
.ac-pane {
  animation: ac-paneIn 0.3s both;
}
@keyframes ac-paneIn {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
}

.ac-below {
  margin-top: 20px;
  text-align: center;
  animation: ac-rise 0.8s 0.25s both;
}
.ac-below p {
  font-size: 12.5px;
  color: var(--tf-dim);
}
.ac-below a {
  color: var(--tf-300);
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
}
.ac-below a:hover {
  color: #c3ccff;
}

/* ── 表单元素：贴齐 B 方案暗色玻璃质感（覆盖共享组件类） ── */
.ac-glass :deep(.input-label) {
  margin-bottom: 7px;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--tf-dim);
}
.ac-glass :deep(.input) {
  padding-top: 11px;
  padding-bottom: 11px;
  font-size: 13.5px;
  color: var(--tf-text);
  background: rgba(11, 14, 25, 0.65);
  border: 1px solid #1d2438;
  border-radius: 9px;
}
.ac-glass :deep(.input::placeholder) {
  color: #6b7590;
}
.ac-glass :deep(.input:focus) {
  border-color: var(--tf-400);
  box-shadow: 0 0 0 4px rgba(83, 110, 255, 0.13);
}
.ac-glass :deep(.input-error) {
  border-color: #ef4444;
}
.ac-glass :deep(.input-error:focus) {
  border-color: #ef4444;
  box-shadow: 0 0 0 4px rgba(239, 68, 68, 0.15);
}
.ac-glass :deep(.input-hint) {
  margin-top: 7px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--tf-dim);
}
.ac-glass :deep(.btn-primary) {
  padding: 12px;
  border-radius: 10px;
  color: #ffffff;
  background: linear-gradient(115deg, #486aff, #7755f3);
  box-shadow: 0 12px 30px rgba(83, 110, 255, 0.35);
}
.ac-glass :deep(.btn-primary:hover) {
  background: linear-gradient(115deg, #486aff, #7755f3);
  transform: translateY(-1px);
  box-shadow: 0 16px 34px rgba(83, 110, 255, 0.45);
}
.ac-glass :deep(.btn:focus) {
  --tw-ring-color: rgba(83, 110, 255, 0.4);
  --tw-ring-offset-color: #0a0d18;
}
.ac-glass :deep(.text-primary-600),
.ac-glass :deep(.text-primary-400),
.ac-glass :deep(.text-primary-300) {
  color: var(--tf-300);
}
.ac-glass :deep(.text-primary-600:hover),
.ac-glass :deep(.text-primary-400:hover),
.ac-glass :deep(.text-primary-300:hover) {
  color: #c3ccff;
}
.ac-glass :deep(input[type='checkbox']) {
  accent-color: var(--tf-500);
}

/* ── 响应式：分级隐藏漂浮卡片 ─────────── */
@media (max-width: 1180px) {
  .fc-prompt,
  .fc-speed {
    display: none;
  }
}
@media (max-width: 900px) {
  .ac-float {
    display: none;
  }
  .ac-center {
    padding-bottom: 90px;
  }
}
@media (max-width: 480px) {
  .ac-glass {
    padding: 26px 20px 22px;
  }
  .ac-announce {
    font-size: 10.5px;
    max-width: 92vw;
    white-space: normal;
    text-align: center;
  }
}
</style>
