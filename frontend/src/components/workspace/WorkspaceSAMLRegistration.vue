<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { workspaceAPI, type WorkspaceIdentityProvider, type WorkspaceSAMLServiceProvider } from '@/api/workspace'
const props = defineProps<{ workspaceId: number; provider: WorkspaceIdentityProvider; canManage: boolean }>()
const emit = defineEmits<{ close: []; updated: [WorkspaceIdentityProvider] }>()
const { t } = useI18n()
const info = ref<WorkspaceSAMLServiceProvider | null>(null)
const loading = ref(false)
const rotating = ref(false)
const error = ref(false)
const copyStatus = ref('')
let generation = 0
let rotationGeneration = 0
let controller: AbortController | null = null
async function load() {
  const current = ++generation
  controller?.abort()
  controller = new AbortController()
  info.value = null
  error.value = false
  loading.value = true
  try { const result = await workspaceAPI.getSAMLServiceProvider(props.workspaceId, props.provider.id, controller.signal); if (current === generation) info.value = result }
  catch { if (current === generation) error.value = true }
  finally { if (current === generation) loading.value = false }
}
async function rotate(action: 'stage' | 'promote') {
  if (!props.canManage || rotating.value || !props.provider.revision) return
  if (action === 'promote' && (!info.value?.next_signing_certificate || !window.confirm(t('workspace.identitySamlPromoteConfirm')))) return
  const current = generation
  const rotation = ++rotationGeneration
  rotating.value = true
  error.value = false
  try { const updated = await workspaceAPI.rotateSAMLKeys(props.workspaceId, props.provider.id, { revision: props.provider.revision, action }); if (current !== generation) return; emit('updated', updated); await load() }
  catch { if (current === generation) error.value = true }
  finally { if (rotation === rotationGeneration) rotating.value = false }
}
async function copy(value: string) {
  try { await navigator.clipboard.writeText(value); copyStatus.value = t('workspace.identitySamlCopied') }
  catch { copyStatus.value = t('workspace.identityCopyError') }
}
watch(() => [props.workspaceId, props.provider.id], () => { ++rotationGeneration; rotating.value = false; copyStatus.value = ''; void load() }, { immediate: true })
onBeforeUnmount(() => { ++generation; ++rotationGeneration; controller?.abort() })
</script>
<template>
  <section class="saml-registration" data-testid="identity-saml-registration">
    <div class="heading"><h3>{{ t('workspace.identitySamlRegistration') }}: {{ provider.name }}</h3><button type="button" class="btn btn-secondary btn-sm" @click="emit('close')">{{ t('common.close') }}</button></div>
    <div v-if="loading" role="status">{{ t('common.loading') }}</div>
    <div v-if="error" role="alert">{{ t('workspace.identitySamlRegistrationError') }} <button type="button" class="btn btn-secondary btn-sm" :disabled="loading || rotating" @click="load">{{ t('common.retry') }}</button></div>
    <dl v-if="info">
      <div v-for="field in (['entity_id', 'acs_url', 'metadata_url', 'signing_certificate', 'next_signing_certificate'] as const)" :key="field">
        <template v-if="info[field]"><dt>{{ t(`workspace.identitySamlSP.${field}`) }}</dt><dd><code>{{ info[field] }}</code><button type="button" class="btn btn-ghost btn-sm" :aria-label="t('workspace.identitySamlCopy', { field: t(`workspace.identitySamlSP.${field}`) })" @click="copy(info[field]!)">{{ t('workspace.identityCopyValue') }}</button></dd></template>
      </div>
    </dl>
    <p role="status" aria-live="polite">{{ copyStatus }}</p>
    <p>{{ t('workspace.identitySamlLimitations') }}</p>
    <p>{{ t('workspace.identitySamlRotationHint') }}</p>
    <div v-if="canManage && info" class="actions"><button type="button" class="btn btn-secondary" data-testid="saml-stage" :disabled="rotating || loading || Boolean(info.next_signing_certificate) || !provider.revision" @click="rotate('stage')">{{ t('workspace.identitySamlStage') }}</button><button v-if="info.next_signing_certificate" type="button" class="btn btn-secondary" data-testid="saml-promote" :disabled="rotating || loading || !provider.revision" @click="rotate('promote')">{{ t('workspace.identitySamlPromote') }}</button><span v-if="rotating" role="status">{{ t('common.processing') }}</span></div>
  </section>
</template>
<style scoped>
.saml-registration { display: grid; min-width: 0; gap: 12px; padding: 18px; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); }
.heading, .actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; justify-content: space-between; }
h3 { color: var(--color-text-primary); font-size: 16px; }
dl, dl > div { display: grid; min-width: 0; gap: 6px; }
dt, p { color: var(--color-text-secondary); font-size: 13px; line-height: 1.5; }
dd { margin: 0; min-width: 0; }
code { white-space: pre-wrap; overflow-wrap: anywhere; color: var(--color-text-primary); font-size: 12px; }
</style>
