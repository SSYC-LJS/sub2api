<template>
  <button
    type="button"
    class="group text-left p-5 rounded-2xl min-h-[280px] w-full bg-white/70 backdrop-blur-xl border border-gray-200/80 shadow-card dark:bg-dark-800/60 dark:border-dark-700/70 hover:-translate-y-1 hover:shadow-card-hover dark:hover:border-primary-500/30 hover:border-gray-300 transition-all duration-300 ease-out flex flex-col"
    @click="emit('click')"
  >
    <!-- Header: icon + name/model + status chip -->
    <div class="flex items-start gap-3">
      <span
        class="w-9 h-9 rounded-xl ring-1 ring-black/5 dark:ring-white/10 grid place-items-center flex-shrink-0"
        :class="[providerGradient(item.provider), providerTintClass]"
      >
        <ProviderIcon :provider="item.provider" :size="20" />
      </span>
      <div class="flex-1 min-w-0">
        <div class="text-base font-semibold truncate text-gray-900 dark:text-gray-100">
          {{ item.name }}
        </div>
        <div class="mt-0.5 flex items-center gap-1.5 min-w-0">
          <span
            class="inline-flex items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium flex-shrink-0"
            :class="providerBadgeClass(item.provider)"
          >
            {{ providerLabel(item.provider) }}
          </span>
          <span class="font-mono text-xs truncate text-gray-500 dark:text-gray-400">
            {{ item.primary_model }}
          </span>
          <span
            v-if="item.group_name"
            class="inline-flex items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300 flex-shrink-0"
          >
            {{ item.group_name }}
          </span>
        </div>
      </div>
      <span
        class="px-2.5 py-1 rounded-full text-xs font-semibold flex-shrink-0"
        :class="statusBadgeClass(item.primary_status)"
      >
        {{ statusLabel(item.primary_status) }}
      </span>
    </div>

    <!-- Metrics -->
    <MonitorMetricPair
      primary-icon="bolt"
      :primary-label="t('monitorCommon.dialogLatency')"
      :primary-value="formatLatency(item.primary_latency_ms)"
      primary-unit="ms"
      secondary-icon="globe"
      :secondary-label="t('monitorCommon.endpointPing')"
      :secondary-value="formatLatency(item.primary_ping_latency_ms)"
      secondary-unit="ms"
    />

    <!-- Request windows: replace the old "recent 60 records" timeline with real traffic windows. -->
    <div class="mt-4 flex-1 border-t border-gray-100 pt-3 dark:border-dark-700/60">
      <div class="mb-2 flex items-center justify-between gap-2">
        <span class="text-[10px] font-semibold uppercase tracking-widest text-gray-400">
          {{ t('channelStatus.requestWindows.title') }}
        </span>
        <span class="text-[10px] text-gray-400 tabular-nums">
          {{ t('monitorCommon.nextUpdateIn', { n: countdownSeconds }) }}
        </span>
      </div>

      <div class="grid grid-cols-3 gap-2">
        <div
          v-for="ws in windowStatsDisplay"
          :key="ws.label"
          class="rounded-xl border px-2 py-2"
          :class="ws.cardClass"
        >
          <div class="flex items-center justify-between gap-1">
            <span class="text-[11px] font-semibold text-gray-600 dark:text-gray-300">{{ ws.label }}</span>
            <span class="rounded-full px-1.5 py-0.5 text-[9px] font-semibold" :class="ws.badgeClass">
              {{ ws.levelLabel }}
            </span>
          </div>
          <div class="mt-1 text-lg font-bold tabular-nums" :class="ws.textClass">{{ ws.count }}</div>
          <div class="text-[10px] text-gray-500 dark:text-gray-400">
            {{ t('channelStatus.requestWindows.requests') }}
          </div>
          <div class="mt-1 space-y-0.5 text-[10px] text-gray-500 dark:text-gray-400">
            <div>{{ t('channelStatus.requestWindows.success') }} {{ ws.success }}</div>
            <div>{{ t('channelStatus.requestWindows.errors') }} {{ ws.errors }}</div>
            <div>{{ t('channelStatus.requestWindows.errorRate') }} {{ ws.errorRate }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Availability row -->
    <MonitorAvailabilityRow
      class="mt-3"
      :window-label="availabilityLabel"
      :value="availabilityValue"
      :samples-label="extraModelsCountLabel"
    />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserMonitorView } from '@/api/channelMonitor'
import {
  useChannelMonitorFormat,
  providerGradient,
} from '@/composables/useChannelMonitorFormat'
import ProviderIcon from './ProviderIcon.vue'
import MonitorMetricPair from './MonitorMetricPair.vue'
import MonitorAvailabilityRow from './MonitorAvailabilityRow.vue'

const PROVIDER_TINT: Record<string, string> = {
  openai: 'text-emerald-600 dark:text-emerald-300',
  anthropic: 'text-orange-600 dark:text-orange-300',
  gemini: 'text-sky-600 dark:text-sky-300',
}

const props = defineProps<{
  item: UserMonitorView
  window: '7d' | '15d' | '30d'
  availabilityValue: number | null
  countdownSeconds: number
}>()

const emit = defineEmits<{
  (e: 'click'): void
}>()

const { t } = useI18n()
const {
  statusLabel,
  statusBadgeClass,
  providerLabel,
  providerBadgeClass,
  formatLatency,
} = useChannelMonitorFormat()

const providerTintClass = computed(() =>
  PROVIDER_TINT[props.item.provider] ?? 'text-gray-500 dark:text-gray-300'
)

const availabilityLabel = computed(() => {
  const win = t(`channelStatus.windowTab.${props.window}`)
  return `${t('monitorCommon.availabilityPrefix')} · ${win}`
})

const extraModelsCountLabel = computed(() => {
  const count = props.item.extra_models?.length ?? 0
  if (count === 0) return undefined
  return t('monitorCommon.extraModelsCount', { n: count })
})

// Window stats: 1h / 12h / 24h request counts with congestion indicator
interface WindowStatDisplay {
  label: string
  count: number
  success: number
  errors: number
  errorRate: string
  levelLabel: string
  cardClass: string
  badgeClass: string
  textClass: string
}

type CongestionLevel = 'idle' | 'normal' | 'busy' | 'congested'

function congestionLevel(requests: number, errors: number): CongestionLevel {
  if (requests === 0) return 'idle'
  const errorRate = errors / requests
  if (errorRate > 0.3) return 'congested'
  if (requests > 100) return 'busy'
  if (errorRate > 0.1) return 'busy'
  return 'normal'
}

const congestionStyles: Record<CongestionLevel, { card: string; badge: string; text: string; labelKey: string }> = {
  idle: {
    card: 'border-gray-100 bg-gray-50/80 dark:border-dark-700 dark:bg-dark-900/40',
    badge: 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400',
    text: 'text-gray-500 dark:text-gray-400',
    labelKey: 'idle',
  },
  normal: {
    card: 'border-emerald-100 bg-emerald-50/70 dark:border-emerald-900/40 dark:bg-emerald-900/15',
    badge: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
    text: 'text-emerald-600 dark:text-emerald-300',
    labelKey: 'normal',
  },
  busy: {
    card: 'border-amber-100 bg-amber-50/80 dark:border-amber-900/40 dark:bg-amber-900/15',
    badge: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
    text: 'text-amber-600 dark:text-amber-300',
    labelKey: 'busy',
  },
  congested: {
    card: 'border-red-100 bg-red-50/80 dark:border-red-900/40 dark:bg-red-900/15',
    badge: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    text: 'text-red-600 dark:text-red-300',
    labelKey: 'congested',
  },
}

function formatErrorRate(requests: number, errors: number): string {
  if (requests <= 0) return '0%'
  return `${((errors / requests) * 100).toFixed(1).replace(/\.0$/, '')}%`
}

const windowStatsDisplay = computed<WindowStatDisplay[]>(() => {
  const ws = props.item.window_stats
  if (!ws) return []
  const windows: Array<{ label: string; req: number; ok: number; err: number }> = [
    { label: '1h', req: ws.requests_1h, ok: ws.success_1h, err: ws.errors_1h },
    { label: '12h', req: ws.requests_12h, ok: ws.success_12h, err: ws.errors_12h },
    { label: '24h', req: ws.requests_24h, ok: ws.success_24h, err: ws.errors_24h },
  ]
  return windows.map(w => {
    const level = congestionLevel(w.req, w.err)
    const style = congestionStyles[level]
    return {
      label: w.label,
      count: w.req,
      success: w.ok,
      errors: w.err,
      errorRate: formatErrorRate(w.req, w.err),
      levelLabel: t(`channelStatus.requestWindows.level.${style.labelKey}`),
      cardClass: style.card,
      badgeClass: style.badge,
      textClass: style.text,
    }
  })
})
</script>
