<script setup lang="ts">
/**
 * LotteryFlipCountdown —— 翻牌样式倒计时。
 * 展示「距开始 / 距开奖」的天/时/分/秒，数字变化时播放翻转动画。
 * 独立组件：传入目标时间与标签即可，内部自走秒。
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps<{
  /** 目标时间（ISO 字符串或 Date）。 */
  target: string | Date
  /** 倒计时标签，如「距开奖」「距开始」。 */
  label: string
  /** 占位模式：倒计时全部显示「-」（已开奖场次）。 */
  dash?: boolean
}>()

const emit = defineEmits<{ (e: 'finish', label: string): void }>()

const now = ref(new Date())
let timer: number | undefined

onMounted(() => {
  timer = window.setInterval(() => (now.value = new Date()), 1000)
})
onBeforeUnmount(() => window.clearInterval(timer))

const remainingMs = computed(() => {
  const target = typeof props.target === 'string' ? new Date(props.target) : props.target
  return target.getTime() - now.value.getTime()
})

// 目标时间到达时触发一次 finish（父组件可借此刷新数据）
const fired = ref(false)
watch(remainingMs, (ms) => {
  if (ms <= 0 && !fired.value) {
    fired.value = true
    emit('finish', props.label)
  }
})

const pad = (n: number) => String(n).padStart(2, '0')

const segments = computed(() => {
  if (props.dash) {
    return [
      { label: '天', value: '-' },
      { label: '时', value: '-' },
      { label: '分', value: '-' },
      { label: '秒', value: '-' }
    ]
  }
  const diff = Math.max(0, remainingMs.value)
  const total = Math.floor(diff / 1000)
  const days = Math.floor(total / 86400)
  const hours = Math.floor((total % 86400) / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const seconds = total % 60
  return [
    { label: '天', value: pad(days) },
    { label: '时', value: pad(hours) },
    { label: '分', value: pad(minutes) },
    { label: '秒', value: pad(seconds) }
  ]
})

/** 是否仍在倒计时（目标未到）。 */
const active = computed(() => props.dash || remainingMs.value > 0)
</script>

<template>
  <div v-if="active" class="flex flex-col items-start gap-2">
    <span class="text-xs font-medium uppercase tracking-wider text-primary-100/80">{{ label }}</span>
    <div class="flex items-start gap-1.5">
      <template v-for="(seg, i) in segments" :key="seg.label">
        <div class="flex flex-col items-center gap-1.5">
          <div class="flip-card">
            <!-- :key 绑定数值：数字变化时重建元素触发翻转动画 -->
            <span :key="seg.value" class="flip-digit">{{ seg.value }}</span>
          </div>
          <span class="text-[11px] text-primary-100/80">{{ seg.label }}</span>
        </div>
        <span v-if="i < segments.length - 1" class="mt-3 text-xl font-bold leading-none text-primary-100/50">:</span>
      </template>
    </div>
  </div>
</template>

<style scoped>
.flip-card {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 56px;
  perspective: 240px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: linear-gradient(to bottom, #1e293b 49.5%, #0f172a 50.5%);
  box-shadow: 0 3px 10px rgba(2, 6, 23, 0.45), inset 0 1px 0 rgba(255, 255, 255, 0.08);
  overflow: hidden;
}
/* 中缝：翻牌钟的上下两半分界 */
.flip-card::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  top: 50%;
  height: 1px;
  background: rgba(2, 6, 23, 0.7);
}
.flip-digit {
  display: block;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 26px;
  font-weight: 700;
  line-height: 1;
  color: #f8fafc;
  /* 翻牌铰链在顶部：数字从上往下翻落 */
  transform-origin: center top;
  animation: lottery-flip 0.45s cubic-bezier(0.3, 1.2, 0.6, 1);
  backface-visibility: hidden;
}
@keyframes lottery-flip {
  0% {
    transform: rotateX(90deg);
    opacity: 0.2;
  }
  55% {
    opacity: 1;
  }
  100% {
    transform: rotateX(0deg);
    opacity: 1;
  }
}
</style>
