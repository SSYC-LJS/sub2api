<template>
  <AppLayout>
    <div class="relative -m-4 min-h-[calc(100vh-5rem)] overflow-hidden bg-gray-50 px-4 py-6 transition-colors duration-300 sm:-m-6 sm:px-6 lg:px-8 dark:bg-dark-900">
      <div class="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(37,99,235,0.08),transparent_34%)] dark:bg-[radial-gradient(circle_at_top,rgba(59,130,246,0.10),transparent_36%)]" />
      <div class="pointer-events-none absolute left-1/2 top-8 h-72 w-72 -translate-x-1/2 rounded-full bg-blue-100/40 blur-3xl dark:bg-blue-500/5" />
      <div class="relative mx-auto max-w-6xl space-y-6">
        <div class="flex animate-page-title flex-col gap-3 text-gray-900 sm:flex-row sm:items-center sm:justify-between dark:text-white">
          <div>
            <p class="text-xs font-bold uppercase tracking-[0.35em] text-blue-600 dark:text-blue-400">Leaderboard</p>
            <h1 class="mt-2 text-3xl font-black tracking-tight sm:text-4xl">{{ t('usageRanking.title') }}</h1>
            <p class="mt-2 max-w-2xl text-sm text-gray-500 dark:text-gray-400">{{ t('usageRanking.description') }}</p>
          </div>
          <button type="button" class="inline-flex items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-4 py-2 text-sm font-semibold text-gray-700 shadow-sm transition hover:-translate-y-0.5 hover:bg-gray-50 disabled:opacity-60 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:bg-dark-700" :disabled="loading" @click="loadRanking">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            {{ t('common.refresh') }}
          </button>
        </div>

        <UserDashboardTokenRanking :ranking="tokenRanking" :loading="loading" />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import UserDashboardTokenRanking from '@/components/user/dashboard/UserDashboardTokenRanking.vue'
import { usageAPI, type UserTokenRankingResponse } from '@/api/usage'

const { t } = useI18n()
const loading = ref(false)
const tokenRanking = ref<UserTokenRankingResponse | null>(null)

const loadRanking = async () => {
  loading.value = true
  try {
    tokenRanking.value = await usageAPI.getDashboardRanking({
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    })
  } catch (error) {
    console.error('Failed to load token ranking:', error)
    tokenRanking.value = null
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadRanking()
})
</script>

<style scoped>
.animate-page-title {
  animation: page-title-in 520ms ease-out both;
}

@keyframes page-title-in {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .animate-page-title {
    animation: none !important;
  }
}
</style>
