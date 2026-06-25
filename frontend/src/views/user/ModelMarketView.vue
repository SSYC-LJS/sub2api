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
              <div class="text-2xl font-semibold text-gray-900 dark:text-white">{{ marketModels.length }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('modelMarket.stats.models') }}</div>
            </div>
            <div class="rounded-2xl border border-gray-200 bg-gray-50/80 p-3 dark:border-dark-700 dark:bg-dark-800/80">
              <div class="text-2xl font-semibold text-gray-900 dark:text-white">{{ providerOptions.length }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('modelMarket.stats.providers') }}</div>
            </div>
            <div class="col-span-2 rounded-2xl border border-gray-200 bg-gray-50/80 p-3 dark:border-dark-700 dark:bg-dark-800/80 sm:col-span-1">
              <div class="text-2xl font-semibold text-gray-900 dark:text-white">{{ visibleGroupCount }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('modelMarket.stats.groups') }}</div>
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
          <div class="mb-5 flex items-center gap-3">
            <div class="h-11 w-11 rounded-2xl bg-gray-200 dark:bg-dark-700"></div>
            <div class="flex-1 space-y-2">
              <div class="h-4 w-2/3 rounded bg-gray-200 dark:bg-dark-700"></div>
              <div class="h-3 w-1/3 rounded bg-gray-100 dark:bg-dark-800"></div>
            </div>
          </div>
          <div class="space-y-3">
            <div class="h-16 rounded-2xl bg-gray-100 dark:bg-dark-800"></div>
            <div class="h-20 rounded-2xl bg-gray-100 dark:bg-dark-800"></div>
          </div>
        </div>
      </section>

      <section v-else-if="filteredModels.length === 0" class="card py-16 text-center">
        <Icon name="inbox" size="xl" class="mx-auto mb-4 h-14 w-14 text-gray-400" />
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('modelMarket.emptyTitle') }}</h3>
        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('modelMarket.emptyDescription') }}</p>
      </section>

      <section v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <article
          v-for="model in filteredModels"
          :key="model.name"
          class="group relative overflow-hidden rounded-3xl border bg-white shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:border-dark-700 dark:bg-dark-900"
          :class="platformBorderClass(model.primaryPlatform)"
        >
          <div class="h-1.5" :class="platformAccentBarClass(model.primaryPlatform)"></div>
          <div class="flex h-full flex-col gap-4 p-5">
            <header class="flex items-start justify-between gap-3">
              <div class="min-w-0 flex items-center gap-3">
                <div
                  class="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-2xl border"
                  :class="[platformBadgeLightClass(model.primaryPlatform), platformBorderClass(model.primaryPlatform)]"
                >
                  <PlatformIcon :platform="model.primaryPlatform as GroupPlatform" size="lg" />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-1.5">
                    <h2 class="min-w-0 truncate text-base font-semibold text-gray-900 dark:text-white" :title="model.name">
                      {{ model.name }}
                    </h2>
                    <button
                      type="button"
                      class="flex-shrink-0 rounded-md p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-primary-500 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                      :title="copiedModelName === model.name ? t('common.copied', 'Copied') : t('common.copy', 'Copy')"
                      @click="copyModelName(model.name)"
                    >
                      <Icon :name="copiedModelName === model.name ? 'check' : 'copy'" size="xs" />
                    </button>
                  </div>
                  <div class="mt-1 flex flex-wrap items-center gap-1.5">
                    <span
                      v-for="provider in model.platforms"
                      :key="provider"
                      class="inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[10px] font-medium uppercase"
                      :class="platformBadgeClass(provider)"
                    >
                      <PlatformIcon :platform="provider as GroupPlatform" size="xs" />
                      {{ platformLabel(provider) }}
                    </span>
                  </div>
                </div>
              </div>
              <span class="rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-gray-300">
                {{ t('modelMarket.groupCount', { count: model.groups.length }) }}
              </span>
            </header>

            <div class="rounded-2xl border border-gray-100 bg-gray-50/70 p-3 dark:border-dark-700 dark:bg-dark-800/60">
              <div class="mb-2 flex items-center justify-between gap-2">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('modelMarket.officialPrice') }}</span>
                <span class="text-[11px] text-gray-400 dark:text-gray-500">{{ billingModeLabel(model.pricing) }}</span>
              </div>
              <PriceSummary
                :pricing="model.pricing"
                :empty-label="t('modelMarket.noPricing')"
                :pricing-key-prefix="'modelMarket.pricing'"
              />
            </div>

            <div class="flex-1 space-y-2">
              <div class="flex items-center justify-between text-xs font-medium text-gray-500 dark:text-gray-400">
                <span>{{ t('modelMarket.currentGroupPrices') }}</span>
                <span>{{ t('modelMarket.multiplierHint') }}</span>
              </div>
              <div class="max-h-64 overflow-auto rounded-2xl border border-gray-100 bg-gray-50/80 text-gray-700 dark:border-dark-700 dark:bg-dark-900/70 dark:text-gray-200">
                <table class="w-full text-xs">
                  <thead class="sticky top-0 z-10 bg-gray-50 text-[10px] uppercase tracking-wide text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                    <tr>
                      <th class="min-w-[80px] px-2 py-2 text-left font-semibold">{{ t('modelMarket.priceTable.group') }}</th>
                      <th class="whitespace-nowrap px-2 py-2 text-right font-semibold">{{ t('modelMarket.priceTable.rate') }}</th>
                      <th
                        v-for="column in priceColumns(model.pricing)"
                        :key="column.kind"
                        class="whitespace-nowrap px-2 py-2 text-right font-semibold"
                      >
                        {{ column.label }}
                      </th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                    <tr
                      v-for="group in model.groups"
                      :key="`${model.name}-${group.id}`"
                      class="hover:bg-gray-50/70 dark:hover:bg-dark-800/70"
                    >
                      <td class="px-2 py-2 align-middle">
                        <div class="flex items-center gap-1.5">
                          <span class="min-w-0 truncate font-medium text-gray-900 dark:text-white" :title="group.name">
                            {{ group.name }}
                          </span>
                          <span
                            class="flex-shrink-0 rounded px-1 py-0.5 text-[9px] font-medium leading-none"
                            :class="group.is_exclusive ? 'bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-400' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'"
                          >
                            {{ group.is_exclusive ? t('modelMarket.exclusive') : t('modelMarket.public') }}
                          </span>
                        </div>
                      </td>
                      <td class="whitespace-nowrap px-2 py-2 text-right align-middle font-semibold text-gray-700 dark:text-gray-100">
                        ×{{ rateLabel(group) }}
                      </td>
                      <td
                        v-for="column in priceColumns(model.pricing)"
                        :key="column.kind"
                        class="whitespace-nowrap px-2 py-2 text-right align-middle tabular-nums text-gray-700 dark:text-gray-100"
                      >
                        {{ groupPriceCell(model.pricing, effectiveRate(group), column.kind) }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </article>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import userChannelsAPI, {
  type UserAvailableChannel,
  type UserAvailableGroup,
  type UserSupportedModelPricing,
} from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import type { GroupPlatform } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatScaled } from '@/utils/pricing'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN,
} from '@/constants/channel'
import {
  platformAccentBarClass,
  platformBadgeClass,
  platformBadgeLightClass,
  platformBorderClass,
  platformLabel,
} from '@/utils/platformColors'

interface MarketGroup extends UserAvailableGroup {
  channelName: string
}

interface MarketModel {
  name: string
  primaryPlatform: string
  platforms: string[]
  groups: MarketGroup[]
  pricing: UserSupportedModelPricing | null
}

const { t } = useI18n()
const appStore = useAppStore()

const channels = ref<UserAvailableChannel[]>([])
const userGroupRates = ref<Record<number, number>>({})
const loading = ref(false)
const searchQuery = ref('')
const selectedProvider = ref('')
const copiedModelName = ref('')

const perMillionScale = 1_000_000

async function copyModelName(name: string) {
  try {
    await navigator.clipboard.writeText(name)
  } catch {
    // Fallback for environments without clipboard API
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

const marketModels = computed<MarketModel[]>(() => buildMarketModels(channels.value))
const providerOptions = computed(() => {
  const providers = new Set<string>()
  for (const model of marketModels.value) {
    model.platforms.forEach((p) => providers.add(p))
  }
  return Array.from(providers).sort((a, b) => platformLabel(a).localeCompare(platformLabel(b)))
})
const visibleGroupCount = computed(() => {
  const ids = new Set<number>()
  for (const model of marketModels.value) model.groups.forEach((g) => ids.add(g.id))
  return ids.size
})

const filteredModels = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return marketModels.value.filter((model) => {
    if (selectedProvider.value && !model.platforms.includes(selectedProvider.value)) return false
    if (!q) return true
    return (
      model.name.toLowerCase().includes(q) ||
      model.platforms.some((p) => platformLabel(p).toLowerCase().includes(q) || p.toLowerCase().includes(q)) ||
      model.groups.some((g) => g.name.toLowerCase().includes(q) || g.channelName.toLowerCase().includes(q))
    )
  })
})

function buildMarketModels(list: UserAvailableChannel[]): MarketModel[] {
  const byName = new Map<string, MarketModel>()
  for (const channel of list) {
    for (const section of channel.platforms || []) {
      const groups = section.groups || []
      for (const model of section.supported_models || []) {
        const key = model.name
        if (!key) continue
        let item = byName.get(key)
        if (!item) {
          item = {
            name: key,
            primaryPlatform: model.platform || section.platform || groups[0]?.platform || '',
            platforms: [],
            groups: [],
            pricing: model.pricing,
          }
          byName.set(key, item)
        }
        const platform = model.platform || section.platform || groups[0]?.platform || ''
        if (platform && !item.platforms.includes(platform)) item.platforms.push(platform)
        if (!item.pricing && model.pricing) item.pricing = model.pricing
        for (const group of groups) {
          if (!item.groups.some((g) => g.id === group.id)) {
            item.groups.push({ ...group, channelName: channel.name })
          }
        }
      }
    }
  }
  return Array.from(byName.values())
    .map((model) => ({
      ...model,
      primaryPlatform: model.primaryPlatform || model.platforms[0] || '',
      platforms: model.platforms.sort((a, b) => platformLabel(a).localeCompare(platformLabel(b))),
      groups: model.groups.sort((a, b) => a.name.localeCompare(b.name)),
    }))
    .sort((a, b) => a.name.localeCompare(b.name))
}

function effectiveRate(group: UserAvailableGroup): number {
  const rate = userGroupRates.value[group.id] ?? group.rate_multiplier ?? 1
  return Number.isFinite(rate) && rate >= 0 ? rate : 1
}

function scaleNumber(value: number | null, multiplier: number): number | null {
  return value == null ? null : value * multiplier
}

function rateLabel(group: UserAvailableGroup): string {
  return effectiveRate(group).toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1')
}

type PriceCellKind = 'input' | 'output' | 'cacheWrite' | 'cacheRead' | 'perRequest' | 'image'

function priceColumns(pricing: UserSupportedModelPricing | null): Array<{ kind: PriceCellKind; label: string }> {
  if (pricing?.billing_mode === BILLING_MODE_PER_REQUEST) {
    return [{ kind: 'perRequest', label: t('modelMarket.priceTable.perRequest') }]
  }
  if (pricing?.billing_mode === BILLING_MODE_IMAGE) {
    return [{ kind: 'image', label: t('modelMarket.priceTable.perImage') }]
  }

  const columns: Array<{ kind: PriceCellKind; label: string }> = [
    { kind: 'input', label: t('modelMarket.priceTable.input') },
    { kind: 'output', label: t('modelMarket.priceTable.output') },
  ]
  if (pricing?.billing_mode === BILLING_MODE_TOKEN) {
    columns.push(
      { kind: 'cacheWrite', label: t('modelMarket.priceTable.cacheWrite') },
      { kind: 'cacheRead', label: t('modelMarket.priceTable.cacheRead') },
    )
  }
  return columns
}

function groupPriceCell(
  pricing: UserSupportedModelPricing | null,
  multiplier: number,
  kind: PriceCellKind,
): string {
  if (!pricing) return '-'
  if (kind === 'perRequest') {
    return formatScaled(scaleNumber(pricing.per_request_price, multiplier), 1)
  }
  if (kind === 'image') {
    return formatScaled(scaleNumber(imageUnitPrice(pricing), multiplier), 1)
  }
  const source = {
    input: pricing.input_price,
    output: pricing.output_price,
    cacheWrite: pricing.cache_write_price,
    cacheRead: pricing.cache_read_price,
  }[kind]
  return formatScaled(scaleNumber(source, multiplier), perMillionScale)
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

function billingModeLabel(pricing: UserSupportedModelPricing | null): string {
  if (!pricing) return t('modelMarket.noPricing')
  switch (pricing.billing_mode) {
    case BILLING_MODE_TOKEN:
      return t('modelMarket.pricing.billingModeToken')
    case BILLING_MODE_PER_REQUEST:
      return t('modelMarket.pricing.billingModePerRequest')
    case BILLING_MODE_IMAGE:
      return t('modelMarket.pricing.billingModeImage')
    default:
      return '-'
  }
}

function imageUnitPrice(pricing: UserSupportedModelPricing | null): number | null {
  if (!pricing) return null
  return pricing.per_request_price ?? pricing.image_output_price ?? null
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

const PriceSummary = defineComponent({
  name: 'ModelMarketPriceSummary',
  props: {
    pricing: { type: Object as PropType<UserSupportedModelPricing | null>, default: null },
    emptyLabel: { type: String, required: true },
    pricingKeyPrefix: { type: String, required: true },
    compact: { type: Boolean, default: false },
  },
  setup(props) {
    const { t } = useI18n()
    const row = (label: string, value: number | null, unit: string, scale: number) =>
      h('div', { class: 'flex items-center justify-between gap-3 text-xs' }, [
        h('span', { class: 'text-gray-500 dark:text-gray-400' }, label),
        h('span', { class: 'font-medium text-gray-800 dark:text-gray-100' }, `${formatScaled(value, scale)} ${unit}`),
      ])

    const prefixKey = (k: string) => `${props.pricingKeyPrefix}.${k}`

    return () => {
      const pricing = props.pricing
      if (!pricing) {
        return h('div', { class: 'text-sm text-gray-500 dark:text-gray-400' }, props.emptyLabel)
      }
      const unitMillion = t(prefixKey('unitPerMillion'))
      const unitRequest = pricing.billing_mode === BILLING_MODE_IMAGE ? t(prefixKey('unitPerImage')) : t(prefixKey('unitPerRequest'))
      const rows = []
      if (pricing.billing_mode === BILLING_MODE_TOKEN) {
        rows.push(row(t(prefixKey('inputPrice')), pricing.input_price, unitMillion, perMillionScale))
        rows.push(row(t(prefixKey('outputPrice')), pricing.output_price, unitMillion, perMillionScale))
        rows.push(row(t(prefixKey('cacheWritePrice')), pricing.cache_write_price, unitMillion, perMillionScale))
        rows.push(row(t(prefixKey('cacheReadPrice')), pricing.cache_read_price, unitMillion, perMillionScale))
        if (pricing.image_output_price != null && pricing.image_output_price > 0) {
          rows.push(row(t(prefixKey('imageOutputPrice')), pricing.image_output_price, unitMillion, perMillionScale))
        }
      } else if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) {
        rows.push(row(t(prefixKey('perRequestPrice')), pricing.per_request_price, unitRequest, 1))
      } else if (pricing.billing_mode === BILLING_MODE_IMAGE) {
        rows.push(row(t(prefixKey('perImagePrice')), imageUnitPrice(pricing), unitRequest, 1))
      }

      if (pricing.intervals?.length && !props.compact) {
        rows.push(
          h('div', { class: 'mt-2 border-t border-gray-100 pt-2 dark:border-dark-700' }, [
            h('div', { class: 'mb-1 text-xs font-medium text-gray-500 dark:text-gray-400' }, t(prefixKey('intervals'))),
            h('div', { class: 'space-y-1' }, pricing.intervals.slice(0, 3).map((iv) =>
              h('div', { class: 'flex items-center justify-between gap-3 text-[11px]' }, [
                h('span', { class: 'text-gray-500 dark:text-gray-400' }, iv.tier_label || formatRange(iv.min_tokens, iv.max_tokens)),
                h('span', { class: 'font-medium text-gray-800 dark:text-gray-100' }, pricing.billing_mode === BILLING_MODE_TOKEN
                  ? `${formatScaled(iv.input_price, perMillionScale)} / ${formatScaled(iv.output_price, perMillionScale)}`
                  : formatScaled(iv.per_request_price, 1)),
              ])
            )),
          ])
        )
      }

      return h('div', { class: 'space-y-1.5 rounded-2xl bg-gray-50/70 p-3 dark:border dark:border-dark-700 dark:bg-dark-900/60' }, rows)
    }
  },
})

function formatRange(min: number, max: number | null): string {
  const maxLabel = max == null ? '∞' : String(max)
  return `(${min}, ${maxLabel}]`
}

onMounted(loadMarket)
</script>
