<template>
  <div class="detection-center-page">
    <div class="page-heading">
      <div>
        <h1>{{ t('admin.detectionCenter.title') }}</h1>
        <p>{{ t('admin.detectionCenter.description') }}</p>
      </div>
      <div class="security-note">🔒 {{ t('admin.detectionCenter.securityNotice') }}</div>
    </div>

    <section class="panel">
      <div class="panel-title">
        <div>
          <h2>{{ t('admin.detectionCenter.configTitle') }}</h2>
          <p>{{ t('admin.detectionCenter.modeHint') }}</p>
        </div>
      </div>

      <div class="form-grid">
        <label class="field span-2">
          <span>{{ t('admin.detectionCenter.baseUrl') }}</span>
          <input v-model.trim="form.base_url" class="control mono" type="url" spellcheck="false"
            :placeholder="t('admin.detectionCenter.baseUrlPlaceholder')" :disabled="busy" />
        </label>

        <label class="field span-2">
          <span>{{ t('admin.detectionCenter.apiKey') }}</span>
          <div class="key-row">
            <input v-model="form.api_key" class="control mono" :type="showKey ? 'text' : 'password'"
              autocomplete="off" spellcheck="false" :placeholder="t('admin.detectionCenter.apiKeyPlaceholder')" :disabled="busy" />
            <button class="btn secondary" type="button" :disabled="busy" @click="showKey = !showKey">
              {{ showKey ? t('admin.detectionCenter.hideKey') : t('admin.detectionCenter.showKey') }}
            </button>
          </div>
        </label>

        <label class="field">
          <span>{{ t('admin.detectionCenter.protocol') }}</span>
          <select v-model="form.protocol" class="control" :disabled="busy" @change="clearDiscovery">
            <option value="auto">{{ t('admin.detectionCenter.protocols.auto') }}</option>
            <option value="openai">{{ t('admin.detectionCenter.protocols.openai') }}</option>
            <option value="anthropic">{{ t('admin.detectionCenter.protocols.anthropic') }}</option>
            <option value="gemini">{{ t('admin.detectionCenter.protocols.gemini') }}</option>
          </select>
        </label>

        <div class="field action-field">
          <span>&nbsp;</span>
          <button class="btn primary" type="button" :disabled="busy || !canDiscover" @click="discoverModels">
            <span v-if="discovering" class="spinner" />
            {{ discovering ? t('admin.detectionCenter.discovering') : t('admin.detectionCenter.discover') }}
          </button>
        </div>
      </div>

      <div v-if="errorMessage" class="error-box">{{ errorMessage }}</div>

      <template v-if="discovery">
        <div class="divider" />
        <div class="discovery-head">
          <div>
            <h3>{{ t('admin.detectionCenter.discoveredTitle') }}</h3>
            <p>
              {{ t('admin.detectionCenter.suggestedProtocol') }}:
              <code>{{ discovery.suggested_protocol }}</code>
              · {{ t('admin.detectionCenter.detectedProtocols') }}:
              <code>{{ discovery.protocols.join(', ') || '—' }}</code>
            </p>
          </div>
        </div>

        <div class="attempts">
          <div v-for="attempt in discovery.attempts" :key="attempt.protocol" class="attempt">
            <div class="attempt-top">
              <strong class="mono">{{ attempt.protocol }}</strong>
              <span :class="['badge', badgeClass(attempt.status)]">{{ statusText(attempt.status) }}</span>
            </div>
            <p>{{ attempt.message }}</p>
            <small v-if="attempt.http_status || attempt.latency_ms">
              <span v-if="attempt.http_status">HTTP {{ attempt.http_status }}</span>
              <span v-if="attempt.http_status && attempt.latency_ms"> · </span>
              <span v-if="attempt.latency_ms">{{ attempt.latency_ms }} ms</span>
            </small>
          </div>
        </div>

        <div v-if="discovery.models.length" class="model-grid">
          <label class="field">
            <span>{{ t('admin.detectionCenter.modelSearch') }}</span>
            <input v-model.trim="modelSearch" class="control" type="search" :placeholder="t('admin.detectionCenter.modelSearch')" />
          </label>
          <label class="field span-model">
            <span>{{ t('admin.detectionCenter.model') }}</span>
            <select v-model="form.model" class="control mono" :disabled="running">
              <option value="" disabled>{{ t('admin.detectionCenter.modelPlaceholder') }}</option>
              <option v-for="model in filteredModels" :key="modelKey(model)" :value="model.id">
                {{ model.id }}{{ model.provider ? ' · ' + model.provider : '' }}{{ model.protocols.length ? ' · ' + model.protocols.join('/') : '' }}
              </option>
            </select>
          </label>
          <label class="field">
            <span>{{ t('admin.detectionCenter.mode') }}</span>
            <select v-model="form.mode" class="control" :disabled="running">
              <option value="standard">{{ t('admin.detectionCenter.modes.standard') }}</option>
              <option value="deep">{{ t('admin.detectionCenter.modes.deep') }}</option>
            </select>
          </label>
          <div class="field action-field">
            <span>&nbsp;</span>
            <button class="btn primary" type="button" :disabled="running || !form.model" @click="runDetection">
              <span v-if="running" class="spinner" />
              {{ running ? t('admin.detectionCenter.running') : t('admin.detectionCenter.start') }}
            </button>
          </div>
        </div>
        <div v-else class="empty">{{ t('admin.detectionCenter.noModels') }}</div>
      </template>
    </section>

    <section class="panel report-panel">
      <div class="report-head">
        <div>
          <h2>{{ t('admin.detectionCenter.reportTitle') }}</h2>
          <p v-if="report"><code>{{ report.report_id }}</code> · {{ report.protocol }} · <code>{{ report.model }}</code></p>
          <p v-else>{{ t('admin.detectionCenter.emptyReport') }}</p>
        </div>
        <div v-if="report" class="report-actions">
          <button class="btn secondary" type="button" @click="copyReport">
            {{ copied ? t('admin.detectionCenter.copied') : t('admin.detectionCenter.copyReport') }}
          </button>
          <button class="btn secondary" type="button" @click="downloadReport">{{ t('admin.detectionCenter.downloadReport') }}</button>
          <button class="btn secondary" type="button" @click="report = null">{{ t('admin.detectionCenter.reset') }}</button>
        </div>
      </div>

      <template v-if="report">
        <div class="summary-grid">
          <div v-for="item in summaryCards" :key="item.key" class="summary-card">
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}</strong>
          </div>
        </div>

        <div v-for="group in groupedProbes" :key="group.category" class="probe-group">
          <div class="group-head">
            <h3>{{ group.category }}</h3>
            <span>{{ group.items.length }}</span>
          </div>

          <details v-for="probe in group.items" :key="probe.id" class="probe" :open="probe.status === 'failed'">
            <summary>
              <div class="probe-copy">
                <div class="probe-title">
                  <strong>{{ probe.name }}</strong>
                  <span :class="['badge', badgeClass(probe.status)]">{{ statusText(probe.status) }}</span>
                </div>
                <p>{{ probe.summary }}</p>
              </div>
              <div class="confidence">
                <span>{{ t('admin.detectionCenter.confidence') }}</span>
                <strong>{{ confidence(probe.confidence) }}</strong>
              </div>
            </summary>

            <div class="probe-details">
              <div v-if="probe.reason_code || probe.failure_reason" class="diagnosis">
                <div v-if="probe.reason_code">
                  <span>{{ t('admin.detectionCenter.reasonCode') }}</span>
                  <code>{{ probe.reason_code }}</code>
                </div>
                <div v-if="probe.failure_reason">
                  <span>{{ t('admin.detectionCenter.failureReason') }}</span>
                  <p>{{ probe.failure_reason }}</p>
                </div>
              </div>

              <div v-if="probe.possible_causes?.length" class="detail">
                <h4>{{ t('admin.detectionCenter.possibleCauses') }}</h4>
                <ul><li v-for="item in probe.possible_causes" :key="item">{{ item }}</li></ul>
              </div>

              <div v-if="probe.recommendations?.length" class="detail">
                <h4>{{ t('admin.detectionCenter.recommendations') }}</h4>
                <ul><li v-for="item in probe.recommendations" :key="item">{{ item }}</li></ul>
              </div>

              <div v-if="probe.evidence?.length" class="detail">
                <h4>{{ t('admin.detectionCenter.evidence') }}</h4>
                <div class="evidence-list">
                  <article v-for="(evidence, index) in probe.evidence" :key="probe.id + '-' + index" class="evidence">
                    <div class="evidence-head">
                      <strong>{{ evidence.label }}</strong>
                      <span v-if="evidence.http_status">HTTP {{ evidence.http_status }}</span>
                      <span v-if="evidence.duration_ms">{{ evidence.duration_ms }} ms</span>
                    </div>
                    <dl>
                      <template v-if="evidence.expected">
                        <dt>{{ t('admin.detectionCenter.expected') }}</dt><dd>{{ evidence.expected }}</dd>
                      </template>
                      <template v-if="evidence.actual">
                        <dt>{{ t('admin.detectionCenter.actual') }}</dt><dd>{{ evidence.actual }}</dd>
                      </template>
                      <template v-if="evidence.response_excerpt">
                        <dt>{{ t('admin.detectionCenter.responseExcerpt') }}</dt><dd><pre>{{ evidence.response_excerpt }}</pre></dd>
                      </template>
                    </dl>
                  </article>
                </div>
              </div>
            </div>
          </details>
        </div>

        <div v-if="report.notes?.length" class="notes">
          <h3>{{ t('admin.detectionCenter.notes') }}</h3>
          <ul><li v-for="note in report.notes" :key="note">{{ note }}</li></ul>
        </div>
      </template>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import detectionCenterAPI, {
  type DetectedModel,
  type DetectionDiscoveryResponse,
  type DetectionMode,
  type DetectionProbeResult,
  type DetectionProtocol,
  type DetectionReport,
} from '@/api/admin/detectionCenter'

const { t } = useI18n()
const form = reactive<{ base_url:string; api_key:string; protocol:DetectionProtocol; model:string; mode:DetectionMode }>({
  base_url: '', api_key: '', protocol: 'auto', model: '', mode: 'deep'
})
const showKey = ref(false)
const discovering = ref(false)
const running = ref(false)
const copied = ref(false)
const errorMessage = ref('')
const modelSearch = ref('')
const discovery = ref<DetectionDiscoveryResponse | null>(null)
const report = ref<DetectionReport | null>(null)

const busy = computed(() => discovering.value || running.value)
const canDiscover = computed(() => Boolean(form.base_url.trim() && form.api_key.trim()))
const filteredModels = computed(() => {
  const query = modelSearch.value.toLowerCase()
  const models = discovery.value?.models ?? []
  if (!query) return models
  return models.filter(model => (model.id + ' ' + model.name + ' ' + (model.provider ?? '')).toLowerCase().includes(query))
})
const groupedProbes = computed(() => {
  const groups = new Map<string, DetectionProbeResult[]>()
  for (const probe of report.value?.probes ?? []) {
    const list = groups.get(probe.category) ?? []
    list.push(probe)
    groups.set(probe.category, list)
  }
  return [...groups.entries()].map(([category, items]) => ({ category, items }))
})
const summaryCards = computed(() => {
  const s = report.value?.summary
  if (!s) return []
  return [
    { key:'total', label:t('admin.detectionCenter.total'), value:s.total },
    { key:'success', label:t('admin.detectionCenter.success'), value:s.success },
    { key:'failed', label:t('admin.detectionCenter.failed'), value:s.failed },
    { key:'partial', label:t('admin.detectionCenter.partial'), value:s.partial },
    { key:'inconclusive', label:t('admin.detectionCenter.inconclusive'), value:s.inconclusive },
    { key:'na', label:t('admin.detectionCenter.notApplicable'), value:s.not_applicable },
    { key:'unavailable', label:t('admin.detectionCenter.unavailable'), value:s.unavailable },
  ]
})

function modelKey(model: DetectedModel) { return model.id + ':' + model.protocols.join(',') }
function clearDiscovery() { discovery.value = null; report.value = null; form.model = ''; modelSearch.value = '' }
function statusText(status:string) {
  const known = ['success','failed','partial','inconclusive','not_applicable','unavailable']
  return known.includes(status) ? t('admin.detectionCenter.status.' + status) : status
}
function badgeClass(status:string) { return 'badge-' + status.replaceAll('_', '-') }
function confidence(value:number) { return Math.round(Math.min(1, Math.max(0, value)) * 100) + '%' }

async function discoverModels() {
  if (!canDiscover.value) { errorMessage.value = t('admin.detectionCenter.errors.required'); return }
  discovering.value = true; errorMessage.value = ''; report.value = null; form.model = ''
  try {
    discovery.value = await detectionCenterAPI.discover({ base_url:form.base_url, api_key:form.api_key, protocol:form.protocol })
    if (discovery.value.models.length === 1) form.model = discovery.value.models[0].id
  } catch (error) {
    discovery.value = null
    errorMessage.value = error instanceof Error ? error.message : t('admin.detectionCenter.errors.discoverFailed')
  } finally { discovering.value = false }
}

async function runDetection() {
  if (!form.model) { errorMessage.value = t('admin.detectionCenter.errors.modelRequired'); return }
  running.value = true; errorMessage.value = ''; copied.value = false
  try {
    report.value = await detectionCenterAPI.run({
      base_url:form.base_url, api_key:form.api_key, protocol:form.protocol, model:form.model, mode:form.mode
    })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('admin.detectionCenter.errors.runFailed')
  } finally { running.value = false }
}

async function copyReport() {
  if (!report.value) return
  try {
    await navigator.clipboard.writeText(JSON.stringify(report.value, null, 2))
    copied.value = true
    window.setTimeout(() => { copied.value = false }, 1500)
  } catch { errorMessage.value = t('admin.detectionCenter.errors.clipboardFailed') }
}
function downloadReport() {
  if (!report.value) return
  const url = URL.createObjectURL(new Blob([JSON.stringify(report.value, null, 2)], { type:'application/json;charset=utf-8' }))
  const a = document.createElement('a')
  a.href = url; a.download = 'detection-report-' + report.value.report_id + '.json'
  document.body.appendChild(a); a.click(); a.remove(); URL.revokeObjectURL(url)
}
</script>

<style scoped>
.detection-center-page{display:flex;flex-direction:column;gap:18px;min-width:0;padding-bottom:28px}
.page-heading,.report-head,.panel-title,.discovery-head,.group-head,.attempt-top,.probe-title,.evidence-head{display:flex;align-items:flex-start;justify-content:space-between;gap:14px}
h1,h2,h3,h4,p{margin:0}.page-heading h1{font-size:24px;line-height:1.25;color:var(--color-text-primary)}.page-heading p,.panel-title p,.report-head p,.discovery-head p,.attempt p,.probe p{margin-top:6px;color:var(--color-text-secondary);font-size:13px;line-height:1.55}
.security-note{max-width:520px;border:1px solid var(--color-border);border-radius:12px;background:var(--color-surface-soft);padding:10px 12px;color:var(--color-text-secondary);font-size:12px;line-height:1.5}
.panel{border:1px solid var(--color-border);border-radius:16px;background:var(--color-surface);box-shadow:var(--shadow-xs);padding:20px}
.panel h2{font-size:17px;color:var(--color-text-primary)}.panel h3{font-size:14px;color:var(--color-text-primary)}
.form-grid,.model-grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(180px,.35fr);gap:14px;margin-top:18px}.span-2{grid-column:span 1}.model-grid{grid-template-columns:minmax(160px,.5fr) minmax(260px,1fr) minmax(150px,.35fr) auto}.span-model{min-width:0}
.field{display:flex;min-width:0;flex-direction:column;gap:7px;color:var(--color-text-secondary);font-size:12px;font-weight:650}.control{width:100%;min-width:0;min-height:40px;border:1px solid var(--color-border);border-radius:10px;background:var(--color-surface);padding:0 12px;color:var(--color-text-primary);outline:none}.control:focus-visible,.btn:focus-visible,summary:focus-visible{border-color:var(--color-primary);box-shadow:0 0 0 3px var(--color-primary-ring)}.mono,code,pre{font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace}
.key-row{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:8px}.btn{min-height:40px;border:1px solid transparent;border-radius:10px;padding:0 14px;font-size:12px;font-weight:700}.btn:disabled{opacity:.55;cursor:not-allowed}.primary{background:var(--color-primary);border-color:var(--color-primary);color:#fff}.secondary{background:var(--color-surface-soft);border-color:var(--color-border);color:var(--color-text-primary)}.action-field{justify-content:flex-end}
.spinner{display:inline-block;width:13px;height:13px;margin-right:6px;border:2px solid currentColor;border-right-color:transparent;border-radius:50%;animation:spin .75s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}
.error-box{margin-top:14px;border:1px solid var(--color-danger);border-radius:10px;padding:10px 12px;color:var(--color-danger);font-size:13px}.divider{margin:18px 0;border-top:1px solid var(--color-border-subtle)}
.attempts,.summary-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:10px;margin-top:14px}.attempt,.summary-card,.evidence,.diagnosis,.notes{border:1px solid var(--color-border);border-radius:12px;background:var(--color-surface-soft);padding:12px}.attempt small{display:block;margin-top:7px;color:var(--color-text-muted)}
.badge{display:inline-flex;align-items:center;border:1px solid var(--color-border);border-radius:999px;padding:2px 8px;font-size:11px;font-weight:750;white-space:nowrap}.badge-success{color:var(--color-success)}.badge-failed{color:var(--color-danger)}.badge-partial,.badge-inconclusive{color:var(--color-warning)}.badge-not-applicable,.badge-unavailable{color:var(--color-text-muted)}
.empty{margin-top:14px;border:1px dashed var(--color-border);border-radius:12px;padding:18px;text-align:center;color:var(--color-text-muted);font-size:13px}
.report-head{align-items:center}.report-actions{display:flex;flex-wrap:wrap;gap:8px}.summary-card span{color:var(--color-text-muted);font-size:11px}.summary-card strong{display:block;margin-top:4px;color:var(--color-text-primary);font-size:21px}
.probe-group{margin-top:20px}.group-head{align-items:center;margin-bottom:6px}.group-head span{border-radius:999px;background:var(--color-surface-soft);padding:2px 8px;color:var(--color-text-muted);font-size:11px}.probe{border-top:1px solid var(--color-border-subtle)}.probe:last-child{border-bottom:1px solid var(--color-border-subtle)}.probe summary{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:16px;cursor:pointer;list-style:none;padding:14px 8px}.probe summary::-webkit-details-marker{display:none}.probe-title{align-items:center;justify-content:flex-start;gap:8px}.probe-title strong{font-size:13px;color:var(--color-text-primary)}.confidence{display:flex;flex-direction:column;align-items:flex-end;color:var(--color-text-muted);font-size:10px}.confidence strong{font-size:14px;color:var(--color-text-primary)}
.probe-details{display:flex;flex-direction:column;gap:13px;padding:0 8px 16px}.diagnosis>div+div{margin-top:10px}.diagnosis span,.detail h4,.evidence dt{color:var(--color-text-muted);font-size:10px;font-weight:750;text-transform:uppercase}.diagnosis p{margin-top:4px;color:var(--color-text-primary);font-size:12px;line-height:1.6}.detail h4{margin-bottom:7px}.detail ul,.notes ul{margin:0;padding-left:18px;color:var(--color-text-secondary);font-size:12px;line-height:1.65}.evidence-list{display:grid;gap:9px}.evidence-head{align-items:center;justify-content:flex-start;flex-wrap:wrap;color:var(--color-text-muted);font-size:10px}.evidence-head strong{margin-right:auto;color:var(--color-text-primary);font-size:12px}.evidence dl{display:grid;grid-template-columns:82px minmax(0,1fr);gap:7px 9px;margin:10px 0 0}.evidence dd{margin:0;min-width:0;color:var(--color-text-secondary);font-size:11px;line-height:1.55;overflow-wrap:anywhere}.evidence pre{max-height:220px;margin:0;overflow:auto;white-space:pre-wrap;word-break:break-word;color:var(--color-text-primary);font-size:10px}.notes{margin-top:20px}.notes h3{margin-bottom:8px}
@media(max-width:1000px){.model-grid{grid-template-columns:1fr 1fr}}@media(max-width:720px){.page-heading,.report-head{flex-direction:column}.security-note{max-width:none}.panel{padding:16px}.form-grid,.model-grid{grid-template-columns:1fr}.key-row{grid-template-columns:1fr}.report-actions{width:100%;flex-direction:column}.probe summary{grid-template-columns:1fr}.confidence{align-items:flex-start}.evidence dl{grid-template-columns:1fr}}
@media(prefers-reduced-motion:reduce){.spinner{animation-duration:1.5s}}
</style>
