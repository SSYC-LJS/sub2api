<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-600 dark:bg-dark-800">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('usageRanking.title') }}</h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('usageRanking.description') }}</p>
          </div>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadRanking">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            {{ t('common.refresh') }}
          </button>
        </div>
      </div>

      <UserDashboardTokenRanking :ranking="tokenRanking" :loading="loading" />
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
