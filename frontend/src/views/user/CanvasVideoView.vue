<template>
  <CanvasWorkspaceNav>
    <div class="canvas-video-page">
      <main id="canvas-workspace-content" class="canvas-video" tabindex="-1">
        <header class="canvas-video__heading">
          <div>
            <span class="canvas-video__eyebrow">
              <Icon name="play" size="sm" aria-hidden="true" />
              {{ t('canvas.video.eyebrow') }}
            </span>
            <h1>{{ t('canvas.video.title') }}</h1>
            <p>{{ t('canvas.video.description') }}</p>
          </div>
          <RouterLink class="canvas-video__back" :to="{ name: 'CanvasHome' }">
            <Icon name="arrowLeft" size="sm" aria-hidden="true" />
            {{ t('canvas.video.backToWorkspace') }}
          </RouterLink>
        </header>

        <div class="canvas-video__workbench">
          <form class="canvas-video__form" :aria-busy="isWorking || undefined" @submit.prevent="handleGenerationSubmit">
            <div class="canvas-video__form-heading">
              <div>
                <h2>{{ t('canvas.video.createTitle') }}</h2>
                <p>{{ t('canvas.video.createDescription') }}</p>
              </div>
              <span>{{ t('canvas.video.sessionOnly') }}</span>
            </div>

            <div class="canvas-video__field">
              <label for="canvas-video-key">{{ t('canvas.video.apiKey') }}</label>
              <select id="canvas-video-key" v-model.number="selectedKeyId" :disabled="loadingKeys || isWorking || Boolean(requestId && canResumeTask)">
                <option :value="0">{{ loadingKeys ? t('canvas.video.loadingKeys') : t('canvas.video.selectKey') }}</option>
                <option v-for="key in videoKeys" :key="key.id" :value="key.id">
                  {{ key.name }} · {{ key.group?.name || 'Grok' }}
                </option>
              </select>
              <p v-if="keysError" class="canvas-video__field-error" role="alert">{{ keysError }}</p>
              <p v-else-if="!loadingKeys && !videoKeys.length" class="canvas-video__field-hint is-warning">
                {{ t('canvas.video.noKeys') }}
                <RouterLink :to="{ name: 'Keys' }">{{ t('canvas.video.manageKeys') }}</RouterLink>
              </p>
              <p v-else class="canvas-video__field-hint">{{ t('canvas.video.apiKeyHelp') }}</p>
            </div>

            <div class="canvas-video__field">
              <label for="canvas-video-prompt">{{ t('canvas.video.prompt') }}</label>
              <textarea
                id="canvas-video-prompt"
                v-model="prompt"
                rows="7"
                maxlength="10000"
                :placeholder="t('canvas.video.promptPlaceholder')"
                :disabled="isWorking"
              ></textarea>
              <span class="canvas-video__counter">{{ prompt.length }} / 10000</span>
            </div>

            <div class="canvas-video__options">
              <div class="canvas-video__field">
                <label for="canvas-video-model">{{ t('canvas.video.model') }}</label>
                <select id="canvas-video-model" v-model="model" :disabled="isWorking">
                  <option v-for="option in videoModelOptions" :key="option" :value="option">{{ option }}</option>
                </select>
              </div>
              <div class="canvas-video__field">
                <label for="canvas-video-duration">{{ t('canvas.video.duration') }}</label>
                <select id="canvas-video-duration" v-model.number="duration" :disabled="isWorking">
                  <option :value="6">{{ t('canvas.video.seconds', { count: 6 }) }}</option>
                  <option :value="8">{{ t('canvas.video.seconds', { count: 8 }) }}</option>
                  <option :value="10">{{ t('canvas.video.seconds', { count: 10 }) }}</option>
                </select>
              </div>
              <div class="canvas-video__field">
                <label for="canvas-video-ratio">{{ t('canvas.video.aspectRatio') }}</label>
                <select id="canvas-video-ratio" v-model="aspectRatio" :disabled="isWorking">
                  <option value="16:9">16:9</option>
                  <option value="9:16">9:16</option>
                  <option value="1:1">1:1</option>
                </select>
              </div>
              <div class="canvas-video__field">
                <label for="canvas-video-resolution">{{ t('canvas.video.resolution') }}</label>
                <select id="canvas-video-resolution" v-model="resolution" :disabled="isWorking || !availableResolutions.length">
                  <option v-for="option in availableResolutions" :key="option" :value="option">{{ option }}</option>
                </select>
                <p v-if="!availableResolutions.length" class="canvas-video__field-error" role="alert">{{ t('canvas.video.noPricedResolution') }}</p>
              </div>
            </div>

            <p class="canvas-video__billing-note">
              <Icon name="infoCircle" size="sm" aria-hidden="true" />
              {{ t('canvas.video.billingNote') }}
            </p>

            <div class="canvas-video__form-actions">
              <button
                v-if="isWorking"
                type="button"
                class="canvas-video__secondary-button"
                data-testid="canvas-video-cancel"
                @click="cancelGeneration"
              >
                <Icon name="x" size="sm" aria-hidden="true" />
                {{ t('canvas.video.cancel') }}
              </button>
              <button
                type="submit"
                class="canvas-video__primary-button"
                data-testid="canvas-video-submit"
                :disabled="!canSubmit"
              >
                <Icon :name="isWorking ? 'refresh' : 'play'" size="sm" :class="{ 'is-spinning': isWorking }" aria-hidden="true" />
                {{ isWorking ? t('canvas.video.processing') : canResumeTask ? t('canvas.video.resume') : t('canvas.video.generate') }}
              </button>
            </div>
          </form>

          <section class="canvas-video__result" aria-labelledby="canvas-video-result-title">
            <div class="canvas-video__result-heading">
              <div>
                <h2 id="canvas-video-result-title">{{ t('canvas.video.resultTitle') }}</h2>
                <p>{{ t('canvas.video.resultDescription') }}</p>
              </div>
              <span v-if="phase !== 'idle'" :class="`is-${phase}`">{{ phaseLabel }}</span>
            </div>

            <div class="canvas-video__stage" aria-live="polite">
              <video v-if="videoURL" :src="videoURL" controls playsinline preload="metadata"></video>
              <div v-else-if="isWorking" class="canvas-video__progress-state" role="status">
                <span class="canvas-video__progress-icon" aria-hidden="true">
                  <Icon name="play" size="lg" />
                </span>
                <h3>{{ phaseLabel }}</h3>
                <p>{{ requestId ? t('canvas.video.waitingWithId', { id: requestId }) : t('canvas.video.submitting') }}</p>
                <progress v-if="progress !== null" :value="progress" max="100">{{ progress }}%</progress>
              </div>
              <div v-else-if="errorMessage" class="canvas-video__error-state" role="alert">
                <span aria-hidden="true"><Icon name="exclamationCircle" size="lg" /></span>
                <h3>{{ phase === 'canceled' ? t('canvas.video.stoppedTitle') : t('canvas.video.failedTitle') }}</h3>
                <p>{{ errorMessage }}</p>
                <div class="canvas-video__error-actions">
                  <button v-if="canResumeTask && requestId" type="button" @click="resumeTask">{{ t('canvas.video.resume') }}</button>
                  <button type="button" @click="resetResult">{{ canResumeTask ? t('canvas.video.newGeneration') : t('canvas.video.retry') }}</button>
                </div>
              </div>
              <div v-else class="canvas-video__empty-state">
                <span aria-hidden="true"><Icon name="play" size="lg" /></span>
                <h3>{{ t('canvas.video.emptyTitle') }}</h3>
                <p>{{ t('canvas.video.emptyDescription') }}</p>
              </div>
            </div>

            <div v-if="videoURL" class="canvas-video__result-actions">
              <span v-if="requestId">{{ t('canvas.video.requestId') }} <code>{{ requestId }}</code></span>
              <button type="button" data-testid="canvas-video-download" @click="downloadVideo">
                <Icon name="download" size="sm" aria-hidden="true" />
                {{ t('canvas.video.download') }}
              </button>
            </div>
          </section>
        </div>
      </main>
    </div>
  </CanvasWorkspaceNav>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import CanvasWorkspaceNav from '@/components/canvas/CanvasWorkspaceNav.vue'
import Icon from '@/components/icons/Icon.vue'
import { keysAPI } from '@/api/keys'
import { videoAPI, type VideoGenerationTask } from '@/api/video'
import type { ApiKey } from '@/types'
import { retryTaskPollingRequest } from '@/utils/taskPolling'

type VideoPhase = 'idle' | 'submitting' | 'pending' | 'downloading' | 'completed' | 'failed' | 'canceled'
type VideoTaskSession = { keyId: number; requestId: string; progress: number | null }

const videoTaskSessionKey = 'modurelay:canvas-video:task'
const defaultVideoModels = ['grok-imagine-video-1.5', 'grok-imagine-video']
const videoResolutionOrder = ['480p', '720p', '1080p'] as const

const { t } = useI18n()
const apiKeys = ref<ApiKey[]>([])
const selectedKeyId = ref(0)
const loadingKeys = ref(false)
const keysError = ref('')
const prompt = ref('')
const model = ref('grok-imagine-video-1.5')
const duration = ref(6)
const aspectRatio = ref('16:9')
const resolution = ref<(typeof videoResolutionOrder)[number]>('480p')
const requestId = ref('')
const phase = ref<VideoPhase>('idle')
const progress = ref<number | null>(null)
const errorMessage = ref('')
const canResumeTask = ref(false)
const videoURL = ref('')
let pollTimer: ReturnType<typeof setTimeout> | null = null
let activeController: AbortController | null = null
let restoredTaskSession: VideoTaskSession | null = null

const videoKeys = computed(() => apiKeys.value.filter(key => (
  key.status === 'active'
  && key.group?.status === 'active'
  && key.group.allow_image_generation === true
  && (key.group.platform === 'grok' || Object.keys(key.group.video_model_prices || {}).length > 0)
)))
const selectedKey = computed(() => videoKeys.value.find(key => key.id === selectedKeyId.value) || null)
const isWorking = computed(() => ['submitting', 'pending', 'downloading'].includes(phase.value))
const videoModelOptions = computed(() => {
  const prices = selectedKey.value?.group?.video_model_prices || {}
  const configured = Object.entries(prices)
    .filter(([id, tiers]) => !grokVideoFamily(id) && pricedResolutions(tiers).length > 0)
    .map(([id]) => id)
  return [...defaultVideoModels, ...configured.filter(id => !defaultVideoModels.includes(id))]
})
const availableResolutions = computed(() => {
  const group = selectedKey.value?.group
  const prices = group?.video_model_prices || {}
  const entry = matchingVideoPriceEntry(prices, model.value)
  const modelTiers = entry ? pricedResolutions(entry) : []
  if (modelTiers.length) return modelTiers
  if (isSeedanceModel(model.value)) return []

  const flatTiers = videoResolutionOrder.filter(tier => {
    const field = `video_price_${tier}` as 'video_price_480p' | 'video_price_720p' | 'video_price_1080p'
    return typeof group?.[field] === 'number' && Number.isFinite(group[field])
  })
  if (flatTiers.length) return flatTiers
  return grokVideoFamily(model.value) ? [...videoResolutionOrder] : []
})
const canSubmit = computed(() => Boolean(
  selectedKey.value
  && !isWorking.value
  && (canResumeTask.value && requestId.value || prompt.value.trim() && availableResolutions.value.includes(resolution.value)),
))
const phaseLabel = computed(() => t(`canvas.video.phase.${phase.value}`))

watch([selectedKeyId, model], () => {
  if (!availableResolutions.value.includes(resolution.value)) {
    resolution.value = availableResolutions.value[0] || '480p'
  }
})

function normalizedVideoModel(modelId: string) {
  const segments = modelId.trim().toLowerCase().split(':')
  return segments.length > 1 ? segments[segments.length - 1].replace(/^(xai|x-ai|grok)\//, '') : segments[0].replace(/^(xai|x-ai|grok)\//, '')
}

function grokVideoFamily(modelId: string) {
  const modelIdLower = normalizedVideoModel(modelId)
  if (modelIdLower.includes('grok-imagine-video') || modelIdLower.includes('grok-video')) {
    return modelIdLower.includes('1.5') ? 'grok-imagine-video-1.5' : 'grok-imagine-video'
  }
  return ''
}

function isSeedanceModel(modelId: string) {
  return normalizedVideoModel(modelId).startsWith('seedance-')
}

function pricedResolutions(tiers: Record<string, number>) {
  return videoResolutionOrder.filter(tier => {
    const price = tiers[tier] ?? tiers[tier.toUpperCase()]
    return typeof price === 'number' && Number.isFinite(price) && price >= 0
  })
}

function matchingVideoPriceEntry(prices: Record<string, Record<string, number>>, modelId: string) {
  const exact = Object.entries(prices).find(([key]) => key.toLowerCase() === modelId.toLowerCase())
  if (exact) return exact[1]
  const family = grokVideoFamily(modelId)
  if (!family) return undefined
  return Object.entries(prices).find(([key]) => grokVideoFamily(key) === family)?.[1]
}

function readVideoTaskSession(): VideoTaskSession | null {
  try {
    const raw = sessionStorage.getItem(videoTaskSessionKey)
    if (!raw) return null
    const value = JSON.parse(raw) as Partial<VideoTaskSession>
    if (typeof value.keyId !== 'number' || !Number.isSafeInteger(value.keyId) || value.keyId <= 0 || typeof value.requestId !== 'string' || !value.requestId.trim()) return null
    return {
      keyId: Number(value.keyId),
      requestId: value.requestId.trim(),
      progress: typeof value.progress === 'number' && Number.isFinite(value.progress) ? value.progress : null,
    }
  } catch {
    return null
  }
}

function persistVideoTaskSession() {
  if (!requestId.value || !selectedKeyId.value) return
  try {
    sessionStorage.setItem(videoTaskSessionKey, JSON.stringify({
      keyId: selectedKeyId.value,
      requestId: requestId.value,
      progress: progress.value,
    } satisfies VideoTaskSession))
  } catch {
    // Session storage may be unavailable in private browser contexts.
  }
}

function clearVideoTaskSession() {
  try {
    sessionStorage.removeItem(videoTaskSessionKey)
  } catch {
    // Session storage may be unavailable in private browser contexts.
  }
}

function errorText(error: unknown) {
  return error instanceof Error
    ? error.message
    : String((error as { message?: unknown } | null)?.message || t('canvas.video.unknownError'))
}

function taskRequestId(task: VideoGenerationTask) {
  return String(task.request_id || task.id || '').trim()
}

function taskStatus(task: VideoGenerationTask) {
  return String(task.status || '').trim().toLowerCase()
}

class VideoTaskFailedError extends Error {}

function updateProgress(task: VideoGenerationTask) {
  if (typeof task.progress !== 'number' || !Number.isFinite(task.progress)) return
  const normalized = task.progress <= 1 ? task.progress * 100 : task.progress
  progress.value = Math.max(0, Math.min(100, Math.round(normalized)))
}

function clearPollTimer() {
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
}

function clearVideoURL() {
  if (videoURL.value) URL.revokeObjectURL(videoURL.value)
  videoURL.value = ''
}

async function loadKeys() {
  loadingKeys.value = true
  keysError.value = ''
  try {
    const response = await keysAPI.list(1, 100, { status: 'active', sort_by: 'created_at', sort_order: 'desc' })
    apiKeys.value = response.items || []
    if (restoredTaskSession) {
      selectedKeyId.value = restoredTaskSession.keyId
    } else if (!selectedKey.value && videoKeys.value.length) {
      selectedKeyId.value = videoKeys.value[0].id
    }
  } catch (error) {
    keysError.value = errorText(error)
  } finally {
    loadingKeys.value = false
  }
}

async function loadCompletedVideo(apiKey: string, id: string, signal: AbortSignal) {
  phase.value = 'downloading'
  const blob = await retryTaskPollingRequest(() => videoAPI.content(apiKey, id, signal), signal)
  clearVideoURL()
  videoURL.value = URL.createObjectURL(blob)
  progress.value = 100
  phase.value = 'completed'
  canResumeTask.value = false
  persistVideoTaskSession()
}

async function pollStatus(apiKey: string, id: string, signal: AbortSignal) {
  if (signal.aborted) return
  try {
    const task = await retryTaskPollingRequest(() => videoAPI.status(apiKey, id, signal), signal)
    updateProgress(task)
    persistVideoTaskSession()
    const status = taskStatus(task)
    if (['done', 'completed', 'succeeded', 'success'].includes(status)) {
      await loadCompletedVideo(apiKey, id, signal)
      return
    }
    if (['failed', 'error', 'expired', 'canceled', 'cancelled'].includes(status)) {
      throw new VideoTaskFailedError(typeof task.error === 'string' ? task.error : t('canvas.video.taskFailed', { status }))
    }
    phase.value = 'pending'
    pollTimer = setTimeout(() => { void pollStatus(apiKey, id, signal) }, 3000)
  } catch (error) {
    if (signal.aborted) return
    errorMessage.value = errorText(error)
    phase.value = 'failed'
    canResumeTask.value = !(error instanceof VideoTaskFailedError)
    if (error instanceof VideoTaskFailedError) clearVideoTaskSession()
    else persistVideoTaskSession()
  } finally {
    if (activeController?.signal === signal && !isWorking.value) activeController = null
  }
}

function handleGenerationSubmit() {
  if (canResumeTask.value && requestId.value) {
    void resumeTask()
    return
  }
  void submitGeneration()
}

async function resumeTask() {
  const key = selectedKey.value
  const id = requestId.value
  if (!key || !id || activeController) return

  clearPollTimer()
  const controller = new AbortController()
  activeController = controller
  errorMessage.value = ''
  canResumeTask.value = false
  phase.value = 'pending'
  persistVideoTaskSession()
  await pollStatus(key.key, id, controller.signal)
}

async function submitGeneration() {
  const key = selectedKey.value
  if (!key || !prompt.value.trim() || !availableResolutions.value.includes(resolution.value) || isWorking.value) return

  clearPollTimer()
  activeController?.abort()
  const controller = new AbortController()
  activeController = controller
  clearVideoURL()
  errorMessage.value = ''
  requestId.value = ''
  canResumeTask.value = false
  clearVideoTaskSession()
  progress.value = null
  phase.value = 'submitting'

  try {
    const task = await videoAPI.submit(key.key, {
      model: model.value,
      prompt: prompt.value.trim(),
      duration: duration.value,
      aspect_ratio: aspectRatio.value,
      resolution: resolution.value,
    }, controller.signal)
    const id = taskRequestId(task)
    if (!id) throw new Error(t('canvas.video.missingRequestId'))
    requestId.value = id
    canResumeTask.value = true
    updateProgress(task)
    phase.value = 'pending'
    persistVideoTaskSession()
    await pollStatus(key.key, id, controller.signal)
  } catch (error) {
    if (controller.signal.aborted) return
    errorMessage.value = errorText(error)
    phase.value = 'failed'
  } finally {
    if (activeController === controller && !isWorking.value) activeController = null
  }
}

function cancelGeneration() {
  clearPollTimer()
  activeController?.abort()
  activeController = null
  phase.value = 'canceled'
  canResumeTask.value = Boolean(requestId.value)
  errorMessage.value = canResumeTask.value ? t('canvas.video.waitingStopped') : ''
  persistVideoTaskSession()
}

function resetResult() {
  errorMessage.value = ''
  requestId.value = ''
  canResumeTask.value = false
  progress.value = null
  phase.value = 'idle'
  clearVideoTaskSession()
}

function downloadVideo() {
  if (!videoURL.value) return
  const anchor = document.createElement('a')
  anchor.href = videoURL.value
  anchor.download = `modurelay-video-${requestId.value || Date.now()}.mp4`
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
}

onMounted(async () => {
  restoredTaskSession = readVideoTaskSession()
  if (restoredTaskSession) {
    selectedKeyId.value = restoredTaskSession.keyId
    requestId.value = restoredTaskSession.requestId
    progress.value = restoredTaskSession.progress
    canResumeTask.value = true
    phase.value = 'pending'
  }
  await loadKeys()
  if (!restoredTaskSession) return
  if (!selectedKey.value) {
    errorMessage.value = t('canvas.video.resumeKeyUnavailable')
    phase.value = 'failed'
    canResumeTask.value = false
    return
  }
  await resumeTask()
})
onUnmounted(() => {
  clearPollTimer()
  activeController?.abort()
  clearVideoURL()
})
</script>

<style scoped>
.canvas-video-page {
  min-height: calc(100dvh - 4rem);
  background: var(--color-bg);
  color: var(--color-text-primary);
}

.canvas-video {
  width: min(1280px, 100%);
  margin: 0 auto;
  padding: 34px 28px 56px;
}

.canvas-video__heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 24px;
}

.canvas-video__heading > div { min-width: 0; }
.canvas-video__eyebrow { display: inline-flex; align-items: center; gap: 7px; color: var(--color-primary); font-size: 13px; font-weight: 700; }
.canvas-video__heading h1 { margin: 8px 0 0; font-size: 28px; font-weight: 750; letter-spacing: 0; line-height: 1.2; }
.canvas-video__heading p { max-width: 680px; margin: 8px 0 0; color: var(--color-text-secondary); font-size: 14px; line-height: 1.65; }

.canvas-video__back,
.canvas-video__primary-button,
.canvas-video__secondary-button,
.canvas-video__result-actions button,
.canvas-video__error-state button {
  display: inline-flex;
  min-height: 40px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 0 14px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  color: var(--color-text-primary);
  font-size: 13px;
  font-weight: 650;
  cursor: pointer;
  text-decoration: none;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.canvas-video__back:hover,
.canvas-video__secondary-button:hover,
.canvas-video__result-actions button:hover,
.canvas-video__error-state button:hover { border-color: var(--color-primary-border); background: var(--color-primary-soft); color: var(--color-primary); }

.canvas-video__workbench {
  display: grid;
  grid-template-columns: minmax(0, 0.86fr) minmax(420px, 1.14fr);
  gap: 20px;
  align-items: stretch;
}

.canvas-video__form,
.canvas-video__result {
  min-width: 0;
  padding: 20px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.canvas-video__form-heading,
.canvas-video__result-heading {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}

.canvas-video__form-heading h2,
.canvas-video__result-heading h2 { margin: 0; font-size: 17px; font-weight: 700; letter-spacing: 0; }
.canvas-video__form-heading p,
.canvas-video__result-heading p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; line-height: 1.55; }
.canvas-video__form-heading > span,
.canvas-video__result-heading > span { flex: 0 0 auto; padding: 4px 8px; border-radius: 999px; background: var(--color-surface-soft); color: var(--color-text-muted); font-size: 12px; white-space: nowrap; }
.canvas-video__result-heading > span.is-completed { background: color-mix(in srgb, var(--color-success) 12%, var(--color-surface)); color: var(--color-success); }
.canvas-video__result-heading > span.is-failed { background: color-mix(in srgb, var(--color-danger) 10%, var(--color-surface)); color: var(--color-danger); }

.canvas-video__field { position: relative; min-width: 0; }
.canvas-video__field + .canvas-video__field { margin-top: 16px; }
.canvas-video__field label { display: block; margin-bottom: 7px; color: var(--color-text-secondary); font-size: 13px; font-weight: 650; }
.canvas-video__field select,
.canvas-video__field textarea {
  width: 100%;
  min-width: 0;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface-soft);
  color: var(--color-text-primary);
  font: inherit;
  font-size: 14px;
  outline: none;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard);
}
.canvas-video__field select { height: 40px; padding: 0 34px 0 11px; }
.canvas-video__field textarea { min-height: 152px; padding: 11px 12px 28px; line-height: 1.6; resize: vertical; }
.canvas-video__field select:focus,
.canvas-video__field textarea:focus { border-color: var(--color-primary); box-shadow: 0 0 0 3px var(--color-primary-ring); }
.canvas-video__field select:disabled,
.canvas-video__field textarea:disabled { cursor: not-allowed; opacity: 0.62; }
.canvas-video__counter { position: absolute; right: 10px; bottom: 8px; color: var(--color-text-muted); font-size: 11px; }
.canvas-video__field-hint,
.canvas-video__field-error { margin: 7px 0 0; font-size: 12px; line-height: 1.5; }
.canvas-video__field-hint { color: var(--color-text-muted); }
.canvas-video__field-hint.is-warning { color: var(--color-warning); }
.canvas-video__field-hint a { color: var(--color-primary); font-weight: 650; }
.canvas-video__field-error { color: var(--color-danger); overflow-wrap: anywhere; }

.canvas-video__options { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; margin-top: 16px; }
.canvas-video__options .canvas-video__field { margin-top: 0; }
.canvas-video__billing-note { display: flex; align-items: flex-start; gap: 8px; margin: 18px 0 0; padding: 10px 11px; border: 1px solid var(--color-border-subtle); border-radius: 8px; background: var(--color-surface-soft); color: var(--color-text-muted); font-size: 12px; line-height: 1.55; }
.canvas-video__billing-note :deep(svg) { flex: 0 0 auto; margin-top: 1px; color: var(--color-accent); }
.canvas-video__form-actions { display: flex; justify-content: flex-end; gap: 9px; margin-top: 20px; }
.canvas-video__primary-button { border-color: transparent; background: var(--color-primary); color: var(--color-text-on-primary, white); }
.canvas-video__primary-button:hover { background: var(--color-primary-hover); transform: translateY(-1px); }
.canvas-video__primary-button:disabled { cursor: not-allowed; opacity: 0.55; transform: none; }

.canvas-video__result { display: flex; min-height: 600px; flex-direction: column; }
.canvas-video__stage { display: grid; min-height: 0; flex: 1; place-items: center; overflow: hidden; border: 1px solid var(--color-border-subtle); border-radius: 8px; background: var(--color-bg-deep); }
.canvas-video__stage video { display: block; width: 100%; max-height: 560px; object-fit: contain; }
.canvas-video__progress-state,
.canvas-video__error-state,
.canvas-video__empty-state { display: flex; width: min(390px, 100%); align-items: center; flex-direction: column; padding: 34px 24px; text-align: center; }
.canvas-video__progress-icon,
.canvas-video__error-state > span,
.canvas-video__empty-state > span { display: grid; width: 52px; height: 52px; place-items: center; border: 1px solid var(--color-primary-border); border-radius: 10px; background: var(--color-primary-soft); color: var(--color-primary); }
.canvas-video__progress-state h3,
.canvas-video__error-state h3,
.canvas-video__empty-state h3 { margin: 16px 0 0; font-size: 16px; font-weight: 700; }
.canvas-video__progress-state p,
.canvas-video__error-state p,
.canvas-video__empty-state p { margin: 7px 0 0; color: var(--color-text-muted); font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; }
.canvas-video__progress-state progress { width: min(260px, 100%); height: 7px; margin-top: 18px; accent-color: var(--color-primary); }
.canvas-video__error-state > span { border-color: color-mix(in srgb, var(--color-danger) 35%, var(--color-border)); background: color-mix(in srgb, var(--color-danger) 9%, var(--color-surface)); color: var(--color-danger); }
.canvas-video__error-actions { display: flex; flex-wrap: wrap; justify-content: center; gap: 8px; margin-top: 18px; }
.canvas-video__error-state button { margin-top: 0; }
.canvas-video__result-actions { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-top: 14px; }
.canvas-video__result-actions > span { min-width: 0; color: var(--color-text-muted); font-size: 12px; }
.canvas-video__result-actions code { color: var(--color-text-secondary); overflow-wrap: anywhere; }

.canvas-video__back:focus-visible,
.canvas-video__primary-button:focus-visible,
.canvas-video__secondary-button:focus-visible,
.canvas-video__result-actions button:focus-visible,
.canvas-video__error-state button:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.is-spinning { animation: canvas-video-spin 0.9s linear infinite; }
@keyframes canvas-video-spin { to { transform: rotate(360deg); } }

@media (max-width: 1080px) {
  .canvas-video__workbench { grid-template-columns: 1fr; }
  .canvas-video__result { min-height: 520px; }
}

@media (max-width: 700px) {
  .canvas-video { padding: 26px 16px 40px; }
  .canvas-video__heading { align-items: flex-start; flex-direction: column; gap: 16px; }
  .canvas-video__heading h1 { font-size: 24px; }
  .canvas-video__back { min-height: 44px; }
  .canvas-video__form,
  .canvas-video__result { padding: 16px; }
  .canvas-video__options { grid-template-columns: 1fr; }
  .canvas-video__field select { height: 44px; }
  .canvas-video__form-actions { align-items: stretch; flex-direction: column-reverse; }
  .canvas-video__primary-button,
  .canvas-video__secondary-button { min-height: 44px; width: 100%; }
  .canvas-video__result { min-height: 430px; }
  .canvas-video__result-actions { align-items: stretch; flex-direction: column; }
  .canvas-video__result-actions button { min-height: 44px; }
}

@media (prefers-reduced-motion: reduce) {
  .canvas-video__back,
  .canvas-video__primary-button,
  .canvas-video__secondary-button,
  .canvas-video__result-actions button,
  .canvas-video__error-state button { transition-duration: 0.01ms; }
  .is-spinning { animation: none; }
}
</style>
