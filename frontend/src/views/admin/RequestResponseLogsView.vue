<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">请求/返回采集</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            查看网关请求的用户入参和返回给用户的上游响应快照。
          </p>
        </div>
        <div class="flex gap-2">
          <button class="btn btn-secondary" type="button" @click="loadAll" :disabled="loading">
            刷新
          </button>
          <button class="btn btn-secondary" type="button" @click="exportCSV" :disabled="exporting">
            {{ exporting ? '导出中...' : '导出 CSV' }}
          </button>
        </div>
      </div>

      <section class="card p-4">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">采集设置</h2>
            <p class="mt-1 text-sm text-amber-600 dark:text-amber-300">
              开启后会持久化用户输入和模型返回正文，可能包含隐私数据，请仅在需要排查/审计时开启。
            </p>
          </div>
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
              <input v-model="settings.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              启用采集
            </label>
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
              最大保存字节
              <input v-model.number="settings.max_body_bytes" type="number" min="1024" max="1048576" step="1024" class="input w-32" />
            </label>
            <button class="btn btn-primary" type="button" :disabled="savingSettings" @click="saveSettings">
              {{ savingSettings ? '保存中...' : '保存设置' }}
            </button>
          </div>
        </div>
      </section>

      <section class="card p-4">
        <div class="grid grid-cols-1 gap-3 md:grid-cols-3 xl:grid-cols-6">
          <input v-model="filters.user_id" class="input" placeholder="用户 ID" />
          <input v-model="filters.api_key_id" class="input" placeholder="API Key ID" />
          <input v-model="filters.group_id" class="input" placeholder="分组 ID" />
          <input v-model="filters.endpoint" class="input" placeholder="Endpoint" />
          <input v-model="filters.model" class="input" placeholder="模型" />
          <input v-model="filters.search" class="input" placeholder="搜索正文/Request ID" />
          <input v-model="filters.start_date" class="input" type="date" />
          <input v-model="filters.end_date" class="input" type="date" />
          <button class="btn btn-primary" type="button" @click="applyFilters">查询</button>
          <button class="btn btn-secondary" type="button" @click="resetFilters">重置</button>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">时间</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">用户/API Key</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">路径</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">模型</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">状态/耗时</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">大小</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="loading">
                <td colspan="7" class="px-4 py-8 text-center text-sm text-gray-500">加载中...</td>
              </tr>
              <tr v-else-if="logs.length === 0">
                <td colspan="7" class="px-4 py-8 text-center text-sm text-gray-500">暂无采集记录</td>
              </tr>
              <tr v-for="log in logs" :key="log.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/60">
                <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-700 dark:text-gray-200">{{ formatTime(log.created_at) }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-200">
                  <div>U: {{ log.user_id }}</div>
                  <div class="text-xs text-gray-500">K: {{ log.api_key_id }}<span v-if="log.group_id"> / G: {{ log.group_id }}</span></div>
                </td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-200">
                  <div class="font-medium">{{ log.method }} {{ log.path }}</div>
                  <div class="text-xs text-gray-500">{{ log.endpoint || '-' }}</div>
                </td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-200">{{ log.model || '-' }}<span v-if="log.stream" class="ml-2 rounded bg-blue-100 px-1.5 py-0.5 text-xs text-blue-700">stream</span></td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-200">{{ log.status_code }} / {{ log.duration_ms }}ms</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-200">
                  <div>入: {{ log.request_body_bytes }}B <span v-if="log.request_truncated" class="text-amber-500">截断</span></div>
                  <div>出: {{ log.response_body_bytes }}B <span v-if="log.response_truncated" class="text-amber-500">截断</span></div>
                </td>
                <td class="px-4 py-3 text-right text-sm">
                  <button class="btn btn-sm btn-secondary" type="button" @click="openDetail(log)">查看</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="border-t border-gray-200 p-4 dark:border-dark-700">
          <Pagination :page="pagination.page" :total="pagination.total" :page-size="pagination.page_size" @update:page="changePage" @update:page-size="changePageSize" />
        </div>
      </section>

      <div v-if="selectedLog" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="selectedLog = null">
        <div class="max-h-[90vh] w-full max-w-6xl overflow-hidden rounded-xl bg-white shadow-xl dark:bg-dark-900">
          <div class="flex items-center justify-between border-b border-gray-200 p-4 dark:border-dark-700">
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">采集详情 #{{ selectedLog.id }}</h3>
              <p class="text-xs text-gray-500">Request ID: {{ selectedLog.request_id || '-' }}</p>
            </div>
            <button class="btn btn-ghost" type="button" @click="selectedLog = null">关闭</button>
          </div>
          <div class="grid max-h-[78vh] grid-cols-1 gap-4 overflow-auto p-4 lg:grid-cols-2">
            <div>
              <h4 class="mb-2 font-medium text-gray-800 dark:text-gray-100">请求入参 <span v-if="selectedLog.request_truncated" class="text-amber-500">（已截断）</span></h4>
              <pre v-if="detailLoading" class="whitespace-pre-wrap break-all rounded-lg bg-gray-100 p-3 text-xs text-gray-400 dark:bg-dark-800">加载中...</pre>
              <template v-else>
                <div v-if="requestImageFiles.length" class="mb-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <div v-for="file in requestImageFiles" :key="`${file.field}-${file.filename}`" class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
                    <img :src="file.data_url" :alt="file.filename" class="max-h-80 w-full rounded object-contain" />
                    <div class="mt-2 break-all text-xs text-gray-600 dark:text-gray-300">
                      {{ file.field }} / {{ file.filename }} / {{ file.content_type || 'image' }} / {{ file.size }}B
                      <span v-if="file.truncated" class="text-amber-500">（预览已截断）</span>
                    </div>
                  </div>
                </div>
                <pre class="whitespace-pre-wrap break-all rounded-lg bg-gray-100 p-3 text-xs text-gray-800 dark:bg-dark-800 dark:text-gray-100">{{ prettyBody(selectedLog.request_body) }}</pre>
              </template>
            </div>
            <div>
              <h4 class="mb-2 font-medium text-gray-800 dark:text-gray-100">返回数据 <span v-if="selectedLog.response_truncated" class="text-amber-500">（已截断）</span></h4>
              <pre v-if="detailLoading" class="whitespace-pre-wrap break-all rounded-lg bg-gray-100 p-3 text-xs text-gray-400 dark:bg-dark-800">加载中...</pre>
              <pre v-else class="whitespace-pre-wrap break-all rounded-lg bg-gray-100 p-3 text-xs text-gray-800 dark:bg-dark-800 dark:text-gray-100">{{ prettyBody(selectedLog.response_body) }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores/app'
import {
  exportRequestResponseLogsUrl,
  getCaptureSettings,
  getRequestResponseLog,
  listRequestResponseLogs,
  updateCaptureSettings,
  type RequestResponseCaptureSettings,
  type RequestResponseLog,
  type RequestResponseLogQueryParams,
} from '@/api/admin/requestResponse'

const appStore = useAppStore()
const logs = ref<RequestResponseLog[]>([])
const loading = ref(false)
const savingSettings = ref(false)
const exporting = ref(false)
const selectedLog = ref<RequestResponseLog | null>(null)
const detailLoading = ref(false)
const settings = reactive<RequestResponseCaptureSettings>({ enabled: false, max_body_bytes: 65536 })
const filters = reactive<Record<string, string>>({
  user_id: '',
  api_key_id: '',
  group_id: '',
  endpoint: '',
  model: '',
  search: '',
  start_date: '',
  end_date: '',
})
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

interface CapturedMultipartFile {
  field: string
  filename: string
  content_type?: string
  size: number
  data_url?: string
  truncated?: boolean
}

const requestImageFiles = computed<CapturedMultipartFile[]>(() => {
  const body = selectedLog.value?.request_body
  if (!body) return []
  try {
    const parsed = JSON.parse(body) as { multipart?: boolean; files?: CapturedMultipartFile[] }
    if (!parsed.multipart || !Array.isArray(parsed.files)) return []
    return parsed.files.filter((file) => typeof file.data_url === 'string' && file.data_url.startsWith('data:image/'))
  } catch {
    return []
  }
})

function numericFilter(value: string): number | undefined {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
}

function buildParams(): RequestResponseLogQueryParams {
  return {
    page: pagination.page,
    page_size: pagination.page_size,
    user_id: numericFilter(filters.user_id),
    api_key_id: numericFilter(filters.api_key_id),
    group_id: numericFilter(filters.group_id),
    endpoint: filters.endpoint.trim() || undefined,
    model: filters.model.trim() || undefined,
    search: filters.search.trim() || undefined,
    start_date: filters.start_date || undefined,
    end_date: filters.end_date || undefined,
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
  }
}

async function loadSettings() {
  const data = await getCaptureSettings()
  settings.enabled = data.enabled
  settings.max_body_bytes = data.max_body_bytes
}

async function loadLogs() {
  loading.value = true
  try {
    const data = await listRequestResponseLogs(buildParams())
    logs.value = data.items || []
    pagination.total = data.total || 0
  } finally {
    loading.value = false
  }
}

async function loadAll() {
  await Promise.all([loadSettings(), loadLogs()])
}

async function saveSettings() {
  savingSettings.value = true
  try {
    const data = await updateCaptureSettings({ enabled: settings.enabled, max_body_bytes: Number(settings.max_body_bytes) || 65536 })
    settings.enabled = data.enabled
    settings.max_body_bytes = data.max_body_bytes
    appStore.showSuccess('采集设置已保存')
  } catch (error) {
    appStore.showError('保存采集设置失败')
    throw error
  } finally {
    savingSettings.value = false
  }
}

function applyFilters() {
  pagination.page = 1
  loadLogs()
}

function resetFilters() {
  Object.keys(filters).forEach((key) => { filters[key] = '' })
  pagination.page = 1
  loadLogs()
}

function changePage(page: number) {
  pagination.page = page
  loadLogs()
}

function changePageSize(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadLogs()
}

async function openDetail(log: RequestResponseLog) {
  selectedLog.value = log
  detailLoading.value = true
  try {
    const full = await getRequestResponseLog(log.id)
    selectedLog.value = full
  } catch {
    appStore.showError('加载详情失败')
  } finally {
    detailLoading.value = false
  }
}

async function exportCSV() {
  exporting.value = true
  try {
    const baseUrl = import.meta.env.VITE_API_BASE_URL || '/api/v1'
    const url = baseUrl + exportRequestResponseLogsUrl(buildParams())
    const token = localStorage.getItem('auth_token') || ''
    const resp = await fetch(url, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
    if (!resp.ok) {
      throw new Error(`export failed: ${resp.status}`)
    }
    const blob = await resp.blob()
    const downloadUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = downloadUrl
    a.download = `request_response_logs_${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(downloadUrl)
  } catch {
    appStore.showError('导出失败')
  } finally {
    exporting.value = false
  }
}

function formatTime(value: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function prettyBody(body: string): string {
  if (!body) return ''
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}

onMounted(loadAll)
</script>
