<template>
  <AppLayout>
    <div class="detection-center">
      <header class="page-hero">
        <span class="page-icon">
          <Icon name="beaker" size="lg" :stroke-width="2" />
        </span>
        <div>
          <h1>{{ t('admin.detectionCenter.title') }}</h1>
          <p>{{ t('admin.detectionCenter.description') }}</p>
        </div>
      </header>

      <section class="panel config-panel">
        <div class="panel-title">
          <span class="panel-title-icon">
            <Icon name="focus" size="sm" :stroke-width="2" />
          </span>
          <h2>{{ t('admin.detectionCenter.configTitle') }}</h2>
        </div>

        <div class="config-grid">
          <label class="field field-half">
            <span>{{ t('admin.detectionCenter.baseUrl') }} <em>*</em></span>
            <div class="control-shell">
              <Icon name="link" size="sm" />
              <input
                v-model.trim="form.base_url"
                type="url"
                spellcheck="false"
                :placeholder="t('admin.detectionCenter.baseUrlPlaceholder')"
                :disabled="busy"
              />
            </div>
          </label>

          <label class="field field-half">
            <span>{{ t('admin.detectionCenter.apiKey') }} <em>*</em></span>
            <div class="control-shell">
              <Icon name="lock" size="sm" />
              <input
                v-model="form.api_key"
                :type="showKey ? 'text' : 'password'"
                autocomplete="off"
                spellcheck="false"
                :placeholder="t('admin.detectionCenter.apiKeyPlaceholder')"
                :disabled="busy"
              />
              <button
                class="icon-action"
                type="button"
                :title="showKey ? t('admin.detectionCenter.hideKey') : t('admin.detectionCenter.showKey')"
                :aria-label="showKey ? t('admin.detectionCenter.hideKey') : t('admin.detectionCenter.showKey')"
                @click.prevent="showKey = !showKey"
              >
                <Icon :name="showKey ? 'eyeOff' : 'eye'" size="sm" />
              </button>
            </div>
          </label>

          <label class="field">
            <span>{{ t('admin.detectionCenter.protocol') }} <em>*</em></span>
            <div class="control-shell">
              <Icon name="database" size="sm" />
              <select v-model="form.protocol" :disabled="busy" @change="clearDiscovery">
                <option value="auto">{{ t('admin.detectionCenter.protocols.auto') }}</option>
                <option value="openai">{{ t('admin.detectionCenter.protocols.openai') }}</option>
                <option value="anthropic">{{ t('admin.detectionCenter.protocols.anthropic') }}</option>
                <option value="gemini">{{ t('admin.detectionCenter.protocols.gemini') }}</option>
              </select>
            </div>
          </label>

          <label class="field">
            <span>{{ t('admin.detectionCenter.model') }} <em>*</em></span>
            <div class="control-shell">
              <Icon name="cube" size="sm" />
              <select v-model="form.model" :disabled="running || !discovery?.models.length">
                <option value="" disabled>{{ t('admin.detectionCenter.modelPlaceholder') }}</option>
                <option v-for="model in filteredModels" :key="modelKey(model)" :value="model.id">
                  {{ model.id }}{{ model.provider ? ' · ' + model.provider : '' }}
                </option>
              </select>
              <button
                v-if="discovery?.models.length"
                class="icon-action"
                type="button"
                :title="t('admin.detectionCenter.discover')"
                :aria-label="t('admin.detectionCenter.discover')"
                :disabled="busy || !canDiscover"
                @click.prevent="discoverModels"
              >
                <Icon name="refresh" size="sm" :class="{ spinning: discovering }" />
              </button>
            </div>
          </label>

          <label class="field">
            <span>{{ t('admin.detectionCenter.mode') }} <em>*</em></span>
            <div class="control-shell">
              <Icon name="cpu" size="sm" />
              <select v-model="form.mode" :disabled="running">
                <option value="standard">{{ t('admin.detectionCenter.modes.standard') }}</option>
                <option value="deep">{{ t('admin.detectionCenter.modes.deep') }}</option>
              </select>
            </div>
          </label>
        </div>

        <div class="config-actions">
          <button class="btn btn-secondary" type="button" :disabled="busy || !canDiscover" @click="discoverModels">
            <Icon name="refresh" size="sm" :class="{ spinning: discovering }" />
            {{ discovering ? t('admin.detectionCenter.discovering') : t('admin.detectionCenter.discover') }}
          </button>
          <button class="btn btn-primary" type="button" :disabled="running || !form.model" @click="runDetection">
            <span v-if="running" class="spinner" />
            <Icon v-else name="play" size="sm" />
            {{ running ? t('admin.detectionCenter.running') : t('admin.detectionCenter.start') }}
          </button>
        </div>

        <div v-if="errorMessage" class="inline-error" role="alert">
          <Icon name="xCircle" size="sm" />
          <span>{{ errorMessage }}</span>
          <button type="button" aria-label="关闭" @click="errorMessage = ''">
            <Icon name="x" size="xs" />
          </button>
        </div>
      </section>

      <section class="summary-row" aria-label="检测汇总">
        <article class="summary-card summary-success">
          <span class="summary-status-icon"><Icon name="checkCircle" size="md" /></span>
          <div class="summary-copy">
            <strong>{{ summaryStats.success }}</strong>
            <span>{{ t('admin.detectionCenter.success') }}</span>
          </div>
          <span class="summary-wave" />
        </article>

        <article class="summary-card summary-failed">
          <span class="summary-status-icon"><Icon name="xCircle" size="md" /></span>
          <div class="summary-copy">
            <strong>{{ summaryStats.failed }}</strong>
            <span>{{ t('admin.detectionCenter.failed') }}</span>
          </div>
          <span class="summary-wave" />
        </article>

        <article class="summary-card summary-partial">
          <span class="summary-status-icon"><Icon name="exclamationTriangle" size="md" /></span>
          <div class="summary-copy">
            <strong>{{ summaryStats.partial }}</strong>
            <span>{{ t('admin.detectionCenter.partialAvailable') }}</span>
          </div>
          <span class="summary-wave" />
        </article>

        <article class="summary-card summary-unavailable">
          <span class="summary-status-icon"><Icon name="minus" size="md" /></span>
          <div class="summary-copy">
            <strong>{{ summaryStats.unavailable }}</strong>
            <span>{{ t('admin.detectionCenter.unavailableSimple') }}</span>
          </div>
          <span class="summary-wave" />
        </article>
      </section>

      <section class="results-grid">
        <aside class="panel capability-panel">
          <div class="result-panel-head">
            <div class="panel-title">
              <span class="panel-title-icon">
                <Icon name="clipboard" size="sm" :stroke-width="2" />
              </span>
              <h2>{{ t('admin.detectionCenter.capabilityItems') }}</h2>
            </div>
            <span class="count-text">{{ t('admin.detectionCenter.itemCount', { count: report?.probes.length || 0 }) }}</span>
          </div>

          <div class="capability-list">
            <button
              v-for="item in capabilityRows"
              :key="item.id"
              type="button"
              :class="['capability-row', { active: selectedProbe?.id === item.id, disabled: !report }]"
              :disabled="!report"
              @click="selectCapability(item.id)"
            >
              <span class="capability-row-icon">
                <Icon :name="item.icon" size="sm" />
              </span>
              <span class="capability-name">{{ item.name }}</span>
              <span v-if="item.status === 'pending'" class="pending-dot">
                <Icon name="minus" size="xs" />
              </span>
              <span v-else :class="['status-dot', statusClass(item.status)]">
                <Icon v-if="item.status === 'success'" name="checkCircle" size="xs" />
                <Icon v-else-if="item.status === 'failed'" name="xCircle" size="xs" />
                <Icon v-else-if="item.status === 'partial'" name="exclamationTriangle" size="xs" />
                <Icon v-else name="minus" size="xs" />
              </span>
            </button>
          </div>
        </aside>

        <article class="panel detail-panel">
          <div class="result-panel-head">
            <div class="panel-title">
              <span class="panel-title-icon">
                <Icon name="document" size="sm" :stroke-width="2" />
              </span>
              <h2>{{ t('admin.detectionCenter.resultDetails') }}</h2>
            </div>

            <div v-if="report" class="detail-actions">
              <button class="mini-action" type="button" :title="t('admin.detectionCenter.copyReport')" @click="copyReport">
                <Icon name="copy" size="xs" />
              </button>
              <button class="mini-action" type="button" :title="t('admin.detectionCenter.downloadReport')" @click="downloadReport">
                <Icon name="download" size="xs" />
              </button>
            </div>
            <span v-else class="empty-status">{{ t('admin.detectionCenter.noResult') }}</span>
          </div>

          <div v-if="!report || !selectedProbe" class="empty-state">
            <span class="empty-illustration">
              <Icon name="document" size="lg" />
              <span class="sparkle sparkle-one">✦</span>
              <span class="sparkle sparkle-two">✦</span>
            </span>
            <strong>{{ t('admin.detectionCenter.noResult') }}</strong>
            <p>{{ t('admin.detectionCenter.noResultHint') }}</p>
            <button class="btn btn-primary empty-start" type="button" :disabled="running || !form.model" @click="runDetection">
              <Icon name="play" size="sm" />
              {{ t('admin.detectionCenter.start') }}
            </button>
          </div>

          <div v-else class="probe-view">
            <div class="probe-header">
              <div class="probe-heading">
                <span :class="['probe-state-icon', statusClass(selectedProbe.status)]">
                  <Icon v-if="selectedProbe.status === 'success'" name="checkCircle" size="md" />
                  <Icon v-else-if="selectedProbe.status === 'failed'" name="xCircle" size="md" />
                  <Icon v-else-if="selectedProbe.status === 'partial'" name="exclamationTriangle" size="md" />
                  <Icon v-else name="infoCircle" size="md" />
                </span>
                <div>
                  <div class="probe-title-line">
                    <h3>{{ selectedProbe.name }}</h3>
                    <span :class="['status-pill', statusClass(selectedProbe.status)]">{{ statusText(selectedProbe.status) }}</span>
                  </div>
                  <p>{{ selectedProbe.summary }}</p>
                </div>
              </div>

              <div class="confidence">
                <span>{{ t('admin.detectionCenter.confidence') }}</span>
                <strong>{{ confidenceText(selectedProbe.confidence) }}</strong>
              </div>
            </div>

            <div v-if="selectedProbe.reason_code || selectedProbe.failure_reason" class="diagnosis-card">
              <div v-if="selectedProbe.reason_code" class="diagnosis-row">
                <span>{{ t('admin.detectionCenter.reasonCode') }}</span>
                <code>{{ selectedProbe.reason_code }}</code>
              </div>
              <div v-if="selectedProbe.failure_reason" class="diagnosis-row">
                <span>{{ t('admin.detectionCenter.failureReason') }}</span>
                <p>{{ selectedProbe.failure_reason }}</p>
              </div>
            </div>

            <div class="probe-columns">
              <div class="probe-column">
                <section v-if="selectedProbe.possible_causes?.length" class="info-section">
                  <h4>{{ t('admin.detectionCenter.possibleCauses') }}</h4>
                  <ul>
                    <li v-for="item in selectedProbe.possible_causes" :key="item">{{ item }}</li>
                  </ul>
                </section>

                <section v-if="selectedProbe.recommendations?.length" class="info-section">
                  <h4>{{ t('admin.detectionCenter.recommendations') }}</h4>
                  <ul>
                    <li v-for="item in selectedProbe.recommendations" :key="item">{{ item }}</li>
                  </ul>
                </section>

                <section v-if="!selectedProbe.possible_causes?.length && !selectedProbe.recommendations?.length" class="success-note">
                  <Icon name="checkCircle" size="sm" />
                  <span>{{ selectedProbe.summary }}</span>
                </section>
              </div>

              <div class="probe-column evidence-column">
                <div class="evidence-title-row">
                  <h4>{{ t('admin.detectionCenter.evidence') }}</h4>
                  <span>{{ selectedProbe.evidence?.length || 0 }}</span>
                </div>

                <div v-if="selectedProbe.evidence?.length" class="evidence-list">
                  <article
                    v-for="(evidence, index) in selectedProbe.evidence"
                    :key="selectedProbe.id + '-' + index"
                    class="evidence-card"
                  >
                    <div class="evidence-head">
                      <strong>{{ evidence.label }}</strong>
                      <span v-if="evidence.http_status">HTTP {{ evidence.http_status }}</span>
                      <span v-if="evidence.duration_ms">{{ evidence.duration_ms }} ms</span>
                    </div>
                    <dl>
                      <template v-if="evidence.expected">
                        <dt>{{ t('admin.detectionCenter.expected') }}</dt>
                        <dd>{{ evidence.expected }}</dd>
                      </template>
                      <template v-if="evidence.actual">
                        <dt>{{ t('admin.detectionCenter.actual') }}</dt>
                        <dd>{{ evidence.actual }}</dd>
                      </template>
                    </dl>
                    <pre v-if="evidence.response_excerpt">{{ evidence.response_excerpt }}</pre>
                  </article>
                </div>
                <div v-else class="evidence-empty">{{ t('admin.detectionCenter.noEvidence') }}</div>
              </div>
            </div>
          </div>
        </article>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import detectionCenterAPI, {
  type DetectedModel,
  type DetectionDiscoveryResponse,
  type DetectionMode,
  type DetectionProtocol,
  type DetectionProbeResult,
  type DetectionReport,
} from '@/api/admin/detectionCenter'

type CapabilityRow = {
  id: string
  name: string
  icon: string
  status: DetectionProbeResult['status'] | 'pending'
}

const { t } = useI18n()

const form = reactive<{
  base_url: string
  api_key: string
  protocol: DetectionProtocol
  model: string
  mode: DetectionMode
}>({
  base_url: '',
  api_key: '',
  protocol: 'auto',
  model: '',
  mode: 'standard'
})

const showKey = ref(false)
const discovering = ref(false)
const running = ref(false)
const copied = ref(false)
const errorMessage = ref('')
const discovery = ref<DetectionDiscoveryResponse | null>(null)
const report = ref<DetectionReport | null>(null)
const selectedProbeId = ref('')

const busy = computed(() => discovering.value || running.value)
const canDiscover = computed(() => Boolean(form.base_url.trim() && form.api_key.trim()))
const filteredModels = computed(() => discovery.value?.models ?? [])

const placeholderCapabilities = computed<CapabilityRow[]>(() => [
  { id: 'placeholder-basic', name: t('admin.detectionCenter.capabilities.basic'), icon: 'link', status: 'pending' },
  { id: 'placeholder-models', name: t('admin.detectionCenter.capabilities.models'), icon: 'cube', status: 'pending' },
  { id: 'placeholder-text', name: t('admin.detectionCenter.capabilities.text'), icon: 'document', status: 'pending' },
  { id: 'placeholder-multi', name: t('admin.detectionCenter.capabilities.multiTurn'), icon: 'cpu', status: 'pending' },
  { id: 'placeholder-stream', name: t('admin.detectionCenter.capabilities.streaming'), icon: 'refresh', status: 'pending' },
  { id: 'placeholder-tools', name: t('admin.detectionCenter.capabilities.tools'), icon: 'focus', status: 'pending' },
  { id: 'placeholder-vision', name: t('admin.detectionCenter.capabilities.vision'), icon: 'eye', status: 'pending' },
])

const capabilityRows = computed<CapabilityRow[]>(() => {
  if (!report.value) return placeholderCapabilities.value
  return report.value.probes.map(probe => ({
    id: probe.id,
    name: probe.name,
    icon: capabilityIcon(probe.category),
    status: probe.status,
  }))
})

const selectedProbe = computed(() => {
  if (!report.value?.probes.length) return null
  return report.value.probes.find(probe => probe.id === selectedProbeId.value) ?? report.value.probes[0]
})

const summaryStats = computed(() => {
  const summary = report.value?.summary
  if (!summary) return { success: 0, failed: 0, partial: 0, unavailable: 0 }
  return {
    success: summary.success,
    failed: summary.failed,
    partial: summary.partial,
    unavailable: summary.inconclusive + summary.not_applicable + summary.unavailable,
  }
})

function modelKey(model: DetectedModel) {
  return model.id + ':' + model.protocols.join(',')
}

function clearDiscovery() {
  discovery.value = null
  report.value = null
  selectedProbeId.value = ''
  form.model = ''
}

function selectCapability(id: string) {
  if (!report.value) return
  selectedProbeId.value = id
}

function capabilityIcon(category: string) {
  const value = category.toLowerCase()
  if (value.includes('stream')) return 'refresh'
  if (value.includes('tool')) return 'focus'
  if (value.includes('structured')) return 'document'
  if (value.includes('cache')) return 'database'
  if (value.includes('citation')) return 'document'
  if (value.includes('reason')) return 'cpu'
  if (value.includes('context')) return 'clipboard'
  return 'link'
}

function statusText(status: string) {
  const known = ['success', 'failed', 'partial', 'inconclusive', 'not_applicable', 'unavailable']
  return known.includes(status) ? t('admin.detectionCenter.status.' + status) : status
}

function statusClass(status: string) {
  return 'status-' + status.replace(/_/g, '-')
}

function confidenceText(value: number) {
  return Math.round(Math.min(1, Math.max(0, value)) * 100) + '%'
}

async function discoverModels() {
  if (!canDiscover.value) {
    errorMessage.value = t('admin.detectionCenter.errors.required')
    return
  }

  discovering.value = true
  errorMessage.value = ''
  report.value = null
  selectedProbeId.value = ''
  form.model = ''

  try {
    discovery.value = await detectionCenterAPI.discover({
      base_url: form.base_url,
      api_key: form.api_key,
      protocol: form.protocol,
    })

    if (discovery.value.models.length === 1) {
      form.model = discovery.value.models[0].id
    }
  } catch (error) {
    discovery.value = null
    errorMessage.value = error instanceof Error ? error.message : t('admin.detectionCenter.errors.discoverFailed')
  } finally {
    discovering.value = false
  }
}

async function runDetection() {
  if (!form.model) {
    errorMessage.value = t('admin.detectionCenter.errors.modelRequired')
    return
  }

  running.value = true
  errorMessage.value = ''
  copied.value = false

  try {
    report.value = await detectionCenterAPI.run({
      base_url: form.base_url,
      api_key: form.api_key,
      protocol: form.protocol,
      model: form.model,
      mode: form.mode,
    })

    const preferred = report.value.probes.find(probe => probe.status === 'failed')
      ?? report.value.probes.find(probe => probe.status === 'partial')
      ?? report.value.probes[0]

    selectedProbeId.value = preferred?.id ?? ''
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('admin.detectionCenter.errors.runFailed')
  } finally {
    running.value = false
  }
}

async function copyReport() {
  if (!report.value) return
  try {
    await navigator.clipboard.writeText(JSON.stringify(report.value, null, 2))
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    errorMessage.value = t('admin.detectionCenter.errors.clipboardFailed')
  }
}

function downloadReport() {
  if (!report.value) return
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(report.value, null, 2)], { type: 'application/json;charset=utf-8' })
  )
  const link = document.createElement('a')
  link.href = url
  link.download = 'detection-report-' + report.value.report_id + '.json'
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}
</script>

<style scoped>
.detection-center {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 18px;
  padding-bottom: 28px;
}

.page-hero {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 4px 2px 2px;
}

.page-icon {
  display: inline-flex;
  height: 54px;
  width: 54px;
  flex: 0 0 54px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 16px;
  background: linear-gradient(145deg, var(--color-primary-soft), var(--color-surface));
  color: var(--color-primary);
  box-shadow: 0 10px 26px color-mix(in srgb, var(--color-primary) 12%, transparent);
}

.page-hero h1 {
  margin: 0;
  color: var(--color-text-primary);
  font-size: 28px;
  font-weight: 780;
  line-height: 1.1;
  letter-spacing: -0.02em;
}

.page-hero p {
  margin: 6px 0 0;
  color: var(--color-text-secondary);
  font-size: 13px;
}

.panel {
  border: 1px solid var(--color-border);
  border-radius: 18px;
  background: color-mix(in srgb, var(--color-surface) 96%, transparent);
  box-shadow: var(--shadow-xs);
}

.config-panel {
  padding: 20px 22px 18px;
}

.panel-title {
  display: flex;
  align-items: center;
  gap: 9px;
}

.panel-title-icon {
  display: inline-flex;
  height: 30px;
  width: 30px;
  flex: 0 0 30px;
  align-items: center;
  justify-content: center;
  border-radius: 9px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.panel-title h2 {
  margin: 0;
  color: var(--color-text-primary);
  font-size: 16px;
  font-weight: 730;
}

.config-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 16px;
  margin-top: 18px;
}

.field {
  display: flex;
  min-width: 0;
  grid-column: span 2;
  flex-direction: column;
  gap: 7px;
}

.field-half {
  grid-column: span 3;
}

.field > span {
  color: var(--color-text-primary);
  font-size: 12px;
  font-weight: 680;
}

.field em {
  color: var(--color-danger);
  font-style: normal;
}

.control-shell {
  display: flex;
  min-width: 0;
  min-height: 42px;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface);
  padding: 0 10px;
  color: var(--color-text-muted);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 38%);
}

.control-shell:focus-within {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

.control-shell input,
.control-shell select {
  min-width: 0;
  flex: 1;
  border: 0;
  background: transparent;
  color: var(--color-text-primary);
  font-size: 13px;
  outline: none;
}

.control-shell input::placeholder {
  color: var(--color-text-muted);
}

.icon-action,
.mini-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--color-text-muted);
}

.icon-action {
  height: 30px;
  width: 30px;
  flex: 0 0 30px;
}

.icon-action:hover:not(:disabled),
.mini-action:hover {
  background: var(--color-surface-soft);
  color: var(--color-primary);
}

.config-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 18px;
}

.btn {
  display: inline-flex;
  min-height: 42px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 1px solid transparent;
  border-radius: 10px;
  padding: 0 18px;
  font-size: 12px;
  font-weight: 720;
  transition:
    transform var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    border-color var(--motion-fast) var(--ease-standard);
}

.btn-secondary {
  border-color: var(--color-primary-border);
  background: var(--color-surface);
  color: var(--color-primary);
}

.btn-primary {
  border-color: var(--color-primary);
  background: linear-gradient(135deg, var(--color-primary), var(--color-primary-hover));
  color: white;
  box-shadow: 0 8px 18px color-mix(in srgb, var(--color-primary) 20%, transparent);
}

.btn:hover:not(:disabled) {
  transform: translateY(-1px);
}

.btn:disabled,
.icon-action:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.inline-error {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  border: 1px solid color-mix(in srgb, var(--color-danger) 28%, var(--color-border));
  border-radius: 9px;
  background: color-mix(in srgb, var(--color-danger) 7%, var(--color-surface));
  padding: 9px 11px;
  color: var(--color-danger);
  font-size: 11px;
}

.inline-error button {
  display: inline-flex;
  margin-left: auto;
  border: 0;
  background: transparent;
  color: inherit;
}

.summary-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.summary-card {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 94px;
  align-items: center;
  gap: 14px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  padding: 16px;
  box-shadow: var(--shadow-xs);
}

.summary-status-icon {
  position: relative;
  z-index: 2;
  display: inline-flex;
  height: 44px;
  width: 44px;
  flex: 0 0 44px;
  align-items: center;
  justify-content: center;
  border-radius: 13px;
}

.summary-copy {
  position: relative;
  z-index: 2;
}

.summary-copy strong {
  display: block;
  color: var(--color-text-primary);
  font-size: 24px;
  font-weight: 780;
  line-height: 1;
}

.summary-copy span {
  display: block;
  margin-top: 6px;
  color: var(--color-text-secondary);
  font-size: 11px;
  font-weight: 650;
}

.summary-wave {
  position: absolute;
  right: -18px;
  bottom: -26px;
  height: 84px;
  width: 190px;
  transform: rotate(-4deg);
  border-radius: 52% 48% 0 0;
  opacity: 0.9;
}

.summary-card::after {
  position: absolute;
  right: 16px;
  bottom: 10px;
  left: 96px;
  height: 5px;
  border-radius: 999px;
  content: '';
  opacity: 0.4;
}

.summary-success .summary-status-icon { background: color-mix(in srgb, var(--color-success) 12%, white); color: var(--color-success); }
.summary-success .summary-wave { background: linear-gradient(180deg, color-mix(in srgb, var(--color-success) 14%, transparent), transparent); }
.summary-success::after { background: var(--color-success); }

.summary-failed .summary-status-icon { background: color-mix(in srgb, var(--color-danger) 11%, white); color: var(--color-danger); }
.summary-failed .summary-wave { background: linear-gradient(180deg, color-mix(in srgb, var(--color-danger) 13%, transparent), transparent); }
.summary-failed::after { background: var(--color-danger); }

.summary-partial .summary-status-icon { background: color-mix(in srgb, var(--color-warning) 14%, white); color: var(--color-warning); }
.summary-partial .summary-wave { background: linear-gradient(180deg, color-mix(in srgb, var(--color-warning) 16%, transparent), transparent); }
.summary-partial::after { background: var(--color-warning); }

.summary-unavailable .summary-status-icon { background: var(--color-surface-soft); color: var(--color-text-muted); }
.summary-unavailable .summary-wave { background: linear-gradient(180deg, color-mix(in srgb, var(--color-info) 10%, transparent), transparent); }
.summary-unavailable::after { background: var(--color-text-muted); }

.results-grid {
  display: grid;
  grid-template-columns: minmax(300px, 0.72fr) minmax(0, 1.58fr);
  gap: 14px;
  align-items: stretch;
}

.capability-panel,
.detail-panel {
  min-height: 360px;
  padding: 0;
  overflow: hidden;
}

.result-panel-head {
  display: flex;
  min-height: 54px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid var(--color-border-subtle);
  padding: 0 16px;
}

.count-text,
.empty-status {
  color: var(--color-text-muted);
  font-size: 10px;
}

.capability-list {
  display: flex;
  flex-direction: column;
}

.capability-row {
  display: grid;
  width: 100%;
  min-height: 43px;
  grid-template-columns: 28px minmax(0, 1fr) 26px;
  gap: 9px;
  align-items: center;
  border: 0;
  border-bottom: 1px solid var(--color-border-subtle);
  background: transparent;
  padding: 0 14px;
  text-align: left;
  transition: background-color var(--motion-fast) var(--ease-standard);
}

.capability-row:last-child {
  border-bottom: 0;
}

.capability-row:not(.disabled):hover {
  background: var(--color-surface-soft);
}

.capability-row.active {
  background: var(--color-primary-soft);
}

.capability-row.disabled {
  cursor: default;
}

.capability-row-icon {
  display: inline-flex;
  height: 28px;
  width: 28px;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: var(--color-text-secondary);
}

.capability-name {
  min-width: 0;
  overflow: hidden;
  color: var(--color-text-primary);
  font-size: 11px;
  font-weight: 620;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pending-dot,
.status-dot {
  display: inline-flex;
  height: 24px;
  width: 24px;
  align-items: center;
  justify-content: center;
  justify-self: end;
  border-radius: 999px;
}

.pending-dot {
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
}

.status-success { color: var(--color-success); }
.status-failed { color: var(--color-danger); }
.status-partial,
.status-inconclusive { color: var(--color-warning); }
.status-not-applicable,
.status-unavailable { color: var(--color-text-muted); }

.detail-actions {
  display: flex;
  gap: 5px;
}

.mini-action {
  height: 28px;
  width: 28px;
}

.empty-state {
  display: flex;
  min-height: 304px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 28px;
  text-align: center;
}

.empty-illustration {
  position: relative;
  display: inline-flex;
  height: 86px;
  width: 104px;
  align-items: center;
  justify-content: center;
  border-radius: 48%;
  background: radial-gradient(circle at center, var(--color-primary-soft), transparent 70%);
  color: var(--color-primary);
}

.empty-illustration::before {
  position: absolute;
  height: 58px;
  width: 58px;
  border: 1px solid var(--color-primary-border);
  border-radius: 17px;
  background: var(--color-surface);
  box-shadow: 0 12px 28px color-mix(in srgb, var(--color-primary) 14%, transparent);
  content: '';
}

.empty-illustration :deep(svg) {
  position: relative;
  z-index: 2;
}

.sparkle {
  position: absolute;
  z-index: 3;
  color: color-mix(in srgb, var(--color-primary) 72%, white);
  font-size: 14px;
}

.sparkle-one { top: 12px; right: 6px; }
.sparkle-two { bottom: 12px; left: 4px; font-size: 10px; }

.empty-state strong {
  margin-top: 12px;
  color: var(--color-text-primary);
  font-size: 14px;
}

.empty-state p {
  margin: 6px 0 0;
  color: var(--color-text-muted);
  font-size: 11px;
}

.empty-start {
  min-height: 38px;
  margin-top: 16px;
  padding: 0 16px;
}

.probe-view {
  padding: 16px;
}

.probe-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.probe-heading {
  display: flex;
  min-width: 0;
  gap: 10px;
}

.probe-state-icon {
  display: inline-flex;
  height: 40px;
  width: 40px;
  flex: 0 0 40px;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  background: var(--color-surface-soft);
}

.probe-title-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.probe-title-line h3 {
  margin: 0;
  color: var(--color-text-primary);
  font-size: 15px;
}

.probe-heading p {
  margin: 5px 0 0;
  color: var(--color-text-secondary);
  font-size: 11px;
  line-height: 1.55;
}

.status-pill {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  border: 1px solid currentColor;
  border-radius: 999px;
  padding: 0 7px;
  font-size: 9px;
  font-weight: 720;
}

.confidence {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  color: var(--color-text-muted);
  font-size: 9px;
}

.confidence strong {
  color: var(--color-text-primary);
  font-size: 17px;
}

.diagnosis-card {
  margin-top: 14px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
  padding: 4px 10px;
}

.diagnosis-row {
  display: grid;
  grid-template-columns: 78px minmax(0, 1fr);
  gap: 10px;
  border-bottom: 1px solid var(--color-border-subtle);
  padding: 8px 0;
}

.diagnosis-row:last-child {
  border-bottom: 0;
}

.diagnosis-row > span {
  color: var(--color-text-muted);
  font-size: 9px;
  font-weight: 700;
}

.diagnosis-row code,
.diagnosis-row p {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: 10px;
  line-height: 1.55;
  overflow-wrap: anywhere;
}

.probe-columns {
  display: grid;
  grid-template-columns: minmax(0, 0.85fr) minmax(0, 1.15fr);
  gap: 14px;
  margin-top: 14px;
}

.probe-column {
  min-width: 0;
}

.evidence-column {
  border-left: 1px solid var(--color-border-subtle);
  padding-left: 14px;
}

.info-section + .info-section {
  margin-top: 14px;
}

.info-section h4,
.evidence-title-row h4 {
  margin: 0;
  color: var(--color-text-primary);
  font-size: 11px;
}

.info-section ul {
  margin: 7px 0 0;
  padding-left: 16px;
  color: var(--color-text-secondary);
  font-size: 10px;
  line-height: 1.7;
}

.success-note {
  display: flex;
  align-items: flex-start;
  gap: 7px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-success) 7%, var(--color-surface));
  padding: 10px;
  color: var(--color-success);
  font-size: 10px;
  line-height: 1.6;
}

.evidence-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.evidence-title-row span {
  display: inline-flex;
  min-width: 22px;
  height: 22px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: 9px;
}

.evidence-list {
  display: grid;
  gap: 8px;
  margin-top: 8px;
}

.evidence-card {
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
  padding: 9px;
}

.evidence-head {
  display: flex;
  align-items: center;
  gap: 7px;
}

.evidence-head strong {
  margin-right: auto;
  color: var(--color-text-primary);
  font-size: 10px;
}

.evidence-head span {
  color: var(--color-text-muted);
  font-size: 9px;
}

.evidence-card dl {
  display: grid;
  grid-template-columns: 48px minmax(0, 1fr);
  gap: 6px 8px;
  margin: 8px 0 0;
}

.evidence-card dt {
  color: var(--color-text-muted);
  font-size: 9px;
  font-weight: 700;
}

.evidence-card dd {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: 10px;
  line-height: 1.55;
}

.evidence-card pre {
  max-height: 170px;
  margin: 8px 0 0;
  overflow: auto;
  border-radius: 8px;
  background: var(--color-bg-deep);
  padding: 8px;
  color: var(--color-text-primary);
  font-size: 9px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
}

.evidence-empty {
  margin-top: 8px;
  border: 1px dashed var(--color-border);
  border-radius: 10px;
  padding: 20px;
  color: var(--color-text-muted);
  font-size: 10px;
  text-align: center;
}

.spinner,
.spinning {
  animation: spin 0.8s linear infinite;
}

.spinner {
  display: inline-block;
  height: 13px;
  width: 13px;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

button:focus-visible,
input:focus-visible,
select:focus-visible {
  outline: none;
}

.btn:focus-visible,
.icon-action:focus-visible,
.mini-action:focus-visible,
.capability-row:focus-visible {
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

@media (max-width: 1180px) {
  .config-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .field,
  .field-half {
    grid-column: span 1;
  }

  .summary-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .results-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .page-hero h1 {
    font-size: 24px;
  }

  .page-icon {
    height: 48px;
    width: 48px;
    flex-basis: 48px;
  }

  .config-panel {
    padding: 16px;
  }

  .config-grid,
  .summary-row {
    grid-template-columns: 1fr;
  }

  .config-actions {
    flex-direction: column;
  }

  .btn {
    width: 100%;
  }

  .probe-header {
    flex-direction: column;
  }

  .confidence {
    align-items: flex-start;
  }

  .probe-columns {
    grid-template-columns: 1fr;
  }

  .evidence-column {
    border-left: 0;
    border-top: 1px solid var(--color-border-subtle);
    padding-top: 14px;
    padding-left: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .btn,
  .capability-row {
    transition-duration: 0.01ms;
  }

  .spinner,
  .spinning {
    animation-duration: 1.6s;
  }
}
</style>
