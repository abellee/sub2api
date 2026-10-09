<script setup lang="ts">
/**
 * 中奖庆祝弹框：弹框内是自绘 SVG。彩带沿抛物线飞出，开始最快，之后逐渐变慢，只播一次。
 * 人已经在抽奖活动页时只保留关闭；其他页面再给一个去查看。
 */
import type { WinnerRecord } from '../../api/lottery'
import { formatValue } from '../../api/lottery'

defineProps<{
  win: WinnerRecord | null
  onLotteryPage: boolean
}>()
const emit = defineEmits<{ (e: 'close', id: number): void; (e: 'view', id: number): void }>()

const CONFETTI_COLORS = ['#f59e0b', '#f97316', '#ec4899', '#8b5cf6', '#3b82f6', '#10b981', '#f43f5e', '#eab308']

type ConfettiPiece = {
  id: string
  side: 'from-left' | 'from-right'
  style: Record<string, string>
}

/** 从左下角、右下角沿抛物线飞过整屏，下落段慢慢淡出，只播一次。 */
function buildConfetti(count: number, layer: number): ConfettiPiece[] {
  const cols = 16
  const rows = 10
  return Array.from({ length: count }, (_, index) => {
    const i = index + layer * 3
    const fromLeft = index % 2 === 0
    const col = (index + layer) % cols
    const row = Math.floor(index / 2) % rows
    const x = 3 + col * (94 / (cols - 1)) + ((i * 5) % 5) - 2
    const y = 6 + row * (98 / (rows - 1)) + ((i * 7) % 5) - 2
    const endX = Math.min(98, Math.max(2, x))
    const endY = Math.min(108, Math.max(4, y))
    const dx = fromLeft ? endX : -(100 - endX)
    return {
      id: `${layer}-${index}`,
      side: fromLeft ? 'from-left' : 'from-right',
      style: {
        background: CONFETTI_COLORS[i % CONFETTI_COLORS.length],
        width: i % 5 === 0 ? '11px' : '7px',
        height: i % 4 === 0 ? '22px' : '14px',
        borderRadius: '2px',
        animationDelay: `${(index % 10) * 0.035}s`,
        animationDuration: `${1.9 + (i % 6) * 0.12}s`,
        animationIterationCount: '1',
        animationFillMode: 'forwards',
        '--dx': `${dx.toFixed(1)}vw`,
        '--dy': `-${endY.toFixed(1)}vh`,
        '--spin': `${(fromLeft ? 1 : -1) * (140 + (i % 6) * 55)}deg`,
      },
    }
  })
}

const backConfetti = buildConfetti(160, 0)
const frontConfetti = buildConfetti(80, 1)
</script>

<template>
  <transition name="lottery-win">
    <div
      v-if="win"
      class="fixed inset-0 z-[2100]"
      role="dialog"
      aria-modal="true"
      aria-labelledby="lottery-win-title"
      data-testid="lottery-win-dialog"
    >
      <div class="absolute inset-0 bg-gray-900/55 backdrop-blur-sm"></div>
      <div class="win-confetti" aria-hidden="true" data-testid="lottery-win-confetti">
        <i v-for="piece in backConfetti" :key="piece.id" :class="piece.side" :style="piece.style"></i>
      </div>

      <div class="relative z-10 flex h-full items-center justify-center px-4">
        <div class="card w-full max-w-sm p-6 text-center shadow-glass">
          <div class="win-stage" aria-hidden="true">
            <div class="win-glow"></div>
            <svg class="win-svg" data-testid="lottery-win-svg" viewBox="0 0 160 160" fill="none">
              <defs>
                <linearGradient id="lw-cup" x1="48" y1="38" x2="112" y2="108" gradientUnits="userSpaceOnUse">
                  <stop stop-color="#fde68a" />
                  <stop offset="0.45" stop-color="#fbbf24" />
                  <stop offset="1" stop-color="#f59e0b" />
                </linearGradient>
                <linearGradient id="lw-ribbon" x1="20" y1="20" x2="70" y2="90" gradientUnits="userSpaceOnUse">
                  <stop stop-color="#c4b5fd" />
                  <stop offset="1" stop-color="#7c3aed" />
                </linearGradient>
                <linearGradient id="lw-base" x1="46" y1="112" x2="114" y2="140" gradientUnits="userSpaceOnUse">
                  <stop stop-color="#fcd34d" />
                  <stop offset="1" stop-color="#d97706" />
                </linearGradient>
              </defs>
              <g class="anim-ribbon anim-ribbon-left">
                <path d="M58 78c-16 6-30 22-24 34 4 8 16 6 24-2 6-6 8-18 0-32Z" fill="url(#lw-ribbon)" />
              </g>
              <g class="anim-ribbon anim-ribbon-right">
                <path d="M102 78c16 6 30 22 24 34-4 8-16 6-24-2-6-6-8-18 0-32Z" fill="url(#lw-ribbon)" />
              </g>
              <g class="anim-cup">
                <path d="M46 58c0 22 10 40 34 40s34-18 34-40V46H46v12Z" fill="url(#lw-cup)" />
                <path d="M46 50c-14 2-20 14-16 24 4 8 14 8 18 2" stroke="#f59e0b" stroke-width="6" stroke-linecap="round" />
                <path d="M114 50c14 2 20 14 16 24-4 8-14 8-18 2" stroke="#f59e0b" stroke-width="6" stroke-linecap="round" />
                <path d="M70 98h20l6 16H64l6-16Z" fill="#fbbf24" />
                <rect x="50" y="114" width="60" height="10" rx="4" fill="url(#lw-base)" />
                <rect x="42" y="124" width="76" height="12" rx="6" fill="url(#lw-base)" />
                <path class="anim-star" d="M80 62l3.2 6.6 7.3 1.1-5.3 5.1 1.3 7.2L80 78.6 73.5 82l1.3-7.2-5.3-5.1 7.3-1.1L80 62Z" fill="#fff7ed" />
              </g>
              <circle class="anim-spark" cx="34" cy="42" r="3" fill="#fde68a" />
              <circle class="anim-spark anim-spark-delay" cx="126" cy="36" r="2.4" fill="#ddd6fe" />
              <circle class="anim-spark anim-spark-late" cx="128" cy="70" r="2" fill="#fb7185" />
            </svg>
          </div>

          <h3 id="lottery-win-title" class="mt-1 text-lg font-bold text-gray-900 dark:text-white">恭喜你中奖了</h3>
          <p v-if="win.activity_name" class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ win.activity_name }}</p>
          <p class="mt-2 text-base font-semibold text-primary-600 dark:text-primary-300">{{ win.prize_name }}</p>
          <div class="mt-3 rounded-xl bg-primary-50 px-4 py-3 dark:bg-primary-900/30">
            <p v-if="win.prize_type === 'balance'" class="text-sm text-primary-800 dark:text-primary-200">
              +{{ formatValue(win.value) }} 余额已到账
            </p>
            <template v-else>
              <p class="text-sm text-primary-800 dark:text-primary-200">兑换码</p>
              <code class="mt-1 inline-block rounded-lg bg-white px-3 py-1.5 font-mono text-sm font-semibold text-gray-900 dark:bg-dark-900 dark:text-primary-300">
                {{ win.redeem_code || '发放中，请稍后在抽奖页查看' }}
              </code>
            </template>
          </div>

          <div class="mt-5 flex justify-center gap-3">
            <button
              v-if="!onLotteryPage"
              type="button"
              class="btn btn-primary flex-1"
              data-testid="lottery-win-view"
              @click="emit('view', win.id)"
            >
              去查看
            </button>
            <button
              type="button"
              class="btn flex-1"
              :class="onLotteryPage ? 'btn-primary' : 'btn-secondary'"
              data-testid="lottery-win-close"
              @click="emit('close', win.id)"
            >
              关闭
            </button>
          </div>
        </div>
      </div>

      <div class="win-confetti win-confetti-front" aria-hidden="true">
        <i v-for="piece in frontConfetti" :key="piece.id" :class="piece.side" :style="piece.style"></i>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.win-stage {
  position: relative;
  width: 9.5rem;
  height: 9.5rem;
  margin: 0 auto;
}
.win-glow {
  position: absolute;
  inset: 8%;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(251, 191, 36, 0.55) 0%, rgba(124, 58, 237, 0.16) 48%, transparent 72%);
  animation: win-glow 2.2s ease-in-out infinite;
}
.win-svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  filter: drop-shadow(0 12px 16px rgba(217, 119, 6, 0.28));
}
.anim-cup {
  transform-origin: 80px 96px;
  animation: win-cup 2.2s ease-in-out infinite;
}
.anim-star {
  transform-origin: 80px 72px;
  animation: win-star 1.6s ease-in-out infinite;
}
.anim-ribbon {
  transform-origin: 80px 90px;
}
.anim-ribbon-left { animation: win-ribbon-left 2.2s ease-in-out infinite; }
.anim-ribbon-right { animation: win-ribbon-right 2.2s ease-in-out infinite; }
.anim-spark { transform-origin: center; animation: win-spark 1.4s ease-in-out infinite; }
.anim-spark-delay { animation-delay: 0.4s; }
.anim-spark-late { animation-delay: 0.8s; }

.win-confetti {
  position: absolute;
  inset: 0;
  z-index: 5;
  overflow: hidden;
  pointer-events: none;
}
.win-confetti-front {
  z-index: 20;
}
.win-confetti i {
  position: absolute;
  bottom: 10px;
  opacity: 0;
  animation-name: win-burst;
  animation-timing-function: linear;
  animation-iteration-count: 1;
  animation-fill-mode: forwards;
}
.win-confetti i.from-left { left: 10px; }
.win-confetti i.from-right { right: 10px; }

@keyframes win-glow {
  0%, 100% { transform: scale(0.92); opacity: 0.7; }
  50% { transform: scale(1.08); opacity: 1; }
}
@keyframes win-cup {
  0%, 100% { transform: translateY(6px) scale(1); }
  50% { transform: translateY(-6px) scale(1.04); }
}
@keyframes win-star {
  0%, 100% { transform: scale(0.85) rotate(-8deg); opacity: 0.8; }
  50% { transform: scale(1.15) rotate(8deg); opacity: 1; }
}
@keyframes win-ribbon-left {
  0%, 100% { transform: rotate(0deg); }
  50% { transform: rotate(-8deg); }
}
@keyframes win-ribbon-right {
  0%, 100% { transform: rotate(0deg); }
  50% { transform: rotate(8deg); }
}
@keyframes win-spark {
  0%, 100% { opacity: 0.2; transform: scale(0.6); }
  50% { opacity: 1; transform: scale(1.3); }
}
@keyframes win-burst {
  /* 前半段仍是抛物线。过了顶点继续落到屏幕外，最后一小段才淡掉。 */
  0% {
    transform: translate3d(0, 0, 0) rotate(0deg) scale(0.7);
    opacity: 0;
  }
  5% {
    transform: translate3d(calc(var(--dx) * 0.1), calc(var(--dy) * 0.36), 0) rotate(calc(var(--spin) * 0.1));
    opacity: 1;
  }
  11% {
    transform: translate3d(calc(var(--dx) * 0.2), calc(var(--dy) * 0.64), 0) rotate(calc(var(--spin) * 0.2));
    opacity: 1;
  }
  16% {
    transform: translate3d(calc(var(--dx) * 0.3), calc(var(--dy) * 0.84), 0) rotate(calc(var(--spin) * 0.3));
    opacity: 1;
  }
  23% {
    transform: translate3d(calc(var(--dx) * 0.4), calc(var(--dy) * 0.96), 0) rotate(calc(var(--spin) * 0.4));
    opacity: 1;
  }
  29% {
    transform: translate3d(calc(var(--dx) * 0.5), var(--dy), 0) rotate(calc(var(--spin) * 0.5));
    opacity: 1;
  }
  37% {
    transform: translate3d(calc(var(--dx) * 0.62), calc(var(--dy) * 0.94), 0) rotate(calc(var(--spin) * 0.58));
    opacity: 1;
  }
  46% {
    transform: translate3d(calc(var(--dx) * 0.76), calc(var(--dy) * 0.7), 0) rotate(calc(var(--spin) * 0.68));
    opacity: 1;
  }
  56% {
    transform: translate3d(calc(var(--dx) * 0.9), calc(var(--dy) * 0.28), 0) rotate(calc(var(--spin) * 0.78));
    opacity: 0.9;
  }
  66% {
    transform: translate3d(calc(var(--dx) * 1.02), calc(var(--dy) * -0.15 + 8vh), 0) rotate(calc(var(--spin) * 0.88));
    opacity: 0.7;
  }
  74% {
    transform: translate3d(calc(var(--dx) * 1.12), calc(var(--dy) * -0.55 + 30vh), 0) rotate(var(--spin));
    opacity: 0;
  }
  100% {
    transform: translate3d(calc(var(--dx) * 1.12), calc(var(--dy) * -0.55 + 30vh), 0) rotate(var(--spin));
    opacity: 0;
  }
}

.lottery-win-enter-active,
.lottery-win-leave-active { transition: opacity 200ms ease; }
.lottery-win-enter-from,
.lottery-win-leave-to { opacity: 0; }

@media (prefers-reduced-motion: reduce) {
  .win-glow,
  .anim-cup,
  .anim-star,
  .anim-ribbon-left,
  .anim-ribbon-right,
  .anim-spark,
  .win-confetti i {
    animation: none;
  }
  .win-confetti { display: none; }
}
</style>
