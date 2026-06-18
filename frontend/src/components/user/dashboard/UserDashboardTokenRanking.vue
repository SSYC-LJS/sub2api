<template>
  <section class="ranking-stage relative overflow-hidden rounded-[28px] border border-slate-200/80 bg-gradient-to-br from-white via-violet-50 to-amber-50 p-5 shadow-[0_24px_70px_rgba(88,80,180,0.16)] transition-colors duration-300 dark:border-white/10 dark:bg-[#070814] dark:bg-none dark:shadow-[0_0_60px_rgba(122,92,255,0.18)]">
    <div class="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(139,92,246,0.18),transparent_42%),radial-gradient(circle_at_bottom_right,rgba(245,158,11,0.16),transparent_36%)] dark:bg-[radial-gradient(circle_at_top,rgba(126,87,255,0.24),transparent_40%),radial-gradient(circle_at_bottom_right,rgba(255,198,87,0.16),transparent_35%)]" />
    <div class="pointer-events-none absolute -left-20 top-10 h-44 w-44 animate-blob rounded-full bg-violet-400/20 blur-3xl dark:bg-violet-500/20" />
    <div class="pointer-events-none absolute -right-16 top-40 h-52 w-52 animate-blob-delayed rounded-full bg-amber-300/20 blur-3xl dark:bg-amber-400/10" />
    <div class="pointer-events-none absolute inset-x-8 top-0 h-px bg-gradient-to-r from-transparent via-violet-400/50 to-transparent dark:via-white/30" />

    <div class="relative mb-6 flex animate-fade-up flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
      <div>
        <div class="inline-flex items-center gap-2 rounded-full border border-violet-200 bg-white/70 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.32em] text-violet-600 shadow-sm backdrop-blur dark:border-white/10 dark:bg-white/5 dark:text-violet-200">
          {{ t('dashboard.tokenRankingTitle') }}
        </div>
        <h3 class="mt-3 text-2xl font-black tracking-tight text-slate-950 sm:text-3xl dark:text-white">{{ t('dashboard.tokenRankingSubtitle') }}</h3>
      </div>
      <div class="inline-flex flex-wrap gap-2 rounded-full border border-slate-200 bg-white/75 p-2 shadow-sm backdrop-blur dark:border-white/10 dark:bg-black/25">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          :class="[
            'rounded-full px-4 py-2 text-sm font-semibold transition-all duration-300 hover:-translate-y-0.5',
            activePeriod === tab.key
              ? 'bg-gradient-to-r from-violet-500 to-fuchsia-500 text-white shadow-[0_10px_28px_rgba(168,85,247,0.35)] dark:shadow-[0_0_20px_rgba(168,85,247,0.45)]'
              : 'text-slate-500 hover:bg-violet-50 hover:text-violet-700 dark:text-white/65 dark:hover:bg-white/10 dark:hover:text-white'
          ]"
          @click="activePeriod = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-16 text-slate-500 dark:text-white/70">
      <LoadingSpinner />
    </div>

    <template v-else>
      <div class="relative grid grid-cols-1 items-end gap-4 lg:grid-cols-3 lg:gap-5">
        <article
          v-for="(slot, index) in podiumSlots"
          :key="slot.key"
          :style="{ animationDelay: `${index * 120}ms` }"
          :class="[
            'podium-card relative overflow-hidden rounded-[24px] border px-5 pb-5 pt-4 text-center backdrop-blur-xl transition-all duration-500 hover:-translate-y-2 hover:scale-[1.015]',
            podiumCardClass(slot.rank),
            slot.rank === 1 ? 'lg:-translate-y-6 lg:scale-[1.05]' : 'lg:translate-y-7',
            slot.placeholder ? 'border-dashed' : ''
          ]"
        >
          <div class="pointer-events-none absolute inset-0 opacity-80" :class="podiumGlowClass(slot.rank)" />
          <div class="pointer-events-none absolute inset-x-8 top-0 h-px bg-gradient-to-r from-transparent via-white/70 to-transparent opacity-70 dark:via-white/40" />
          <div class="relative mx-auto flex h-20 w-20 items-center justify-center">
            <div class="absolute inset-0 animate-pulse-slow rounded-full blur-xl" :class="medalGlowClass(slot.rank)" />
            <div class="relative flex h-20 w-20 animate-float items-center justify-center rounded-full border border-white/50 bg-white/75 text-3xl shadow-inner dark:border-white/15 dark:bg-black/45">
              {{ rankMedal(slot.rank) }}
            </div>
          </div>
          <div class="relative mt-3 flex items-center justify-center gap-2 text-xs font-semibold uppercase tracking-[0.2em] text-slate-500 dark:text-white/60">
            <span class="h-px w-8 bg-slate-300 dark:bg-white/20" />
            <span>{{ slot.placeholder ? t('dashboard.rankingPlaceholder') : `TOP ${slot.rank}` }}</span>
            <span class="h-px w-8 bg-slate-300 dark:bg-white/20" />
          </div>
          <h4 class="relative mt-3 truncate text-xl font-black text-slate-950 dark:text-white" :title="displayName(slot)">{{ displayName(slot) }}</h4>
          <p class="relative mt-1 text-sm font-semibold text-slate-600 dark:text-white/70">{{ rankDescription(slot.rank) }}</p>
          <p class="relative mt-4 text-4xl font-black tracking-tight text-slate-950 sm:text-5xl dark:text-white">
            {{ slot.placeholder ? '—' : formatTokens(slot.item.tokens) }}
          </p>
          <div class="relative mt-3 flex items-center justify-center gap-2 text-xs text-slate-500 dark:text-white/55">
            <span>{{ slot.placeholder ? '—' : formatNumber(slot.item.requests) }} {{ t('dashboard.requests') }}</span>
            <span class="h-1 w-1 rounded-full bg-slate-300 dark:bg-white/30" />
            <span>{{ slot.placeholder ? '—' : `$${formatCost(slot.item.actual_cost)}` }}</span>
          </div>
        </article>
      </div>

      <div class="mt-8 grid gap-3">
        <article
          v-for="(slot, index) in restSlots"
          :key="slot.key"
          :style="{ animationDelay: `${420 + index * 70}ms` }"
          class="rank-row group relative overflow-hidden rounded-2xl border border-slate-200/80 bg-white/70 px-4 py-3 text-slate-950 shadow-[0_14px_36px_rgba(88,80,180,0.10)] backdrop-blur-md transition-all duration-300 hover:-translate-y-1 hover:border-violet-300 hover:bg-white hover:shadow-[0_18px_42px_rgba(124,58,237,0.16)] dark:border-white/10 dark:bg-white/[0.04] dark:text-white dark:shadow-[0_0_24px_rgba(0,0,0,0.18)] dark:hover:border-violet-400/40 dark:hover:bg-white/[0.07]"
        >
          <div class="pointer-events-none absolute inset-y-0 left-0 w-1 bg-gradient-to-b from-violet-400 via-fuchsia-500 to-amber-300 opacity-80" />
          <div class="pointer-events-none absolute inset-y-0 left-8 w-28 bg-gradient-to-r from-violet-400/10 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100 dark:from-violet-400/20" />
          <div class="flex items-center gap-4 pl-2">
            <div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-slate-200 bg-white/75 text-lg font-black text-slate-700 shadow-inner dark:border-white/10 dark:bg-black/35 dark:text-white/90">
              {{ slot.rank }}
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-3">
                <span class="truncate text-base font-semibold" :class="slot.placeholder ? 'text-slate-400 dark:text-white/45' : 'text-slate-950 dark:text-white'" :title="displayName(slot)">
                  {{ displayName(slot) }}
                </span>
                <span v-if="slot.placeholder" class="rounded-full border border-slate-200 bg-white/60 px-2 py-0.5 text-[11px] text-slate-400 dark:border-white/10 dark:bg-white/5 dark:text-white/50">
                  {{ t('dashboard.rankingPlaceholder') }}
                </span>
              </div>
              <p class="mt-1 text-xs text-slate-500 dark:text-white/45">{{ slot.placeholder ? '——' : `${formatNumber(slot.item.requests)} ${t('dashboard.requests')}` }}</p>
            </div>
            <div class="shrink-0 text-right">
              <p class="text-2xl font-black tracking-tight" :class="slot.placeholder ? 'text-slate-300 dark:text-white/20' : 'text-slate-950 dark:text-white'">
                {{ slot.placeholder ? '—' : formatTokens(slot.item.tokens) }}
              </p>
              <p class="text-xs text-slate-500 dark:text-white/45">{{ slot.placeholder ? '—' : `$${formatCost(slot.item.actual_cost)}` }}</p>
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
  if (rank === 1) return 'border-amber-300/80 bg-[linear-gradient(180deg,rgba(254,243,199,0.95),rgba(255,255,255,0.76))] shadow-[0_24px_52px_rgba(245,158,11,0.22)] dark:border-amber-300/70 dark:bg-[linear-gradient(180deg,rgba(255,214,102,0.22),rgba(11,13,30,0.9))] dark:shadow-[0_0_40px_rgba(245,158,11,0.22)]'
  if (rank === 2) return 'border-sky-200 bg-[linear-gradient(180deg,rgba(224,242,254,0.92),rgba(255,255,255,0.72))] shadow-[0_20px_46px_rgba(59,130,246,0.16)] dark:border-sky-300/50 dark:bg-[linear-gradient(180deg,rgba(96,165,250,0.18),rgba(11,13,30,0.88))] dark:shadow-[0_0_34px_rgba(59,130,246,0.18)]'
  return 'border-orange-200 bg-[linear-gradient(180deg,rgba(255,237,213,0.92),rgba(255,255,255,0.72))] shadow-[0_20px_46px_rgba(249,115,22,0.16)] dark:border-orange-300/50 dark:bg-[linear-gradient(180deg,rgba(251,146,60,0.18),rgba(11,13,30,0.88))] dark:shadow-[0_0_34px_rgba(249,115,22,0.18)]'
}

const podiumGlowClass = (rank: number) => {
  if (rank === 1) return 'bg-[radial-gradient(circle,rgba(251,191,36,0.22),transparent_62%)] dark:bg-[radial-gradient(circle,rgba(250,204,21,0.35),transparent_60%)]'
  if (rank === 2) return 'bg-[radial-gradient(circle,rgba(96,165,250,0.20),transparent_62%)] dark:bg-[radial-gradient(circle,rgba(96,165,250,0.30),transparent_60%)]'
  return 'bg-[radial-gradient(circle,rgba(251,146,60,0.20),transparent_62%)] dark:bg-[radial-gradient(circle,rgba(251,146,60,0.28),transparent_60%)]'
}

const medalGlowClass = (rank: number) => {
  if (rank === 1) return 'bg-amber-300/35 dark:bg-amber-400/30'
  if (rank === 2) return 'bg-sky-300/30 dark:bg-sky-400/25'
  return 'bg-orange-300/30 dark:bg-orange-400/25'
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
