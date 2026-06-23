<template>
  <view class="shell-page">
    <view v-if="setupVisible" class="setup-page">
      <view class="setup-card">
        <text class="setup-eyebrow">{{ appName }} Mobile</text>
        <text class="setup-title">请输入 Sub2API 域名</text>
        <text class="setup-desc">
          移动端只作为 WebView 壳运行，真实后端仍部署在你的服务器上。请输入已部署好的 Sub2API 前端访问地址。
        </text>

        <view class="input-group">
          <text class="input-label">Sub2API 地址</text>
          <input
            v-model="urlInput"
            class="url-input"
            type="text"
            :placeholder="exampleAppUrl"
            confirm-type="go"
            @input="handleUrlInput"
            @confirm="saveAndOpen"
          />
          <text class="input-hint">支持直接输入域名，例如 sub2api.example.com；未写协议时默认补全 https://。</text>
        </view>

        <text v-if="urlError" class="error-text">{{ urlError }}</text>

        <button class="primary-button" type="primary" :loading="saving" :disabled="saving" @tap="saveAndOpen">
          {{ saving ? '正在打开...' : '保存并打开' }}
        </button>

        <button v-if="savedUrl" class="ghost-button" @tap="cancelSetup">
          取消，继续使用当前地址
        </button>
      </view>
    </view>

    <template v-else>
      <!-- #ifdef H5 -->
      <view class="h5-preview-page">
        <view class="setup-card">
          <text class="setup-eyebrow">H5 预览提示</text>
          <text class="setup-title">请用 App 运行验证 WebView</text>
          <text class="setup-desc">
            当前地址 {{ savedUrl }} 已保存。HBuilderX 内置浏览器的 web-view 会以 iframe 方式预览，若站点设置了 X-Frame-Options 或 frame-ancestors，浏览器会拒绝嵌入并显示“拒绝连接”。App 端原生 WebView 不走 iframe，请运行到 Android、iOS 或 HarmonyOS 验证。
          </text>
          <button class="primary-button" type="primary" @tap="openInBrowser">在浏览器打开当前地址</button>
          <button class="ghost-button" @tap="openSetup">重新输入域名</button>
        </view>
      </view>
      <!-- #endif -->

      <!-- #ifndef H5 -->
      <web-view
        id="sub2apiWebview"
        class="shell-webview"
        :src="webviewUrl"
        :fullscreen="true"
        :webview-styles="webviewStyles"
        :update-title="true"
        @load="handleLoad"
        @error="handleError"
        @message="handleMessage"
      />

      <view v-if="showLoading" class="shell-overlay shell-loading">
        <view class="spinner" />
        <text class="title">正在加载 {{ appName }}</text>
        <text class="subtitle">{{ webviewUrl }}</text>
      </view>

      <view v-if="loadError" class="shell-overlay shell-error">
        <text class="title">页面加载失败</text>
        <text class="subtitle">请检查网络连接，或点击“更换域名”重新输入 Sub2API 地址。</text>
        <view class="error-actions">
          <button class="retry-button" type="primary" @tap="reloadWebview">重新加载</button>
          <button class="retry-button" @tap="openSetup">更换域名</button>
        </view>
      </view>
      <!-- #endif -->
    </template>
  </view>
</template>

<script setup lang="ts">
import { computed, getCurrentInstance, onMounted, ref } from 'vue'
import { onBackPress, onNavigationBarButtonTap } from '@dcloudio/uni-app'
import {
  isValidSub2ApiUrl,
  mobileShellConfig,
  normalizeSub2ApiUrl,
} from '../../src/config'

const appName = mobileShellConfig.appName
const storageKey = mobileShellConfig.storageKey
const exampleAppUrl = mobileShellConfig.exampleAppUrl

const savedUrl = ref('')
const urlInput = ref('')
const urlError = ref('')
const setupVisible = ref(true)
const reloadVersion = ref(0)
const loaded = ref(false)
const loadError = ref(false)
const saving = ref(false)

const webviewUrl = computed(() => {
  if (!savedUrl.value) return ''
  if (reloadVersion.value === 0) return savedUrl.value
  const separator = savedUrl.value.includes('?') ? '&' : '?'
  return `${savedUrl.value}${separator}_mobile_reload=${reloadVersion.value}`
})

const showLoading = computed(() => Boolean(savedUrl.value) && !loaded.value && !loadError.value)

const webviewStyles = {
  progress: {
    color: '#2563eb'
  }
}

function handleUrlInput() {
  urlError.value = ''
}

function readSavedUrl(): string {
  try {
    return uni.getStorageSync(storageKey) || ''
  } catch (error) {
    console.warn('[mobile-shell] read saved url failed:', error)
    return ''
  }
}

function persistUrl(url: string) {
  try {
    uni.setStorageSync(storageKey, url)
  } catch (error) {
    console.warn('[mobile-shell] save url failed:', error)
  }
}

function saveAndOpen() {
  if (saving.value) return
  saving.value = true
  console.log('[mobile-shell] save button tapped:', urlInput.value)
  const normalized = normalizeSub2ApiUrl(urlInput.value)
  if (!normalized || !isValidSub2ApiUrl(normalized)) {
    saving.value = false
    urlError.value = '请输入有效的 http(s) 地址或域名'
    uni.showToast({ title: '域名格式不正确', icon: 'none' })
    return
  }

  savedUrl.value = normalized
  persistUrl(normalized)
  setupVisible.value = false
  loaded.value = false
  loadError.value = false
  reloadVersion.value += 1
  saving.value = false
  uni.showToast({ title: '已保存域名', icon: 'success' })
}

function openSetup() {
  urlInput.value = savedUrl.value || ''
  urlError.value = ''
  setupVisible.value = true
  loaded.value = false
  loadError.value = false
}

function cancelSetup() {
  if (!savedUrl.value) return
  setupVisible.value = false
  loaded.value = false
  loadError.value = false
  reloadVersion.value += 1
}

function handleLoad() {
  loaded.value = true
  loadError.value = false
}

function handleError(event: unknown) {
  console.warn('[mobile-shell] web-view load error:', event)
  loaded.value = false
  loadError.value = true
}

function handleMessage(event: unknown) {
  // Step 4 (payment/OAuth/external-link bridge) is intentionally left for later confirmation.
  console.log('[mobile-shell] web-view message:', event)
}

function reloadWebview() {
  loaded.value = false
  loadError.value = false
  reloadVersion.value += 1
}

function openInBrowser() {
  if (!savedUrl.value) return
  // #ifdef H5
  window.open(savedUrl.value, '_blank', 'noopener,noreferrer')
  // #endif
}

function getWebviewContext() {
  const instance = getCurrentInstance()
  return uni.createWebviewContext('sub2apiWebview', instance?.proxy)
}

onMounted(() => {
  const stored = readSavedUrl()
  if (stored && isValidSub2ApiUrl(stored)) {
    savedUrl.value = normalizeSub2ApiUrl(stored)
    urlInput.value = savedUrl.value
    setupVisible.value = false
  } else {
    urlInput.value = ''
    setupVisible.value = true
  }
})

onNavigationBarButtonTap(() => {
  openSetup()
})

onBackPress(() => {
  if (setupVisible.value) {
    if (savedUrl.value) {
      setupVisible.value = false
      return true
    }
    return false
  }

  try {
    const webview = getWebviewContext()
    webview?.back?.()
    return true
  } catch (error) {
    console.warn('[mobile-shell] web-view back failed:', error)
    return false
  }
})
</script>

<style scoped>
.shell-page {
  width: 100vw;
  height: 100vh;
  background: #f8fafc;
}

.shell-webview {
  width: 100vw;
  height: 100vh;
  background: #ffffff;
}

.setup-page,
.h5-preview-page {
  display: flex;
  min-height: 100vh;
  align-items: center;
  justify-content: center;
  padding: 24px;
  padding-top: calc(24px + env(safe-area-inset-top));
  padding-bottom: calc(24px + env(safe-area-inset-bottom));
  background: linear-gradient(180deg, #eff6ff 0%, #f8fafc 45%, #ffffff 100%);
}

.setup-card {
  width: 100%;
  max-width: 420px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 24px;
  border-radius: 20px;
  background: #ffffff;
  box-shadow: 0 18px 45px rgba(15, 23, 42, 0.12);
}

.setup-eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.setup-title {
  color: #0f172a;
  font-size: 24px;
  font-weight: 800;
}

.setup-desc {
  color: #64748b;
  font-size: 14px;
  line-height: 1.6;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 6px;
}

.input-label {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.url-input {
  height: 46px;
  padding: 0 14px;
  border: 1px solid #cbd5e1;
  border-radius: 12px;
  background: #ffffff;
  color: #0f172a;
  font-size: 16px;
}

.input-hint,
.error-text {
  font-size: 12px;
  line-height: 1.5;
}

.input-hint {
  color: #64748b;
}

.error-text {
  color: #dc2626;
}

.primary-button,
.ghost-button {
  width: 100%;
  margin: 0;
  border-radius: 12px;
}

.ghost-button {
  color: #2563eb;
  background: #eff6ff;
}

.shell-overlay {
  position: fixed;
  inset: 0;
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 24px;
  padding-top: calc(72px + env(safe-area-inset-top));
  background: rgba(255, 255, 255, 0.96);
  text-align: center;
}

.spinner {
  width: 28px;
  height: 28px;
  border: 3px solid #dbeafe;
  border-top-color: #2563eb;
  border-radius: 999px;
  animation: spin 0.8s linear infinite;
}

.title {
  color: #111827;
  font-size: 17px;
  font-weight: 600;
}

.subtitle {
  max-width: 80vw;
  color: #6b7280;
  font-size: 13px;
  line-height: 1.5;
  word-break: break-all;
}

.error-actions {
  display: flex;
  gap: 10px;
  margin-top: 8px;
}

.retry-button {
  margin: 0;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
