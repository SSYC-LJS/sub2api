<template>
  <section class="ranking-stage relative overflow-hidden rounded-2xl border border-gray-200 bg-white p-5 shadow-sm transition-colors duration-300 dark:border-dark-600 dark:bg-dark-800">
    <div class="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(20,184,166,0.10),transparent_42%),linear-gradient(180deg,rgba(249,250,251,0.72),transparent_34%)] dark:bg-[radial-gradient(circle_at_top,rgba(20,184,166,0.16),transparent_42%),linear-gradient(180deg,rgba(31,41,55,0.28),transparent_36%)]" />
    <div class="pointer-events-none absolute -left-20 top-10 h-44 w-44 animate-blob rounded-full bg-primary-500/10 blur-3xl dark:bg-primary-500/10" />
    <div class="pointer-events-none absolute -right-16 top-40 h-52 w-52 animate-blob-delayed rounded-full bg-gray-300/20 blur-3xl dark:bg-dark-600/30" />
    <div class="pointer-events-none absolute inset-x-8 top-0 h-px bg-gradient-to-r from-transparent via-primary-400/35 to-transparent dark:via-primary-400/25" />

    <div class="relative mb-6 flex animate-fade-up flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
      <div>
        <div class="inline-flex items-center gap-2 rounded-full border border-primary-100 bg-primary-50 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.32em] text-primary-600 shadow-sm dark:border-primary-500/20 dark:bg-primary-500/10 dark:text-primary-300">
          {{ t('dashboard.tokenRankingTitle') }}
        </div>
        <h3 class="mt-3 text-2xl font-black tracking-tight text-gray-900 sm:text-3xl dark:text-white">{{ t('dashboard.tokenRankingSubtitle') }}</h3>
      </div>
      <div class="inline-flex flex-wrap gap-2 rounded-lg border border-gray-200 bg-gray-50 p-1.5 shadow-sm dark:border-dark-600 dark:bg-dark-700">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          :class="[
            'rounded-md px-4 py-2 text-sm font-semibold transition-all duration-300 hover:-translate-y-0.5',
            activePeriod === tab.key
              ? 'bg-primary-600 text-white shadow-sm hover:bg-primary-700 dark:bg-primary-500 dark:hover:bg-primary-600'
              : 'text-gray-500 hover:bg-white hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-600 dark:hover:text-white'
          ]"
          @click="activePeriod = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-16 text-gray-500 dark:text-gray-400">
      <LoadingSpinner />
    </div>

    <template v-else>
      <div class="relative grid grid-cols-1 items-end gap-4 lg:grid-cols-3 lg:gap-5">
        <article
          v-for="(slot, index) in podiumSlots"
          :key="slot.key"
          :style="{ animationDelay: `${index * 120}ms` }"
          :class="[
            'podium-card relative overflow-hidden rounded-2xl border px-5 pb-5 pt-4 text-center backdrop-blur-xl transition-all duration-500 hover:-translate-y-2 hover:scale-[1.015]',
            podiumCardClass(slot.rank),
            slot.rank === 1 ? 'z-10 ring-2 ring-primary-300/70 lg:-translate-y-8 lg:scale-[1.1] dark:ring-primary-400/30' : 'lg:translate-y-7',
            slot.placeholder ? 'border-dashed opacity-80' : ''
          ]"
        >
          <div v-if="slot.rank === 1" class="absolute right-4 top-4 z-10 rounded-full border border-primary-200 bg-white/85 px-3 py-1 text-[11px] font-black tracking-[0.22em] text-primary-700 shadow-sm dark:border-primary-500/30 dark:bg-dark-800/85 dark:text-primary-300">
            NO.1
          </div>
          <div class="pointer-events-none absolute inset-0 opacity-90" :class="podiumGlowClass(slot.rank)" />
          <div class="pointer-events-none absolute inset-x-8 top-0 h-px bg-gradient-to-r from-transparent via-primary-300/70 to-transparent opacity-70 dark:via-primary-300/30" />
          <div class="relative mx-auto flex h-20 w-20 items-center justify-center">
            <div class="absolute inset-0 animate-pulse-slow rounded-full blur-xl" :class="medalGlowClass(slot.rank)" />
            <div :class="[
              'relative flex animate-float items-center justify-center rounded-full border bg-white shadow-inner dark:bg-dark-700',
              slot.rank === 1 ? 'h-24 w-24 border-primary-200 text-4xl shadow-glow dark:border-primary-500/35' : 'h-20 w-20 border-gray-200 text-3xl dark:border-dark-500'
            ]">
              {{ rankMedal(slot.rank) }}
            </div>
          </div>
          <div class="relative mt-3 flex items-center justify-center gap-2 text-xs font-semibold uppercase tracking-[0.2em] text-gray-500 dark:text-gray-400">
            <span class="h-px w-8 bg-gray-200 dark:bg-dark-500" />
            <span>{{ slot.placeholder ? t('dashboard.rankingPlaceholder') : `TOP ${slot.rank}` }}</span>
            <span class="h-px w-8 bg-gray-200 dark:bg-dark-500" />
          </div>
          <h4 class="relative mt-3 truncate text-xl font-black text-gray-900 dark:text-white" :title="displayName(slot)">{{ displayName(slot) }}</h4>
          <p class="relative mt-1 text-sm font-semibold text-gray-500 dark:text-gray-400">{{ rankDescription(slot.rank) }}</p>
          <p class="relative mt-4 font-black tracking-tight text-gray-900 dark:text-white" :class="slot.rank === 1 ? 'text-5xl sm:text-6xl' : 'text-4xl sm:text-5xl'">
            {{ slot.placeholder ? '—' : formatTokens(slot.item.tokens) }}
          </p>
          <div class="relative mt-3 flex items-center justify-center gap-2 text-xs text-gray-500 dark:text-gray-400">
            <span>{{ slot.placeholder ? '—' : formatNumber(slot.item.requests) }} {{ t('dashboard.requests') }}</span>
            <span class="h-1 w-1 rounded-full bg-gray-300 dark:bg-dark-500" />
            <span>{{ slot.placeholder ? '—' : `$${formatCost(slot.item.actual_cost)}` }}</span>
          </div>
        </article>
      </div>

      <div class="mt-8 grid gap-3">
        <article
          v-for="(slot, index) in restSlots"
          :key="slot.key"
          :style="{ animationDelay: `${420 + index * 70}ms` }"
          class="rank-row group relative overflow-hidden rounded-xl border border-gray-200 bg-white px-4 py-3 text-gray-900 shadow-sm transition-all duration-300 hover:-translate-y-1 hover:border-primary-200 hover:bg-primary-50/40 hover:shadow-md dark:border-dark-600 dark:bg-dark-800 dark:text-white dark:hover:border-primary-500/30 dark:hover:bg-dark-700"
        >
          <div class="pointer-events-none absolute inset-y-0 left-0 w-1 bg-primary-500 opacity-80 dark:bg-primary-400" />
          <div class="pointer-events-none absolute inset-y-0 left-8 w-28 bg-gradient-to-r from-primary-500/10 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100 dark:from-primary-400/10" />
          <div class="flex items-center gap-4 pl-2">
            <div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl border border-gray-200 bg-gray-50 text-lg font-black text-gray-700 shadow-inner dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200">
              {{ slot.rank }}
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-3">
                <span class="truncate text-base font-semibold" :class="slot.placeholder ? 'text-gray-400 dark:text-gray-500' : 'text-gray-900 dark:text-white'" :title="displayName(slot)">
                  {{ displayName(slot) }}
                </span>
                <span v-if="slot.placeholder" class="rounded-full border border-gray-200 bg-gray-50 px-2 py-0.5 text-[11px] text-gray-400 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-500">
                  {{ t('dashboard.rankingPlaceholder') }}
                </span>
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ slot.placeholder ? '——' : `${formatNumber(slot.item.requests)} ${t('dashboard.requests')}` }}</p>
            </div>
            <div class="shrink-0 text-right">
              <p class="text-2xl font-black tracking-tight" :class="slot.placeholder ? 'text-gray-300 dark:text-dark-500' : 'text-gray-900 dark:text-white'">
                {{ slot.placeholder ? '—' : formatTokens(slot.item.tokens) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ slot.placeholder ? '—' : `$${formatCost(slot.item.actual_cost)}` }}</p>
            </div>
          </div>
        </article>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { UserTokenRankingItem, UserTokenRankingResponse } from '@/api/usage'

const props = defineProps<{
  ranking: UserTokenRankingResponse | null
  loading: boolean
}>()

const { t } = useI18n()
const activePeriod = ref<'all' | 'today' | 'week' | 'month'>('all')

type RankingSlot = {
  key: string
  rank: number
  item: UserTokenRankingItem
  placeholder: boolean
}

const tabs = computed(() => [
  { key: 'all' as const, label: '总榜' },
  { key: 'today' as const, label: t('dashboard.rankingToday') },
  { key: 'week' as const, label: t('dashboard.rankingWeek') },
  { key: 'month' as const, label: t('dashboard.rankingMonth') },
])

const items = computed(() => {
  const period = props.ranking?.[activePeriod.value]
  return period?.ranking ?? []
})

const rankingSlots = computed<RankingSlot[]>(() =>
  Array.from({ length: 10 }, (_, index) => {
    const rank = index + 1
    const item = items.value[index]
    if (item) {
      return { key: `user-${item.user_id}-${activePeriod.value}-${rank}`, rank, item, placeholder: false }
    }
    return {
      key: `placeholder-${activePeriod.value}-${rank}`,
      rank,
      placeholder: true,
      item: {
        user_id: 0,
        email: '',
        username: t('dashboard.rankingPlaceholder'),
        actual_cost: 0,
        requests: 0,
        tokens: 0,
      },
    }
  })
)
const podiumSlots = computed(() => [rankingSlots.value[1], rankingSlots.value[0], rankingSlots.value[2]].filter(Boolean))
const restSlots = computed(() => rankingSlots.value.slice(3))

const maskValue = (value: string) => {
  const text = (value || '').trim()
  if (!text) return '***'
  return `${text.slice(0, 3)}***${text.slice(-3)}`
}

const displayName = (slot: RankingSlot) => {
  if (slot.placeholder) return t('dashboard.rankingPlaceholder')
  const username = slot.item.username?.trim()
  if (username) return username
  return maskValue(slot.item.email)
}

const rankMedal = (rank: number) => {
  if (rank === 1) return '🥇'
  if (rank === 2) return '🥈'
  if (rank === 3) return '🥉'
  return rank.toString()
}

const rankDescription = (rank: number) => {
  if (rank === 1) return t('dashboard.rankingFirstDescription')
  if (rank === 2) return t('dashboard.rankingSecondDescription')
  if (rank === 3) return t('dashboard.rankingThirdDescription')
  return ''
}

const formatNumber = (value: number) => new Intl.NumberFormat().format(value || 0)
const formatTokens = (value: number) => {
  const n = value || 0
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(2)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return formatNumber(n)
}
const formatCost = (value: number) => (value || 0).toFixed(4)

const podiumCardClass = (rank: number) => {
  if (rank === 1) return 'border-primary-200 bg-primary-50/90 shadow-md dark:border-primary-500/30 dark:bg-primary-500/10 dark:shadow-[0_0_26px_rgba(20,184,166,0.10)]'
  if (rank === 2) return 'border-gray-200 bg-gray-50/90 shadow-sm dark:border-dark-600 dark:bg-dark-700/80'
  return 'border-gray-200 bg-gray-50/90 shadow-sm dark:border-dark-600 dark:bg-dark-700/80'
}

const podiumGlowClass = (rank: number) => {
  if (rank === 1) return 'bg-[radial-gradient(circle,rgba(20,184,166,0.12),transparent_62%)] dark:bg-[radial-gradient(circle,rgba(20,184,166,0.18),transparent_60%)]'
  return 'bg-[radial-gradient(circle,rgba(148,163,184,0.12),transparent_62%)] dark:bg-[radial-gradient(circle,rgba(75,85,99,0.20),transparent_60%)]'
}

const medalGlowClass = (rank: number) => {
  if (rank === 1) return 'bg-primary-300/25 dark:bg-primary-400/20'
  return 'bg-gray-300/25 dark:bg-gray-500/20'
}
</script>

<style scoped>
.ranking-stage {
  animation: stage-in 520ms ease-out both;
}

.podium-card,
.rank-row {
  animation: fade-up 560ms cubic-bezier(0.22, 1, 0.36, 1) both;
}

.animate-fade-up {
  animation: fade-up 520ms ease-out both;
}

.animate-float {
  animation: float-medal 3.6s ease-in-out infinite;
}

.animate-pulse-slow {
  animation: pulse-soft 2.8s ease-in-out infinite;
}

.animate-blob {
  animation: blob-drift 9s ease-in-out infinite;
}

.animate-blob-delayed {
  animation: blob-drift 11s ease-in-out infinite reverse;
}

@keyframes stage-in {
  from {
    opacity: 0;
    transform: scale(0.985);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

@keyframes fade-up {
  from {
    opacity: 0;
    transform: translateY(18px) scale(0.98);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes float-medal {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-7px);
  }
}

@keyframes pulse-soft {
  0%,
  100% {
    opacity: 0.55;
    transform: scale(0.96);
  }
  50% {
    opacity: 0.95;
    transform: scale(1.08);
  }
}

@keyframes blob-drift {
  0%,
  100% {
    transform: translate3d(0, 0, 0) scale(1);
  }
  50% {
    transform: translate3d(22px, -18px, 0) scale(1.12);
  }
}

@media (prefers-reduced-motion: reduce) {
  .ranking-stage,
  .podium-card,
  .rank-row,
  .animate-fade-up,
  .animate-float,
  .animate-pulse-slow,
  .animate-blob,
  .animate-blob-delayed {
    animation: none !important;
  }
}
</style>
