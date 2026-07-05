<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div class="inline-flex items-center gap-2 rounded-full border border-primary-200 bg-primary-50 px-3 py-1 text-xs font-medium text-primary-700 dark:border-primary-900/50 dark:bg-primary-900/20 dark:text-primary-300">
            <Icon name="users" size="xs" />
            母账号额度管理
          </div>
          <h1 class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">子账号管理</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">在 Sub2API 页面框架内管理子账号额度，周额度优先，其次无期限可用额度，最后回退子账号自有余额。</p>
        </div>
        <button class="btn btn-secondary" type="button" @click="refreshAll" :disabled="loading || summaryLoading">
          <Icon name="refresh" size="sm" :class="loading || summaryLoading ? 'animate-spin' : ''" />
          <span>刷新</span>
        </button>
      </div>

      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
        <div class="card p-5">
          <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400"><Icon name="users" size="sm" class="text-primary-500" />子账号数量</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ relations.length }}</p>
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">当前活跃子账号</p>
        </div>
        <div class="card p-5">
          <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400"><Icon name="calendar" size="sm" class="text-violet-500" />本周额度剩余</p>
          <p class="mt-2 text-2xl font-semibold text-violet-600 dark:text-violet-400">{{ formatMoney(totalWeeklyRemainingQuota) }}</p>
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">优先扣除，每周自动刷新</p>
        </div>
        <div class="card p-5">
          <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400"><Icon name="dollar" size="sm" class="text-emerald-500" />无期限可用</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ formatMoney(totalPermanentRemainingQuota) }}</p>
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">周额度不足后使用</p>
        </div>
        <div class="card p-5">
          <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400"><Icon name="chart" size="sm" class="text-amber-500" />母账号已承担</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ formatMoney(totalParentUsedQuota) }}</p>
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">周额度 + 无期限额度</p>
        </div>
        <div class="card p-5">
          <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400"><Icon name="checkCircle" size="sm" class="text-blue-500" />总剩余额度</p>
          <p class="mt-2 text-2xl font-semibold text-blue-600 dark:text-blue-400">{{ formatMoney(totalRemainingQuota) }}</p>
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">子账号可继续使用的母账号额度</p>
        </div>
      </div>

      <section class="card overflow-visible p-6">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <h2 class="flex items-center gap-2 text-base font-semibold text-gray-900 dark:text-white"><Icon name="userPlus" size="sm" class="text-primary-500" />添加子账号</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">搜索已注册账号，分别设置每周额度和无期限当前可用额度。</p>
          </div>
          <div class="rounded-xl border border-primary-200 bg-primary-50 px-3 py-2 text-xs text-primary-700 dark:border-primary-900/40 dark:bg-primary-900/20 dark:text-primary-300">
            扣费顺序：周额度 → 无期限额度 → 子账号自有余额
          </div>
        </div>

        <div class="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1fr)_180px_180px_auto]">
          <div class="relative">
            <label class="input-label">搜索已注册账号</label>
            <div class="relative">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="candidateSearch" class="input pl-9" type="text" autocomplete="off" placeholder="输入邮箱或用户名，至少 2 个字符" @focus="candidateFocused = true" @keydown.down.prevent="moveCandidateHighlight(1)" @keydown.up.prevent="moveCandidateHighlight(-1)" @keydown.enter.prevent="handleCandidateEnter" @keydown.esc="closeCandidateSuggestions" />
            </div>
            <div v-if="showCandidateSuggestions" class="absolute left-0 right-0 z-20 mt-2 max-h-80 overflow-auto rounded-xl border border-gray-200 bg-white py-1 shadow-xl dark:border-dark-700 dark:bg-dark-800">
              <div v-if="candidateLoading" class="flex items-center gap-2 px-3 py-3 text-sm text-gray-500 dark:text-dark-400"><Icon name="refresh" size="sm" class="animate-spin" />搜索中...</div>
              <template v-else>
                <button v-for="(candidate, index) in candidates" :key="candidate.id" class="flex w-full items-center justify-between gap-3 px-3 py-2.5 text-left transition hover:bg-gray-50 dark:hover:bg-dark-700/70" :class="index === highlightedCandidateIndex ? 'bg-primary-50 dark:bg-primary-900/20' : ''" type="button" @mousedown.prevent="selectCandidate(candidate)">
                  <span class="flex min-w-0 items-center gap-3">
                    <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary-100 text-sm font-semibold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">{{ accountInitial(candidate) }}</span>
                    <span class="min-w-0">
                      <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">{{ candidate.email }}</span>
                      <span class="mt-0.5 block truncate text-xs text-gray-500 dark:text-dark-400">{{ candidate.username || '未设置用户名' }} · 自有余额 {{ formatMoney(candidate.balance) }}</span>
                    </span>
                  </span>
                  <span class="shrink-0 rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-dark-300">选择</span>
                </button>
                <div v-if="!candidates.length" class="px-3 py-6 text-center text-sm text-gray-500 dark:text-dark-400">未找到可添加账号</div>
              </template>
            </div>
            <p v-if="candidateSearch.trim().length === 1" class="mt-1 text-xs text-gray-500 dark:text-dark-400">继续输入 1 个字符后开始搜索。</p>
          </div>
          <QuotaInput v-model="newWeeklyQuota" label="每周额度" />
          <QuotaInput v-model="newQuota" label="无期限可用额度" />
          <div class="flex items-end">
            <button class="btn btn-primary w-full xl:w-auto" type="button" :disabled="!selectedCandidate || submitting" @click="addSelectedSubAccount">
              <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else name="plus" size="sm" />
              <span>{{ submitting ? '添加中' : '添加子账号' }}</span>
            </button>
          </div>
        </div>

        <div v-if="selectedCandidate" class="mt-5 rounded-xl border border-primary-200 bg-primary-50 p-4 dark:border-primary-900/50 dark:bg-primary-900/20">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex min-w-0 items-center gap-3">
              <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary-600 text-sm font-semibold text-white">{{ accountInitial(selectedCandidate) }}</span>
              <div class="min-w-0">
                <div class="text-xs font-medium uppercase tracking-wide text-primary-600 dark:text-primary-300">已选择账号</div>
                <div class="mt-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ selectedCandidate.email }}</div>
                <div class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ selectedCandidate.username || '未设置用户名' }} · 自有余额 {{ formatMoney(selectedCandidate.balance) }}</div>
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
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">周额度会按自然周懒刷新；无期限额度只展示和调整当前可用额度，不展示历史总额度。</p>
          </div>
          <span class="inline-flex w-fit items-center rounded-full bg-gray-100 px-3 py-1 text-sm font-medium text-gray-700 dark:bg-dark-700 dark:text-dark-300">{{ relations.length }} 个</span>
        </div>
        <div v-if="loading" class="flex justify-center py-12"><div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div></div>
        <div v-else-if="!relations.length" class="p-6">
          <div class="rounded-xl border border-dashed border-gray-300 p-8 text-center dark:border-dark-700">
            <Icon name="users" size="xl" class="mx-auto text-gray-400 dark:text-dark-500" />
            <p class="mt-3 text-sm font-medium text-gray-700 dark:text-gray-300">暂无子账号</p>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">通过上方搜索已注册账号并分配额度后，会显示在这里。</p>
          </div>
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[1080px] text-left text-sm">
            <thead class="bg-gray-50/80 dark:bg-dark-800/80">
              <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                <th class="px-5 py-4 font-medium">账号</th>
                <th class="px-5 py-4 text-right font-medium">周额度</th>
                <th class="px-5 py-4 text-right font-medium">无期限可用</th>
                <th class="px-5 py-4 text-right font-medium">子账号余额</th>
                <th class="px-5 py-4 font-medium">额度进度</th>
                <th class="px-5 py-4 text-right font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="relation in relations" :key="relation.child_user_id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
                <td class="px-5 py-4">
                  <div class="flex min-w-0 items-center gap-3">
                    <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gray-100 text-sm font-semibold text-gray-700 dark:bg-dark-700 dark:text-dark-200">{{ relationInitial(relation) }}</span>
                    <div class="min-w-0">
                      <div class="truncate font-medium text-gray-900 dark:text-white">{{ relation.child?.email || `用户 #${relation.child_user_id}` }}</div>
                      <div class="mt-0.5 truncate text-xs text-gray-500 dark:text-dark-400">{{ relation.child?.username || '未设置用户名' }}</div>
                    </div>
                  </div>
                </td>
                <td class="px-5 py-4 text-right">
                  <div v-if="editingChildUserId === relation.child_user_id" class="flex justify-end"><QuotaInput v-model="inlineWeeklyQuota" compact /></div>
                  <div v-else>
                    <div class="font-medium text-violet-600 dark:text-violet-400">{{ formatMoney(relation.weekly_remaining_quota ?? weeklyRemainingQuota(relation)) }}</div>
                    <div class="text-xs text-gray-500 dark:text-dark-400">{{ formatMoney(relation.weekly_used_quota || 0) }} / {{ formatMoney(relation.weekly_allocated_quota || 0) }}</div>
                  </div>
                </td>
                <td class="px-5 py-4 text-right">
                  <div v-if="editingChildUserId === relation.child_user_id" class="flex justify-end"><QuotaInput v-model="inlineQuota" compact /></div>
                  <div v-else>
                    <div class="font-medium text-emerald-600 dark:text-emerald-400">{{ formatMoney(permanentRemainingQuota(relation)) }}</div>
                    <div class="text-xs text-gray-500 dark:text-dark-400">当前可用额度</div>
                  </div>
                </td>
                <td class="px-5 py-4 text-right">
                  <div class="font-semibold text-blue-600 dark:text-blue-400">{{ formatMoney(relation.child?.balance || 0) }}</div>
                  <div class="text-xs text-gray-500 dark:text-dark-400">母账号剩余 {{ formatMoney(totalRelationRemainingQuota(relation)) }}</div>
                </td>
                <td class="px-5 py-4">
                  <div class="w-48">
                    <div class="mb-1 flex items-center justify-between text-xs text-gray-500 dark:text-dark-400"><span>{{ quotaUsagePercent(relation) }}%</span><span>{{ quotaStatusLabel(relation) }}</span></div>
                    <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"><div class="h-full rounded-full transition-all" :class="quotaBarClass(relation)" :style="{ width: `${quotaUsagePercent(relation)}%` }"></div></div>
                  </div>
                </td>
                <td class="px-5 py-4">
                  <div v-if="editingChildUserId === relation.child_user_id" class="flex justify-end gap-2">
                    <button class="btn btn-primary btn-sm" type="button" :disabled="submitting" @click="saveInlineQuota(relation)">保存</button>
                    <button class="btn btn-secondary btn-sm" type="button" :disabled="submitting" @click="cancelInlineQuota">取消</button>
                  </div>
                  <div v-else class="flex justify-end gap-2">
                    <button class="btn btn-secondary btn-sm" type="button" @click="startInlineQuotaEdit(relation)">改额度</button>
                    <button class="btn btn-secondary btn-sm" type="button" @click="selectUsageChild(relation.child_user_id)">筛选统计</button>
                    <button class="btn btn-danger btn-sm" type="button" @click="removeSubAccount(relation.child_user_id)">解绑</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-200 p-6 dark:border-dark-700">
          <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">子账号使用分析</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">仅统计当前母账号下子账号使用母账号额度的请求，按时间和子账号筛选查看模型分布、分组使用分布。</p>
            </div>
            <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <div>
                <label class="input-label">子账号</label>
                <select v-model.number="usageChildId" class="input" @change="loadUsageSummary">
                  <option :value="0">全部子账号</option>
                  <option v-for="relation in relations" :key="relation.child_user_id" :value="relation.child_user_id">{{ relation.child?.email || `用户 #${relation.child_user_id}` }}</option>
                </select>
              </div>
              <div>
                <label class="input-label">开始日期</label>
                <input v-model="usageStartDate" class="input" type="date" @change="loadUsageSummary" />
              </div>
              <div>
                <label class="input-label">结束日期</label>
                <input v-model="usageEndDate" class="input" type="date" @change="loadUsageSummary" />
              </div>
              <div class="flex items-end"><button class="btn btn-secondary w-full" type="button" @click="loadUsageSummary"><Icon name="search" size="sm" />查询</button></div>
            </div>
          </div>
        </div>
        <div v-if="summaryLoading" class="flex justify-center py-12"><div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div></div>
        <div v-else class="space-y-6 p-6">
          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"><p class="text-sm text-gray-500 dark:text-dark-400">请求数</p><p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ usageSummary.total_requests }}</p></div>
            <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"><p class="text-sm text-gray-500 dark:text-dark-400">Token</p><p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ formatNumber(usageSummary.total_tokens) }}</p></div>
            <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"><p class="text-sm text-gray-500 dark:text-dark-400">实际费用</p><p class="mt-2 text-2xl font-semibold text-amber-600 dark:text-amber-400">{{ formatMoney(usageSummary.total_actual_cost) }}</p></div>
            <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"><p class="text-sm text-gray-500 dark:text-dark-400">母账号额度扣除</p><p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ formatMoney(usageSummary.total_parent_quota_used) }}</p></div>
          </div>
          <div v-if="!usageSummary.models.length && !usageSummary.groups.length" class="rounded-xl border border-dashed border-gray-300 p-8 text-center dark:border-dark-700">
            <Icon name="chart" size="xl" class="mx-auto text-gray-400 dark:text-dark-500" />
            <p class="mt-3 text-sm font-medium text-gray-700 dark:text-gray-300">暂无使用数据</p>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">调整时间范围或选择其他子账号后重试。</p>
          </div>
          <div v-else class="grid gap-6 xl:grid-cols-2">
            <ModelDistributionChart :model-stats="usageSummary.models" title="模型分布" subtitle="按 Token 统计使用母账号额度的子账号模型分布" />
            <GroupDistributionChart :group-stats="usageSummary.groups" title="分组使用分布" subtitle="按 Token 统计使用母账号额度的子账号分组分布" />
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import GroupDistributionChart from '@/components/charts/GroupDistributionChart.vue'
import { subAccountsAPI, type SubAccountCandidate, type SubAccountRelation, type SubAccountUsageSummary } from '@/api/subAccounts'
import { useAppStore } from '@/stores/app'
import { formatCurrency } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'

const QuotaInput = defineComponent({
  name: 'QuotaInput',
  props: { modelValue: { type: Number, default: 0 }, label: { type: String, default: '' }, compact: { type: Boolean, default: false } },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () => h('div', props.compact ? { class: 'relative' } : {}, [
      props.label && !props.compact ? h('label', { class: 'input-label' }, props.label) : null,
      h('div', { class: 'relative' }, [
        h('span', { class: 'pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-gray-400' }, '￥'),
        h('input', {
          class: props.compact ? 'input h-9 w-32 pl-8 text-right' : 'input pl-8',
          type: 'number', min: '0', step: '0.000001', placeholder: '0.00', value: props.modelValue,
          onInput: (event: Event) => emit('update:modelValue', Number((event.target as HTMLInputElement).value || 0))
        })
      ])
    ])
  }
})

const appStore = useAppStore()
const loading = ref(false)
const submitting = ref(false)
const candidateLoading = ref(false)
const candidateFocused = ref(false)
const summaryLoading = ref(false)
const relations = ref<SubAccountRelation[]>([])
const candidates = ref<SubAccountCandidate[]>([])
const selectedCandidate = ref<SubAccountCandidate | null>(null)
const candidateSearch = ref('')
const newQuota = ref(0)
const newWeeklyQuota = ref(0)
const usageChildId = ref(0)
const usageStartDate = ref(defaultStartDate())
const usageEndDate = ref(defaultEndDate())
const usageSummary = ref<SubAccountUsageSummary>(emptyUsageSummary())
const highlightedCandidateIndex = ref(-1)
const candidateSearchTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const candidateSearchSeq = ref(0)
const editingChildUserId = ref<number | null>(null)
const inlineQuota = ref(0)
const inlineWeeklyQuota = ref(0)

const totalWeeklyRemainingQuota = computed(() => relations.value.reduce((sum, item) => sum + weeklyRemainingQuota(item), 0))
const totalPermanentRemainingQuota = computed(() => relations.value.reduce((sum, item) => sum + permanentRemainingQuota(item), 0))
const totalParentUsedQuota = computed(() => relations.value.reduce((sum, item) => sum + Number(item.used_quota || 0) + Number(item.weekly_used_quota || 0), 0))
const totalRemainingQuota = computed(() => totalWeeklyRemainingQuota.value + totalPermanentRemainingQuota.value)
const showCandidateSuggestions = computed(() => candidateFocused.value && candidateSearch.value.trim().length >= 2 && !selectedCandidate.value)

function emptyUsageSummary(): SubAccountUsageSummary {
  return { total_requests: 0, total_tokens: 0, total_actual_cost: 0, total_parent_quota_used: 0, models: [], groups: [] }
}
function defaultStartDate(): string { const d = new Date(); d.setDate(d.getDate() - 6); return toDateInput(d) }
function defaultEndDate(): string { return toDateInput(new Date()) }
function toDateInput(date: Date): string { return date.toISOString().slice(0, 10) }
function formatMoney(value: number): string { return formatCurrency(Number(value || 0)) }
function formatNumber(value: number): string { return new Intl.NumberFormat('zh-CN').format(Number(value || 0)) }
function accountInitial(account: Pick<SubAccountCandidate, 'email' | 'username'>): string { return (account.username || account.email || '?').trim().slice(0, 1).toUpperCase() }
function relationInitial(relation: SubAccountRelation): string { return (relation.child?.username || relation.child?.email || String(relation.child_user_id)).trim().slice(0, 1).toUpperCase() }
function permanentRemainingQuota(relation: SubAccountRelation): number { return Math.max(0, Number(relation.remaining_quota ?? (Number(relation.allocated_quota || 0) - Number(relation.used_quota || 0)))) }
function weeklyRemainingQuota(relation: SubAccountRelation): number { return Math.max(0, Number(relation.weekly_remaining_quota ?? (Number(relation.weekly_allocated_quota || 0) - Number(relation.weekly_used_quota || 0)))) }
function totalRelationRemainingQuota(relation: SubAccountRelation): number { return Number(relation.total_remaining_quota ?? (permanentRemainingQuota(relation) + weeklyRemainingQuota(relation))) }
function quotaUsagePercent(relation: SubAccountRelation): number {
  const allocated = Number(relation.allocated_quota || 0) + Number(relation.weekly_allocated_quota || 0)
  if (allocated <= 0) return 0
  const used = Number(relation.used_quota || 0) + Number(relation.weekly_used_quota || 0)
  return Math.min(100, Math.round((used / allocated) * 100))
}
function quotaStatusLabel(relation: SubAccountRelation): string { const p = quotaUsagePercent(relation); if (Number(relation.allocated_quota || 0) + Number(relation.weekly_allocated_quota || 0) <= 0) return '未分配'; if (p >= 100) return '已用尽'; if (p >= 80) return '偏高'; return '正常' }
function quotaBarClass(relation: SubAccountRelation): string { const p = quotaUsagePercent(relation); if (p >= 100) return 'bg-red-500'; if (p >= 80) return 'bg-amber-500'; return 'bg-primary-500' }
function closeCandidateSuggestions(): void { candidateFocused.value = false; highlightedCandidateIndex.value = -1 }
function moveCandidateHighlight(step: number): void { if (!candidates.value.length) return; const next = highlightedCandidateIndex.value + step; highlightedCandidateIndex.value = next < 0 ? candidates.value.length - 1 : next >= candidates.value.length ? 0 : next }
function handleCandidateEnter(): void { if (highlightedCandidateIndex.value >= 0 && candidates.value[highlightedCandidateIndex.value]) selectCandidate(candidates.value[highlightedCandidateIndex.value]); else if (candidates.value.length === 1) selectCandidate(candidates.value[0]) }
function selectCandidate(candidate: SubAccountCandidate): void { selectedCandidate.value = candidate; candidateSearch.value = candidate.email; candidates.value = []; closeCandidateSuggestions() }
function clearSelectedCandidate(): void { selectedCandidate.value = null; candidateSearch.value = ''; candidates.value = []; candidateFocused.value = true }

async function loadRelations(): Promise<void> { loading.value = true; try { const data = await subAccountsAPI.list(); relations.value = data.items || [] } catch (error: any) { appStore.showError(error.response?.data?.detail || '加载子账号失败') } finally { loading.value = false } }
async function searchCandidatesNow(query: string): Promise<void> { const search = query.trim(); if (search.length < 2) { candidates.value = []; highlightedCandidateIndex.value = -1; return } const seq = candidateSearchSeq.value + 1; candidateSearchSeq.value = seq; candidateLoading.value = true; try { const data = await subAccountsAPI.searchCandidates(search); if (seq !== candidateSearchSeq.value) return; candidates.value = data.items || []; highlightedCandidateIndex.value = candidates.value.length ? 0 : -1 } catch (error: any) { if (seq !== candidateSearchSeq.value) return; candidates.value = []; highlightedCandidateIndex.value = -1; appStore.showError(error.response?.data?.detail || '搜索账号失败') } finally { if (seq === candidateSearchSeq.value) candidateLoading.value = false } }
async function addSelectedSubAccount(): Promise<void> { if (!selectedCandidate.value) { appStore.showError('请先选择要添加的账号'); return } await addSubAccount(selectedCandidate.value.id) }
async function addSubAccount(childUserId: number): Promise<void> { if (newQuota.value < 0 || newWeeklyQuota.value < 0) { appStore.showError('额度不能小于 0'); return } submitting.value = true; try { await subAccountsAPI.add(childUserId, Number(newQuota.value || 0), Number(newWeeklyQuota.value || 0)); appStore.showSuccess('子账号已添加'); clearSelectedCandidate(); newQuota.value = 0; newWeeklyQuota.value = 0; await refreshAll() } catch (error: any) { appStore.showError(error.response?.data?.detail || '添加子账号失败') } finally { submitting.value = false } }
function startInlineQuotaEdit(relation: SubAccountRelation): void { editingChildUserId.value = relation.child_user_id; inlineQuota.value = permanentRemainingQuota(relation); inlineWeeklyQuota.value = Number(relation.weekly_allocated_quota || 0) }
function cancelInlineQuota(): void { editingChildUserId.value = null; inlineQuota.value = 0; inlineWeeklyQuota.value = 0 }
async function saveInlineQuota(relation: SubAccountRelation): Promise<void> { if (inlineQuota.value < 0 || inlineWeeklyQuota.value < 0) { appStore.showError('额度不能小于 0'); return } submitting.value = true; try { await subAccountsAPI.updateQuota(relation.child_user_id, Number(inlineQuota.value || 0), Number(inlineWeeklyQuota.value || 0)); appStore.showSuccess('额度已更新'); cancelInlineQuota(); await refreshAll() } catch (error: any) { appStore.showError(error.response?.data?.detail || '更新额度失败') } finally { submitting.value = false } }
async function removeSubAccount(childUserId: number): Promise<void> { if (!window.confirm('确定要解绑这个子账号吗？')) return; submitting.value = true; try { await subAccountsAPI.remove(childUserId); appStore.showSuccess('子账号已解绑'); if (usageChildId.value === childUserId) usageChildId.value = 0; await refreshAll() } catch (error: any) { appStore.showError(error.response?.data?.detail || '解绑子账号失败') } finally { submitting.value = false } }
function selectUsageChild(childUserId: number): void { usageChildId.value = childUserId; loadUsageSummary() }
async function loadUsageSummary(): Promise<void> { summaryLoading.value = true; try { usageSummary.value = await subAccountsAPI.usageSummary({ child_user_id: usageChildId.value || undefined, start_date: usageStartDate.value, end_date: usageEndDate.value }) } catch (error: any) { usageSummary.value = emptyUsageSummary(); appStore.showError(error.response?.data?.detail || '加载使用分析失败') } finally { summaryLoading.value = false } }
async function refreshAll(): Promise<void> { await loadRelations(); await loadUsageSummary() }

watch(candidateSearch, (value) => { const search = value.trim(); if (selectedCandidate.value && search !== selectedCandidate.value.email) selectedCandidate.value = null; if (candidateSearchTimer.value) { clearTimeout(candidateSearchTimer.value); candidateSearchTimer.value = null } if (search.length < 2 || selectedCandidate.value) { candidates.value = []; highlightedCandidateIndex.value = -1; candidateLoading.value = false; return } candidateLoading.value = true; candidateSearchTimer.value = setTimeout(() => searchCandidatesNow(search), 250) })
onMounted(() => { refreshAll() })
</script>
