<template>
  <AppLayout>
    <div class="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
      <section class="relative overflow-hidden rounded-3xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="pointer-events-none absolute inset-0 bg-gradient-to-br from-primary-500/8 via-transparent to-blue-500/8 dark:from-primary-500/12 dark:to-blue-500/10"></div>
        <div class="relative flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div class="space-y-2">
            <div class="inline-flex items-center gap-2 rounded-full border border-primary-500/20 bg-primary-500/10 px-3 py-1 text-xs font-medium text-primary-600 dark:text-primary-300">
              <Icon name="sparkles" size="sm" />
              {{ t('modelMarket.badge') }}
            </div>
            <div>
              <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-3xl">
                {{ t('modelMarket.title') }}
              </h1>
              <p class="mt-2 max-w-3xl text-sm text-gray-500 dark:text-gray-400">
                {{ t('modelMarket.description') }}
              </p>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:min-w-[360px]">
            <div class="rounded-2xl border border-gray-200 bg-gray-50/80 p-3 dark:border-dark-700 dark:bg-dark-800/80">
              <div class="text-2xl font-semibold text-gray-900 dark:text-white">{{ marketGroups.length }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('modelMarket.stats.groups') }}</div>
            </div>
            <div class="rounded-2xl border border-gray-200 bg-gray-50/80 p-3 dark:border-dark-700 dark:bg-dark-800/80">
              <div class="text-2xl font-semibold text-gray-900 dark:text-white">{{ uniqueModelCount }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('modelMarket.stats.models') }}</div>
            </div>
            <div class="col-span-2 rounded-2xl border border-gray-200 bg-gray-50/80 p-3 dark:border-dark-700 dark:bg-dark-800/80 sm:col-span-1">
              <div class="text-2xl font-semibold text-gray-900 dark:text-white">{{ providerOptions.length }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('modelMarket.stats.providers') }}</div>
            </div>
          </div>
        </div>
      </section>

      <section class="card p-4">
        <div class="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
          <div class="relative w-full xl:max-w-md">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500" />
            <input
              v-model="searchQuery"
              type="text"
              class="input pl-10"
              :placeholder="t('modelMarket.searchPlaceholder')"
            />
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              :class="providerButtonClass('')"
              @click="selectedProvider = ''"
            >
              {{ t('modelMarket.allProviders') }}
            </button>
            <button
              v-for="provider in providerOptions"
              :key="provider"
              type="button"
              :class="providerButtonClass(provider)"
              @click="selectedProvider = provider"
            >
              <PlatformIcon :platform="provider as GroupPlatform" size="xs" />
              {{ platformLabel(provider) }}
            </button>
            <button
              @click="loadMarket"
              :disabled="loading"
              class="btn btn-secondary ml-0 xl:ml-2"
              :title="t('common.refresh', 'Refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </section>

      <section v-if="loading" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <div v-for="i in 6" :key="i" class="card h-72 animate-pulse p-5">
          <div class="mb-5 flex items-start justify-between gap-3">
            <div class="space-y-3">
              <div class="h-6 w-36 rounded bg-gray-200 dark:bg-dark-700"></div>
              <div class="h-4 w-24 rounded bg-gray-100 dark:bg-dark-800"></div>
            </div>
            <div class="h-9 w-20 rounded-full bg-gray-100 dark:bg-dark-800"></div>
          </div>
          <div class="space-y-3">
            <div class="h-8 rounded-2xl bg-gray-100 dark:bg-dark-800"></div>
            <div class="h-24 rounded-2xl bg-gray-100 dark:bg-dark-800"></div>
          </div>
        </div>
      </section>

      <section v-else-if="filteredGroups.length === 0" class="card py-16 text-center">
        <Icon name="inbox" size="xl" class="mx-auto mb-4 h-14 w-14 text-gray-400" />
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('modelMarket.emptyTitle') }}</h3>
        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('modelMarket.emptyDescription') }}</p>
      </section>

      <section v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <article
          v-for="group in filteredGroups"
          :key="group.id"
          class="group relative overflow-hidden rounded-3xl border bg-white shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:border-dark-700 dark:bg-dark-900"
          :class="recommendationCardClass(group.recommendation.level)"
        >
          <div class="h-1.5" :class="recommendationAccentClass(group.recommendation.level)"></div>
          <div class="flex h-full flex-col gap-4 p-5">
            <header class="space-y-3">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0 flex-1">
                  <div class="flex min-w-0 items-center gap-2">
                    <h2 class="min-w-0 truncate text-xl font-bold tracking-tight text-gray-900 dark:text-white" :title="group.name">
                      {{ group.name }}
                    </h2>
                    <span
                      class="flex-shrink-0 rounded-full px-2.5 py-1 text-xs font-bold tabular-nums"
                      :class="recommendationPillClass(group.recommendation.level)"
                    >
                      ×{{ formatRate(group.rate) }}
                    </span>
                  </div>
                  <div class="mt-2 flex flex-wrap items-center gap-1.5">
                    <span
                      v-for="provider in group.platforms"
                      :key="provider"
                      class="inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[10px] font-medium uppercase"
                      :class="platformBadgeClass(provider)"
                    >
                      <PlatformIcon :platform="provider as GroupPlatform" size="xs" />
                      {{ platformLabel(provider) }}
                    </span>
                    <span
                      class="rounded px-1.5 py-0.5 text-[10px] font-medium leading-none"
                      :class="group.isExclusive ? 'bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-400' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'"
                    >
                      {{ group.isExclusive ? t('modelMarket.exclusive') : t('modelMarket.public') }}
                    </span>
                  </div>
                </div>
                <div class="flex-shrink-0 text-right">
                  <div class="text-lg font-black" :class="recommendationTextClass(group.recommendation.level)">
                    {{ group.recommendation.label }}
                  </div>
                  <div class="mt-1 text-[11px] text-gray-400 dark:text-gray-500">
                    {{ t('modelMarket.modelCount', { count: group.models.length }) }}
                  </div>
                </div>
              </div>

              <div
                v-if="group.recommendation.level === 'super'"
                class="rounded-2xl border border-rose-200 bg-rose-50 px-3 py-2 text-xs font-semibold text-rose-700 shadow-sm dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-200"
              >
                🔥 {{ t('modelMarket.recommendation.superHint') }}
              </div>
              <div
                v-else-if="group.recommendation.level === 'recommended'"
                class="rounded-2xl border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs font-semibold text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-200"
              >
                ✨ {{ t('modelMarket.recommendation.recommendedHint') }}
              </div>
            </header>

            <div class="flex-1 rounded-2xl border border-gray-100 bg-gray-50/80 p-3 dark:border-dark-700 dark:bg-dark-900/70">
              <div class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">
                <span>{{ t('modelMarket.availableModels') }}</span>
              </div>
              <div class="flex max-h-72 flex-wrap gap-2 overflow-auto pr-1">
                <button
                  v-for="model in group.models"
                  :key="`${group.id}-${model.name}`"
                  type="button"
                  class="inline-flex max-w-full items-center gap-1.5 rounded-xl border border-gray-200 bg-white px-2.5 py-1.5 text-left text-xs font-medium text-gray-700 transition-colors hover:border-primary-300 hover:text-primary-600 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-primary-500/60 dark:hover:text-primary-300"
                  :title="copiedModelName === model.name ? t('common.copied', 'Copied') : model.name"
                  @click="copyModelName(model.name)"
                >
                  <PlatformIcon :platform="model.platform as GroupPlatform" size="xs" />
                  <span class="truncate">{{ model.name }}</span>
                  <Icon :name="copiedModelName === model.name ? 'check' : 'copy'" size="xs" class="flex-shrink-0 text-gray-400" />
                </button>
              </div>
            </div>
          </div>
        </article>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import userChannelsAPI, {
  type UserAvailableChannel,
  type UserAvailableGroup,
  type UserSupportedModel,
} from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import type { GroupPlatform } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  platformBadgeClass,
  platformLabel,
} from '@/utils/platformColors'

type RecommendationLevel = 'normal' | 'moderate' | 'recommended' | 'super'

interface MarketGroupModel {
  name: string
  platform: string
}

interface MarketGroupCard {
  id: number
  name: string
  rate: number
  isExclusive: boolean
  platforms: string[]
  models: MarketGroupModel[]
  recommendation: {
    level: RecommendationLevel
    label: string
  }
}

const { t } = useI18n()
const appStore = useAppStore()

const channels = ref<UserAvailableChannel[]>([])
const userGroupRates = ref<Record<number, number>>({})
const loading = ref(false)
const searchQuery = ref('')
const selectedProvider = ref('')
const copiedModelName = ref('')

async function copyModelName(name: string) {
  try {
    await navigator.clipboard.writeText(name)
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = name
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    try {
      document.execCommand('copy')
    } catch {
      /* noop */
    }
    document.body.removeChild(textarea)
  }
  copiedModelName.value = name
  setTimeout(() => {
    if (copiedModelName.value === name) {
      copiedModelName.value = ''
    }
  }, 2000)
}

const marketGroups = computed<MarketGroupCard[]>(() => buildMarketGroups(channels.value))

const providerOptions = computed(() => {
  const providers = new Set<string>()
  for (const group of marketGroups.value) {
    group.platforms.forEach((p) => providers.add(p))
  }
  return Array.from(providers).sort((a, b) => platformLabel(a).localeCompare(platformLabel(b)))
})

const uniqueModelCount = computed(() => {
  const names = new Set<string>()
  for (const group of marketGroups.value) {
    group.models.forEach((model) => names.add(model.name))
  }
  return names.size
})

const filteredGroups = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return marketGroups.value.filter((group) => {
    if (selectedProvider.value && !group.platforms.includes(selectedProvider.value)) return false
    if (!q) return true
    return (
      group.name.toLowerCase().includes(q) ||
      group.platforms.some((p) => platformLabel(p).toLowerCase().includes(q) || p.toLowerCase().includes(q)) ||
      group.models.some((model) => model.name.toLowerCase().includes(q))
    )
  })
})

function buildMarketGroups(list: UserAvailableChannel[]): MarketGroupCard[] {
  const byID = new Map<number, {
    group: UserAvailableGroup
    platforms: Set<string>
    models: Map<string, MarketGroupModel>
  }>()

  for (const channel of list) {
    for (const section of channel.platforms || []) {
      const sectionPlatform = section.platform || ''
      for (const group of section.groups || []) {
        let item = byID.get(group.id)
        if (!item) {
          item = {
            group,
            platforms: new Set<string>(),
            models: new Map<string, MarketGroupModel>(),
          }
          byID.set(group.id, item)
        }
        const groupPlatform = group.platform || sectionPlatform
        if (groupPlatform) item.platforms.add(groupPlatform)

        for (const model of section.supported_models || []) {
          addGroupModel(item.models, model, groupPlatform)
        }
      }
    }
  }

  return Array.from(byID.values())
    .map(({ group, platforms, models }) => {
      const rate = effectiveRate(group)
      return {
        id: group.id,
        name: group.name,
        rate,
        isExclusive: group.is_exclusive,
        platforms: Array.from(platforms).sort((a, b) => platformLabel(a).localeCompare(platformLabel(b))),
        models: Array.from(models.values()).sort((a, b) => a.name.localeCompare(b.name)),
        recommendation: recommendationForRate(rate),
      }
    })
    .sort((a, b) => a.rate - b.rate || a.name.localeCompare(b.name))
}

function addGroupModel(models: Map<string, MarketGroupModel>, model: UserSupportedModel, fallbackPlatform: string) {
  const name = model.name
  if (!name) return
  const platform = model.platform || fallbackPlatform || ''
  const key = `${platform}:${name}`
  if (!models.has(key)) {
    models.set(key, { name, platform })
  }
}

function effectiveRate(group: UserAvailableGroup): number {
  const rate = userGroupRates.value[group.id] ?? group.rate_multiplier ?? 1
  return Number.isFinite(rate) && rate >= 0 ? rate : 1
}

function formatRate(rate: number): string {
  return rate.toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1')
}

function recommendationForRate(rate: number): MarketGroupCard['recommendation'] {
  if (rate >= 1) return { level: 'normal', label: t('modelMarket.recommendation.normal') }
  if (rate >= 0.5) return { level: 'moderate', label: t('modelMarket.recommendation.moderate') }
  if (rate >= 0.1) return { level: 'recommended', label: t('modelMarket.recommendation.recommended') }
  return { level: 'super', label: t('modelMarket.recommendation.super') }
}

function recommendationCardClass(level: RecommendationLevel): string[] {
  const map: Record<RecommendationLevel, string[]> = {
    normal: ['border-gray-200', 'dark:border-dark-700'],
    moderate: ['border-blue-200/80', 'shadow-blue-500/5', 'dark:border-blue-500/30'],
    recommended: ['border-emerald-300/80', 'shadow-emerald-500/10', 'dark:border-emerald-500/40'],
    super: ['border-rose-300/90', 'shadow-rose-500/15', 'ring-1', 'ring-rose-200/70', 'dark:border-rose-500/50', 'dark:ring-rose-500/20'],
  }
  return map[level]
}

function recommendationAccentClass(level: RecommendationLevel): string {
  const map: Record<RecommendationLevel, string> = {
    normal: 'bg-gray-200 dark:bg-dark-700',
    moderate: 'bg-gradient-to-r from-blue-400 to-cyan-400',
    recommended: 'bg-gradient-to-r from-emerald-400 to-lime-400',
    super: 'bg-gradient-to-r from-rose-500 via-orange-400 to-yellow-300',
  }
  return map[level]
}

function recommendationPillClass(level: RecommendationLevel): string {
  const map: Record<RecommendationLevel, string> = {
    normal: 'bg-gray-100 text-gray-700 dark:bg-dark-800 dark:text-gray-200',
    moderate: 'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200',
    recommended: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-200',
    super: 'bg-rose-50 text-rose-700 dark:bg-rose-500/10 dark:text-rose-200',
  }
  return map[level]
}

function recommendationTextClass(level: RecommendationLevel): string {
  const map: Record<RecommendationLevel, string> = {
    normal: 'text-gray-600 dark:text-gray-300',
    moderate: 'text-blue-600 dark:text-blue-300',
    recommended: 'text-emerald-600 dark:text-emerald-300',
    super: 'text-rose-600 dark:text-rose-300',
  }
  return map[level]
}

function providerButtonClass(provider: string): string[] {
  const active = selectedProvider.value === provider
  return [
    'inline-flex items-center gap-1.5 rounded-xl border px-3 py-2 text-sm font-medium transition-colors',
    active
      ? provider
        ? platformBadgeClass(provider)
        : 'border-primary-500/30 bg-primary-500/10 text-primary-600 dark:text-primary-300'
      : 'border-gray-200 bg-white text-gray-600 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 dark:hover:bg-dark-800',
  ]
}

async function loadMarket() {
  loading.value = true
  try {
    const [list, rates] = await Promise.all([
      userChannelsAPI.getModelMarket(),
      userGroupsAPI.getUserGroupRates().catch((err: unknown) => {
        console.error('Failed to load user group rates:', err)
        return {} as Record<number, number>
      }),
    ])
    channels.value = list
    userGroupRates.value = rates
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(loadMarket)
</script>
