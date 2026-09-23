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
          <form class="canvas-video__form" :aria-busy="isWorking || undefined" @submit.prevent="submitGeneration">
            <div class="canvas-video__form-heading">
              <div>
                <h2>{{ t('canvas.video.createTitle') }}</h2>
                <p>{{ t('canvas.video.createDescription') }}</p>
              </div>
              <span>{{ t('canvas.video.sessionOnly') }}</span>
            </div>

            <div class="canvas-video__field">
              <label for="canvas-video-key">{{ t('canvas.video.apiKey') }}</label>
              <select id="canvas-video-key" v-model.number="selectedKeyId" :disabled="loadingKeys || isWorking">
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
                  <option value="grok-imagine-video-1.5">grok-imagine-video-1.5</option>
                  <option value="grok-imagine-video">grok-imagine-video</option>
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
                <select id="canvas-video-resolution" v-model="resolution" :disabled="isWorking">
                  <option value="480p">480p</option>
                  <option value="720p">720p</option>
                  <option value="1080p">1080p</option>
                </select>
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
                {{ isWorking ? t('canvas.video.processing') : t('canvas.video.generate') }}
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
                <h3>{{ t('canvas.video.failedTitle') }}</h3>
                <p>{{ errorMessage }}</p>
                <button type="button" @click="resetResult">{{ t('canvas.video.retry') }}</button>
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
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import CanvasWorkspaceNav from '@/components/canvas/CanvasWorkspaceNav.vue'
import Icon from '@/components/icons/Icon.vue'
import { keysAPI } from '@/api/keys'
import { videoAPI, type VideoGenerationTask } from '@/api/video'
import type { ApiKey } from '@/types'

type VideoPhase = 'idle' | 'submitting' | 'pending' | 'downloading' | 'completed' | 'failed' | 'canceled'

const { t } = useI18n()
const apiKeys = ref<ApiKey[]>([])
const selectedKeyId = ref(0)
const loadingKeys = ref(false)
const keysError = ref('')
const prompt = ref('')
const model = ref('grok-imagine-video-1.5')
const duration = ref(6)
const aspectRatio = ref('16:9')
const resolution = ref('480p')
const requestId = ref('')
const phase = ref<VideoPhase>('idle')
const progress = ref<number | null>(null)
const errorMessage = ref('')
const videoURL = ref('')
let pollTimer: ReturnType<typeof setTimeout> | null = null
let activeController: AbortController | null = null

const videoKeys = computed(() => apiKeys.value.filter(key => (
  key.status === 'active'
  && key.group?.status === 'active'
  && key.group.platform === 'grok'
  && key.group.allow_image_generation === true
)))
const selectedKey = computed(() => videoKeys.value.find(key => key.id === selectedKeyId.value) || null)
const isWorking = computed(() => ['submitting', 'pending', 'downloading'].includes(phase.value))
const canSubmit = computed(() => Boolean(selectedKey.value && prompt.value.trim() && !isWorking.value))
const phaseLabel = computed(() => t(`canvas.video.phase.${phase.value}`))

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
    if (!selectedKey.value && videoKeys.value.length) selectedKeyId.value = videoKeys.value[0].id
  } catch (error) {
    keysError.value = errorText(error)
  } finally {
    loadingKeys.value = false
  }
}

async function loadCompletedVideo(apiKey: string, id: string, signal: AbortSignal) {
  phase.value = 'downloading'
  const blob = await videoAPI.content(apiKey, id, signal)
  clearVideoURL()
  videoURL.value = URL.createObjectURL(blob)
  progress.value = 100
  phase.value = 'completed'
}

async function pollStatus(apiKey: string, id: string, signal: AbortSignal) {
  if (signal.aborted) return
  try {
    const task = await videoAPI.status(apiKey, id, signal)
    updateProgress(task)
    const status = taskStatus(task)
    if (['done', 'completed', 'succeeded', 'success'].includes(status)) {
      await loadCompletedVideo(apiKey, id, signal)
      return
    }
    if (['failed', 'error', 'expired', 'canceled', 'cancelled'].includes(status)) {
      throw new Error(typeof task.error === 'string' ? task.error : t('canvas.video.taskFailed', { status }))
    }
    phase.value = 'pending'
    pollTimer = setTimeout(() => { void pollStatus(apiKey, id, signal) }, 3000)
  } catch (error) {
    if (signal.aborted) return
    errorMessage.value = errorText(error)
    phase.value = 'failed'
  }
}

async function submitGeneration() {
  const key = selectedKey.value
  if (!key || !prompt.value.trim() || isWorking.value) return

  clearPollTimer()
  activeController?.abort()
  const controller = new AbortController()
  activeController = controller
  clearVideoURL()
  errorMessage.value = ''
  requestId.value = ''
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
    updateProgress(task)
    phase.value = 'pending'
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
  progress.value = null
}

function resetResult() {
  errorMessage.value = ''
  requestId.value = ''
  progress.value = null
  phase.value = 'idle'
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

onMounted(() => { void loadKeys() })
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
.canvas-video__error-state button { margin-top: 18px; }
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
