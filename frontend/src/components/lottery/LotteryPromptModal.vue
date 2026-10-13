<script setup lang="ts">
/**
 * LotteryPromptModal —— 全局资格引导弹窗。
 * 动画为自绘 SVG + CSS（悬浮礼盒 + 开盖 + 纸屑雨 + 光晕），无第三方依赖、保证渲染。
 * 样式对齐 Sub2API 主站（card 风格 + primary 渐变按钮）。
 */
import type { ActivityView } from '../../api/lottery'

defineProps<{ activity: ActivityView | null }>()
const emit = defineEmits<{ (e: 'confirm', activityId: number): void; (e: 'dismiss', activityId: number): void }>()

const CONFETTI_COLORS = ['#f59e0b', '#8b5cf6', '#ec4899', '#10b981', '#3b82f6', '#f43f5e']

// 纸屑参数（伪随机但固定，保证每次弹出观感一致）
function confettiStyle(n: number) {
  const i = n - 1
  return {
    left: `${6 + ((i * 29) % 88)}%`,
    background: CONFETTI_COLORS[i % CONFETTI_COLORS.length],
    width: i % 3 === 0 ? '5px' : '4px',
    height: i % 4 === 0 ? '10px' : '7px',
    borderRadius: i % 2 === 0 ? '1px' : '50%',
    animationDelay: `${(i % 6) * 0.35 + (i > 5 ? 0.15 : 0)}s`,
    animationDuration: `${1.6 + (i % 4) * 0.28}s`,
    // 每片下落时的水平摆动幅度与旋转不同
    '--sway': `${(i % 2 === 0 ? 1 : -1) * (6 + (i % 3) * 5)}px`,
    '--spin': `${(i % 2 === 0 ? 1 : -1) * (200 + (i % 5) * 120)}deg`,
  }
}
</script>

<template>
  <transition name="lottery-modal">
    <div
      v-if="activity"
      class="fixed inset-0 z-[2000] flex items-center justify-center bg-gray-900/60 px-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
    >
      <div class="card w-full max-w-sm p-6 text-center shadow-glass">
        <!-- 动画舞台：光晕 + 旋转光环 + 悬浮礼盒 + 纸屑 -->
        <div class="lottery-anim" aria-hidden="true">
          <div class="lottery-glow"></div>
          <div class="lottery-ring"></div>
          <div class="lottery-confetti">
            <i v-for="n in 12" :key="n" :style="confettiStyle(n)"></i>
          </div>
          <svg class="lottery-gift" viewBox="0 0 120 120" fill="none">
            <defs>
              <linearGradient id="lp-box" x1="30" y1="56" x2="92" y2="104" gradientUnits="userSpaceOnUse">
                <stop stop-color="#fbbf24" />
                <stop offset="1" stop-color="#f97316" />
              </linearGradient>
              <linearGradient id="lp-lid" x1="24" y1="44" x2="96" y2="62" gradientUnits="userSpaceOnUse">
                <stop stop-color="#fcd34d" />
                <stop offset="1" stop-color="#fb923c" />
              </linearGradient>
              <linearGradient id="lp-ribbon" x1="0" y1="0" x2="0" y2="1">
                <stop stop-color="#a78bfa" />
                <stop offset="1" stop-color="#6366f1" />
              </linearGradient>
            </defs>
            <!-- 蝴蝶结 + 盒盖（定时开合） -->
            <g class="anim-lid">
              <path d="M60 44C54 34 42 32 40 40c-2 7 10 9 20 4Z" fill="url(#lp-ribbon)" />
              <path d="M60 44c6-10 18-12 20-4 2 7-10 9-20 4Z" fill="url(#lp-ribbon)" />
              <circle cx="60" cy="44" r="5" fill="#8b5cf6" />
              <rect x="25" y="46" width="70" height="16" rx="5" fill="url(#lp-lid)" />
              <rect x="53" y="46" width="14" height="16" fill="url(#lp-ribbon)" />
            </g>
            <!-- 盒身 -->
            <g class="anim-box">
              <rect x="31" y="62" width="58" height="42" rx="6" fill="url(#lp-box)" />
              <rect x="53" y="62" width="14" height="42" fill="url(#lp-ribbon)" />
              <rect x="36" y="66" width="8" height="34" rx="4" fill="#fff" opacity=".25" />
            </g>
          </svg>
          <i class="spark s1">✦</i>
          <i class="spark s2">✦</i>
          <i class="spark s3">✦</i>
        </div>

        <h3 class="mt-2 text-lg font-bold text-gray-900 dark:text-white">🎉 你有一场抽奖可参与</h3>
        <p class="mt-1.5 text-sm leading-relaxed text-gray-500 dark:text-dark-400">
          你符合本次活动的参与条件，赶紧去试试手气吧！
        </p>
        <div class="mt-5 flex justify-center gap-3">
          <button class="btn btn-secondary flex-1" @click="emit('dismiss', activity.id)">稍后再说</button>
          <button class="btn btn-primary flex-1" @click="emit('confirm', activity.id)">去参与</button>
        </div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
/* ===== 动画舞台 ===== */
.lottery-anim {
  position: relative;
  width: 9rem;
  height: 9rem;
  margin: 0 auto;
}

/* 背景脉冲光晕 */
.lottery-glow {
  position: absolute;
  inset: 12%;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(245, 158, 11, 0.35) 0%, rgba(139, 92, 246, 0.18) 45%, transparent 70%);
  animation: lp-glow 2.4s ease-in-out infinite;
}
@keyframes lp-glow {
  0%, 100% { transform: scale(0.92); opacity: 0.65; }
  50% { transform: scale(1.08); opacity: 1; }
}

/* 旋转虚线光环 */
.lottery-ring {
  position: absolute;
  inset: 6%;
  border-radius: 50%;
  border: 2px dashed rgba(139, 92, 246, 0.45);
  animation: lp-spin 9s linear infinite;
}
@keyframes lp-spin {
  to { transform: rotate(360deg); }
}

/* 礼盒：整体悬浮 + 盖子定时开合 */
.lottery-gift {
  position: absolute;
  left: 12%;
  top: 12%;
  width: 76%;
  height: 76%;
  filter: drop-shadow(0 10px 14px rgba(249, 115, 22, 0.35));
  animation: lp-float 2.4s ease-in-out infinite;
}
@keyframes lp-float {
  0%, 100% { transform: translateY(4%); }
  50% { transform: translateY(-7%); }
}
.anim-lid {
  animation: lp-lid 2.4s ease-in-out infinite;
  transform-origin: 60px 60px;
}
@keyframes lp-lid {
  0%, 100% { transform: translateY(0) rotate(0deg); }
  15% { transform: translateY(-6px) rotate(-4deg); }
  30% { transform: translateY(-2px) rotate(2deg); }
  45%, 85% { transform: translateY(0) rotate(0deg); }
}

/* 纸屑雨 */
.lottery-confetti {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
}
.lottery-confetti i {
  position: absolute;
  top: -12px;
  opacity: 0;
  animation: lp-fall linear infinite;
}
@keyframes lp-fall {
  0% { transform: translate(0, -8px) rotate(0deg); opacity: 0; }
  12% { opacity: 1; }
  85% { opacity: 1; }
  100% { transform: translate(var(--sway, 6px), 150px) rotate(var(--spin, 240deg)); opacity: 0; }
}

/* 闪光星 */
.spark {
  position: absolute;
  font-style: normal;
  color: #fbbf24;
  text-shadow: 0 0 6px rgba(251, 191, 36, 0.8);
  animation: lp-twinkle 1.8s ease-in-out infinite;
}
.spark.s1 { top: 6%; right: 10%; font-size: 14px; }
.spark.s2 { bottom: 14%; left: 6%; font-size: 11px; animation-delay: 0.6s; }
.spark.s3 { top: 22%; left: 12%; font-size: 9px; animation-delay: 1.2s; }
@keyframes lp-twinkle {
  0%, 100% { opacity: 0.15; transform: scale(0.7); }
  50% { opacity: 1; transform: scale(1.15); }
}

/* 弹窗过渡 */
.lottery-modal-enter-active,
.lottery-modal-leave-active {
  transition: opacity 200ms ease;
}
.lottery-modal-enter-from,
.lottery-modal-leave-to {
  opacity: 0;
}
</style>
