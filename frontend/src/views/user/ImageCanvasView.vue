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
                使用你自己的 API Key 和模型广场中的真实可用图片模型生成图片；每个生成/编辑节点独立执行，互不影响。
              </p>
            </div>
          </div>
          <button class="btn btn-secondary" :disabled="loading" @click="loadAll">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            刷新
          </button>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[380px_minmax(0,1fr)]">
        <aside class="card h-fit p-5">
          <div class="mb-5 flex items-center justify-between gap-3">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">新建生图任务</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">点击生成后会立即在画布中出现任务节点。</p>
            </div>
          </div>

          <div v-if="keys.length === 0" class="rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
            <p class="font-semibold">你还没有可用 Key</p>
            <p class="mt-1">先创建 API Key 后才能在画布中生图。</p>
            <button class="btn btn-primary mt-3" @click="router.push('/keys')">去创建 Key</button>
          </div>

          <div v-else class="space-y-4">
            <label class="block text-sm">
              <span class="mb-1.5 block font-medium text-gray-700 dark:text-gray-300">使用 Key</span>
              <select v-model="selectedKeyId" class="input">
                <option :value="0">请选择 Key</option>
                <option v-for="key in keys" :key="key.id" :value="key.id">{{ key.name }} · {{ key.group?.name || '未分组' }}</option>
              </select>
            </label>

            <label class="block text-sm">
              <span class="mb-1.5 block font-medium text-gray-700 dark:text-gray-300">模型广场可用图片模型</span>
              <select v-model="form.model" class="input" :disabled="availableImageModels.length === 0">
                <option v-if="availableImageModels.length === 0" value="">当前 Key 所属分组暂无图片模型</option>
                <option v-for="model in availableImageModels" :key="model.name" :value="model.name">
                  {{ model.name }} · {{ model.groupName }}
                </option>
              </select>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">模型只来自模型广场真实返回的数据，并按当前 Key 的分组过滤。</p>
            </label>

            <div class="grid grid-cols-2 gap-3">
              <label class="block text-sm">
                <span class="mb-1.5 block font-medium text-gray-700 dark:text-gray-300">尺寸</span>
                <select v-model="form.size" class="input">
                  <option value="1024x1024">1024x1024</option>
                  <option value="1024x1536">1024x1536</option>
                  <option value="1536x1024">1536x1024</option>
                  <option value="auto">auto</option>
                </select>
              </label>
              <label class="block text-sm">
                <span class="mb-1.5 block font-medium text-gray-700 dark:text-gray-300">格式</span>
                <select v-model="form.outputFormat" class="input">
                  <option value="png">png</option>
                  <option value="jpeg">jpeg</option>
                  <option value="webp">webp</option>
                </select>
              </label>
            </div>

            <label class="block text-sm">
              <span class="mb-1.5 block font-medium text-gray-700 dark:text-gray-300">提示词</span>
              <textarea v-model="form.prompt" rows="5" class="input resize-none" placeholder="描述你想生成的图片..."></textarea>
            </label>

            <button :disabled="!canCreateTask" class="btn btn-primary w-full justify-center" @click="createGenerateTask">
              <Icon name="sparkles" size="md" />
              生成图片
            </button>

            <div v-if="globalError" class="rounded-2xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200 whitespace-pre-wrap">{{ globalError }}</div>
          </div>
        </aside>

        <main class="card min-h-[720px] overflow-hidden p-0">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-5 py-4 dark:border-dark-700">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">画布节点</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">历史图片和当前任务都会显示在这里，每个节点可单独编辑。</p>
            </div>
            <div class="flex gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span class="rounded-full bg-gray-100 px-2.5 py-1 dark:bg-dark-800">历史 {{ historyCount }}</span>
              <span class="rounded-full bg-primary-50 px-2.5 py-1 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">任务 {{ runningCount }}</span>
            </div>
          </div>

          <div class="h-[calc(100vh-18rem)] min-h-[640px] overflow-auto bg-gray-50/80 p-5 dark:bg-dark-950/50">
            <div v-if="loading" class="grid gap-4 sm:grid-cols-2 2xl:grid-cols-3">
              <div v-for="i in 6" :key="i" class="h-96 animate-pulse rounded-3xl bg-white dark:bg-dark-900"></div>
            </div>
            <div v-else-if="nodes.length === 0" class="flex h-full min-h-[500px] flex-col items-center justify-center text-center">
              <Icon name="inbox" size="xl" class="mb-4 h-14 w-14 text-gray-400" />
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">画布还是空的</h3>
              <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">生成第一张图后，它会出现在这里。</p>
            </div>
            <div v-else class="grid auto-rows-max gap-4 sm:grid-cols-2 2xl:grid-cols-3">
              <article
                v-for="node in nodes"
                :key="node.localId"
                class="overflow-hidden rounded-3xl border bg-white shadow-sm transition hover:-translate-y-0.5 hover:shadow-md dark:border-dark-700 dark:bg-dark-900"
                :class="node.status === 'failed' ? 'border-red-200 dark:border-red-500/40' : 'border-gray-200'"
              >
                <div class="relative aspect-square bg-gray-100 dark:bg-dark-800">
                  <img v-if="nodeImageSrc(node)" :src="nodeImageSrc(node)" :alt="node.prompt" class="h-full w-full object-cover" loading="lazy" />
                  <div v-else class="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
                    <Icon :name="node.status === 'failed' ? 'exclamationCircle' : 'sparkles'" size="xl" :class="node.status === 'failed' ? 'text-red-400' : 'text-primary-400'" />
                    <div>
                      <p class="font-semibold text-gray-900 dark:text-white">{{ statusText(node) }}</p>
                      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ node.model }}</p>
                    </div>
                  </div>
                  <div class="absolute left-3 top-3 rounded-full px-2.5 py-1 text-xs font-semibold" :class="statusBadgeClass(node.status)">
                    {{ statusText(node) }}
                  </div>
                </div>

                <div class="space-y-3 p-4">
                  <div class="flex items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
                    <span>{{ node.operation === 'edit' ? '编辑' : '生成' }}</span>
                    <span>{{ node.outputFormat || 'png' }}</span>
                  </div>

                  <p class="line-clamp-3 text-sm text-gray-800 dark:text-gray-200">{{ node.prompt }}</p>

                  <div class="flex flex-wrap gap-2 text-[11px] text-gray-600 dark:text-gray-300">
                    <span class="rounded-full bg-gray-100 px-2 py-1 dark:bg-dark-800">{{ node.model }}</span>
                    <span v-if="node.size" class="rounded-full bg-gray-100 px-2 py-1 dark:bg-dark-800">{{ node.size }}</span>
                    <span class="rounded-full bg-gray-100 px-2 py-1 dark:bg-dark-800">{{ node.apiKeyName }}</span>
                  </div>

                  <div v-if="node.error" class="rounded-2xl border border-red-200 bg-red-50 p-3 text-xs text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200 whitespace-pre-wrap">
                    {{ node.error }}
                  </div>

                  <div v-if="node.editing" class="space-y-2 rounded-2xl border border-primary-200 bg-primary-50 p-3 dark:border-primary-500/30 dark:bg-primary-500/10">
                    <textarea v-model="node.editPrompt" rows="3" class="input resize-none text-sm" placeholder="描述如何修改这张图片..."></textarea>
                    <div class="grid grid-cols-2 gap-2">
                      <select v-model="node.editModel" class="input text-sm">
                        <option v-for="model in availableImageModels" :key="`edit-${node.localId}-${model.name}`" :value="model.name">{{ model.name }}</option>
                      </select>
                      <select v-model="node.editSize" class="input text-sm">
                        <option value="1024x1024">1024x1024</option>
                        <option value="1024x1536">1024x1536</option>
                        <option value="1536x1024">1536x1024</option>
                        <option value="auto">auto</option>
                      </select>
                    </div>
                    <div class="flex gap-2">
                      <button class="btn btn-primary flex-1 justify-center" :disabled="node.status === 'editing' || !node.editPrompt.trim()" @click="runEditTask(node)">
                        {{ node.status === 'editing' ? '编辑中...' : '提交编辑' }}
                      </button>
                      <button class="btn btn-secondary" :disabled="node.status === 'editing'" @click="node.editing = false">取消</button>
                    </div>
                  </div>

                  <div class="flex gap-2">
                    <button class="btn btn-secondary flex-1 justify-center" :disabled="!nodeImageSrc(node) || node.status === 'editing'" @click="toggleNodeEdit(node)">修改图片</button>
                    <button class="btn btn-secondary flex-1 justify-center" :disabled="!nodeImageSrc(node)" @click="downloadOriginal(node)">原格式下载</button>
                  </div>
                </div>
              </article>
            </div>
          </div>
        </main>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import AppLayout from '@/components/layout/AppLayout.vue'
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
  historyId?: number
  apiKeyId: number
  apiKeyName: string
  operation: 'generate' | 'edit'
  model: string
  prompt: string
  size: string
  outputFormat: string
  imageUrl?: string
  b64Json?: string
  mimeType: string
  sourceImageUrl?: string
  status: NodeStatus
  error: string
  createdAt: string
  editing: boolean
  editPrompt: string
  editModel: string
  editSize: string
}

const router = useRouter()
const keys = ref<ApiKey[]>([])
const channels = ref<UserAvailableChannel[]>([])
const nodes = ref<CanvasNode[]>([])
const loading = ref(false)
const globalError = ref('')
const selectedKeyId = ref(0)
let localSeq = 0

const form = reactive({
  model: '',
  size: '1024x1024',
  outputFormat: 'png',
  prompt: ''
})

const selectedKey = computed(() => keys.value.find(key => key.id === selectedKeyId.value) || null)

const allImageModels = computed<ImageModelOption[]>(() => {
  const result = new Map<string, ImageModelOption>()
  for (const channel of channels.value) {
    for (const section of channel.platforms || []) {
      for (const group of section.groups || []) {
        for (const model of section.supported_models || []) {
          const name = String(model.name || '').trim()
          if (!isImageModel(name, model.pricing?.billing_mode)) continue
          const platform = model.platform || group.platform || section.platform || ''
          const key = `${group.id}:${platform}:${name}`
          result.set(key, {
            name,
            groupId: group.id,
            groupName: group.name,
            platform
          })
        }
      }
    }
  }
  return Array.from(result.values()).sort((a, b) => a.groupName.localeCompare(b.groupName) || a.name.localeCompare(b.name))
})

const availableImageModels = computed(() => {
  const groupId = selectedKey.value?.group_id
  if (!groupId) return allImageModels.value
  return allImageModels.value.filter(model => model.groupId === groupId)
})

const canCreateTask = computed(() => Boolean(selectedKey.value?.key && form.model && form.prompt.trim()))
const runningCount = computed(() => nodes.value.filter(node => node.status === 'generating' || node.status === 'editing').length)
const historyCount = computed(() => nodes.value.filter(node => node.historyId).length)

watch(availableImageModels, models => {
  if (!models.some(model => model.name === form.model)) {
    form.model = models[0]?.name || ''
  }
}, { immediate: true })

watch(selectedKeyId, () => {
  if (!availableImageModels.value.some(model => model.name === form.model)) {
    form.model = availableImageModels.value[0]?.name || ''
  }
})

function isImageModel(name: string, billingMode?: string): boolean {
  const normalized = name.toLowerCase()
  if (!normalized) return false
  if (billingMode === 'image') return true
  return /(^gpt-image-|^dall-e-|image|imagine|imagen|flux|ideogram|stable-diffusion|sdxl)/i.test(normalized)
}

function nodeImageSrc(node: CanvasNode): string {
  if (node.b64Json) return `data:${node.mimeType || mimeFromFormat(node.outputFormat)};base64,${node.b64Json}`
  return node.imageUrl || ''
}

function mimeFromFormat(format?: string): string {
  const normalized = (format || 'png').toLowerCase()
  if (normalized === 'jpg') return 'image/jpeg'
  return `image/${normalized}`
}

function nodeFromHistory(item: ImageCanvasHistoryItem): CanvasNode {
  return {
    localId: `history-${item.id}`,
    historyId: item.id,
    apiKeyId: item.api_key_id,
    apiKeyName: item.api_key_name,
    operation: item.operation,
    model: item.model,
    prompt: item.prompt,
    size: item.size,
    outputFormat: item.output_format || 'png',
    imageUrl: item.image_url,
    b64Json: item.b64_json,
    mimeType: item.mime_type || mimeFromFormat(item.output_format),
    sourceImageUrl: item.source_image_url,
    status: 'completed',
    error: '',
    createdAt: item.created_at,
    editing: false,
    editPrompt: '',
    editModel: item.model,
    editSize: item.size || '1024x1024'
  }
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

async function loadKeys() {
  const response = await keysAPI.list(1, 100, { status: 'active' })
  keys.value = response.items.filter(key => key.status === 'active' && key.key)
  if (!selectedKeyId.value && keys.value.length > 0) selectedKeyId.value = keys.value[0].id
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

function createGenerateTask() {
  const key = selectedKey.value
  if (!key?.key) {
    globalError.value = '请选择一个可用 Key'
    return
  }
  if (!form.model) {
    globalError.value = '当前 Key 所属分组没有模型广场可用的图片模型'
    return
  }
  const prompt = form.prompt.trim()
  if (!prompt) {
    globalError.value = '请输入提示词'
    return
  }
  globalError.value = ''
  const node: CanvasNode = {
    localId: `task-${Date.now()}-${++localSeq}`,
    apiKeyId: key.id,
    apiKeyName: key.name,
    operation: 'generate',
    model: form.model,
    prompt,
    size: form.size,
    outputFormat: form.outputFormat || 'png',
    mimeType: mimeFromFormat(form.outputFormat),
    status: 'generating',
    error: '',
    createdAt: new Date().toISOString(),
    editing: false,
    editPrompt: '',
    editModel: form.model,
    editSize: form.size
  }
  nodes.value.unshift(node)
  form.prompt = ''
  void runGenerateTask(node, key.key)
}

async function runGenerateTask(node: CanvasNode, apiKey: string) {
  try {
    const response = await imageCanvasAPI.generateImage(apiKey, {
      model: node.model,
      prompt: node.prompt,
      size: node.size,
      output_format: node.outputFormat || 'png',
      response_format: 'b64_json',
      n: 1
    })
    await applyImagesToNode(node, response, 'generate')
  } catch (error) {
    node.status = 'failed'
    node.error = extractDisplayError(error)
  }
}

async function runEditTask(node: CanvasNode) {
  const key = keys.value.find(item => item.id === node.apiKeyId)
  if (!key?.key) {
    node.error = '找不到此节点使用的 Key，请刷新后重试'
    node.status = 'failed'
    return
  }
  const prompt = node.editPrompt.trim()
  if (!prompt) return
  node.status = 'editing'
  node.error = ''
  try {
    const blob = await nodeToBlob(node)
    const formData = new FormData()
    formData.append('model', node.editModel || node.model)
    formData.append('prompt', prompt)
    formData.append('size', node.editSize || node.size)
    formData.append('output_format', node.outputFormat || 'png')
    formData.append('response_format', 'b64_json')
    formData.append('image', blob, `source.${node.outputFormat || 'png'}`)
    const response = await imageCanvasAPI.editImage(key.key, formData)
    await applyImagesToNode(node, response, 'edit', prompt)
    node.editing = false
    node.editPrompt = ''
  } catch (error) {
    node.status = nodeImageSrc(node) ? 'completed' : 'failed'
    node.error = extractDisplayError(error)
  }
}

async function applyImagesToNode(node: CanvasNode, response: OpenAIImagesResponse, operation: 'generate' | 'edit', editPrompt?: string) {
  const image = response.data?.[0]
  if (!image) throw new Error('生图接口没有返回图片')
  const prompt = image.revised_prompt || editPrompt || node.prompt
  const saved = await imageCanvasAPI.saveHistory({
    api_key_id: node.apiKeyId,
    operation,
    model: operation === 'edit' ? (node.editModel || node.model) : node.model,
    prompt,
    size: operation === 'edit' ? (node.editSize || node.size) : node.size,
    output_format: node.outputFormat || response.output_format || 'png',
    image_url: image.url || '',
    b64_json: image.b64_json || '',
    mime_type: mimeFromFormat(node.outputFormat),
    source_image_url: operation === 'edit' ? nodeImageSrc(node) : ''
  })
  const next = nodeFromHistory(saved)
  Object.assign(node, next, { localId: node.localId })
}

async function nodeToBlob(node: CanvasNode): Promise<Blob> {
  if (node.b64Json) {
    const bytes = Uint8Array.from(atob(node.b64Json), char => char.charCodeAt(0))
    return new Blob([bytes], { type: node.mimeType || mimeFromFormat(node.outputFormat) })
  }
  if (!node.imageUrl) throw new Error('节点没有可编辑的图片内容')
  const response = await fetch(node.imageUrl)
  if (!response.ok) throw new Error('无法读取图片，请尝试下载后重新生成')
  return response.blob()
}

function toggleNodeEdit(node: CanvasNode) {
  node.editing = !node.editing
  if (node.editing) {
    node.editPrompt = node.editPrompt || `基于这张图片进行修改：${node.prompt}`
    node.editModel = availableImageModels.value.some(model => model.name === node.model) ? node.model : (availableImageModels.value[0]?.name || node.model)
    node.editSize = node.size || '1024x1024'
    node.error = ''
  }
}

function downloadOriginal(node: CanvasNode) {
  const src = nodeImageSrc(node)
  if (!src) return
  const a = document.createElement('a')
  a.href = src
  a.download = `image-canvas-${node.historyId || node.localId}.${node.outputFormat || 'png'}`
  document.body.appendChild(a)
  a.click()
  a.remove()
}

onMounted(loadAll)
</script>
