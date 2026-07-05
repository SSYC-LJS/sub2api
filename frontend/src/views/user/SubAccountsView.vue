<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">子账号管理</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">为已注册账号分配母账号额度，并查看子账号使用记录。</p>
      </div>
      <button class="btn btn-primary" type="button" @click="refreshAll" :disabled="loading">
        刷新
      </button>
    </div>

    <section class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
      <div class="mb-4 flex items-center justify-between gap-3">
        <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">添加子账号</h2>
      </div>
      <div class="grid gap-3 lg:grid-cols-[1fr_160px_auto]">
        <div>
          <label class="input-label">搜索已注册账号</label>
          <input v-model.trim="candidateSearch" class="input" type="text" placeholder="输入邮箱或用户名，至少 2 个字符" @keyup.enter="searchCandidates" />
        </div>
        <div>
          <label class="input-label">分配额度</label>
          <input v-model.number="newQuota" class="input" type="number" min="0" step="0.000001" />
        </div>
        <div class="flex items-end">
          <button class="btn btn-secondary w-full" type="button" :disabled="candidateLoading" @click="searchCandidates">
            {{ candidateLoading ? '搜索中' : '搜索' }}
          </button>
        </div>
      </div>
      <div v-if="candidates.length" class="mt-4 divide-y divide-gray-100 rounded-lg border border-gray-200 dark:divide-dark-700 dark:border-dark-700">
        <div v-for="candidate in candidates" :key="candidate.id" class="flex flex-col gap-3 p-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <div class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">{{ candidate.email }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ candidate.username || '未设置用户名' }} · 余额 {{ formatMoney(candidate.balance) }}</div>
          </div>
          <button class="btn btn-primary shrink-0" type="button" :disabled="submitting" @click="addSubAccount(candidate.id)">
            添加
          </button>
        </div>
      </div>
    </section>

    <section class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
      <div class="mb-4 flex items-center justify-between gap-3">
        <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">我的子账号</h2>
        <span class="text-sm text-gray-500 dark:text-gray-400">{{ relations.length }} 个</span>
      </div>
      <div v-if="loading" class="py-8 text-center text-sm text-gray-500">加载中...</div>
      <div v-else-if="!relations.length" class="py-8 text-center text-sm text-gray-500">暂无子账号</div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-700/50">
            <tr>
              <th class="px-3 py-2 text-left font-medium text-gray-500">账号</th>
              <th class="px-3 py-2 text-right font-medium text-gray-500">分配额度</th>
              <th class="px-3 py-2 text-right font-medium text-gray-500">已用额度</th>
              <th class="px-3 py-2 text-right font-medium text-gray-500">剩余额度</th>
              <th class="px-3 py-2 text-right font-medium text-gray-500">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="relation in relations" :key="relation.child_user_id">
              <td class="px-3 py-3">
                <div class="font-medium text-gray-900 dark:text-gray-100">{{ relation.child?.email || `用户 #${relation.child_user_id}` }}</div>
                <div class="text-xs text-gray-500">{{ relation.child?.username || '未设置用户名' }}</div>
              </td>
              <td class="px-3 py-3 text-right">{{ formatMoney(relation.allocated_quota) }}</td>
              <td class="px-3 py-3 text-right">{{ formatMoney(relation.used_quota) }}</td>
              <td class="px-3 py-3 text-right">{{ formatMoney(Math.max(0, relation.allocated_quota - relation.used_quota)) }}</td>
              <td class="px-3 py-3">
                <div class="flex justify-end gap-2">
                  <button class="btn btn-secondary btn-sm" type="button" @click="openQuotaEdit(relation)">改额度</button>
                  <button class="btn btn-secondary btn-sm" type="button" @click="selectUsageChild(relation.child_user_id)">记录</button>
                  <button class="btn btn-danger btn-sm" type="button" @click="removeSubAccount(relation.child_user_id)">解绑</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
      <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">子账号使用记录</h2>
        <select v-model.number="usageChildId" class="input max-w-xs" @change="loadUsage(1)">
          <option :value="0">全部子账号</option>
          <option v-for="relation in relations" :key="relation.child_user_id" :value="relation.child_user_id">
            {{ relation.child?.email || `用户 #${relation.child_user_id}` }}
          </option>
        </select>
      </div>
      <div v-if="usageLoading" class="py-8 text-center text-sm text-gray-500">加载中...</div>
      <div v-else-if="!usageRows.length" class="py-8 text-center text-sm text-gray-500">暂无使用记录</div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-700/50">
            <tr>
              <th class="px-3 py-2 text-left font-medium text-gray-500">时间</th>
              <th class="px-3 py-2 text-left font-medium text-gray-500">子账号</th>
              <th class="px-3 py-2 text-left font-medium text-gray-500">模型</th>
              <th class="px-3 py-2 text-right font-medium text-gray-500">母账号额度</th>
              <th class="px-3 py-2 text-right font-medium text-gray-500">实际费用</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="row in usageRows" :key="row.id">
              <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ formatDate(row.created_at) }}</td>
              <td class="px-3 py-3">{{ childLabel(row.user_id) }}</td>
              <td class="px-3 py-3">{{ row.model }}</td>
              <td class="px-3 py-3 text-right">{{ formatMoney(row.parent_quota_used || 0) }}</td>
              <td class="px-3 py-3 text-right">{{ formatMoney(row.actual_cost) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="usageTotalPages > 1" class="mt-4 flex items-center justify-end gap-2">
        <button class="btn btn-secondary btn-sm" type="button" :disabled="usagePage <= 1" @click="loadUsage(usagePage - 1)">上一页</button>
        <span class="text-sm text-gray-500">{{ usagePage }} / {{ usageTotalPages }}</span>
        <button class="btn btn-secondary btn-sm" type="button" :disabled="usagePage >= usageTotalPages" @click="loadUsage(usagePage + 1)">下一页</button>
      </div>
    </section>

    <BaseDialog :show="quotaDialogOpen" title="修改子账号额度" width="narrow" @close="quotaDialogOpen = false">
      <div class="space-y-3">
        <div class="text-sm text-gray-600 dark:text-gray-300">{{ editingRelation?.child?.email || `用户 #${editingRelation?.child_user_id}` }}</div>
        <div>
          <label class="input-label">分配额度</label>
          <input v-model.number="editingQuota" class="input" type="number" min="0" step="0.000001" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" type="button" @click="quotaDialogOpen = false">取消</button>
          <button class="btn btn-primary" type="button" :disabled="submitting" @click="saveQuota">保存</button>
        </div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { subAccountsAPI, type SubAccountCandidate, type SubAccountRelation } from '@/api/subAccounts'
import { useAppStore } from '@/stores/app'
import type { UsageLog } from '@/types'
import { formatCurrency, formatDateTime } from '@/utils/format'
import BaseDialog from '@/components/common/BaseDialog.vue'

const appStore = useAppStore()
const loading = ref(false)
const submitting = ref(false)
const candidateLoading = ref(false)
const usageLoading = ref(false)
const relations = ref<SubAccountRelation[]>([])
const candidates = ref<SubAccountCandidate[]>([])
const usageRows = ref<UsageLog[]>([])
const candidateSearch = ref('')
const newQuota = ref(0)
const usageChildId = ref(0)
const usagePage = ref(1)
const usageTotalPages = ref(1)
const quotaDialogOpen = ref(false)
const editingRelation = ref<SubAccountRelation | null>(null)
const editingQuota = ref(0)

const childMap = computed(() => {
  const map = new Map<number, string>()
  for (const relation of relations.value) {
    map.set(relation.child_user_id, relation.child?.email || `用户 #${relation.child_user_id}`)
  }
  return map
})

function formatMoney(value: number): string {
  return formatCurrency(Number(value || 0))
}

function formatDate(value: string): string {
  return formatDateTime(value) || '-'
}

function childLabel(userId: number): string {
  return childMap.value.get(userId) || `用户 #${userId}`
}

async function loadRelations(): Promise<void> {
  loading.value = true
  try {
    const data = await subAccountsAPI.list()
    relations.value = data.items || []
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || '加载子账号失败')
  } finally {
    loading.value = false
  }
}

async function searchCandidates(): Promise<void> {
  if (candidateSearch.value.trim().length < 2) {
    appStore.showError('请输入至少 2 个字符')
    return
  }
  candidateLoading.value = true
  try {
    const data = await subAccountsAPI.searchCandidates(candidateSearch.value.trim())
    candidates.value = data.items || []
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || '搜索账号失败')
  } finally {
    candidateLoading.value = false
  }
}

async function addSubAccount(childUserId: number): Promise<void> {
  if (newQuota.value < 0) {
    appStore.showError('分配额度不能小于 0')
    return
  }
  submitting.value = true
  try {
    await subAccountsAPI.add(childUserId, Number(newQuota.value || 0))
    appStore.showSuccess('子账号已添加')
    candidates.value = []
    candidateSearch.value = ''
    await refreshAll()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || '添加子账号失败')
  } finally {
    submitting.value = false
  }
}

function openQuotaEdit(relation: SubAccountRelation): void {
  editingRelation.value = relation
  editingQuota.value = relation.allocated_quota
  quotaDialogOpen.value = true
}

async function saveQuota(): Promise<void> {
  if (!editingRelation.value) return
  if (editingQuota.value < 0) {
    appStore.showError('分配额度不能小于 0')
    return
  }
  submitting.value = true
  try {
    await subAccountsAPI.updateQuota(editingRelation.value.child_user_id, Number(editingQuota.value || 0))
    appStore.showSuccess('额度已更新')
    quotaDialogOpen.value = false
    await refreshAll()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || '更新额度失败')
  } finally {
    submitting.value = false
  }
}

async function removeSubAccount(childUserId: number): Promise<void> {
  if (!window.confirm('确定要解绑这个子账号吗？')) return
  submitting.value = true
  try {
    await subAccountsAPI.remove(childUserId)
    appStore.showSuccess('子账号已解绑')
    if (usageChildId.value === childUserId) usageChildId.value = 0
    await refreshAll()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || '解绑子账号失败')
  } finally {
    submitting.value = false
  }
}

function selectUsageChild(childUserId: number): void {
  usageChildId.value = childUserId
  loadUsage(1)
}

async function loadUsage(page = usagePage.value): Promise<void> {
  usageLoading.value = true
  try {
    const data = await subAccountsAPI.usage({
      child_user_id: usageChildId.value || undefined,
      page,
      page_size: 20
    })
    usageRows.value = data.items || []
    usagePage.value = data.page || page
    usageTotalPages.value = data.pages || 1
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || '加载使用记录失败')
  } finally {
    usageLoading.value = false
  }
}

async function refreshAll(): Promise<void> {
  await loadRelations()
  await loadUsage(1)
}

onMounted(() => {
  refreshAll()
})
</script>
