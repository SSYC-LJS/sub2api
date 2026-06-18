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

    <div v-else-if="items.length === 0" class="rounded-lg border border-dashed border-gray-200 py-8 text-center dark:border-dark-600">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('dashboard.tokenRankingEmpty') }}</p>
    </div>

    <template v-else>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
        <div
          v-for="(item, index) in topThree"
          :key="item.user_id"
          class="rounded-xl border border-gray-200 bg-gradient-to-br from-white to-gray-50 p-4 dark:border-dark-600 dark:from-dark-700 dark:to-dark-800"
        >
          <div class="flex items-center justify-between">
            <span :class="rankBadgeClass(index)">TOP {{ index + 1 }}</span>
            <span class="text-xs text-gray-400">{{ formatNumber(item.requests) }} {{ t('dashboard.requests') }}</span>
          </div>
          <p class="mt-3 truncate text-base font-semibold text-gray-900 dark:text-white" :title="displayName(item)">{{ displayName(item) }}</p>
          <p class="mt-2 text-2xl font-bold text-amber-600 dark:text-amber-400">{{ formatTokens(item.tokens) }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">${{ formatCost(item.actual_cost) }}</p>
        </div>
      </div>

      <div v-if="restItems.length > 0" class="mt-4 divide-y divide-gray-100 rounded-lg border border-gray-200 dark:divide-dark-600 dark:border-dark-600">
        <div v-for="(item, index) in restItems" :key="item.user_id" class="flex items-center justify-between gap-3 px-3 py-2.5">
          <div class="flex min-w-0 items-center gap-3">
            <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ index + 4 }}</span>
            <span class="truncate text-sm font-medium text-gray-900 dark:text-white" :title="displayName(item)">{{ displayName(item) }}</span>
          </div>
          <div class="shrink-0 text-right">
            <p class="font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ formatTokens(item.tokens) }}</p>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ formatNumber(item.requests) }} {{ t('dashboard.requests') }}</p>
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

const tabs = computed(() => [
  { key: 'today' as const, label: t('dashboard.rankingToday') },
  { key: 'week' as const, label: t('dashboard.rankingWeek') },
  { key: 'month' as const, label: t('dashboard.rankingMonth') },
])

const items = computed(() => props.ranking?.[activePeriod.value]?.ranking ?? [])
const topThree = computed(() => items.value.slice(0, 3))
const restItems = computed(() => items.value.slice(3, 10))

const maskValue = (value: string) => {
  const text = (value || '').trim()
  if (!text) return '***'
  if (text.length <= 6) return `${text.slice(0, 3)}***${text.slice(-3)}`
  return `${text.slice(0, 3)}***${text.slice(-3)}`
}

const displayName = (item: UserTokenRankingItem) => {
  const username = item.username?.trim()
  if (username) return username
  return maskValue(item.email)
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
  'rounded-full px-2 py-0.5 text-xs font-bold',
  index === 0 ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300' : '',
  index === 1 ? 'bg-slate-100 text-slate-700 dark:bg-slate-700 dark:text-slate-200' : '',
  index === 2 ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300' : '',
]
</script>
