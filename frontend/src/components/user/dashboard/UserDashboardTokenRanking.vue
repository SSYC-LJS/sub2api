<template>
  <section class="relative overflow-hidden rounded-[28px] border border-white/10 bg-[#070814] p-5 shadow-[0_0_60px_rgba(122,92,255,0.18)]">
    <div class="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(126,87,255,0.22),transparent_40%),radial-gradient(circle_at_bottom_right,rgba(255,198,87,0.16),transparent_35%)]" />
    <div class="pointer-events-none absolute -left-20 top-10 h-44 w-44 rounded-full bg-violet-500/20 blur-3xl" />
    <div class="pointer-events-none absolute -right-16 top-40 h-52 w-52 rounded-full bg-amber-400/10 blur-3xl" />

    <div class="relative mb-5 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
      <div>
        <div class="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.32em] text-violet-200">
          {{ t('dashboard.tokenRankingTitle') }}
        </div>
        <h3 class="mt-3 text-2xl font-black tracking-tight text-white sm:text-3xl">{{ t('dashboard.tokenRankingSubtitle') }}</h3>
      </div>
      <div class="inline-flex flex-wrap gap-2 rounded-full border border-white/10 bg-black/25 p-2 backdrop-blur">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          :class="[
            'rounded-full px-4 py-2 text-sm font-semibold transition-all duration-200',
            activePeriod === tab.key
              ? 'bg-gradient-to-r from-violet-500 to-fuchsia-500 text-white shadow-[0_0_20px_rgba(168,85,247,0.45)]'
              : 'text-white/65 hover:bg-white/10 hover:text-white'
          ]"
          @click="activePeriod = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-16 text-white/70">
      <LoadingSpinner />
    </div>

    <template v-else>
      <div class="relative grid grid-cols-1 items-end gap-4 lg:grid-cols-3 lg:gap-5">
        <article
          v-for="slot in podiumSlots"
          :key="slot.key"
          :class="[
            'relative overflow-hidden rounded-[24px] border px-5 pb-5 pt-4 text-center backdrop-blur-xl transition-transform duration-300',
            podiumCardClass(slot.rank),
            slot.rank === 1 ? 'lg:-translate-y-4' : slot.rank === 2 ? 'lg:translate-y-6' : 'lg:translate-y-8',
            slot.placeholder ? 'border-dashed' : ''
          ]"
        >
          <div class="pointer-events-none absolute inset-0 opacity-70" :class="podiumGlowClass(slot.rank)" />
          <div class="relative mx-auto flex h-20 w-20 items-center justify-center">
            <div class="absolute inset-0 rounded-full blur-xl" :class="medalGlowClass(slot.rank)" />
            <div class="relative flex h-20 w-20 items-center justify-center rounded-full border border-white/15 bg-black/45 text-3xl">
              {{ rankMedal(slot.rank) }}
            </div>
          </div>
          <div class="relative mt-3 flex items-center justify-center gap-2 text-xs font-semibold uppercase tracking-[0.2em] text-white/60">
            <span class="h-px w-8 bg-white/20" />
            <span>{{ slot.placeholder ? t('dashboard.rankingPlaceholder') : `TOP ${slot.rank}` }}</span>
            <span class="h-px w-8 bg-white/20" />
          </div>
          <h4 class="relative mt-3 truncate text-xl font-black text-white" :title="displayName(slot)">{{ displayName(slot) }}</h4>
          <p class="relative mt-1 text-sm font-semibold text-white/70">{{ rankDescription(slot.rank) }}</p>
          <p class="relative mt-4 text-4xl font-black tracking-tight text-white sm:text-5xl">
            {{ slot.placeholder ? '—' : formatTokens(slot.item.tokens) }}
          </p>
          <div class="relative mt-3 flex items-center justify-center gap-2 text-xs text-white/55">
            <span>{{ slot.placeholder ? '—' : formatNumber(slot.item.requests) }} {{ t('dashboard.requests') }}</span>
            <span class="h-1 w-1 rounded-full bg-white/30" />
            <span>{{ slot.placeholder ? '—' : `$${formatCost(slot.item.actual_cost)}` }}</span>
          </div>
        </article>
      </div>

      <div class="mt-6 grid gap-3">
        <article
          v-for="slot in restSlots"
          :key="slot.key"
          class="group relative overflow-hidden rounded-2xl border border-white/10 bg-white/[0.04] px-4 py-3 text-white shadow-[0_0_24px_rgba(0,0,0,0.18)] backdrop-blur-md transition-all duration-200 hover:border-violet-400/40 hover:bg-white/[0.07]"
        >
          <div class="pointer-events-none absolute inset-y-0 left-0 w-1 bg-gradient-to-b from-violet-400 via-fuchsia-500 to-amber-300 opacity-80" />
          <div class="flex items-center gap-4 pl-2">
            <div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-black/35 text-lg font-black text-white/90 shadow-inner">
              {{ slot.rank }}
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-3">
                <span class="truncate text-base font-semibold" :class="slot.placeholder ? 'text-white/45' : 'text-white'" :title="displayName(slot)">
                  {{ displayName(slot) }}
                </span>
                <span v-if="slot.placeholder" class="rounded-full border border-white/10 bg-white/5 px-2 py-0.5 text-[11px] text-white/50">
                  {{ t('dashboard.rankingPlaceholder') }}
                </span>
              </div>
              <p class="mt-1 text-xs text-white/45">{{ slot.placeholder ? '——' : `${formatNumber(slot.item.requests)} ${t('dashboard.requests')}` }}</p>
            </div>
            <div class="shrink-0 text-right">
              <p class="text-2xl font-black tracking-tight" :class="slot.placeholder ? 'text-white/20' : 'text-white'">
                {{ slot.placeholder ? '—' : formatTokens(slot.item.tokens) }}
              </p>
              <p class="text-xs text-white/45">{{ slot.placeholder ? '—' : `$${formatCost(slot.item.actual_cost)}` }}</p>
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
const podiumSlots = computed(() => rankingSlots.value.slice(0, 3))
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
  if (rank === 1) return 'border-amber-300/70 bg-[linear-gradient(180deg,rgba(255,214,102,0.22),rgba(11,13,30,0.9))] shadow-[0_0_40px_rgba(245,158,11,0.22)]'
  if (rank === 2) return 'border-sky-300/50 bg-[linear-gradient(180deg,rgba(96,165,250,0.18),rgba(11,13,30,0.88))] shadow-[0_0_34px_rgba(59,130,246,0.18)]'
  return 'border-orange-300/50 bg-[linear-gradient(180deg,rgba(251,146,60,0.18),rgba(11,13,30,0.88))] shadow-[0_0_34px_rgba(249,115,22,0.18)]'
}

const podiumGlowClass = (rank: number) => {
  if (rank === 1) return 'bg-[radial-gradient(circle,rgba(250,204,21,0.35),transparent_60%)]'
  if (rank === 2) return 'bg-[radial-gradient(circle,rgba(96,165,250,0.30),transparent_60%)]'
  return 'bg-[radial-gradient(circle,rgba(251,146,60,0.28),transparent_60%)]'
}

const medalGlowClass = (rank: number) => {
  if (rank === 1) return 'bg-amber-400/30'
  if (rank === 2) return 'bg-sky-400/25'
  return 'bg-orange-400/25'
}
</script>
