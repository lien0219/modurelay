<template>
  <AppLayout>
    <div class="detection-center">
      <div class="topline">
        <button class="back-btn" type="button" @click="goBack">
          <Icon name="arrowLeft" size="sm" :stroke-width="2" />
          <span>{{ t('admin.detectionCenter.back') }}</span>
        </button>
        <div class="breadcrumb">
          <span>{{ t('admin.detectionCenter.admin') }}</span>
          <Icon name="chevronRight" size="xs" />
          <strong>{{ t('admin.detectionCenter.title') }}</strong>
        </div>
      </div>

      <header class="hero">
        <div class="hero-main">
          <span class="hero-icon">
            <Icon name="beaker" size="lg" :stroke-width="2" />
          </span>
          <div class="hero-copy">
            <h1>{{ t('admin.detectionCenter.title') }}</h1>
            <p>{{ t('admin.detectionCenter.description') }}</p>
          </div>
        </div>
        <div class="security-note">
          <span class="security-icon">
            <Icon name="lock" size="sm" :stroke-width="2" />
          </span>
          <span>{{ t('admin.detectionCenter.securityNotice') }}</span>
        </div>
      </header>

      <section class="surface target-panel">
        <div class="section-head">
          <div class="section-title">
            <span class="section-icon">
              <Icon name="focus" size="sm" :stroke-width="2" />
            </span>
            <div>
              <h2>{{ t('admin.detectionCenter.configTitle') }}</h2>
              <p>{{ t('admin.detectionCenter.configDescription') }}</p>
            </div>
          </div>
        </div>

        <div class="form-grid">
          <label class="field field-wide">
            <span class="label">{{ t('admin.detectionCenter.baseUrl') }} <em>*</em></span>
            <div class="input-shell">
              <Icon name="link" size="sm" />
              <input
                v-model.trim="form.base_url"
                type="url"
                spellcheck="false"
                :placeholder="t('admin.detectionCenter.baseUrlPlaceholder')"
                :disabled="busy"
              />
            </div>
            <small>{{ t('admin.detectionCenter.baseUrlHint') }}</small>
          </label>

          <label class="field">
            <span class="label">{{ t('admin.detectionCenter.apiKey') }} <em>*</em></span>
            <div class="input-shell">
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
                class="icon-btn"
                type="button"
                :title="showKey ? t('admin.detectionCenter.hideKey') : t('admin.detectionCenter.showKey')"
                :aria-label="showKey ? t('admin.detectionCenter.hideKey') : t('admin.detectionCenter.showKey')"
                @click.prevent="showKey = !showKey"
              >
                <Icon :name="showKey ? 'eyeOff' : 'eye'" size="sm" />
              </button>
            </div>
            <small>{{ t('admin.detectionCenter.keyHint') }}</small>
          </label>

          <label class="field">
            <span class="label">{{ t('admin.detectionCenter.protocol') }} <em>*</em></span>
            <div class="input-shell">
              <Icon name="cube" size="sm" />
              <select v-model="form.protocol" :disabled="busy" @change="clearDiscovery">
                <option value="auto">{{ t('admin.detectionCenter.protocols.auto') }}</option>
                <option value="openai">{{ t('admin.detectionCenter.protocols.openai') }}</option>
                <option value="anthropic">{{ t('admin.detectionCenter.protocols.anthropic') }}</option>
                <option value="gemini">{{ t('admin.detectionCenter.protocols.gemini') }}</option>
              </select>
            </div>
            <small>{{ t('admin.detectionCenter.protocolHint') }}</small>
          </label>

          <label class="field">
            <span class="label">{{ t('admin.detectionCenter.model') }} <em>*</em></span>
            <div class="input-shell">
              <Icon name="cpu" size="sm" />
              <select v-model="form.model" :disabled="running || !discovery?.models.length">
                <option value="" disabled>{{ t('admin.detectionCenter.modelPlaceholder') }}</option>
                <option v-for="model in filteredModels" :key="modelKey(model)" :value="model.id">
                  {{ model.id }}{{ model.provider ? ' · ' + model.provider : '' }}
                </option>
              </select>
              <button
                class="icon-btn"
                type="button"
                :title="t('admin.detectionCenter.discover')"
                :aria-label="t('admin.detectionCenter.discover')"
                :disabled="busy || !canDiscover"
                @click.prevent="discoverModels"
              >
                <Icon name="refresh" size="sm" :class="{ spinning: discovering }" />
              </button>
            </div>
            <small>{{ t('admin.detectionCenter.modelHint') }}</small>
          </label>

          <div class="field mode-field">
            <span class="label">{{ t('admin.detectionCenter.mode') }} <em>*</em></span>
            <div class="mode-switch" role="radiogroup" :aria-label="t('admin.detectionCenter.mode')">
              <button
                type="button"
                :class="{ active: form.mode === 'standard' }"
                role="radio"
                :aria-checked="form.mode === 'standard'"
                :disabled="running"
                @click="form.mode = 'standard'"
              >
                <Icon name="checkCircle" size="sm" />
                {{ t('admin.detectionCenter.modes.standard') }}
              </button>
              <button
                type="button"
                :class="{ active: form.mode === 'deep' }"
                role="radio"
                :aria-checked="form.mode === 'deep'"
                :disabled="running"
                @click="form.mode = 'deep'"
              >
                <Icon name="search" size="sm" />
                {{ t('admin.detectionCenter.modes.deep') }}
              </button>
            </div>
            <small>{{ t('admin.detectionCenter.modeHint') }}</small>
          </div>
        </div>

        <div class="target-actions">
          <button class="btn secondary" type="button" :disabled="busy" @click="resetAll">
            <Icon name="refresh" size="sm" />
            {{ t('admin.detectionCenter.reset') }}
          </button>
          <button class="btn secondary strong" type="button" :disabled="busy || !canDiscover" @click="discoverModels">
            <Icon name="database" size="sm" />
            {{ discovering ? t('admin.detectionCenter.discovering') : t('admin.detectionCenter.discover') }}
          </button>
          <button class="btn primary" type="button" :disabled="running || !form.model" @click="runDetection">
            <Icon v-if="!running" name="play" size="sm" />
            <span v-else class="spinner" />
            {{ running ? t('admin.detectionCenter.running') : t('admin.detectionCenter.start') }}
          </button>
        </div>

        <div v-if="errorMessage" class="alert alert-error" role="alert">
          <Icon name="xCircle" size="sm" />
          <span>{{ errorMessage }}</span>
          <button class="alert-close" type="button" aria-label="关闭" @click="errorMessage = ''">
            <Icon name="x" size="xs" />
          </button>
        </div>
      </section>

      <section v-if="discovery" class="surface discovery-panel">
        <div class="section-head discovery-heading">
          <div class="section-title">
            <span class="section-icon">
              <Icon name="database" size="sm" :stroke-width="2" />
            </span>
            <div>
              <h2>{{ t('admin.detectionCenter.discoveredTitle') }}</h2>
              <p>{{ t('admin.detectionCenter.discoveryDescription') }}</p>
            </div>
          </div>
          <button class="btn compact secondary" type="button" :disabled="busy || !canDiscover" @click="discoverModels">
            <Icon name="refresh" size="xs" />
            {{ t('admin.detectionCenter.rediscover') }}
          </button>
        </div>

        <div class="discovery-body">
          <div class="protocol-summary">
            <div class="protocol-summary-top">
              <span class="success-dot">
                <Icon name="checkCircle" size="sm" />
              </span>
              <strong>{{ t('admin.detectionCenter.detectedProtocols') }}</strong>
              <span v-for="protocol in discovery.protocols" :key="protocol" class="protocol-chip">
                {{ protocolLabel(protocol) }}
              </span>
            </div>
            <div class="base-row">
              <span>{{ t('admin.detectionCenter.baseAddress') }}</span>
              <code>{{ form.base_url }}</code>
            </div>
          </div>

          <div class="model-summary">
            <div class="model-summary-title">
              <strong>{{ t('admin.detectionCenter.foundModels', { count: discovery.models.length }) }}</strong>
              <span>{{ t('admin.detectionCenter.suggestedProtocol') }}: {{ discovery.suggested_protocol }}</span>
            </div>
            <div class="model-chips">
              <button
                v-for="model in visibleModels"
                :key="modelKey(model)"
                type="button"
                :class="['model-chip', { selected: form.model === model.id }]"
                @click="form.model = model.id"
              >
                <Icon name="cube" size="xs" />
                <span>{{ model.id }}</span>
              </button>
              <span v-if="remainingModelCount > 0" class="more-chip">+{{ remainingModelCount }}</span>
            </div>
          </div>
        </div>

        <div class="attempt-row">
          <div v-for="attempt in discovery.attempts" :key="attempt.protocol" class="attempt-item">
            <span class="attempt-name">{{ attempt.protocol }}</span>
            <span :class="['status-pill', statusClass(attempt.status)]">{{ statusText(attempt.status) }}</span>
            <span class="attempt-meta" v-if="attempt.http_status">HTTP {{ attempt.http_status }}</span>
            <span class="attempt-meta" v-if="attempt.latency_ms">{{ attempt.latency_ms }} ms</span>
          </div>
        </div>
      </section>

      <section class="surface report-panel">
        <div class="section-head report-heading">
          <div class="section-title">
            <span class="section-icon">
              <Icon name="document" size="sm" :stroke-width="2" />
            </span>
            <div>
              <h2>{{ t('admin.detectionCenter.reportTitle') }}</h2>
              <p v-if="report">
                <code>{{ report.report_id }}</code>
                <span class="dot-sep">·</span>
                {{ report.protocol }}
                <span class="dot-sep">·</span>
                <code>{{ report.model }}</code>
              </p>
              <p v-else>{{ t('admin.detectionCenter.emptyReport') }}</p>
            </div>
          </div>

          <div v-if="report" class="report-actions">
            <span class="report-time">{{ reportTime }}</span>
            <button class="btn compact secondary" type="button" @click="copyReport">
              <Icon name="copy" size="xs" />
              {{ copied ? t('admin.detectionCenter.copied') : t('admin.detectionCenter.copyReport') }}
            </button>
            <button class="btn compact secondary" type="button" @click="downloadReport">
              <Icon name="download" size="xs" />
              {{ t('admin.detectionCenter.downloadReport') }}
            </button>
          </div>
        </div>

        <template v-if="report">
          <div class="summary-grid">
            <article class="summary-card summary-total">
              <span class="summary-icon primary-soft"><Icon name="clipboard" size="sm" /></span>
              <div><span>{{ t('admin.detectionCenter.total') }}</span><strong>{{ report.summary.total }}</strong></div>
            </article>
            <article class="summary-card">
              <span class="summary-icon success-soft"><Icon name="checkCircle" size="sm" /></span>
              <div><span>{{ t('admin.detectionCenter.success') }}</span><strong class="success-text">{{ report.summary.success }}</strong><small>{{ percent(report.summary.success) }}</small></div>
            </article>
            <article class="summary-card">
              <span class="summary-icon danger-soft"><Icon name="xCircle" size="sm" /></span>
              <div><span>{{ t('admin.detectionCenter.failed') }}</span><strong class="danger-text">{{ report.summary.failed }}</strong><small>{{ percent(report.summary.failed) }}</small></div>
            </article>
            <article class="summary-card">
              <span class="summary-icon warning-soft"><Icon name="exclamationTriangle" size="sm" /></span>
              <div><span>{{ t('admin.detectionCenter.partial') }}</span><strong class="warning-text">{{ report.summary.partial }}</strong><small>{{ percent(report.summary.partial) }}</small></div>
            </article>
            <article class="summary-card">
              <span class="summary-icon info-soft"><Icon name="infoCircle" size="sm" /></span>
              <div><span>{{ t('admin.detectionCenter.inconclusive') }}</span><strong>{{ report.summary.inconclusive }}</strong><small>{{ percent(report.summary.inconclusive) }}</small></div>
            </article>
            <article class="summary-card muted-card">
              <span class="summary-icon neutral-soft"><Icon name="minus" size="sm" /></span>
              <div><span>{{ t('admin.detectionCenter.notApplicable') }}</span><strong>{{ report.summary.not_applicable }}</strong></div>
            </article>
            <article class="summary-card muted-card">
              <span class="summary-icon neutral-soft"><Icon name="ban" size="sm" /></span>
              <div><span>{{ t('admin.detectionCenter.unavailable') }}</span><strong>{{ report.summary.unavailable }}</strong></div>
            </article>
          </div>

          <div class="result-workbench">
            <aside class="capability-list" aria-label="能力检测结果">
              <div class="capability-list-head">
                <div>
                  <strong>{{ t('admin.detectionCenter.capabilityResults') }}</strong>
                  <span>{{ report.probes.length }}</span>
                </div>
              </div>
              <button
                v-for="probe in report.probes"
                :key="probe.id"
                type="button"
                :class="['capability-item', { active: selectedProbe?.id === probe.id }]"
                @click="selectedProbeId = probe.id"
              >
                <span class="capability-icon">
                  <Icon name="cpu" size="sm" />
                </span>
                <span class="capability-copy">
                  <strong>{{ probe.name }}</strong>
                  <small>{{ probe.category }}</small>
                </span>
                <span :class="['status-pill', statusClass(probe.status)]">{{ statusText(probe.status) }}</span>
                <Icon name="chevronRight" size="xs" class="capability-chevron" />
              </button>
            </aside>

            <article v-if="selectedProbe" class="probe-detail">
              <div class="probe-detail-head">
                <div class="probe-detail-title">
                  <span :class="['detail-icon', statusClass(selectedProbe.status)]">
                    <Icon v-if="selectedProbe.status === 'success'" name="checkCircle" size="md" />
                    <Icon v-else-if="selectedProbe.status === 'failed'" name="xCircle" size="md" />
                    <Icon v-else-if="selectedProbe.status === 'partial'" name="exclamationTriangle" size="md" />
                    <Icon v-else name="infoCircle" size="md" />
                  </span>
                  <div>
                    <div class="title-line">
                      <h3>{{ selectedProbe.name }}</h3>
                      <span :class="['status-pill', statusClass(selectedProbe.status)]">{{ statusText(selectedProbe.status) }}</span>
                    </div>
                    <p>{{ selectedProbe.summary }}</p>
                  </div>
                </div>
                <div class="confidence-box">
                  <span>{{ t('admin.detectionCenter.confidence') }}</span>
                  <strong>{{ confidence(selectedProbe.confidence) }}</strong>
                </div>
              </div>

              <div :class="['result-banner', statusClass(selectedProbe.status)]">
                <Icon v-if="selectedProbe.status === 'success'" name="checkCircle" size="sm" />
                <Icon v-else-if="selectedProbe.status === 'failed'" name="xCircle" size="sm" />
                <Icon v-else-if="selectedProbe.status === 'partial'" name="exclamationTriangle" size="sm" />
                <Icon v-else name="infoCircle" size="sm" />
                <div>
                  <strong>{{ t('admin.detectionCenter.result') }}：{{ statusText(selectedProbe.status) }}</strong>
                  <span>{{ selectedProbe.failure_reason || selectedProbe.summary }}</span>
                </div>
              </div>

              <div class="detail-grid">
                <div class="diagnosis-column">
                  <div v-if="selectedProbe.reason_code" class="info-row">
                    <span>{{ t('admin.detectionCenter.reasonCode') }}</span>
                    <code>{{ selectedProbe.reason_code }}</code>
                  </div>
                  <div v-if="selectedProbe.failure_reason" class="info-row">
                    <span>{{ t('admin.detectionCenter.failureReason') }}</span>
                    <p>{{ selectedProbe.failure_reason }}</p>
                  </div>
                  <div v-if="selectedProbe.possible_causes?.length" class="info-block">
                    <span>{{ t('admin.detectionCenter.possibleCauses') }}</span>
                    <ul><li v-for="item in selectedProbe.possible_causes" :key="item">{{ item }}</li></ul>
                  </div>
                  <div v-if="selectedProbe.recommendations?.length" class="info-block">
                    <span>{{ t('admin.detectionCenter.recommendations') }}</span>
                    <ul><li v-for="item in selectedProbe.recommendations" :key="item">{{ item }}</li></ul>
                  </div>
                  <div v-if="!selectedProbe.failure_reason && !selectedProbe.possible_causes?.length && !selectedProbe.recommendations?.length" class="success-detail">
                    <Icon name="checkCircle" size="sm" />
                    <span>{{ selectedProbe.summary }}</span>
                  </div>
                </div>

                <div class="evidence-column">
                  <div class="evidence-head">
                    <div>
                      <strong>{{ t('admin.detectionCenter.evidence') }}</strong>
                      <span>{{ selectedProbe.evidence?.length || 0 }} {{ t('admin.detectionCenter.items') }}</span>
                    </div>
                  </div>
                  <div v-if="selectedProbe.evidence?.length" class="evidence-stack">
                    <article v-for="(evidence, index) in selectedProbe.evidence" :key="selectedProbe.id + '-' + index" class="evidence-card">
                      <div class="evidence-title">
                        <strong>{{ evidence.label }}</strong>
                        <div>
                          <span v-if="evidence.http_status">HTTP {{ evidence.http_status }}</span>
                          <span v-if="evidence.duration_ms">{{ evidence.duration_ms }} ms</span>
                        </div>
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
            </article>
          </div>

          <div v-if="report.notes?.length" class="report-notes">
            <Icon name="infoCircle" size="sm" />
            <div>
              <strong>{{ t('admin.detectionCenter.notes') }}</strong>
              <ul><li v-for="note in report.notes" :key="note">{{ note }}</li></ul>
            </div>
          </div>
        </template>

        <div v-else class="report-empty">
          <span class="empty-icon"><Icon name="document" size="lg" /></span>
          <strong>{{ t('admin.detectionCenter.emptyReportTitle') }}</strong>
          <p>{{ t('admin.detectionCenter.emptyReport') }}</p>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import detectionCenterAPI, {
  type DetectedModel,
  type DetectionDiscoveryResponse,
  type DetectionMode,
  type DetectionProtocol,
  type DetectionReport,
} from '@/api/admin/detectionCenter'

const { t } = useI18n()
const router = useRouter()

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
  mode: 'deep'
})

const showKey = ref(false)
const discovering = ref(false)
const running = ref(false)
const copied = ref(false)
const errorMessage = ref('')
const modelSearch = ref('')
const discovery = ref<DetectionDiscoveryResponse | null>(null)
const report = ref<DetectionReport | null>(null)
const selectedProbeId = ref('')

const busy = computed(() => discovering.value || running.value)
const canDiscover = computed(() => Boolean(form.base_url.trim() && form.api_key.trim()))

const filteredModels = computed(() => {
  const query = modelSearch.value.toLowerCase()
  const models = discovery.value?.models ?? []
  if (!query) return models
  return models.filter(model =>
    (model.id + ' ' + model.name + ' ' + (model.provider ?? '')).toLowerCase().includes(query)
  )
})

const visibleModels = computed(() => (discovery.value?.models ?? []).slice(0, 4))
const remainingModelCount = computed(() => Math.max(0, (discovery.value?.models.length ?? 0) - visibleModels.value.length))

const selectedProbe = computed(() => {
  if (!report.value?.probes.length) return null
  return report.value.probes.find(probe => probe.id === selectedProbeId.value) ?? report.value.probes[0]
})

const reportTime = computed(() => {
  if (!report.value?.completed_at) return ''
  const date = new Date(report.value.completed_at)
  return Number.isNaN(date.getTime()) ? report.value.completed_at : date.toLocaleString()
})

function modelKey(model: DetectedModel) {
  return model.id + ':' + model.protocols.join(',')
}

function protocolLabel(protocol: string) {
  if (protocol === 'openai') return t('admin.detectionCenter.protocols.openai')
  if (protocol === 'anthropic') return t('admin.detectionCenter.protocols.anthropic')
  if (protocol === 'gemini') return t('admin.detectionCenter.protocols.gemini')
  return protocol
}

function clearDiscovery() {
  discovery.value = null
  report.value = null
  form.model = ''
  modelSearch.value = ''
  selectedProbeId.value = ''
}

function resetAll() {
  form.base_url = ''
  form.api_key = ''
  form.protocol = 'auto'
  form.model = ''
  form.mode = 'deep'
  discovery.value = null
  report.value = null
  selectedProbeId.value = ''
  modelSearch.value = ''
  errorMessage.value = ''
  showKey.value = false
}

function goBack() {
  const previous = window.history.state?.back
  if (previous) {
    router.back()
    return
  }
  router.push('/admin/dashboard')
}

function statusText(status: string) {
  const known = ['success', 'failed', 'partial', 'inconclusive', 'not_applicable', 'unavailable']
  return known.includes(status) ? t('admin.detectionCenter.status.' + status) : status
}

function statusClass(status: string) {
  return 'status-' + status.replace(/_/g, '-')
}

function confidence(value: number) {
  return Math.round(Math.min(1, Math.max(0, value)) * 100) + '%'
}

function percent(value: number) {
  const total = report.value?.summary.total || 0
  return total > 0 ? ((value / total) * 100).toFixed(1) + '%' : '0%'
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
      protocol: form.protocol
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
      mode: form.mode
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
  gap: 16px;
  padding-bottom: 28px;
}

.topline {
  display: flex;
  align-items: center;
  gap: 12px;
}

.back-btn,
.breadcrumb,
.btn,
.icon-btn,
.mode-switch button,
.model-chip,
.capability-item {
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.back-btn {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--color-border);
  border-radius: 9px;
  background: var(--color-surface);
  padding: 0 11px;
  color: var(--color-text-primary);
  font-size: 13px;
  font-weight: 650;
  box-shadow: var(--shadow-xs);
}

.back-btn:hover {
  border-color: var(--color-primary-border);
  color: var(--color-primary);
}

.breadcrumb {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
  color: var(--color-text-muted);
  font-size: 12px;
}

.breadcrumb strong {
  color: var(--color-text-secondary);
  font-weight: 650;
}

.hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
}

.hero-main {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 14px;
}

.hero-icon {
  display: inline-flex;
  height: 48px;
  width: 48px;
  flex: 0 0 48px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 14px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.hero-copy {
  min-width: 0;
}

.hero-copy h1 {
  margin: 0;
  color: var(--color-text-primary);
  font-size: 24px;
  font-weight: 750;
  line-height: 1.2;
}

.hero-copy p {
  max-width: 760px;
  margin: 5px 0 0;
  color: var(--color-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.security-note {
  display: flex;
  max-width: 470px;
  align-items: flex-start;
  gap: 9px;
  border: 1px solid color-mix(in srgb, var(--color-warning) 26%, var(--color-border));
  border-radius: 12px;
  background: color-mix(in srgb, var(--color-warning) 8%, var(--color-surface));
  padding: 10px 12px;
  color: var(--color-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.security-icon {
  display: inline-flex;
  height: 28px;
  width: 28px;
  flex: 0 0 28px;
  align-items: center;
  justify-content: center;
  border-radius: 9px;
  background: color-mix(in srgb, var(--color-warning) 14%, transparent);
  color: var(--color-warning);
}

.surface {
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.target-panel,
.discovery-panel,
.report-panel {
  padding: 18px;
}

.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.section-title {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 10px;
}

.section-icon {
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

.section-title h2 {
  margin: 0;
  color: var(--color-text-primary);
  font-size: 16px;
  font-weight: 720;
}

.section-title p,
.report-heading p {
  margin: 4px 0 0;
  color: var(--color-text-muted);
  font-size: 12px;
  line-height: 1.55;
}

.form-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(320px, 0.8fr) minmax(300px, 0.72fr);
  gap: 14px 16px;
  margin-top: 18px;
}

.field {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 6px;
}

.field-wide {
  grid-column: span 2;
}

.field .label {
  color: var(--color-text-primary);
  font-size: 12px;
  font-weight: 680;
}

.field .label em {
  color: var(--color-danger);
  font-style: normal;
}

.field small {
  min-height: 18px;
  color: var(--color-text-muted);
  font-size: 11px;
  line-height: 1.5;
}

.input-shell {
  display: flex;
  min-width: 0;
  min-height: 40px;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface);
  padding: 0 10px;
  color: var(--color-text-muted);
}

.input-shell:focus-within {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

.input-shell input,
.input-shell select {
  min-width: 0;
  flex: 1;
  border: 0;
  background: transparent;
  color: var(--color-text-primary);
  font-size: 13px;
  outline: none;
}

.input-shell input::placeholder {
  color: var(--color-text-muted);
}

.input-shell input:disabled,
.input-shell select:disabled {
  color: var(--color-text-disabled);
}

.icon-btn {
  display: inline-flex;
  height: 30px;
  width: 30px;
  flex: 0 0 30px;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--color-text-muted);
}

.icon-btn:hover:not(:disabled) {
  background: var(--color-surface-soft);
  color: var(--color-primary);
}

.mode-field {
  min-width: 0;
}

.mode-switch {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
  min-height: 40px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
  padding: 3px;
}

.mode-switch button {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 650;
}

.mode-switch button.active {
  border-color: var(--color-primary-border);
  background: var(--color-surface);
  color: var(--color-primary);
  box-shadow: var(--shadow-xs);
}

.target-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 6px;
}

.btn {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  border: 1px solid transparent;
  border-radius: 9px;
  padding: 0 14px;
  font-size: 12px;
  font-weight: 700;
}

.btn.compact {
  min-height: 34px;
  padding: 0 11px;
}

.btn.primary {
  border-color: var(--color-primary);
  background: var(--color-primary);
  color: white;
}

.btn.primary:hover:not(:disabled) {
  border-color: var(--color-primary-hover);
  background: var(--color-primary-hover);
  transform: translateY(-1px);
}

.btn.secondary {
  border-color: var(--color-border);
  background: var(--color-surface);
  color: var(--color-text-secondary);
}

.btn.secondary.strong {
  border-color: var(--color-primary-border);
  color: var(--color-primary);
}

.btn.secondary:hover:not(:disabled) {
  border-color: var(--color-primary-border);
  background: var(--color-surface-soft);
  color: var(--color-primary);
}

.btn:disabled,
.icon-btn:disabled,
.mode-switch button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.alert {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 14px;
  border-radius: 10px;
  padding: 9px 11px;
  font-size: 12px;
}

.alert-error {
  border: 1px solid color-mix(in srgb, var(--color-danger) 32%, var(--color-border));
  background: color-mix(in srgb, var(--color-danger) 7%, var(--color-surface));
  color: var(--color-danger);
}

.alert-close {
  display: inline-flex;
  margin-left: auto;
  border: 0;
  background: transparent;
  color: inherit;
}

.discovery-heading {
  align-items: center;
}

.discovery-body {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
  gap: 18px;
  margin-top: 16px;
  border: 1px solid var(--color-border-subtle);
  border-radius: 12px;
  background: var(--color-surface-soft);
  padding: 13px 14px;
}

.protocol-summary {
  min-width: 0;
  border-right: 1px solid var(--color-border);
  padding-right: 18px;
}

.protocol-summary-top {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 7px;
}

.protocol-summary-top strong,
.model-summary-title strong {
  color: var(--color-text-primary);
  font-size: 12px;
}

.success-dot {
  display: inline-flex;
  color: var(--color-success);
}

.protocol-chip,
.more-chip {
  display: inline-flex;
  min-height: 24px;
  align-items: center;
  border: 1px solid color-mix(in srgb, var(--color-success) 24%, var(--color-border));
  border-radius: 999px;
  background: color-mix(in srgb, var(--color-success) 8%, var(--color-surface));
  padding: 0 8px;
  color: var(--color-success);
  font-size: 10px;
  font-weight: 700;
  white-space: nowrap;
}

.base-row {
  display: grid;
  grid-template-columns: 70px minmax(0, 1fr);
  gap: 8px;
  margin-top: 11px;
  align-items: center;
  color: var(--color-text-muted);
  font-size: 11px;
}

.base-row code {
  min-width: 0;
  overflow-wrap: anywhere;
  border-radius: 7px;
  background: var(--color-surface);
  padding: 6px 8px;
  color: var(--color-text-secondary);
  font-size: 11px;
}

.model-summary {
  min-width: 0;
}

.model-summary-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.model-summary-title span {
  color: var(--color-text-muted);
  font-size: 10px;
}

.model-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.model-chip {
  display: inline-flex;
  max-width: 220px;
  min-height: 30px;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--color-border);
  border-radius: 9px;
  background: var(--color-surface);
  padding: 0 10px;
  color: var(--color-text-secondary);
  font-size: 11px;
}

.model-chip span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-chip:hover,
.model-chip.selected {
  border-color: var(--color-primary-border);
  color: var(--color-primary);
}

.model-chip.selected {
  background: var(--color-primary-soft);
}

.more-chip {
  border-color: var(--color-border);
  background: var(--color-surface);
  color: var(--color-text-muted);
}

.attempt-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.attempt-item {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: 1px solid var(--color-border-subtle);
  border-radius: 9px;
  background: var(--color-surface);
  padding: 6px 8px;
}

.attempt-name {
  color: var(--color-text-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 10px;
  font-weight: 700;
}

.attempt-meta {
  color: var(--color-text-muted);
  font-size: 10px;
}

.report-heading {
  align-items: center;
}

.report-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 7px;
}

.report-time {
  margin-right: 4px;
  color: var(--color-text-muted);
  font-size: 10px;
}

.dot-sep {
  margin: 0 5px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 10px;
  margin-top: 16px;
}

.summary-card {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 9px;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  background: var(--color-surface);
  padding: 11px;
}

.summary-card > div {
  min-width: 0;
}

.summary-card span {
  display: block;
  color: var(--color-text-muted);
  font-size: 10px;
}

.summary-card strong {
  display: inline-block;
  margin-top: 2px;
  color: var(--color-text-primary);
  font-size: 20px;
  line-height: 1;
}

.summary-card small {
  margin-left: 6px;
  color: var(--color-text-muted);
  font-size: 9px;
}

.summary-icon {
  display: inline-flex !important;
  height: 34px;
  width: 34px;
  flex: 0 0 34px;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
}

.primary-soft { background: var(--color-primary-soft); color: var(--color-primary) !important; }
.success-soft { background: color-mix(in srgb, var(--color-success) 11%, var(--color-surface)); color: var(--color-success) !important; }
.danger-soft { background: color-mix(in srgb, var(--color-danger) 9%, var(--color-surface)); color: var(--color-danger) !important; }
.warning-soft { background: color-mix(in srgb, var(--color-warning) 10%, var(--color-surface)); color: var(--color-warning) !important; }
.info-soft { background: color-mix(in srgb, var(--color-info) 9%, var(--color-surface)); color: var(--color-info) !important; }
.neutral-soft { background: var(--color-surface-soft); color: var(--color-text-muted) !important; }
.success-text { color: var(--color-success) !important; }
.danger-text { color: var(--color-danger) !important; }
.warning-text { color: var(--color-warning) !important; }

.result-workbench {
  display: grid;
  grid-template-columns: minmax(280px, 0.72fr) minmax(0, 1.7fr);
  gap: 12px;
  margin-top: 12px;
}

.capability-list,
.probe-detail {
  min-width: 0;
  border: 1px solid var(--color-border);
  border-radius: 13px;
  background: var(--color-surface);
}

.capability-list {
  align-self: start;
  overflow: hidden;
}

.capability-list-head {
  padding: 12px 13px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.capability-list-head > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.capability-list-head strong {
  color: var(--color-text-primary);
  font-size: 12px;
}

.capability-list-head span {
  display: inline-flex;
  min-width: 22px;
  height: 22px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: 10px;
}

.capability-item {
  display: grid;
  width: 100%;
  grid-template-columns: 28px minmax(0, 1fr) auto 14px;
  gap: 8px;
  align-items: center;
  border: 0;
  border-bottom: 1px solid var(--color-border-subtle);
  background: transparent;
  padding: 9px 11px;
  text-align: left;
}

.capability-item:last-child {
  border-bottom: 0;
}

.capability-item:hover {
  background: var(--color-surface-soft);
}

.capability-item.active {
  background: var(--color-primary-soft);
}

.capability-icon {
  display: inline-flex;
  height: 28px;
  width: 28px;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
}

.capability-item.active .capability-icon {
  background: var(--color-surface);
  color: var(--color-primary);
}

.capability-copy {
  min-width: 0;
}

.capability-copy strong {
  display: block;
  overflow: hidden;
  color: var(--color-text-primary);
  font-size: 11px;
  font-weight: 670;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.capability-copy small {
  display: block;
  margin-top: 2px;
  color: var(--color-text-muted);
  font-size: 9px;
}

.capability-chevron {
  color: var(--color-text-muted);
}

.status-pill {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  border: 1px solid var(--color-border);
  border-radius: 999px;
  padding: 0 7px;
  font-size: 9px;
  font-weight: 750;
  white-space: nowrap;
}

.status-success {
  border-color: color-mix(in srgb, var(--color-success) 24%, var(--color-border));
  background: color-mix(in srgb, var(--color-success) 8%, var(--color-surface));
  color: var(--color-success);
}

.status-failed {
  border-color: color-mix(in srgb, var(--color-danger) 24%, var(--color-border));
  background: color-mix(in srgb, var(--color-danger) 7%, var(--color-surface));
  color: var(--color-danger);
}

.status-partial,
.status-inconclusive {
  border-color: color-mix(in srgb, var(--color-warning) 28%, var(--color-border));
  background: color-mix(in srgb, var(--color-warning) 8%, var(--color-surface));
  color: var(--color-warning);
}

.status-not-applicable,
.status-unavailable {
  border-color: var(--color-border);
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
}

.probe-detail {
  padding: 14px;
}

.probe-detail-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.probe-detail-title {
  display: flex;
  min-width: 0;
  gap: 10px;
}

.detail-icon {
  display: inline-flex;
  height: 38px;
  width: 38px;
  flex: 0 0 38px;
  align-items: center;
  justify-content: center;
  border-radius: 11px;
}

.title-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.title-line h3 {
  margin: 0;
  color: var(--color-text-primary);
  font-size: 15px;
  font-weight: 730;
}

.probe-detail-title p {
  margin: 4px 0 0;
  color: var(--color-text-muted);
  font-size: 11px;
  line-height: 1.55;
}

.confidence-box {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  color: var(--color-text-muted);
  font-size: 9px;
}

.confidence-box strong {
  color: var(--color-text-primary);
  font-size: 16px;
}

.result-banner {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 13px;
  border: 1px solid currentColor;
  border-radius: 10px;
  padding: 10px 11px;
}

.result-banner > div {
  min-width: 0;
}

.result-banner strong,
.result-banner span {
  display: block;
}

.result-banner strong {
  font-size: 11px;
}

.result-banner span {
  margin-top: 2px;
  color: var(--color-text-secondary);
  font-size: 10px;
  line-height: 1.5;
}

.detail-grid {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
  gap: 14px;
  margin-top: 14px;
}

.diagnosis-column {
  min-width: 0;
  padding-right: 14px;
  border-right: 1px solid var(--color-border-subtle);
}

.info-row,
.info-block {
  display: grid;
  grid-template-columns: 82px minmax(0, 1fr);
  gap: 8px;
  padding: 8px 0;
  border-bottom: 1px solid var(--color-border-subtle);
}

.info-row > span,
.info-block > span {
  color: var(--color-text-muted);
  font-size: 10px;
  font-weight: 700;
}

.info-row p {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: 11px;
  line-height: 1.55;
}

.info-row code {
  overflow-wrap: anywhere;
  color: var(--color-text-primary);
  font-size: 10px;
}

.info-block ul {
  margin: 0;
  padding-left: 16px;
  color: var(--color-text-secondary);
  font-size: 11px;
  line-height: 1.7;
}

.success-detail {
  display: flex;
  align-items: center;
  gap: 8px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-success) 8%, var(--color-surface));
  padding: 10px 11px;
  color: var(--color-success);
  font-size: 11px;
}

.evidence-column {
  min-width: 0;
}

.evidence-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.evidence-head > div {
  display: flex;
  align-items: center;
  gap: 7px;
}

.evidence-head strong {
  color: var(--color-text-primary);
  font-size: 11px;
}

.evidence-head span {
  color: var(--color-text-muted);
  font-size: 9px;
}

.evidence-stack {
  display: grid;
  gap: 8px;
  margin-top: 8px;
}

.evidence-card {
  min-width: 0;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
  padding: 9px;
}

.evidence-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.evidence-title strong {
  min-width: 0;
  color: var(--color-text-primary);
  font-size: 10px;
}

.evidence-title > div {
  display: flex;
  gap: 6px;
}

.evidence-title span {
  color: var(--color-text-muted);
  font-size: 9px;
  white-space: nowrap;
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
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.evidence-card pre {
  max-height: 170px;
  margin: 8px 0 0;
  overflow: auto;
  border-radius: 8px;
  background: var(--color-bg-deep);
  padding: 9px;
  color: var(--color-text-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
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

.report-notes {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 12px;
  border: 1px solid var(--color-border-subtle);
  border-radius: 10px;
  background: var(--color-surface-soft);
  padding: 10px 11px;
  color: var(--color-info);
}

.report-notes strong {
  color: var(--color-text-primary);
  font-size: 10px;
}

.report-notes ul {
  margin: 5px 0 0;
  padding-left: 16px;
  color: var(--color-text-secondary);
  font-size: 10px;
  line-height: 1.6;
}

.report-empty {
  display: flex;
  min-height: 210px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 32px;
  color: var(--color-text-muted);
  text-align: center;
}

.empty-icon {
  display: inline-flex;
  height: 52px;
  width: 52px;
  align-items: center;
  justify-content: center;
  border-radius: 14px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
}

.report-empty strong {
  margin-top: 12px;
  color: var(--color-text-primary);
  font-size: 13px;
}

.report-empty p {
  max-width: 480px;
  margin: 5px 0 0;
  font-size: 11px;
  line-height: 1.55;
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

.back-btn:focus-visible,
.btn:focus-visible,
.icon-btn:focus-visible,
.mode-switch button:focus-visible,
.model-chip:focus-visible,
.capability-item:focus-visible {
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

@media (max-width: 1280px) {
  .form-grid {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }

  .field-wide {
    grid-column: span 1;
  }

  .summary-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

@media (max-width: 980px) {
  .hero {
    align-items: flex-start;
    flex-direction: column;
  }

  .security-note {
    max-width: none;
  }

  .discovery-body,
  .detail-grid,
  .result-workbench {
    grid-template-columns: 1fr;
  }

  .protocol-summary,
  .diagnosis-column {
    border-right: 0;
    border-bottom: 1px solid var(--color-border);
    padding-right: 0;
    padding-bottom: 12px;
  }

  .summary-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .detection-center {
    gap: 13px;
  }

  .topline,
  .report-heading,
  .section-head {
    align-items: flex-start;
    flex-direction: column;
  }

  .hero-main {
    align-items: flex-start;
  }

  .hero-icon {
    height: 42px;
    width: 42px;
    flex-basis: 42px;
  }

  .target-panel,
  .discovery-panel,
  .report-panel {
    padding: 14px;
  }

  .form-grid,
  .summary-grid {
    grid-template-columns: 1fr;
  }

  .target-actions,
  .report-actions {
    width: 100%;
    flex-direction: column;
  }

  .btn {
    width: 100%;
  }

  .model-summary-title {
    align-items: flex-start;
    flex-direction: column;
  }

  .capability-item {
    grid-template-columns: 28px minmax(0, 1fr) auto;
  }

  .capability-chevron {
    display: none;
  }

  .probe-detail-head {
    align-items: flex-start;
    flex-direction: column;
  }

  .confidence-box {
    align-items: flex-start;
  }
}

@media (prefers-reduced-motion: reduce) {
  .back-btn,
  .breadcrumb,
  .btn,
  .icon-btn,
  .mode-switch button,
  .model-chip,
  .capability-item {
    transition-duration: 0.01ms;
  }

  .spinner,
  .spinning {
    animation-duration: 1.6s;
  }
}
</style>
