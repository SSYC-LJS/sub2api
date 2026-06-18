<template>
  <AppLayout>
    <div class="relative -m-4 min-h-[calc(100vh-5rem)] overflow-hidden bg-[#050511] px-4 py-6 sm:-m-6 sm:px-6 lg:px-8">
      <div class="pointer-events-none absolute inset-0 bg-[linear-gradient(135deg,rgba(124,58,237,0.12),transparent_35%),radial-gradient(circle_at_80%_0%,rgba(245,158,11,0.12),transparent_32%)]" />
      <div class="relative mx-auto max-w-6xl space-y-6">
        <div class="flex flex-col gap-3 text-white sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p class="text-xs font-bold uppercase tracking-[0.35em] text-violet-300">Leaderboard</p>
            <h1 class="mt-2 text-3xl font-black tracking-tight sm:text-4xl">{{ t('usageRanking.title') }}</h1>
            <p class="mt-2 max-w-2xl text-sm text-white/55">{{ t('usageRanking.description') }}</p>
          </div>
          <button type="button" class="inline-flex items-center justify-center gap-2 rounded-full border border-white/10 bg-white/10 px-4 py-2 text-sm font-semibold text-white shadow-lg backdrop-blur transition hover:bg-white/15 disabled:opacity-60" :disabled="loading" @click="loadRanking">
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
