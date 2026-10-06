<template>
  <section class="policy-panel" :data-testid="editable ? 'policy-settings' : effective ? 'effective-policy-readonly' : 'policy-readonly'" :aria-readonly="!editable">
    <header class="policy-panel__heading">
      <div>
        <h2>{{ scopeLabel }}</h2>
        <p>{{ editable || !effective ? t('workspace.policyDescription') : t('workspace.effectivePolicyDescription') }}</p>
      </div>
      <span v-if="policy" class="policy-revision">{{ t('workspace.policyRevision', { revision: policy.revision }) }}</span>
    </header>

    <div v-if="loading" class="policy-state" role="status">{{ t('common.loading') }}</div>
    <div v-else-if="error" class="policy-state policy-state--error" role="alert">{{ t('workspace.policyLoadError') }}</div>
    <form v-else-if="editable" class="policy-form" @submit.prevent="submit">
      <div class="policy-form__grid">
        <label><span>{{ t('workspace.allowedModels') }}</span><select v-model="draft.allowed_models_mode" class="input" name="allowed-models-mode"><option value="inherit">{{ t('workspace.policyModes.inherit') }}</option><option value="restricted">{{ t('workspace.policyModes.restricted') }}</option><option value="deny">{{ t('workspace.policyModes.deny') }}</option></select></label>
        <label><span>{{ t('workspace.allowedPlatforms') }}</span><select v-model="draft.allowed_platforms_mode" class="input" name="allowed-platforms-mode"><option value="inherit">{{ t('workspace.policyModes.inherit') }}</option><option value="restricted">{{ t('workspace.policyModes.restricted') }}</option><option value="deny">{{ t('workspace.policyModes.deny') }}</option></select></label>
        <label v-if="draft.allowed_models_mode === 'restricted'" class="policy-form__wide"><span>{{ t('workspace.allowedModels') }}</span><textarea v-model="draft.allowed_models" class="input" rows="3" :placeholder="t('workspace.modelsHint')"></textarea></label>
        <label v-if="draft.allowed_platforms_mode === 'restricted'" class="policy-form__wide"><span>{{ t('workspace.allowedPlatforms') }}</span><textarea v-model="draft.allowed_platforms" class="input" rows="2" :placeholder="t('workspace.platformsHint')"></textarea></label>
        <label><span>{{ t('workspace.rpmLimit') }}</span><input v-model="draft.rpm_limit" class="input" type="number" min="1" step="1" inputmode="numeric"></label>
        <label><span>{{ t('workspace.dailyRequestLimit') }}</span><input v-model="draft.daily_request_limit" class="input" type="number" min="1" step="1" inputmode="numeric"></label>
        <label><span>{{ t('workspace.monthlyRequestLimit') }}</span><input v-model="draft.monthly_request_limit" class="input" type="number" min="1" step="1" inputmode="numeric"></label>
        <label><span>{{ t('workspace.dailyTokenLimit') }}</span><input v-model="draft.daily_token_limit" class="input" type="number" min="1" step="1" inputmode="numeric"></label>
        <label><span>{{ t('workspace.monthlyTokenLimit') }}</span><input v-model="draft.monthly_token_limit" class="input" type="number" min="1" step="1" inputmode="numeric"></label>
      </div>
      <p class="policy-hint">{{ t('workspace.policyTriStateHint') }}</p>
      <p v-if="validationError" class="policy-state--error" role="alert">{{ t(validationError) }}</p>
      <p v-if="conflict" class="policy-state--error" role="alert">{{ t('workspace.policyRevisionConflict') }} <button type="button" class="btn btn-secondary btn-sm" @click="emit('reload')">{{ t('common.refresh') }}</button></p>
      <button v-else type="button" class="btn btn-secondary btn-sm policy-reload" :disabled="saving || loading" @click="emit('reload')">{{ t('common.refresh') }}</button>
      <div class="policy-actions"><button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('common.save') }}</button></div>
    </form>
    <dl v-else class="policy-summary">
      <div><dt>{{ t('workspace.allowedModels') }}</dt><dd v-for="[scope, layer] in policyLayers" :key="`model-${scope}`"><strong>{{ policyScope(scope) }}:</strong> {{ formatList(layer.allowed_models) }}</dd><dd v-if="!policyLayers.length">{{ t('workspace.policyNoAdditionalRestrictions') }}</dd></div>
      <div><dt>{{ t('workspace.allowedPlatforms') }}</dt><dd v-for="[scope, layer] in policyLayers" :key="`platform-${scope}`"><strong>{{ policyScope(scope) }}:</strong> {{ formatList(layer.allowed_platforms) }}</dd><dd v-if="!policyLayers.length">{{ t('workspace.policyNoAdditionalRestrictions') }}</dd></div>
      <div><dt>{{ t('workspace.rpmLimit') }}</dt><dd>{{ displayLimit(effective ? effective.rpm_limit : policy?.rpm_limit) }}</dd></div>
      <div><dt>{{ t('workspace.dailyRequestLimit') }}</dt><dd>{{ displayLimit(effective ? effective.daily_request_limit : policy?.daily_request_limit) }}</dd></div>
      <div><dt>{{ t('workspace.monthlyRequestLimit') }}</dt><dd>{{ displayLimit(effective ? effective.monthly_request_limit : policy?.monthly_request_limit) }}</dd></div>
      <div><dt>{{ t('workspace.dailyTokenLimit') }}</dt><dd>{{ displayLimit(effective ? effective.daily_token_limit : policy?.daily_token_limit) }}</dd></div>
      <div><dt>{{ t('workspace.monthlyTokenLimit') }}</dt><dd>{{ displayLimit(effective ? effective.monthly_token_limit : policy?.monthly_token_limit) }}</dd></div>
    </dl>
    <div v-if="!editable && effective?.revisions" class="policy-provenance">
      <span>{{ t('workspace.policyProvenance') }}</span>
      <code v-for="(revision, scope) in effective.revisions" :key="scope">{{ scope }}@{{ revision }}</code>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EffectivePolicy, Policy, PolicyUpdate } from '@/api/policies'

type Mode = 'inherit' | 'restricted' | 'deny'
type NumericDraft = number | string
type Draft = { allowed_models_mode: Mode; allowed_platforms_mode: Mode; allowed_models: string; allowed_platforms: string; rpm_limit: NumericDraft; daily_request_limit: NumericDraft; monthly_request_limit: NumericDraft; daily_token_limit: NumericDraft; monthly_token_limit: NumericDraft }

const props = withDefaults(defineProps<{ policy?: Policy | null; effective?: EffectivePolicy | null; scopeLabel: string; editable?: boolean; loading?: boolean; saving?: boolean; error?: boolean; conflict?: boolean }>(), { policy: null, effective: null, editable: false, loading: false, saving: false, error: false, conflict: false })
const emit = defineEmits<{ save: [PolicyUpdate]; reload: [] }>()
const { t } = useI18n()
const mode = (values: string[] | null | undefined): Mode => values == null ? 'inherit' : values.length ? 'restricted' : 'deny'
const text = (values: string[] | null | undefined) => values?.join('\n') || ''
const numberText = (value: number | null | undefined): NumericDraft => value == null ? '' : value
const draft = reactive<Draft>({ allowed_models_mode: 'inherit', allowed_platforms_mode: 'inherit', allowed_models: '', allowed_platforms: '', rpm_limit: '', daily_request_limit: '', monthly_request_limit: '', daily_token_limit: '', monthly_token_limit: '' })
const validationError = ref('')
const policyLayers = computed<[string, Policy][]>(() => {
  const layers = props.effective?.layers
  if (layers) return Object.entries(layers).filter((entry): entry is [string, Policy] => entry[1] != null)
  return props.policy ? [[props.policy.scope, props.policy]] : []
})

function reset(value: Policy | null | undefined) {
  validationError.value = ''
  Object.assign(draft, {
    allowed_models_mode: mode(value?.allowed_models), allowed_platforms_mode: mode(value?.allowed_platforms), allowed_models: text(value?.allowed_models), allowed_platforms: text(value?.allowed_platforms),
    rpm_limit: numberText(value?.rpm_limit), daily_request_limit: numberText(value?.daily_request_limit), monthly_request_limit: numberText(value?.monthly_request_limit), daily_token_limit: numberText(value?.daily_token_limit), monthly_token_limit: numberText(value?.monthly_token_limit),
  })
}
watch(() => props.policy, reset, { immediate: true })
function list(modeValue: Mode, value: string) { return modeValue === 'inherit' ? null : modeValue === 'deny' ? [] : [...new Set(value.split(/[\n,]/).map(item => item.trim()).filter(Boolean))] }
function limit(value: NumericDraft) {
  if (typeof value === 'string' && !value.trim()) return null
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : null
}
function submit() {
  if (!props.policy) return
  const limits = [draft.rpm_limit, draft.daily_request_limit, draft.monthly_request_limit, draft.daily_token_limit, draft.monthly_token_limit]
  if (limits.some(value => value !== '' && limit(value) == null)) { validationError.value = 'workspace.policyInvalidNumericLimit'; return }
  if ((draft.allowed_models_mode === 'restricted' && !list('restricted', draft.allowed_models)?.length) || (draft.allowed_platforms_mode === 'restricted' && !list('restricted', draft.allowed_platforms)?.length)) { validationError.value = 'workspace.policyAllowlistRequired'; return }
  validationError.value = ''
  emit('save', { expected_revision: props.policy.revision, allowed_models: list(draft.allowed_models_mode, draft.allowed_models), allowed_platforms: list(draft.allowed_platforms_mode, draft.allowed_platforms), rpm_limit: limit(draft.rpm_limit), daily_request_limit: limit(draft.daily_request_limit), monthly_request_limit: limit(draft.monthly_request_limit), daily_token_limit: limit(draft.daily_token_limit), monthly_token_limit: limit(draft.monthly_token_limit) })
}
function formatList(value: string[] | null | undefined) { return value == null ? t('workspace.policyModes.inherit') : value.length ? value.join(', ') : t('workspace.policyModes.deny') }
function displayLimit(value: number | null | undefined) { return value == null ? t('workspace.unlimited') : String(value) }
function policyScope(scope: string) { return t(`workspace.policyLayers.${scope}`) }
</script>

<style scoped>
.policy-panel { min-width: 0; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }
.policy-panel__heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 16px; }
.policy-panel__heading > div { min-width: 0; }
.policy-panel h2 { margin: 0; color: var(--color-text-primary); font-size: 17px; }
.policy-panel p { margin: 6px 0 0; color: var(--color-text-secondary); font-size: 13px; }
.policy-revision { flex: 0 0 auto; color: var(--color-text-muted); font-family: var(--font-mono, monospace); font-size: 12px; }
.policy-form { display: grid; gap: 14px; margin-top: 18px; }
.policy-form__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.policy-form label { display: grid; min-width: 0; gap: 6px; color: var(--color-text-secondary); font-size: 13px; }
.policy-form__wide { grid-column: 1 / -1; }
.policy-form textarea { resize: vertical; }
.policy-hint { overflow-wrap: anywhere; }
.policy-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.policy-state { padding: 28px 0; color: var(--color-text-muted); text-align: center; }
.policy-state--error { color: var(--color-danger); }
.policy-summary { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; margin: 18px 0 0; }
.policy-summary div { min-width: 0; }
.policy-summary dt { color: var(--color-text-muted); font-size: 12px; }
.policy-summary dd { margin: 5px 0 0; color: var(--color-text-primary); overflow-wrap: anywhere; }
.policy-provenance { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 18px; color: var(--color-text-muted); font-size: 12px; }
.policy-provenance code { border: 1px solid var(--color-border-subtle); border-radius: 6px; background: var(--color-surface-soft); padding: 3px 6px; }
@media (max-width: 640px) { .policy-panel__heading { align-items: stretch; flex-direction: column; }.policy-form__grid, .policy-summary { grid-template-columns: 1fr; }.policy-form__wide { grid-column: auto; } }
</style>
