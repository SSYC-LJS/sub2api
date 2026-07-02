<template>
  <div class="relative min-h-[calc(100vh-7rem)] overflow-hidden rounded-3xl border border-gray-200 bg-slate-950 text-white shadow-sm dark:border-dark-700">
    <div class="absolute inset-0 bg-[radial-gradient(circle_at_1px_1px,rgba(148,163,184,0.26)_1px,transparent_0)] [background-size:28px_28px]"></div>

    <aside class="absolute left-4 top-4 z-20 w-[380px] max-w-[calc(100vw-2rem)] rounded-3xl border border-white/10 bg-slate-900/90 p-5 shadow-2xl backdrop-blur">
      <div class="mb-4 flex items-start justify-between gap-3">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.28em] text-cyan-300">Image Canvas</p>
          <h1 class="mt-1 text-2xl font-bold">生图无限画布</h1>
          <p class="mt-1 text-sm text-slate-300">选择自己的 Key 和模型生成图片，成功结果会自动进入画布历史。</p>
        </div>
        <button class="rounded-xl border border-white/10 px-3 py-2 text-sm text-slate-200 hover:bg-white/10" @click="loadAll">刷新</button>
      </div>

      <div v-if="keys.length === 0" class="mb-4 rounded-2xl border border-amber-400/30 bg-amber-500/10 p-4 text-sm text-amber-100">
        <p class="font-semibold">你还没有可用 Key</p>
        <p class="mt-1 text-amber-100/80">先创建 API Key 后才能在画布中生图。</p>
        <button class="mt-3 rounded-xl bg-amber-400 px-4 py-2 font-semibold text-slate-950 hover:bg-amber-300" @click="router.push('/keys')">去创建 Key</button>
      </div>

      <div class="space-y-3">
        <label class="block text-sm">
          <span class="mb-1 block text-slate-300">使用 Key</span>
          <select v-model="selectedKeyId" class="w-full rounded-xl border border-white/10 bg-slate-950 px-3 py-2 text-white outline-none focus:border-cyan-400">
            <option :value="0">请选择 Key</option>
            <option v-for="key in keys" :key="key.id" :value="key.id">{{ key.name }} · {{ key.group?.name || '未分组' }}</option>
          </select>
        </label>

        <label class="block text-sm">
          <span class="mb-1 block text-slate-300">生图模型</span>
          <select v-model="form.model" class="w-full rounded-xl border border-white/10 bg-slate-950 px-3 py-2 text-white outline-none focus:border-cyan-400">
            <option v-for="model in imageModels" :key="model" :value="model">{{ model }}</option>
          </select>
        </label>

        <div class="grid grid-cols-2 gap-3">
          <label class="block text-sm">
            <span class="mb-1 block text-slate-300">尺寸</span>
            <select v-model="form.size" class="w-full rounded-xl border border-white/10 bg-slate-950 px-3 py-2 text-white outline-none focus:border-cyan-400">
              <option value="1024x1024">1024x1024</option>
              <option value="1024x1536">1024x1536</option>
              <option value="1536x1024">1536x1024</option>
              <option value="auto">auto</option>
            </select>
          </label>
          <label class="block text-sm">
            <span class="mb-1 block text-slate-300">格式</span>
            <select v-model="form.outputFormat" class="w-full rounded-xl border border-white/10 bg-slate-950 px-3 py-2 text-white outline-none focus:border-cyan-400">
              <option value="png">png</option>
              <option value="jpeg">jpeg</option>
              <option value="webp">webp</option>
            </select>
          </label>
        </div>

        <label class="block text-sm">
          <span class="mb-1 block text-slate-300">提示词</span>
          <textarea v-model="form.prompt" rows="5" class="w-full resize-none rounded-xl border border-white/10 bg-slate-950 px-3 py-2 text-white outline-none focus:border-cyan-400" placeholder="描述你想生成的图片..."></textarea>
        </label>

        <div v-if="editingItem" class="rounded-2xl border border-cyan-400/30 bg-cyan-500/10 p-3 text-sm text-cyan-100">
          <div class="flex items-center justify-between gap-2">
            <span>正在编辑历史图片 #{{ editingItem.id }}</span>
            <button class="text-cyan-200 underline" @click="cancelEdit">取消</button>
          </div>
        </div>

        <button :disabled="submitting || !canSubmit" class="w-full rounded-2xl bg-cyan-400 px-4 py-3 font-bold text-slate-950 transition hover:bg-cyan-300 disabled:cursor-not-allowed disabled:opacity-50" @click="submitImage">
          {{ submitting ? '处理中...' : editingItem ? '修改图片' : '生成图片' }}
        </button>

        <div v-if="errorMessage" class="rounded-2xl border border-red-400/30 bg-red-500/10 p-3 text-sm text-red-100 whitespace-pre-wrap">{{ errorMessage }}</div>
      </div>
    </aside>

    <main class="relative z-10 h-[calc(100vh-7rem)] overflow-auto pl-[420px] pr-8 pt-8 max-lg:pl-8 max-lg:pt-[650px]">
      <div class="relative min-h-[1400px] min-w-[1500px] pb-16">
        <div v-if="loading" class="absolute left-1/2 top-40 -translate-x-1/2 rounded-2xl border border-white/10 bg-white/10 px-5 py-3 text-slate-200">正在加载历史...</div>
        <div v-else-if="historyItems.length === 0" class="absolute left-1/2 top-40 w-96 -translate-x-1/2 rounded-3xl border border-white/10 bg-white/10 p-8 text-center text-slate-200 backdrop-blur">
          <p class="text-lg font-semibold">画布还是空的</p>
          <p class="mt-2 text-sm text-slate-300">生成第一张图后，它会出现在这里。</p>
        </div>

        <article
          v-for="(item, index) in historyItems"
          :key="item.id"
          class="absolute w-72 overflow-hidden rounded-3xl border border-white/10 bg-slate-900/90 shadow-2xl backdrop-blur transition hover:-translate-y-1 hover:border-cyan-300/50"
          :style="cardStyle(index)"
        >
          <div class="aspect-square bg-slate-800">
            <img :src="imageSrc(item)" :alt="item.prompt" class="h-full w-full object-cover" loading="lazy" />
          </div>
          <div class="space-y-3 p-4">
            <div class="flex items-center justify-between gap-2 text-xs text-slate-400">
              <span>{{ item.operation === 'edit' ? '编辑' : '生成' }}</span>
              <span>{{ item.output_format || 'png' }}</span>
            </div>
            <p class="line-clamp-3 text-sm text-slate-100">{{ item.prompt }}</p>
            <div class="flex flex-wrap gap-2 text-[11px] text-slate-300">
              <span class="rounded-full bg-white/10 px-2 py-1">{{ item.model }}</span>
              <span v-if="item.size" class="rounded-full bg-white/10 px-2 py-1">{{ item.size }}</span>
              <span class="rounded-full bg-white/10 px-2 py-1">{{ item.api_key_name }}</span>
            </div>
            <div class="flex gap-2">
              <button class="flex-1 rounded-xl border border-white/10 px-3 py-2 text-sm hover:bg-white/10" @click="startEdit(item)">修改图片</button>
              <button class="flex-1 rounded-xl bg-white px-3 py-2 text-sm font-semibold text-slate-950 hover:bg-cyan-100" @click="downloadOriginal(item)">原格式下载</button>
            </div>
          </div>
        </article>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import { apiClient } from '@/api/client'
import { keysAPI } from '@/api/keys'
import { imageCanvasAPI, type ImageCanvasHistoryItem, type OpenAIImagesResponse } from '@/api/imageCanvas'
import type { ApiKey } from '@/types'

const router = useRouter()
const keys = ref<ApiKey[]>([])
const imageModels = ref<string[]>(['gpt-image-2', 'gpt-image-1'])
const selectedKeyId = ref(0)
const historyItems = ref<ImageCanvasHistoryItem[]>([])
const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const editingItem = ref<ImageCanvasHistoryItem | null>(null)

const form = reactive({
  model: 'gpt-image-2',
  size: '1024x1024',
  outputFormat: 'png',
  prompt: ''
})

const selectedKey = computed(() => keys.value.find(key => key.id === selectedKeyId.value) || null)
const canSubmit = computed(() => Boolean(selectedKey.value?.key && form.model && form.prompt.trim()))

function imageSrc(item: ImageCanvasHistoryItem): string {
  if (item.b64_json) return `data:${item.mime_type || mimeFromFormat(item.output_format)};base64,${item.b64_json}`
  return item.image_url || ''
}

function mimeFromFormat(format?: string): string {
  const normalized = (format || 'png').toLowerCase()
  if (normalized === 'jpg') return 'image/jpeg'
  return `image/${normalized}`
}

function cardStyle(index: number): Record<string, string> {
  const col = index % 4
  const row = Math.floor(index / 4)
  const offsetY = col % 2 === 0 ? 0 : 120
  return {
    left: `${col * 340 + 40}px`,
    top: `${row * 520 + offsetY + 40}px`
  }
}

function extractError(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data as any
    return data?.error?.message || data?.message || error.message || '请求失败'
  }
  if (typeof error === 'object' && error && 'message' in error) return String((error as any).message)
  return '请求失败，请稍后重试'
}

async function loadKeys() {
  const response = await keysAPI.list(1, 100, { status: 'active' })
  keys.value = response.items.filter(key => key.status === 'active' && key.key)
  if (!selectedKeyId.value && keys.value.length > 0) selectedKeyId.value = keys.value[0].id
}

async function loadModels() {
  try {
    const { data } = await apiClient.get<any[]>('/model-market')
    const names = new Set<string>()
    for (const channel of data || []) {
      for (const section of channel.platform_sections || []) {
        for (const model of section.supported_models || []) {
          const name = String(model.name || '').trim()
          if (/^(gpt-image-|dall-e-)/i.test(name)) names.add(name)
        }
      }
      for (const model of channel.supported_models || []) {
        const name = String(model.name || '').trim()
        if (/^(gpt-image-|dall-e-)/i.test(name)) names.add(name)
      }
    }
    const models = Array.from(names)
    if (models.length > 0) {
      imageModels.value = models
      if (!models.includes(form.model)) form.model = models[0]
    }
  } catch {
    // 保留默认模型候选
  }
}

async function loadHistory() {
  loading.value = true
  try {
    const response = await imageCanvasAPI.listHistory(1, 200)
    historyItems.value = response.items || []
  } finally {
    loading.value = false
  }
}

async function loadAll() {
  errorMessage.value = ''
  await Promise.all([loadKeys(), loadModels(), loadHistory()])
}

async function submitImage() {
  if (!selectedKey.value) {
    errorMessage.value = '请选择一个可用 Key'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    const response = editingItem.value ? await submitEdit(selectedKey.value.key) : await submitGenerate(selectedKey.value.key)
    await persistResponse(response)
    cancelEdit()
    form.prompt = ''
    await loadHistory()
  } catch (error) {
    errorMessage.value = extractError(error)
  } finally {
    submitting.value = false
  }
}

async function submitGenerate(apiKey: string): Promise<OpenAIImagesResponse> {
  return imageCanvasAPI.generateImage(apiKey, {
    model: form.model,
    prompt: form.prompt.trim(),
    size: form.size,
    output_format: form.outputFormat || 'png',
    response_format: 'b64_json',
    n: 1
  })
}

async function submitEdit(apiKey: string): Promise<OpenAIImagesResponse> {
  if (!editingItem.value) throw new Error('请选择要编辑的图片')
  const blob = await historyItemToBlob(editingItem.value)
  const formData = new FormData()
  formData.append('model', form.model)
  formData.append('prompt', form.prompt.trim())
  formData.append('size', form.size)
  formData.append('output_format', form.outputFormat || 'png')
  formData.append('response_format', 'b64_json')
  formData.append('image', blob, `source.${editingItem.value.output_format || 'png'}`)
  return imageCanvasAPI.editImage(apiKey, formData)
}

async function historyItemToBlob(item: ImageCanvasHistoryItem): Promise<Blob> {
  if (item.b64_json) {
    const bytes = Uint8Array.from(atob(item.b64_json), char => char.charCodeAt(0))
    return new Blob([bytes], { type: item.mime_type || mimeFromFormat(item.output_format) })
  }
  if (!item.image_url) throw new Error('历史图片没有可编辑的图片内容')
  const response = await fetch(item.image_url)
  if (!response.ok) throw new Error('无法读取历史图片，请尝试下载后重新上传')
  return response.blob()
}

async function persistResponse(response: OpenAIImagesResponse) {
  const images = response.data || []
  if (images.length === 0) throw new Error('生图接口没有返回图片')
  for (const image of images) {
    await imageCanvasAPI.saveHistory({
      api_key_id: selectedKeyId.value,
      operation: editingItem.value ? 'edit' : 'generate',
      model: form.model,
      prompt: image.revised_prompt || form.prompt.trim(),
      size: form.size,
      output_format: form.outputFormat || response.output_format || 'png',
      image_url: image.url || '',
      b64_json: image.b64_json || '',
      mime_type: mimeFromFormat(form.outputFormat),
      source_image_url: editingItem.value ? imageSrc(editingItem.value) : ''
    })
  }
}

function startEdit(item: ImageCanvasHistoryItem) {
  editingItem.value = item
  form.prompt = `基于这张图片进行修改：${item.prompt}`
  form.model = item.model || form.model
  form.size = item.size || form.size
  form.outputFormat = item.output_format || 'png'
  errorMessage.value = ''
}

function cancelEdit() {
  editingItem.value = null
}

function downloadOriginal(item: ImageCanvasHistoryItem) {
  const a = document.createElement('a')
  a.href = imageSrc(item)
  a.download = `image-canvas-${item.id}.${item.output_format || 'png'}`
  document.body.appendChild(a)
  a.click()
  a.remove()
}

onMounted(loadAll)
</script>
