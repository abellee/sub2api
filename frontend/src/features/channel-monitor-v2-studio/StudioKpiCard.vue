<template>
  <div
    class="stat-card studio-kpi !min-h-[8.5rem] !flex-col !gap-2.5 !rounded-3xl !border-0 !p-4 shadow-sm ring-1 ring-gray-900/5 dark:ring-dark-700"
    :class="[accentClass, joined ? 'studio-kpi--joined' : '']"
    :title="title || undefined"
  >
    <div class="studio-kpi-head flex w-full items-start">
      <div class="studio-kpi-identity flex min-w-0 flex-1 items-start gap-2">
        <span
          class="mt-0.5 inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-white/80 ring-1 ring-gray-900/5 dark:bg-dark-700"
        >
          <StudioBrandIcon :platform="platform" size="md" />
        </span>
        <div class="studio-kpi-copy">
          <span
            class="studio-kpi-name stat-label block text-[13px] font-black tracking-tight text-gray-900 dark:text-white"
            :title="label"
          >
            {{ label }}
          </span>
          <div class="studio-kpi-brand-row mt-1 flex items-center gap-1">
            <span
              v-if="brandLabel"
              class="studio-kpi-brand"
              :style="{ '--studio-brand-color': brandColor }"
            >{{ brandLabel }}</span>
            <span
              v-if="rateLabel"
              class="studio-kpi-rate"
              :title="rateLabel"
            >{{ rateLabel }}</span>
          </div>
        </div>
      </div>
      <span
        class="studio-kpi-status ml-auto shrink-0 text-right whitespace-nowrap"
        :class="statusClass"
        :aria-label="`${statusHeading} ${statusLabel}`"
      >{{ statusLabel }}</span>
    </div>

    <dl class="studio-kpi-metrics grid w-full">
      <div>
        <dt class="studio-kpi-metric-label text-[10px] font-semibold text-gray-400">{{ successLabel }}</dt>
        <dd
          class="stat-value mt-0.5 block overflow-visible text-[13px] tabular-nums leading-tight !text-clip !whitespace-normal"
          :class="successClass"
        >{{ successRate }}</dd>
      </div>
      <div>
        <dt class="studio-kpi-metric-label text-[10px] font-semibold text-gray-400">{{ ttftLabel }}</dt>
        <dd class="mt-0.5 text-[13px] font-semibold tabular-nums leading-tight" :class="ttftClass">
          {{ ttft }}
        </dd>
      </div>
      <div>
        <dt class="studio-kpi-metric-label text-[10px] font-semibold text-gray-400">{{ cacheLabel }}</dt>
        <dd class="mt-0.5 text-[13px] font-semibold tabular-nums leading-tight" :class="cacheClass">
          {{ cacheRate }}
        </dd>
      </div>
    </dl>

    <StudioSparkChart
      v-if="buckets.length"
      class="studio-kpi-spark"
      :buckets="buckets"
      :coverage="coverage"
      :show-throughput="showThroughput"
      :label="label"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { MonitorCoverage } from '@/api/channelMonitorV2'
import type { GroupPlatform } from '@/types'
import StudioBrandIcon from './StudioBrandIcon.vue'
import StudioSparkChart from './StudioSparkChart.vue'
import { studioBrandFill } from './studioBrand'
import type { StudioBucketPoint } from './studioBuckets'
import { metricTextClass, type StudioAccent, type StudioTone } from './studioFormat'

const props = withDefaults(
  defineProps<{
    label: string
    platform?: GroupPlatform
    brandLabel?: string
    successLabel: string
    successRate: string
    ttftLabel: string
    ttft: string
    cacheLabel: string
    cacheRate: string
    statusHeading: string
    statusLabel: string
    state?: StudioTone
    successState?: StudioTone
    ttftState?: StudioTone
    cacheState?: StudioTone
    title?: string
    rateLabel?: string
    joined?: boolean
    accent?: StudioAccent
    buckets?: StudioBucketPoint[]
    coverage?: MonitorCoverage | null
    showThroughput?: boolean
  }>(),
  {
    accent: 'teal',
    buckets: () => [],
    coverage: null,
    showThroughput: true,
    joined: false,
  },
)

function isMissing(value: string | undefined) {
  const next = (value || '').trim()
  return next === '' || next === '-' || next === '—'
}

const brandColor = computed(() => studioBrandFill(props.platform))

const accentClass = computed(() => {
  if (props.accent === 'coral') return 'studio-kpi--coral'
  if (props.accent === 'indigo') return 'studio-kpi--indigo'
  if (props.accent === 'amber') return 'studio-kpi--amber'
  if (props.accent === 'sky') return 'studio-kpi--sky'
  return 'studio-kpi--teal'
})

const successClass = computed(() => metricTextClass(props.successState, isMissing(props.successRate)))
const ttftClass = computed(() => metricTextClass(props.ttftState, isMissing(props.ttft)))
const cacheClass = computed(() => metricTextClass(props.cacheState, isMissing(props.cacheRate)))

const statusClass = computed(() => {
  if (props.state === 'healthy') return 'studio-kpi-status--healthy'
  if (props.state === 'warning') return 'studio-kpi-status--warning'
  if (props.state === 'critical') return 'studio-kpi-status--critical'
  return 'studio-kpi-status--unknown'
})
</script>

<style scoped>
.studio-kpi {
  --studio-faces-bg: rgb(241 245 249);
  position: relative;
  z-index: 2;
  contain: layout style;
}
.studio-kpi--joined {
  height: 100%;
  border-radius: 0 !important;
  box-shadow: none;
  --tw-ring-shadow: 0 0 #0000;
}
.studio-kpi--teal {
  background: linear-gradient(180deg, rgb(240 253 250) 0%, rgb(255 255 255) 58%);
}
.studio-kpi--coral {
  background: linear-gradient(180deg, rgb(255 247 237) 0%, rgb(255 255 255) 58%);
}
.studio-kpi--indigo {
  background: linear-gradient(180deg, rgb(238 242 255) 0%, rgb(255 255 255) 58%);
}
.studio-kpi--amber {
  background: linear-gradient(180deg, rgb(255 251 235) 0%, rgb(255 255 255) 58%);
}
.studio-kpi--sky {
  background: linear-gradient(180deg, rgb(240 249 255) 0%, rgb(255 255 255) 58%);
}
.studio-kpi--joined.studio-kpi--teal {
  background: linear-gradient(180deg, rgb(240 253 250) 0%, var(--studio-faces-bg) 100%);
}
.studio-kpi--joined.studio-kpi--coral {
  background: linear-gradient(180deg, rgb(255 247 237) 0%, var(--studio-faces-bg) 100%);
}
.studio-kpi--joined.studio-kpi--indigo {
  background: linear-gradient(180deg, rgb(238 242 255) 0%, var(--studio-faces-bg) 100%);
}
.studio-kpi--joined.studio-kpi--amber {
  background: linear-gradient(180deg, rgb(255 251 235) 0%, var(--studio-faces-bg) 100%);
}
.studio-kpi--joined.studio-kpi--sky {
  background: linear-gradient(180deg, rgb(240 249 255) 0%, var(--studio-faces-bg) 100%);
}
@container studio-row (min-width: 34rem) {
  .studio-kpi--joined.studio-kpi--teal {
    background: linear-gradient(115deg, rgb(240 253 250) 0%, rgb(240 253 250) 42%, var(--studio-faces-bg) 100%);
  }
  .studio-kpi--joined.studio-kpi--coral {
    background: linear-gradient(115deg, rgb(255 247 237) 0%, rgb(255 247 237) 42%, var(--studio-faces-bg) 100%);
  }
  .studio-kpi--joined.studio-kpi--indigo {
    background: linear-gradient(115deg, rgb(238 242 255) 0%, rgb(238 242 255) 42%, var(--studio-faces-bg) 100%);
  }
  .studio-kpi--joined.studio-kpi--amber {
    background: linear-gradient(115deg, rgb(255 251 235) 0%, rgb(255 251 235) 42%, var(--studio-faces-bg) 100%);
  }
  .studio-kpi--joined.studio-kpi--sky {
    background: linear-gradient(115deg, rgb(240 249 255) 0%, rgb(240 249 255) 42%, var(--studio-faces-bg) 100%);
  }
}
.dark .studio-kpi {
  --studio-faces-bg: rgb(30 41 59);
}
.dark .studio-kpi--teal {
  background: linear-gradient(180deg, rgb(19 78 74) 0%, rgb(15 23 42) 58%);
}
.dark .studio-kpi--coral {
  background: linear-gradient(180deg, rgb(67 32 18) 0%, rgb(15 23 42) 58%);
}
.dark .studio-kpi--indigo {
  background: linear-gradient(180deg, rgb(49 46 129) 0%, rgb(15 23 42) 58%);
}
.dark .studio-kpi--amber {
  background: linear-gradient(180deg, rgb(69 47 12) 0%, rgb(15 23 42) 58%);
}
.dark .studio-kpi--sky {
  background: linear-gradient(180deg, rgb(12 74 110) 0%, rgb(15 23 42) 58%);
}
.dark .studio-kpi--joined.studio-kpi--teal {
  background: linear-gradient(180deg, rgb(19 78 74) 0%, var(--studio-faces-bg) 100%);
}
.dark .studio-kpi--joined.studio-kpi--coral {
  background: linear-gradient(180deg, rgb(67 32 18) 0%, var(--studio-faces-bg) 100%);
}
.dark .studio-kpi--joined.studio-kpi--indigo {
  background: linear-gradient(180deg, rgb(49 46 129) 0%, var(--studio-faces-bg) 100%);
}
.dark .studio-kpi--joined.studio-kpi--amber {
  background: linear-gradient(180deg, rgb(69 47 12) 0%, var(--studio-faces-bg) 100%);
}
.dark .studio-kpi--joined.studio-kpi--sky {
  background: linear-gradient(180deg, rgb(12 74 110) 0%, var(--studio-faces-bg) 100%);
}
@container studio-row (min-width: 34rem) {
  .dark .studio-kpi--joined.studio-kpi--teal {
    background: linear-gradient(115deg, rgb(19 78 74) 0%, rgb(19 78 74) 42%, var(--studio-faces-bg) 100%);
  }
  .dark .studio-kpi--joined.studio-kpi--coral {
    background: linear-gradient(115deg, rgb(67 32 18) 0%, rgb(67 32 18) 42%, var(--studio-faces-bg) 100%);
  }
  .dark .studio-kpi--joined.studio-kpi--indigo {
    background: linear-gradient(115deg, rgb(49 46 129) 0%, rgb(49 46 129) 42%, var(--studio-faces-bg) 100%);
  }
  .dark .studio-kpi--joined.studio-kpi--amber {
    background: linear-gradient(115deg, rgb(69 47 12) 0%, rgb(69 47 12) 42%, var(--studio-faces-bg) 100%);
  }
  .dark .studio-kpi--joined.studio-kpi--sky {
    background: linear-gradient(115deg, rgb(12 74 110) 0%, rgb(12 74 110) 42%, var(--studio-faces-bg) 100%);
  }
}

.studio-kpi-head {
  gap: 0.85rem;
}
.studio-kpi-identity {
  padding-right: 0.15rem;
}
.studio-kpi-copy {
  min-width: 8em;
  max-width: 100%;
  font-size: 13px;
}
.studio-kpi-name {
  max-width: 8em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.studio-kpi-brand-row {
  flex-wrap: wrap;
}
.studio-kpi-brand {
  display: inline-flex;
  flex: 0 0 auto;
  min-width: 0;
  max-width: none;
  align-items: center;
  overflow: visible;
  white-space: nowrap;
  border-radius: 9999px;
  background: color-mix(in srgb, var(--studio-brand-color) 14%, transparent);
  padding: 0.05rem 0.45rem;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.4;
  color: var(--studio-brand-color);
}
.studio-kpi-rate {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  border-radius: 9999px;
  background: rgb(243 244 246);
  padding: 0.05rem 0.4rem;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.4;
  font-variant-numeric: tabular-nums;
  color: rgb(71 85 105);
  white-space: nowrap;
}
.dark .studio-kpi-rate {
  background: rgb(30 41 59 / 0.85);
  color: rgb(203 213 225);
}

.studio-kpi-metrics {
  margin-top: 0.55rem;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}
.studio-kpi-metrics > div {
  min-width: 0;
  text-align: center;
}
.dark .studio-kpi-metric-label {
  color: rgb(255 255 255);
}

.studio-kpi-status {
  border-radius: 9999px;
  padding: 0.2rem 0.55rem;
  font-size: 13px;
  font-weight: 800;
  line-height: 1.3;
  letter-spacing: 0.01em;
}
.studio-kpi-status--healthy {
  background: rgb(209 250 229);
  color: rgb(4 120 87);
}
.studio-kpi-status--warning {
  background: rgb(254 243 199);
  color: rgb(180 83 9);
}
.studio-kpi-status--critical {
  background: rgb(254 226 226);
  color: rgb(185 28 28);
}
.studio-kpi-status--unknown {
  background: rgb(243 244 246);
  color: rgb(107 114 128);
}
.dark .studio-kpi-status--healthy {
  background: rgb(6 78 59 / 0.55);
  color: rgb(110 231 183);
}
.dark .studio-kpi-status--warning {
  background: rgb(120 53 15 / 0.55);
  color: rgb(253 186 116);
}
.dark .studio-kpi-status--critical {
  background: rgb(127 29 29 / 0.5);
  color: rgb(252 165 165);
}
.dark .studio-kpi-status--unknown {
  background: rgb(51 65 85 / 0.85);
  color: rgb(148 163 184);
}

.studio-kpi-spark {
  color: #14b8a6;
}
.studio-kpi--coral .studio-kpi-spark {
  color: #f97316;
}
.studio-kpi--indigo .studio-kpi-spark {
  color: #6366f1;
}
.studio-kpi--amber .studio-kpi-spark {
  color: #d97706;
}
.studio-kpi--sky .studio-kpi-spark {
  color: #0284c7;
}
</style>
