<template>
  <AppLayout>
    <div class="relative -m-4 min-h-[calc(100vh-5rem)] overflow-hidden bg-gradient-to-br from-slate-50 via-violet-50 to-amber-50 px-4 py-6 transition-colors duration-300 sm:-m-6 sm:px-6 lg:px-8 dark:bg-[#050511] dark:bg-none">
      <div class="pointer-events-none absolute inset-0 bg-[linear-gradient(135deg,rgba(124,58,237,0.10),transparent_35%),radial-gradient(circle_at_80%_0%,rgba(245,158,11,0.14),transparent_32%)] dark:bg-[linear-gradient(135deg,rgba(124,58,237,0.12),transparent_35%),radial-gradient(circle_at_80%_0%,rgba(245,158,11,0.12),transparent_32%)]" />
      <div class="pointer-events-none absolute left-1/2 top-8 h-72 w-72 -translate-x-1/2 rounded-full bg-white/55 blur-3xl dark:bg-violet-500/10" />
      <div class="relative mx-auto max-w-6xl space-y-6">
        <div class="flex animate-page-title flex-col gap-3 text-slate-950 sm:flex-row sm:items-center sm:justify-between dark:text-white">
          <div>
            <p class="text-xs font-bold uppercase tracking-[0.35em] text-violet-600 dark:text-violet-300">Leaderboard</p>
            <h1 class="mt-2 text-3xl font-black tracking-tight sm:text-4xl">{{ t('usageRanking.title') }}</h1>
            <p class="mt-2 max-w-2xl text-sm text-slate-500 dark:text-white/55">{{ t('usageRanking.description') }}</p>
          </div>
          <button type="button" class="inline-flex items-center justify-center gap-2 rounded-full border border-slate-200 bg-white/75 px-4 py-2 text-sm font-semibold text-slate-700 shadow-lg backdrop-blur transition hover:-translate-y-0.5 hover:bg-white disabled:opacity-60 dark:border-white/10 dark:bg-white/10 dark:text-white dark:hover:bg-white/15" :disabled="loading" @click="loadRanking">
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
