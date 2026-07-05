<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <div class="inline-flex items-center gap-2 rounded-full border border-primary-200 bg-primary-50 px-3 py-1 text-xs font-medium text-primary-700 dark:border-primary-900/50 dark:bg-primary-900/20 dark:text-primary-300">
          <Icon name="users" size="xs" />
          母账号额度管理
        </div>
        <h1 class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">子账号管理</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">为已注册账号分配母账号额度，并查看子账号使用记录。</p>
      </div>
      <button class="btn btn-secondary" type="button" @click="refreshAll" :disabled="loading || usageLoading">
        <Icon name="refresh" size="sm" :class="loading || usageLoading ? 'animate-spin' : ''" />
        <span>刷新</span>
      </button>
    </div>

    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <div class="card p-5">
        <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400">
          <Icon name="users" size="sm" class="text-primary-500" />
          子账号数量
        </p>
        <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ relations.length }}</p>
        <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">当前母账号下的活跃子账号</p>
      </div>
      <div class="card p-5">
        <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400">
          <Icon name="dollar" size="sm" class="text-emerald-500" />
          已分配额度
        </p>
        <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ formatMoney(totalAllocatedQuota) }}</p>
        <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">所有子账号额度合计</p>
      </div>
      <div class="card p-5">
        <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400">
          <Icon name="chart" size="sm" class="text-amber-500" />
          已使用额度
        </p>
        <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ formatMoney(totalUsedQuota) }}</p>
        <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">从母账号体系扣除的额度</p>
      </div>
      <div class="card p-5">
        <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400">
          <Icon name="checkCircle" size="sm" class="text-blue-500" />
          剩余额度
        </p>
        <p class="mt-2 text-2xl font-semibold text-blue-600 dark:text-blue-400">{{ formatMoney(totalRemainingQuota) }}</p>
        <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">仍可由母账号体系支付</p>
      </div>
    </div>

    <section class="card overflow-visible p-6">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h2 class="flex items-center gap-2 text-base font-semibold text-gray-900 dark:text-white">
            <Icon name="userPlus" size="sm" class="text-primary-500" />
            添加子账号
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">搜索已注册账号，选择后设置该子账号可使用的母账号额度。</p>
        </div>
        <div class="rounded-xl border border-primary-200 bg-primary-50 px-3 py-2 text-xs text-primary-700 dark:border-primary-900/40 dark:bg-primary-900/20 dark:text-primary-300">
          子账号超出分配额度后将自动使用自己的账号付款
        </div>
      </div>

      <div class="mt-5 grid gap-4 lg:grid-cols-[minmax(0,1fr)_200px_auto]">
        <div class="relative">
          <label class="input-label">搜索已注册账号</label>
          <div class="relative">
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="candidateSearch"
              class="input pl-9"
              type="text"
              autocomplete="off"
              placeholder="输入邮箱或用户名，至少 2 个字符"
              @focus="candidateFocused = true"
              @keydown.down.prevent="moveCandidateHighlight(1)"
              @keydown.up.prevent="moveCandidateHighlight(-1)"
              @keydown.enter.prevent="handleCandidateEnter"
              @keydown.esc="closeCandidateSuggestions"
            />
          </div>
          <div
            v-if="showCandidateSuggestions"
            class="absolute left-0 right-0 z-20 mt-2 max-h-80 overflow-auto rounded-xl border border-gray-200 bg-white py-1 shadow-xl dark:border-dark-700 dark:bg-dark-800"
          >
            <div v-if="candidateLoading" class="flex items-center gap-2 px-3 py-3 text-sm text-gray-500 dark:text-dark-400">
              <Icon name="refresh" size="sm" class="animate-spin" />
              搜索中...
            </div>
            <template v-else>
              <button
                v-for="(candidate, index) in candidates"
                :key="candidate.id"
                class="flex w-full items-center justify-between gap-3 px-3 py-2.5 text-left transition hover:bg-gray-50 dark:hover:bg-dark-700/70"
                :class="index === highlightedCandidateIndex ? 'bg-primary-50 dark:bg-primary-900/20' : ''"
                type="button"
                @mousedown.prevent="selectCandidate(candidate)"
              >
                <span class="flex min-w-0 items-center gap-3">
                  <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary-100 text-sm font-semibold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">
                    {{ accountInitial(candidate) }}
                  </span>
                  <span class="min-w-0">
                    <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">{{ candidate.email }}</span>
                    <span class="mt-0.5 block truncate text-xs text-gray-500 dark:text-dark-400">
                      {{ candidate.username || '未设置用户名' }} · 余额 {{ formatMoney(candidate.balance) }}
                    </span>
                  </span>
                </span>
                <span class="shrink-0 rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-dark-300">选择</span>
              </button>
              <div v-if="!candidates.length" class="px-3 py-6 text-center text-sm text-gray-500 dark:text-dark-400">
                未找到可添加账号
              </div>
            </template>
          </div>
          <p v-if="candidateSearch.trim().length === 1" class="mt-1 text-xs text-gray-500 dark:text-dark-400">继续输入 1 个字符后开始搜索。</p>
        </div>
        <div>
          <label class="input-label">分配额度</label>
          <div class="relative">
            <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-gray-400">￥</span>
            <input v-model.number="newQuota" class="input pl-8" type="number" min="0" step="0.000001" placeholder="0.00" />
          </div>
        </div>
        <div class="flex items-end">
          <button class="btn btn-primary w-full lg:w-auto" type="button" :disabled="!selectedCandidate || submitting" @click="addSelectedSubAccount">
            <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
            <Icon v-else name="plus" size="sm" />
            <span>{{ submitting ? '添加中' : '添加子账号' }}</span>
          </button>
        </div>
      </div>

      <div v-if="selectedCandidate" class="mt-5 rounded-xl border border-primary-200 bg-primary-50 p-4 dark:border-primary-900/50 dark:bg-primary-900/20">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex min-w-0 items-center gap-3">
            <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary-600 text-sm font-semibold text-white">
              {{ accountInitial(selectedCandidate) }}
            </span>
            <div class="min-w-0">
              <div class="text-xs font-medium uppercase tracking-wide text-primary-600 dark:text-primary-300">已选择账号</div>
              <div class="mt-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ selectedCandidate.email }}</div>
              <div class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                {{ selectedCandidate.username || '未设置用户名' }} · 余额 {{ formatMoney(selectedCandidate.balance) }}
              </div>
            </div>
          </div>
          <button class="btn btn-secondary btn-sm" type="button" @click="clearSelectedCandidate">重新选择</button>
        </div>
      </div>
    </section>

    <section class="card overflow-hidden">
      <div class="flex flex-col gap-3 border-b border-gray-200 p-6 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">我的子账号</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">管理子账号额度，查看指定子账号记录或解除绑定。</p>
        </div>
        <span class="inline-flex w-fit items-center rounded-full bg-gray-100 px-3 py-1 text-sm font-medium text-gray-700 dark:bg-dark-700 dark:text-dark-300">{{ relations.length }} 个</span>
      </div>
      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>
      <div v-else-if="!relations.length" class="p-6">
        <div class="rounded-xl border border-dashed border-gray-300 p-8 text-center dark:border-dark-700">
          <Icon name="users" size="xl" class="mx-auto text-gray-400 dark:text-dark-500" />
          <p class="mt-3 text-sm font-medium text-gray-700 dark:text-gray-300">暂无子账号</p>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">通过上方搜索已注册账号并分配额度后，会显示在这里。</p>
        </div>
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[820px] text-left text-sm">
          <thead class="bg-gray-50/80 dark:bg-dark-800/80">
            <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
              <th class="px-5 py-4 font-medium">账号</th>
              <th class="px-5 py-4 text-right font-medium">额度进度</th>
              <th class="px-5 py-4 text-right font-medium">分配额度</th>
              <th class="px-5 py-4 text-right font-medium">已用 / 剩余</th>
              <th class="px-5 py-4 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="relation in relations" :key="relation.child_user_id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
              <td class="px-5 py-4">
                <div class="flex min-w-0 items-center gap-3">
                  <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gray-100 text-sm font-semibold text-gray-700 dark:bg-dark-700 dark:text-dark-200">
                    {{ relationInitial(relation) }}
                  </span>
                  <div class="min-w-0">
                    <div class="truncate font-medium text-gray-900 dark:text-white">{{ relation.child?.email || `用户 #${relation.child_user_id}` }}</div>
                    <div class="mt-0.5 truncate text-xs text-gray-500 dark:text-dark-400">{{ relation.child?.username || '未设置用户名' }}</div>
                  </div>
                </div>
              </td>
              <td class="px-5 py-4 text-right">
                <div class="ml-auto w-40">
                  <div class="mb-1 flex items-center justify-between text-xs text-gray-500 dark:text-dark-400">
                    <span>{{ quotaUsagePercent(relation) }}%</span>
                    <span>{{ quotaStatusLabel(relation) }}</span>
                  </div>
                  <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                    <div class="h-full rounded-full transition-all" :class="quotaBarClass(relation)" :style="{ width: `${quotaUsagePercent(relation)}%` }"></div>
                  </div>
                </div>
              </td>
              <td class="px-5 py-4 text-right">
                <div v-if="editingChildUserId === relation.child_user_id" class="flex justify-end gap-2">
                  <div class="relative">
                    <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-gray-400">￥</span>
                    <input v-model.number="inlineQuota" class="input h-9 w-36 pl-8 text-right" type="number" min="0" step="0.000001" />
                  </div>
                  <button class="btn btn-primary btn-sm" type="button" :disabled="submitting" @click="saveInlineQuota(relation)">保存</button>
                  <button class="btn btn-secondary btn-sm" type="button" :disabled="submitting" @click="cancelInlineQuota">取消</button>
                </div>
                <span v-else class="font-medium text-gray-900 dark:text-white">{{ formatMoney(relation.allocated_quota) }}</span>
              </td>
              <td class="px-5 py-4 text-right">
                <div class="font-medium text-amber-600 dark:text-amber-400">{{ formatMoney(relation.used_quota) }}</div>
                <div class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">剩余 {{ formatMoney(remainingQuota(relation)) }}</div>
              </td>
              <td class="px-5 py-4">
                <div class="flex justify-end gap-2">
                  <button class="btn btn-secondary btn-sm" type="button" @click="startInlineQuotaEdit(relation)">改额度</button>
                  <button class="btn btn-secondary btn-sm" type="button" @click="selectUsageChild(relation.child_user_id)">记录</button>
                  <button class="btn btn-danger btn-sm" type="button" @click="removeSubAccount(relation.child_user_id)">解绑</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="card overflow-hidden">
      <div class="flex flex-col gap-3 border-b border-gray-200 p-6 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">子账号使用记录</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">仅展示当前母账号体系下子账号产生的使用记录。</p>
        </div>
        <select v-model.number="usageChildId" class="input max-w-xs" @change="loadUsage(1)">
          <option :value="0">全部子账号</option>
          <option v-for="relation in relations" :key="relation.child_user_id" :value="relation.child_user_id">
            {{ relation.child?.email || `用户 #${relation.child_user_id}` }}
          </option>
        </select>
      </div>
      <div v-if="usageLoading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>
      <div v-else-if="!usageRows.length" class="p-6">
        <div class="rounded-xl border border-dashed border-gray-300 p-8 text-center dark:border-dark-700">
          <Icon name="chart" size="xl" class="mx-auto text-gray-400 dark:text-dark-500" />
          <p class="mt-3 text-sm font-medium text-gray-700 dark:text-gray-300">暂无使用记录</p>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">子账号产生请求后，会在这里展示母账号额度扣除情况。</p>
        </div>
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[760px] text-left text-sm">
          <thead class="bg-gray-50/80 dark:bg-dark-800/80">
            <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
              <th class="px-5 py-4 font-medium">时间</th>
              <th class="px-5 py-4 font-medium">子账号</th>
              <th class="px-5 py-4 font-medium">模型</th>
              <th class="px-5 py-4 text-right font-medium">母账号额度</th>
              <th class="px-5 py-4 text-right font-medium">实际费用</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in usageRows" :key="row.id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
              <td class="whitespace-nowrap px-5 py-4 text-gray-600 dark:text-gray-300">{{ formatDate(row.created_at) }}</td>
              <td class="px-5 py-4 font-medium text-gray-900 dark:text-white">{{ childLabel(row.user_id) }}</td>
              <td class="px-5 py-4 text-gray-700 dark:text-gray-300">{{ row.model || '-' }}</td>
              <td class="px-5 py-4 text-right font-medium text-primary-600 dark:text-primary-400">{{ formatMoney(row.parent_quota_used || 0) }}</td>
              <td class="px-5 py-4 text-right text-gray-700 dark:text-gray-300">{{ formatMoney(row.actual_cost) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="usageTotalPages > 1" class="flex items-center justify-end gap-2 border-t border-gray-200 px-6 py-4 dark:border-dark-700">
        <button class="btn btn-secondary btn-sm" type="button" :disabled="usagePage <= 1" @click="loadUsage(usagePage - 1)">上一页</button>
        <span class="text-sm text-gray-500 dark:text-dark-400">{{ usagePage }} / {{ usageTotalPages }}</span>
        <button class="btn btn-secondary btn-sm" type="button" :disabled="usagePage >= usageTotalPages" @click="loadUsage(usagePage + 1)">下一页</button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { subAccountsAPI, type SubAccountCandidate, type SubAccountRelation } from '@/api/subAccounts'
import { useAppStore } from '@/stores/app'
import type { UsageLog } from '@/types'
import { formatCurrency, formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'

const appStore = useAppStore()
const loading = ref(false)
const submitting = ref(false)
const candidateLoading = ref(false)
const candidateFocused = ref(false)
const usageLoading = ref(false)
const relations = ref<SubAccountRelation[]>([])
const candidates = ref<SubAccountCandidate[]>([])
const selectedCandidate = ref<SubAccountCandidate | null>(null)
const usageRows = ref<UsageLog[]>([])
const candidateSearch = ref('')
const newQuota = ref(0)
const usageChildId = ref(0)
const usagePage = ref(1)
const usageTotalPages = ref(1)
const highlightedCandidateIndex = ref(-1)
const candidateSearchTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const candidateSearchSeq = ref(0)
const editingChildUserId = ref<number | null>(null)
const inlineQuota = ref(0)

const childMap = computed(() => {
  const map = new Map<number, string>()
  for (const relation of relations.value) {
    map.set(relation.child_user_id, relation.child?.email || `用户 #${relation.child_user_id}`)
  }
  return map
})

const totalAllocatedQuota = computed(() => relations.value.reduce((sum, item) => sum + Number(item.allocated_quota || 0), 0))
const totalUsedQuota = computed(() => relations.value.reduce((sum, item) => sum + Number(item.used_quota || 0), 0))
const totalRemainingQuota = computed(() => Math.max(0, totalAllocatedQuota.value - totalUsedQuota.value))

const showCandidateSuggestions = computed(() => {
  return candidateFocused.value && candidateSearch.value.trim().length >= 2 && !selectedCandidate.value
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

function accountInitial(account: Pick<SubAccountCandidate, 'email' | 'username'>): string {
  const text = account.username || account.email || '?'
  return text.trim().slice(0, 1).toUpperCase()
}

function relationInitial(relation: SubAccountRelation): string {
  const text = relation.child?.username || relation.child?.email || String(relation.child_user_id)
  return text.trim().slice(0, 1).toUpperCase()
}

function remainingQuota(relation: SubAccountRelation): number {
  return Math.max(0, Number(relation.allocated_quota || 0) - Number(relation.used_quota || 0))
}

function quotaUsagePercent(relation: SubAccountRelation): number {
  const allocated = Number(relation.allocated_quota || 0)
  if (allocated <= 0) return 0
  return Math.min(100, Math.round((Number(relation.used_quota || 0) / allocated) * 100))
}

function quotaStatusLabel(relation: SubAccountRelation): string {
  const percent = quotaUsagePercent(relation)
  if (Number(relation.allocated_quota || 0) <= 0) return '未分配'
  if (percent >= 100) return '已用尽'
  if (percent >= 80) return '偏高'
  return '正常'
}

function quotaBarClass(relation: SubAccountRelation): string {
  const percent = quotaUsagePercent(relation)
  if (percent >= 100) return 'bg-red-500'
  if (percent >= 80) return 'bg-amber-500'
  return 'bg-primary-500'
}

function closeCandidateSuggestions(): void {
  candidateFocused.value = false
  highlightedCandidateIndex.value = -1
}

function moveCandidateHighlight(step: number): void {
  if (!candidates.value.length) return
  const next = highlightedCandidateIndex.value + step
  if (next < 0) {
    highlightedCandidateIndex.value = candidates.value.length - 1
  } else if (next >= candidates.value.length) {
    highlightedCandidateIndex.value = 0
  } else {
    highlightedCandidateIndex.value = next
  }
}

function handleCandidateEnter(): void {
  if (highlightedCandidateIndex.value >= 0 && candidates.value[highlightedCandidateIndex.value]) {
    selectCandidate(candidates.value[highlightedCandidateIndex.value])
  } else if (candidates.value.length === 1) {
    selectCandidate(candidates.value[0])
  }
}

function selectCandidate(candidate: SubAccountCandidate): void {
  selectedCandidate.value = candidate
  candidateSearch.value = candidate.email
  candidates.value = []
  closeCandidateSuggestions()
}

function clearSelectedCandidate(): void {
  selectedCandidate.value = null
  candidateSearch.value = ''
  candidates.value = []
  candidateFocused.value = true
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

async function searchCandidatesNow(query: string): Promise<void> {
  const search = query.trim()
  if (search.length < 2) {
    candidates.value = []
    highlightedCandidateIndex.value = -1
    return
  }
  const seq = candidateSearchSeq.value + 1
  candidateSearchSeq.value = seq
  candidateLoading.value = true
  try {
    const data = await subAccountsAPI.searchCandidates(search)
    if (seq !== candidateSearchSeq.value) return
    candidates.value = data.items || []
    highlightedCandidateIndex.value = candidates.value.length ? 0 : -1
  } catch (error: any) {
    if (seq !== candidateSearchSeq.value) return
    candidates.value = []
    highlightedCandidateIndex.value = -1
    appStore.showError(error.response?.data?.detail || '搜索账号失败')
  } finally {
    if (seq === candidateSearchSeq.value) candidateLoading.value = false
  }
}

async function addSelectedSubAccount(): Promise<void> {
  if (!selectedCandidate.value) {
    appStore.showError('请先选择要添加的账号')
    return
  }
  await addSubAccount(selectedCandidate.value.id)
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
    clearSelectedCandidate()
    await refreshAll()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || '添加子账号失败')
  } finally {
    submitting.value = false
  }
}

function startInlineQuotaEdit(relation: SubAccountRelation): void {
  editingChildUserId.value = relation.child_user_id
  inlineQuota.value = relation.allocated_quota
}

function cancelInlineQuota(): void {
  editingChildUserId.value = null
  inlineQuota.value = 0
}

async function saveInlineQuota(relation: SubAccountRelation): Promise<void> {
  if (inlineQuota.value < 0) {
    appStore.showError('分配额度不能小于 0')
    return
  }
  submitting.value = true
  try {
    await subAccountsAPI.updateQuota(relation.child_user_id, Number(inlineQuota.value || 0))
    appStore.showSuccess('额度已更新')
    cancelInlineQuota()
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

watch(candidateSearch, (value) => {
  const search = value.trim()
  if (selectedCandidate.value && search !== selectedCandidate.value.email) {
    selectedCandidate.value = null
  }
  if (candidateSearchTimer.value) {
    clearTimeout(candidateSearchTimer.value)
    candidateSearchTimer.value = null
  }
  if (search.length < 2 || selectedCandidate.value) {
    candidates.value = []
    highlightedCandidateIndex.value = -1
    candidateLoading.value = false
    return
  }
  candidateLoading.value = true
  candidateSearchTimer.value = setTimeout(() => {
    searchCandidatesNow(search)
  }, 250)
})

onMounted(() => {
  refreshAll()
})
</script>
