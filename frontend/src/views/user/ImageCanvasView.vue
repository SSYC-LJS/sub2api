<template>
  <AppLayout>
    <div class="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
      <section class="relative overflow-hidden rounded-3xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="pointer-events-none absolute inset-0 bg-gradient-to-br from-primary-500/8 via-transparent to-blue-500/8 dark:from-primary-500/12 dark:to-blue-500/10"></div>
        <div class="relative flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div class="space-y-2">
            <div class="inline-flex items-center gap-2 rounded-full border border-primary-500/20 bg-primary-500/10 px-3 py-1 text-xs font-medium text-primary-600 dark:text-primary-300">
              <Icon name="sparkles" size="sm" />
              生图工作台
            </div>
            <div>
              <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-3xl">生图无限画布</h1>
              <p class="mt-2 max-w-3xl text-sm text-gray-500 dark:text-gray-400">
                每个生图节点从上往下排列；后续修改会沿着该节点向右延展，每个节点独立生成、编辑和重试。
              </p>
            </div>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="loadAll">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
              刷新
            </button>
            <button class="btn btn-primary" :disabled="keys.length === 0" @click="openGenerateDialog()">
              <Icon name="plus" size="md" />
              新建生图
            </button>
          </div>
        </div>
      </section>

      <section class="card min-h-[760px] overflow-hidden p-0">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">画布节点</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">点击图片卡片可放大预览；卡片内可下载、修改或重试。</p>
          </div>
          <div class="flex gap-2 text-xs text-gray-500 dark:text-gray-400">
            <span class="rounded-full bg-gray-100 px-2.5 py-1 dark:bg-dark-800">主节点 {{ chains.length }}</span>
            <span class="rounded-full bg-primary-50 px-2.5 py-1 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">执行中 {{ runningCount }}</span>
          </div>
        </div>

        <div v-if="keys.length === 0" class="m-5 rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
          <p class="font-semibold">你还没有可用 Key</p>
          <p class="mt-1">先创建 API Key 后才能在画布中生图。</p>
          <button class="btn btn-primary mt-3" @click="router.push('/keys')">去创建 Key</button>
        </div>

        <div v-if="globalError" class="mx-5 mt-5 rounded-2xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200 whitespace-pre-wrap">{{ globalError }}</div>

        <div class="h-[calc(100vh-17rem)] min-h-[680px] overflow-auto bg-gray-50/80 p-5 dark:bg-dark-950/50">
          <div v-if="loading" class="space-y-4">
            <div v-for="i in 4" :key="i" class="h-80 animate-pulse rounded-3xl bg-white dark:bg-dark-900"></div>
          </div>
          <div v-else-if="chains.length === 0" class="flex h-full min-h-[520px] flex-col items-center justify-center text-center">
            <Icon name="inbox" size="xl" class="mb-4 h-14 w-14 text-gray-400" />
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">画布还是空的</h3>
            <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">点击“新建生图”创建第一个节点。</p>
          </div>
          <div v-else class="space-y-5">
            <section v-for="chain in chains" :key="chain.rootId" class="min-w-max rounded-3xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
              <div class="mb-3 flex items-center justify-between gap-3">
                <div>
                  <h3 class="font-semibold text-gray-900 dark:text-white">生图节点 #{{ chain.index + 1 }}</h3>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">修改步骤从左往右排列</p>
                </div>
                <button class="btn btn-secondary" @click="openEditDialog(chain.nodes[chain.nodes.length - 1])" :disabled="!canUseImage(chain.nodes[chain.nodes.length - 1])">
                  <Icon name="edit" size="sm" />
                  继续修改
                </button>
              </div>
              <div class="flex items-stretch gap-4 overflow-x-auto pb-2">
                <template v-for="(node, nodeIndex) in chain.nodes" :key="node.localId">
                  <article class="w-72 flex-shrink-0 overflow-hidden rounded-2xl border bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800" :class="node.status === 'failed' ? 'border-red-200 dark:border-red-500/40' : 'border-gray-200'">
                    <button class="relative block aspect-[4/3] w-full bg-gray-100 text-left dark:bg-dark-700" :disabled="!canUseImage(node)" @click="openPreview(node)">
                      <img v-if="canUseImage(node)" :src="nodeImageSrc(node)" :alt="node.prompt" class="h-full w-full object-cover" loading="lazy" />
                      <div v-else class="flex h-full flex-col items-center justify-center gap-2 p-5 text-center">
                        <Icon :name="node.imageExpired ? 'clock' : node.status === 'failed' ? 'exclamationCircle' : 'sparkles'" size="xl" :class="node.imageExpired ? 'text-amber-400' : node.status === 'failed' ? 'text-red-400' : 'text-primary-400'" />
                        <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ node.imageExpired ? '图片已过期' : statusText(node) }}</span>
                        <span v-if="node.imageExpired" class="text-xs text-gray-500 dark:text-gray-400">生成图片只保留 2 小时，请重试重新生成</span>
                      </div>
                      <span class="absolute left-3 top-3 rounded-full px-2.5 py-1 text-xs font-semibold" :class="statusBadgeClass(node.status)">{{ statusText(node) }}</span>
                    </button>
                    <div class="space-y-3 p-4">
                      <div class="flex items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
                        <span>{{ node.operation === 'edit' ? `修改 ${nodeIndex}` : '原始生成' }}</span>
                        <span>png</span>
                      </div>
                      <p class="line-clamp-4 min-h-[5rem] text-sm text-gray-800 dark:text-gray-200">{{ node.prompt }}</p>
                      <div class="flex flex-wrap gap-2 text-[11px] text-gray-600 dark:text-gray-300">
                        <span class="rounded-full bg-gray-100 px-2 py-1 dark:bg-dark-900">{{ node.model }}</span>
                        <span class="rounded-full bg-gray-100 px-2 py-1 dark:bg-dark-900">{{ node.size }}</span>
                        <span class="rounded-full bg-gray-100 px-2 py-1 dark:bg-dark-900">{{ node.apiKeyName }}</span>
                      </div>
                      <div v-if="node.error" class="rounded-xl border border-red-200 bg-red-50 p-2 text-xs text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200 whitespace-pre-wrap">{{ node.error }}</div>
                      <div class="grid grid-cols-3 gap-2">
                        <button class="btn btn-secondary justify-center px-2 text-xs" :disabled="!canUseImage(node)" @click="downloadOriginal(node)">下载</button>
                        <button class="btn btn-secondary justify-center px-2 text-xs" :disabled="!canUseImage(node) || isBusy(node)" @click="openEditDialog(node)">修改</button>
                        <button class="btn btn-secondary justify-center px-2 text-xs" :disabled="isBusy(node)" @click="retryNode(node)">重试</button>
                      </div>
                    </div>
                  </article>
                  <div v-if="nodeIndex < chain.nodes.length - 1" class="flex flex-shrink-0 items-center text-gray-300 dark:text-dark-600">
                    <Icon name="chevronRight" size="lg" />
                  </div>
                </template>
              </div>
            </section>
          </div>
        </div>
      </section>
      <BaseDialog :show="taskDialog.show" :title="taskDialog.mode === 'edit' ? '修改图片' : '新建生图'" width="wide" close-on-click-outside @close="closeTaskDialog">
        <div class="space-y-4">
          <div v-if="taskDialog.sourceNode" class="rounded-2xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
            <p class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">基于此图片继续修改</p>
            <img :src="nodeImageSrc(taskDialog.sourceNode)" class="max-h-48 rounded-xl object-contain" alt="source" />
          </div>
          <div class="grid gap-4 md:grid-cols-2">
            <label class="block text-sm">
              <span class="mb-1.5 block font-medium text-gray-700 dark:text-gray-300">使用 Key</span>
              <select v-model="taskForm.apiKeyId" class="input">
                <option :value="0">请选择 Key</option>
                <option v-for="key in keys" :key="key.id" :value="key.id">{{ key.name }} · {{ key.group?.name || '未分组' }}</option>
              </select>
            </label>
            <label class="block text-sm">
              <span class="mb-1.5 block font-medium text-gray-700 dark:text-gray-300">模型广场可用图片模型</span>
              <select v-model="taskForm.model" class="input" :disabled="dialogImageModels.length === 0">
                <option v-if="dialogImageModels.length === 0" value="">当前 Key 所属分组暂无图片模型</option>
                <option v-for="model in dialogImageModels" :key="`${model.groupId}-${model.platform}-${model.name}`" :value="model.name">{{ model.name }} · {{ model.groupName }}</option>
              </select>
            </label>
          </div>
          <div class="grid gap-4 md:grid-cols-2">
            <label class="block text-sm">
              <span class="mb-1.5 block font-medium text-gray-700 dark:text-gray-300">尺寸大小</span>
              <input v-model="taskForm.size" class="input" placeholder="例如 1024x1024、1024x1536、auto" />
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">完全自定义；留空默认 1024x1024。</p>
            </label>
            <label class="block text-sm">
              <span class="mb-1.5 block font-medium text-gray-700 dark:text-gray-300">生图数量</span>
              <input v-model.number="taskForm.count" type="number" min="1" max="8" class="input" />
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">每张图会生成一个独立节点；格式固定 png。</p>
            </label>
          </div>
          <label class="block text-sm">
            <span class="mb-1.5 block font-medium text-gray-700 dark:text-gray-300">提示词</span>
            <textarea v-model="taskForm.prompt" rows="5" class="input resize-none" placeholder="描述你想生成或修改的图片..."></textarea>
          </label>
          <div v-if="taskDialog.error" class="rounded-2xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200 whitespace-pre-wrap">{{ taskDialog.error }}</div>
        </div>
        <template #footer>
          <button class="btn btn-secondary" @click="closeTaskDialog">取消</button>
          <button class="btn btn-primary" :disabled="!canSubmitDialog" @click="submitTaskDialog">{{ taskDialog.mode === 'edit' ? '开始修改' : '开始生成' }}</button>
        </template>
      </BaseDialog>

      <BaseDialog :show="!!previewNode" title="图片预览" width="extra-wide" close-on-click-outside @close="previewNode = null">
        <div v-if="previewNode" class="space-y-4">
          <div class="flex justify-center rounded-2xl bg-gray-100 p-3 dark:bg-dark-800">
            <img :src="nodeImageSrc(previewNode)" :alt="previewNode.prompt" class="max-h-[70vh] rounded-xl object-contain" />
          </div>
          <div class="rounded-2xl border border-gray-200 p-4 text-sm dark:border-dark-700">
            <p class="font-medium text-gray-900 dark:text-white">{{ previewNode.prompt }}</p>
            <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ previewNode.model }} · {{ previewNode.size }} · {{ previewNode.apiKeyName }}</p>
          </div>
        </div>
        <template #footer>
          <button v-if="previewNode" class="btn btn-secondary" @click="downloadOriginal(previewNode)">下载原图</button>
          <button class="btn btn-primary" @click="previewNode = null">关闭</button>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { keysAPI } from '@/api/keys'
import userChannelsAPI, { type UserAvailableChannel } from '@/api/channels'
import { imageCanvasAPI, type ImageCanvasHistoryItem, type OpenAIImagesResponse } from '@/api/imageCanvas'
import type { ApiKey } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

interface ImageModelOption {
  name: string
  groupId: number
  groupName: string
  platform: string
}

type NodeStatus = 'completed' | 'generating' | 'editing' | 'failed'

interface CanvasNode {
  localId: string
  rootId: string
  parentId?: string
  historyId?: number
  apiKeyId: number
  apiKeyName: string
  operation: 'generate' | 'edit'
  model: string
  prompt: string
  size: string
  imageUrl?: string
  b64Json?: string
  mimeType: string
  sourceImageUrl?: string
  imageExpired: boolean
  expiresAt?: string
  status: NodeStatus
  error: string
  createdAt: string
  retryPayload: TaskPayload
}

interface CanvasChain {
  rootId: string
  index: number
  nodes: CanvasNode[]
}

interface TaskPayload {
  apiKeyId: number
  model: string
  prompt: string
  size: string
  count: number
}

const router = useRouter()
const keys = ref<ApiKey[]>([])
const channels = ref<UserAvailableChannel[]>([])
const nodes = ref<CanvasNode[]>([])
const loading = ref(false)
const globalError = ref('')
const previewNode = ref<CanvasNode | null>(null)
let localSeq = 0

const taskDialog = reactive({
  show: false,
  mode: 'generate' as 'generate' | 'edit',
  sourceNode: null as CanvasNode | null,
  error: ''
})

const taskForm = reactive<TaskPayload>({
  apiKeyId: 0,
  model: '',
  prompt: '',
  size: '1024x1024',
  count: 1
})

const allImageModels = computed<ImageModelOption[]>(() => {
  const result = new Map<string, ImageModelOption>()
  for (const channel of channels.value) {
    for (const section of channel.platforms || []) {
      for (const group of section.groups || []) {
        for (const model of section.supported_models || []) {
          const name = String(model.name || '').trim()
          if (!isImageModel(name, model.pricing?.billing_mode)) continue
          const platform = model.platform || group.platform || section.platform || ''
          result.set(`${group.id}:${platform}:${name}`, { name, groupId: group.id, groupName: group.name, platform })
        }
      }
    }
  }
  return Array.from(result.values()).sort((a, b) => a.groupName.localeCompare(b.groupName) || a.name.localeCompare(b.name))
})

const dialogImageModels = computed(() => {
  const key = keys.value.find(item => item.id === taskForm.apiKeyId)
  const groupId = key?.group_id
  if (!groupId) return allImageModels.value
  return allImageModels.value.filter(model => model.groupId === groupId)
})

const canSubmitDialog = computed(() => Boolean(
  keys.value.find(key => key.id === taskForm.apiKeyId)?.key &&
  taskForm.model &&
  taskForm.prompt.trim() &&
  normalizeCount(taskForm.count) > 0
))

const chains = computed<CanvasChain[]>(() => {
  const byRoot = new Map<string, CanvasNode[]>()
  for (const node of nodes.value) {
    const rootId = node.rootId || node.localId
    const list = byRoot.get(rootId) || []
    list.push(node)
    byRoot.set(rootId, list)
  }
  return Array.from(byRoot.entries()).map(([rootId, list], index) => ({
    rootId,
    index,
    nodes: list.sort((a, b) => Date.parse(a.createdAt) - Date.parse(b.createdAt))
  })).sort((a, b) => Date.parse(b.nodes[0]?.createdAt || '') - Date.parse(a.nodes[0]?.createdAt || ''))
})

const runningCount = computed(() => nodes.value.filter(isBusy).length)

watch(dialogImageModels, models => {
  if (!models.some(model => model.name === taskForm.model)) taskForm.model = models[0]?.name || ''
}, { immediate: true })

watch(() => taskForm.apiKeyId, () => {
  if (!dialogImageModels.value.some(model => model.name === taskForm.model)) taskForm.model = dialogImageModels.value[0]?.name || ''
})

function isImageModel(name: string, billingMode?: string): boolean {
  const normalized = name.toLowerCase()
  if (!normalized) return false
  if (billingMode === 'image') return true
  return /(^gpt-image-|^dall-e-|image|imagine|imagen|flux|ideogram|stable-diffusion|sdxl)/i.test(normalized)
}

function normalizeSize(size: string): string {
  return size.trim() || '1024x1024'
}

function normalizeCount(count: number): number {
  const parsed = Number(count)
  if (!Number.isFinite(parsed)) return 1
  return Math.max(1, Math.min(8, Math.floor(parsed)))
}

function isBusy(node: CanvasNode): boolean {
  return node.status === 'generating' || node.status === 'editing'
}

function nodeImageSrc(node: CanvasNode): string {
  if (node.imageExpired) return ''
  if (node.b64Json) return `data:${node.mimeType || 'image/png'};base64,${node.b64Json}`
  return node.imageUrl || ''
}

function canUseImage(node: CanvasNode | null | undefined): boolean {
  return Boolean(node && !node.imageExpired && nodeImageSrc(node))
}

function statusText(node: CanvasNode): string {
  if (node.status === 'generating') return '生成中'
  if (node.status === 'editing') return '编辑中'
  if (node.status === 'failed') return '失败'
  return '已完成'
}

function statusBadgeClass(status: NodeStatus): string {
  if (status === 'failed') return 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-200'
  if (status === 'generating' || status === 'editing') return 'bg-primary-100 text-primary-700 dark:bg-primary-500/20 dark:text-primary-200'
  return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-200'
}

function nodeFromHistory(item: ImageCanvasHistoryItem): CanvasNode {
  const rootId = item.source_image_url ? `history-${item.id}` : `history-${item.id}`
  return {
    localId: `history-${item.id}`,
    rootId,
    historyId: item.id,
    apiKeyId: item.api_key_id,
    apiKeyName: item.api_key_name,
    operation: item.operation,
    model: item.model,
    prompt: item.prompt,
    size: item.size || '1024x1024',
    imageUrl: item.image_url,
    b64Json: item.b64_json,
    mimeType: item.mime_type || 'image/png',
    sourceImageUrl: item.source_image_url,
    imageExpired: Boolean(item.image_expired),
    expiresAt: item.expires_at,
    status: 'completed',
    error: '',
    createdAt: item.created_at,
    retryPayload: {
      apiKeyId: item.api_key_id,
      model: item.model,
      prompt: item.prompt,
      size: item.size || '1024x1024',
      count: 1
    }
  }
}

async function loadKeys() {
  const response = await keysAPI.list(1, 100, { status: 'active' })
  keys.value = response.items.filter(key => key.status === 'active' && key.key)
  if (!taskForm.apiKeyId && keys.value.length > 0) taskForm.apiKeyId = keys.value[0].id
}

async function loadModels() {
  channels.value = await userChannelsAPI.getModelMarket()
}

async function loadHistory() {
  const response = await imageCanvasAPI.listHistory(1, 200)
  nodes.value = (response.items || []).map(nodeFromHistory)
}

async function loadAll() {
  loading.value = true
  globalError.value = ''
  try {
    await Promise.all([loadKeys(), loadModels(), loadHistory()])
  } catch (error) {
    globalError.value = extractDisplayError(error)
  } finally {
    loading.value = false
  }
}

function extractDisplayError(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data as any
    return data?.error?.message || data?.message || error.message || '请求失败'
  }
  return extractApiErrorMessage(error, '请求失败，请稍后重试')
}

function openGenerateDialog(payload?: Partial<TaskPayload>) {
  taskDialog.show = true
  taskDialog.mode = 'generate'
  taskDialog.sourceNode = null
  taskDialog.error = ''
  taskForm.apiKeyId = payload?.apiKeyId || taskForm.apiKeyId || keys.value[0]?.id || 0
  taskForm.model = payload?.model || dialogImageModels.value[0]?.name || taskForm.model
  taskForm.prompt = payload?.prompt || ''
  taskForm.size = payload?.size || '1024x1024'
  taskForm.count = payload?.count || 1
}

function openEditDialog(node: CanvasNode) {
  if (!canUseImage(node)) return
  taskDialog.show = true
  taskDialog.mode = 'edit'
  taskDialog.sourceNode = node
  taskDialog.error = ''
  taskForm.apiKeyId = node.apiKeyId
  taskForm.model = dialogImageModels.value.some(model => model.name === node.model) ? node.model : (dialogImageModels.value[0]?.name || node.model)
  taskForm.prompt = `基于这张图片进行修改：${node.prompt}`
  taskForm.size = node.size || '1024x1024'
  taskForm.count = 1
}

function closeTaskDialog() {
  taskDialog.show = false
  taskDialog.error = ''
  taskDialog.sourceNode = null
}

function submitTaskDialog() {
  const payload = normalizePayload(taskForm)
  const key = keys.value.find(item => item.id === payload.apiKeyId)
  if (!key?.key) {
    taskDialog.error = '请选择一个可用 Key'
    return
  }
  if (!payload.model) {
    taskDialog.error = '当前 Key 所属分组没有模型广场可用的图片模型'
    return
  }
  if (!payload.prompt.trim()) {
    taskDialog.error = '请输入提示词'
    return
  }
  const sourceNode = taskDialog.sourceNode
  const mode = taskDialog.mode
  closeTaskDialog()
  if (mode === 'edit' && sourceNode) {
    for (let i = 0; i < payload.count; i++) createEditNode(sourceNode, payload, key.key)
  } else {
    for (let i = 0; i < payload.count; i++) createGenerateNode(payload, key)
  }
}

function normalizePayload(payload: TaskPayload): TaskPayload {
  return {
    apiKeyId: payload.apiKeyId,
    model: payload.model,
    prompt: payload.prompt.trim(),
    size: normalizeSize(payload.size),
    count: normalizeCount(payload.count)
  }
}

function createGenerateNode(payload: TaskPayload, key: ApiKey) {
  const localId = `task-${Date.now()}-${++localSeq}`
  const node: CanvasNode = {
    localId,
    rootId: localId,
    apiKeyId: key.id,
    apiKeyName: key.name,
    operation: 'generate',
    model: payload.model,
    prompt: payload.prompt,
    size: payload.size,
    mimeType: 'image/png',
    imageExpired: false,
    status: 'generating',
    error: '',
    createdAt: new Date().toISOString(),
    retryPayload: { ...payload, count: 1 }
  }
  nodes.value.unshift(node)
  void runGenerateTask(node, key.key)
}

function createEditNode(sourceNode: CanvasNode, payload: TaskPayload, apiKey: string) {
  const node: CanvasNode = {
    localId: `edit-${Date.now()}-${++localSeq}`,
    rootId: sourceNode.rootId || sourceNode.localId,
    parentId: sourceNode.localId,
    apiKeyId: payload.apiKeyId,
    apiKeyName: keys.value.find(key => key.id === payload.apiKeyId)?.name || sourceNode.apiKeyName,
    operation: 'edit',
    model: payload.model,
    prompt: payload.prompt,
    size: payload.size,
    mimeType: 'image/png',
    sourceImageUrl: nodeImageSrc(sourceNode),
    imageExpired: false,
    status: 'editing',
    error: '',
    createdAt: new Date().toISOString(),
    retryPayload: { ...payload, count: 1 }
  }
  const index = nodes.value.findIndex(item => item.localId === sourceNode.localId)
  nodes.value.splice(index >= 0 ? index + 1 : 0, 0, node)
  void runEditTask(node, sourceNode, apiKey)
}

async function runGenerateTask(node: CanvasNode, apiKey: string) {
  node.status = 'generating'
  node.error = ''
  try {
    const response = await imageCanvasAPI.generateImage(apiKey, {
      model: node.model,
      prompt: node.prompt,
      size: node.size,
      output_format: 'png',
      response_format: 'b64_json',
      n: 1
    })
    await applyImagesToNode(node, response, 'generate')
  } catch (error) {
    node.status = 'failed'
    node.error = extractDisplayError(error)
  }
}

async function runEditTask(node: CanvasNode, sourceNode: CanvasNode, apiKey: string) {
  node.status = 'editing'
  node.error = ''
  try {
    const blob = await nodeToBlob(sourceNode)
    const formData = new FormData()
    formData.append('model', node.model)
    formData.append('prompt', node.prompt)
    formData.append('size', node.size)
    formData.append('output_format', 'png')
    formData.append('response_format', 'b64_json')
    formData.append('image', blob, 'source.png')
    const response = await imageCanvasAPI.editImage(apiKey, formData)
    await applyImagesToNode(node, response, 'edit')
  } catch (error) {
    node.status = 'failed'
    node.error = extractDisplayError(error)
  }
}

async function applyImagesToNode(node: CanvasNode, response: OpenAIImagesResponse, operation: 'generate' | 'edit') {
  const image = response.data?.[0]
  if (!image) throw new Error('生图接口没有返回图片')
  const saved = await imageCanvasAPI.saveHistory({
    api_key_id: node.apiKeyId,
    operation,
    model: node.model,
    prompt: image.revised_prompt || node.prompt,
    size: node.size,
    output_format: 'png',
    image_url: image.url || '',
    b64_json: image.b64_json || '',
    mime_type: 'image/png',
    source_image_url: operation === 'edit' ? node.sourceImageUrl || '' : ''
  })
  Object.assign(node, {
    historyId: saved.id,
    apiKeyName: saved.api_key_name || node.apiKeyName,
    prompt: saved.prompt,
    imageUrl: saved.image_url,
    b64Json: saved.b64_json,
    mimeType: saved.mime_type || 'image/png',
    status: 'completed',
    error: '',
    createdAt: saved.created_at || node.createdAt
  })
}

async function nodeToBlob(node: CanvasNode): Promise<Blob> {
  if (node.b64Json) {
    const bytes = Uint8Array.from(atob(node.b64Json), char => char.charCodeAt(0))
    return new Blob([bytes], { type: 'image/png' })
  }
  if (!node.imageUrl) throw new Error('节点没有可编辑的图片内容')
  const response = await fetch(node.imageUrl)
  if (!response.ok) throw new Error('无法读取图片，请尝试下载后重新生成')
  return response.blob()
}

function retryNode(node: CanvasNode) {
  const key = keys.value.find(item => item.id === node.retryPayload.apiKeyId)
  if (!key?.key) {
    node.status = 'failed'
    node.error = '找不到此节点使用的 Key，请刷新后重试'
    return
  }
  if (node.operation === 'edit') {
    const sourceNode = nodes.value.find(item => item.localId === node.parentId)
    if (!sourceNode) {
      node.status = 'failed'
      node.error = '找不到原始图片节点，无法重试编辑'
      return
    }
    node.model = node.retryPayload.model
    node.prompt = node.retryPayload.prompt
    node.size = node.retryPayload.size
    void runEditTask(node, sourceNode, key.key)
    return
  }
  node.model = node.retryPayload.model
  node.prompt = node.retryPayload.prompt
  node.size = node.retryPayload.size
  void runGenerateTask(node, key.key)
}

function openPreview(node: CanvasNode) {
  if (canUseImage(node)) previewNode.value = node
}

function downloadOriginal(node: CanvasNode) {
  if (!canUseImage(node)) return
  const src = nodeImageSrc(node)
  if (!src) return
  const a = document.createElement('a')
  a.href = src
  a.download = `image-canvas-${node.historyId || node.localId}.png`
  document.body.appendChild(a)
  a.click()
  a.remove()
}

onMounted(loadAll)
</script>
