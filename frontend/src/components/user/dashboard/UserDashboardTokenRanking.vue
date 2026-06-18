<template>
  <div class="card p-4">
    <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('dashboard.tokenRankingTitle') }}</h3>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('dashboard.tokenRankingSubtitle') }}</p>
      </div>
      <div class="inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          :class="[
            'rounded-md px-3 py-1.5 text-xs font-medium transition-colors',
            activePeriod === tab.key
              ? 'bg-white text-blue-600 shadow-sm dark:bg-dark-600 dark:text-blue-400'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
          ]"
          @click="activePeriod = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-8">
      <LoadingSpinner />
    </div>

    <template v-else>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
        <div
          v-for="slot in topThree"
          :key="slot.key"
          :class="[
            'rounded-xl border border-gray-200 bg-gradient-to-br from-white to-gray-50 p-4 dark:border-dark-600 dark:from-dark-700 dark:to-dark-800',
            slot.placeholder ? 'border-dashed opacity-75' : ''
          ]"
        >
          <div class="flex items-center justify-between">
            <span :class="rankBadgeClass(slot.rank - 1)">{{ rankMedal(slot.rank) }}</span>
            <span class="text-xs text-gray-400">
              {{ slot.placeholder ? '—' : formatNumber(slot.item.requests) }} {{ t('dashboard.requests') }}
            </span>
          </div>
          <p :class="['mt-3 truncate text-base font-semibold', slot.placeholder ? 'text-gray-400 dark:text-gray-500' : 'text-gray-900 dark:text-white']" :title="displayName(slot)">
            {{ displayName(slot) }}
          </p>
          <p class="mt-1 text-sm font-semibold text-amber-600 dark:text-amber-400">{{ rankDescription(slot.rank) }}</p>
          <p :class="['mt-2 text-2xl font-bold', slot.placeholder ? 'text-gray-300 dark:text-gray-600' : 'text-amber-600 dark:text-amber-400']">
            {{ slot.placeholder ? '—' : formatTokens(slot.item.tokens) }}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ slot.placeholder ? '—' : `$${formatCost(slot.item.actual_cost)}` }}</p>
        </div>
      </div>

      <div class="mt-4 divide-y divide-gray-100 rounded-lg border border-gray-200 dark:divide-dark-600 dark:border-dark-600">
        <div v-for="slot in restItems" :key="slot.key" class="flex items-center justify-between gap-3 px-3 py-2.5">
          <div class="flex min-w-0 items-center gap-3">
            <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ slot.rank }}</span>
            <span :class="['truncate text-sm font-medium', slot.placeholder ? 'text-gray-400 dark:text-gray-500' : 'text-gray-900 dark:text-white']" :title="displayName(slot)">
              {{ displayName(slot) }}
            </span>
          </div>
          <div class="shrink-0 text-right">
            <p :class="['font-mono text-sm font-semibold', slot.placeholder ? 'text-gray-300 dark:text-gray-600' : 'text-gray-900 dark:text-white']">
              {{ slot.placeholder ? '—' : formatTokens(slot.item.tokens) }}
            </p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ slot.placeholder ? '—' : formatNumber(slot.item.requests) }} {{ t('dashboard.requests') }}
            </p>
          </div>
        </div>
      </div>
    </template>
  </div>
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
const activePeriod = ref<'today' | 'week' | 'month'>('today')

type RankingSlot = {
  key: string
  rank: number
  item: UserTokenRankingItem
  placeholder: boolean
}

const tabs = computed(() => [
  { key: 'today' as const, label: t('dashboard.rankingToday') },
  { key: 'week' as const, label: t('dashboard.rankingWeek') },
  { key: 'month' as const, label: t('dashboard.rankingMonth') },
])

const items = computed(() => props.ranking?.[activePeriod.value]?.ranking ?? [])
const rankingSlots = computed<RankingSlot[]>(() =>
  Array.from({ length: 10 }, (_, index) => {
    const rank = index + 1
    const item = items.value[index]
    if (item) {
      return { key: `user-${item.user_id}-${rank}`, rank, item, placeholder: false }
    }
    return {
      key: `placeholder-${rank}`,
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
const topThree = computed(() => rankingSlots.value.slice(0, 3))
const restItems = computed(() => rankingSlots.value.slice(3, 10))

const maskValue = (value: string) => {
  const text = (value || '').trim()
  if (!text) return '***'
  if (text.length <= 6) return `${text.slice(0, 3)}***${text.slice(-3)}`
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

const rankBadgeClass = (index: number) => [
  'rounded-full px-2 py-0.5 text-lg font-bold',
  index === 0 ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300' : '',
  index === 1 ? 'bg-slate-100 text-slate-700 dark:bg-slate-700 dark:text-slate-200' : '',
  index === 2 ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300' : '',
]
</script>
