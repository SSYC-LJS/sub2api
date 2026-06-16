<template>
  <div class="relative">
    <span
      v-if="displayVersion"
      class="inline-flex items-center gap-1 rounded-lg bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-dark-400"
      :title="t('version.manualUpdateOnly')"
    >
      v{{ displayVersion }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
  version?: string
}>()

const displayVersion = computed(() => props.version || '')
</script>
